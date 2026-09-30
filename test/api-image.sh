#!/usr/bin/env bash
# Tests de la imagen api en aislamiento (docker run, sin compose).
set -u
cd "$(dirname "$0")/.." || exit 1
# shellcheck source=test/lib.sh
. test/lib.sh

IMG=${API_TEST_IMAGE:?API_TEST_IMAGE no definida (usa make test-api-image)}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
printf 'valor-que-no-debe-salir' > "$tmp/pw"
chmod -R a+rX "$tmp"

echo "api-image:"
check_fails  "sin variables → sale con error" docker run --rm "$IMG"
check_output "sin variables → nombra API_PORT" 'API_PORT' docker run --rm "$IMG"
check_no_output "el error no muestra el valor de un secreto" 'valor-que-no-debe-salir' \
  docker run --rm -v "$tmp:/s:ro" -e POSTGRES_PASSWORD_FILE=/s/pw "$IMG"
check_output "corre como non-root (uid 65532)" '^65532:65532$' docker image inspect -f '{{.Config.User}}' "$IMG"
check_output "tiene healthcheck" 'healthcheck' docker image inspect -f '{{.Config.Healthcheck.Test}}' "$IMG"

summary
