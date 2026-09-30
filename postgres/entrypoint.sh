#!/bin/sh
# Arranca postgres con el entrypoint oficial y, en cada arranque, aplica la contraseña de
# POSTGRES_PASSWORD_FILE (rotación sin borrar datos). El healthcheck espera el archivo $APPLIED.
# Nunca imprime la contraseña.
set -eu

APPLIED=/tmp/.password-applied
rm -f "$APPLIED"

docker-entrypoint.sh "$@" &
pid=$!
trap 'kill -TERM "$pid" 2>/dev/null' TERM INT

# ALTER USER es idempotente: se reintenta hasta que el servidor definitivo lo acepta
# (durante la primera inicialización, el entrypoint oficial usa un servidor temporal).
apply_password() {
  PG_NEW_PASSWORD=$(cat "$POSTGRES_PASSWORD_FILE") psql -v ON_ERROR_STOP=1 -q \
    -h /var/run/postgresql -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1 <<'SQL' || return 1
\getenv pw PG_NEW_PASSWORD
ALTER ROLE CURRENT_USER PASSWORD :'pw';
SQL
  [ -n "${POSTGRES_APP_USER:-}" ] || return 0
  apply_app_role
}

# apply_app_role crea (o actualiza) el rol con el que se conecta la aplicación: sin superusuario,
# solo puede crear y usar objetos en el esquema public (migraciones incluidas). Las tablas que
# haya creado el superusuario (bases anteriores a este rol) pasan a ser suyas.
apply_app_role() {
  PG_APP_PASSWORD=$(cat "$POSTGRES_APP_PASSWORD_FILE") psql -v ON_ERROR_STOP=1 -q \
    -h /var/run/postgresql -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1 <<'SQL'
\getenv app_user POSTGRES_APP_USER
\getenv app_pw PG_APP_PASSWORD
\getenv db POSTGRES_DB
SELECT format('CREATE ROLE %I LOGIN', :'app_user')
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_user') \gexec
ALTER ROLE :"app_user" WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'app_pw';
GRANT CONNECT ON DATABASE :"db" TO :"app_user";
GRANT USAGE, CREATE ON SCHEMA public TO :"app_user";
SELECT format('ALTER TABLE public.%I OWNER TO %I', tablename, :'app_user')
  FROM pg_tables WHERE schemaname = 'public' AND tableowner <> :'app_user' \gexec
SQL
}

until apply_password; do
  kill -0 "$pid" 2>/dev/null || break
  sleep 1
done
kill -0 "$pid" 2>/dev/null && touch "$APPLIED" && echo "entrypoint: contraseña aplicada"

set +e
wait "$pid"
status=$?
while kill -0 "$pid" 2>/dev/null; do
  wait "$pid"
  status=$?
done
exit "$status"
