#!/usr/bin/env bash
# Tests de integración del compose de desarrollo.
# Requiere los servicios levantados: make test-infra lo hace.
set -u
cd "$(dirname "$0")/.." || exit 1
# shellcheck source=test/lib.sh
. test/lib.sh
load_env

read -ra DC <<< "${COMPOSE:?COMPOSE no definida (usa make test-infra)}"

echo "infra: postgres"
check_output "responde a select 1" '^1$' \
  "${DC[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc 'select 1'
# Por TCP a la IP del contenedor (127.0.0.1 no pide contraseña en el pg_hba de la imagen oficial).
pg_tcp() {
  # shellcheck disable=SC2016  # se expande dentro del contenedor
  "${DC[@]}" exec -T -e PGPASSWORD="$1" postgres \
    sh -c 'psql -h "$(hostname -i)" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "select 1"'
}
check_output "autentica por TCP con la contraseña del secreto" '^1$' pg_tcp "$POSTGRES_PASSWORD"
check_fails  "rechaza por TCP una contraseña errónea" pg_tcp contraseña-incorrecta
check_output "puerto publicado solo en 127.0.0.1" '^127\.0\.0\.1:' "${DC[@]}" port postgres 5432

echo "infra: api"
API="http://127.0.0.1:${API_HOST_PORT}"
check_output "healthz responde ok" '"status":"ok"' curl -s "$API/healthz"
check_output "puerto publicado solo en 127.0.0.1" '^127\.0\.0\.1:' "${DC[@]}" port api "$API_PORT"
# shellcheck disable=SC2016  # el $k es de la plantilla Go de docker inspect
check_output "api está en la red compartida" "$SHARED_NETWORK" \
  docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}' "$("${DC[@]}" ps -q api)"
check_output "la api usa el rol de aplicación, sin superusuario" "^f\$" \
  "${DC[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc \
  "select rolsuper from pg_stat_activity a join pg_roles r on r.rolname = a.usename where a.application_name = '' and a.usename = '$POSTGRES_APP_USER' limit 1"
check_output "la tabla items es del rol de aplicación" "^${POSTGRES_APP_USER}\$" \
  "${DC[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "select tableowner from pg_tables where tablename = 'items'"
check_output "migraciones aplicadas (tabla items)" '^items$' \
  "${DC[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "select to_regclass('items')"

echo "infra: items protegidos con Bearer"
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; } # gitleaks:allow (token inválido a propósito)
check_output "GET /api/items sin token → 401" '^401$' code "$API/api/items"
check_output "GET /api/items con Bearer inválido → 401" '^401$' \
  code -H 'Authorization: Bearer no.es.valido' "$API/api/items" # gitleaks:allow
check_output "POST /api/items sin token → 401" '^401$' \
  code -X POST -H 'Content-Type: application/json' -d '{"title":"x"}' "$API/api/items"
check_output "responde con CSP" '[Cc]ontent-[Ss]ecurity-[Pp]olicy' curl -sI "$API/healthz"
check_no_output "los logs no contienen el Bearer" 'no\.es\.valido' "${DC[@]}" logs api

summary
