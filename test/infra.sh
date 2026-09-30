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
check_output "migraciones aplicadas (tabla items)" '^items$' \
  "${DC[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "select to_regclass('items')"

summary
