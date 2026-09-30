#!/usr/bin/env bash
# Rotación de la contraseña de postgres de extremo a extremo, sin borrar datos.
# Al final restaura el .env original.
set -u
cd "$(dirname "$0")/.." || exit 1
# shellcheck source=test/lib.sh
. test/lib.sh
load_env

read -ra DC <<< "${COMPOSE:?COMPOSE no definida (usa make test-rotation)}"

rotated=$(mktemp)
trap 'rm -f "$rotated"; make -s up >/dev/null 2>&1' EXIT
sed -E 's/^([A-Z0-9_]+_PASSWORD)=(.*)$/\1=\2-rotada/' "${ENV_FILE:-.env}" > "$rotated"

# shellcheck disable=SC2016  # se expande dentro del contenedor
pg_tcp() { "${DC[@]}" exec -T -e PGPASSWORD="$1" postgres \
  sh -c 'psql -h "$(hostname -i)" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "select 1"'; }

echo "rotation: contraseña nueva sin borrar datos"
check "make up aplica el .env rotado" make -s up ENV_FILE="$rotated"
check_output "postgres: la contraseña nueva funciona" '^1$' pg_tcp "${POSTGRES_PASSWORD}-rotada"
check_fails "postgres: la vieja ya no" pg_tcp "$POSTGRES_PASSWORD"
check_output "api: sigue sana con la contraseña rotada" '"status":"ok"' curl -s "http://127.0.0.1:${API_HOST_PORT}/healthz"

echo "rotation: vuelta al .env original"
check "make up vuelve a aplicar el .env original" make -s up
check_output "postgres: la contraseña original funciona de nuevo" '^1$' pg_tcp "$POSTGRES_PASSWORD"

summary
