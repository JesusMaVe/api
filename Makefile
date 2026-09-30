SHELL := /bin/bash
.DEFAULT_GOAL := help

ENV_FILE    ?= .env
ENV_EXAMPLE ?= .env.example
# Repo auth hermano: de ahí sale la clave pública del JWT (secrets/jwt_public_key).
AUTH_DIR    ?= ../auth
COMPOSE     := docker compose -f docker-compose.yml -f docker-compose.dev.yml
export COMPOSE

# Versiones fijadas de las herramientas (único lugar; el CI usa estos mismos targets).
GITLEAKS_IMAGE   := zricethezav/gitleaks:v8.30.1
HADOLINT_IMAGE   := hadolint/hadolint:v2.15.1
SHELLCHECK_IMAGE := koalaman/shellcheck:v0.11.0
GOSEC_VERSION       := v2.29.0
GOVULNCHECK_VERSION := v1.8.0

POSTGRES_TEST_IMAGE := api-postgres:test
API_TEST_IMAGE      := api:test

SHELL_SCRIPTS := $(shell find . -name '*.sh' -not -path './.git/*')
DOCKERFILES   := $(shell find . -name 'Dockerfile*' -not -path './.git/*')

.PHONY: help env secrets up down clean logs test test-repo test-postgres-image test-go test-api-image test-infra test-rotation lint secrets-scan

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "} {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

env: ## Crea .env desde .env.example con secretos aleatorios (no sobrescribe)
	@scripts/gen-env.sh "$(ENV_EXAMPLE)" "$(ENV_FILE)"

secrets: ## Escribe los *_PASSWORD de .env en secrets/ e importa la clave pública del JWT desde $(AUTH_DIR)
	@scripts/sync-secrets.sh "$(ENV_FILE)" secrets
	@scripts/import-jwt-key.sh "$(AUTH_DIR)" secrets

up: ## Levanta los servicios (espera a healthy); si cambió algún secreto, recrea los contenedores
	@changed=$$(scripts/sync-secrets.sh "$(ENV_FILE)" secrets && scripts/import-jwt-key.sh "$(AUTH_DIR)" secrets) || exit 1; \
	if [ -n "$$changed" ]; then echo "secretos nuevos o cambiados: $$(echo $$changed) → se recrean los contenedores"; fi; \
	$(COMPOSE) up -d --build --wait $${changed:+--force-recreate}

down: ## Detiene los servicios
	$(COMPOSE) down

clean: ## Detiene los servicios y BORRA los volúmenes (datos de postgres)
	$(COMPOSE) down -v

logs: ## Muestra los logs de los servicios
	$(COMPOSE) logs --no-color

test: test-repo test-postgres-image test-go test-api-image test-infra test-rotation ## Corre todos los tests

test-repo: ## Tests del esqueleto del repo
	@test/repo.sh

test-postgres-image: ## Tests de la imagen postgres en aislamiento
	docker build -q -t $(POSTGRES_TEST_IMAGE) postgres >/dev/null
	@POSTGRES_TEST_IMAGE=$(POSTGRES_TEST_IMAGE) test/postgres-image.sh

test-go: ## Tests de Go (unitarios + integración con la imagen postgres vía testcontainers)
	docker build -q -t $(POSTGRES_TEST_IMAGE) postgres >/dev/null
	POSTGRES_TEST_IMAGE=$(POSTGRES_TEST_IMAGE) go test -race -count=1 ./...

test-api-image: ## Tests de la imagen api en aislamiento
	docker build -q -t $(API_TEST_IMAGE) . >/dev/null
	@API_TEST_IMAGE=$(API_TEST_IMAGE) test/api-image.sh

test-infra: up ## Tests de integración del compose
	@test/infra.sh

test-rotation: up ## Rotación de la contraseña de postgres de extremo a extremo (restaura tu .env al final)
	@test/rotation.sh

lint: ## shellcheck + hadolint + gofmt, go vet, gosec y govulncheck
	docker run --rm -v "$(CURDIR):/mnt" -w /mnt $(SHELLCHECK_IMAGE) -x $(SHELL_SCRIPTS)
	docker run --rm -v "$(CURDIR):/mnt" -w /mnt $(HADOLINT_IMAGE) hadolint $(DOCKERFILES)
	test -z "$$(gofmt -l . | tee /dev/stderr)"
	go vet ./...
	go run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) -quiet ./...
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

secrets-scan: ## Busca secretos en el historial de git (gitleaks)
	docker run --rm -v "$(CURDIR):/repo" $(GITLEAKS_IMAGE) git --no-banner --redact /repo
