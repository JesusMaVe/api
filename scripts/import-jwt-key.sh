#!/usr/bin/env bash
# Deja en <secretos>/jwt_public_key la clave pública con la que auth-svc firma los JWT.
# Si existe <repo auth>/secrets/jwt_public_key la copia (y la vuelve a copiar si auth la rotó).
# Si no hay repo auth y todavía no hay clave (CI), genera un par de desarrollo: sirve para
# levantar y probar la API, pero los tokens reales de auth-svc no validarán.
# Imprime "jwt_public_key" cuando la clave cambia, para que `make up` recree los contenedores.
set -euo pipefail

auth_dir=${1:?uso: import-jwt-key.sh <repo auth> <directorio de secretos>}
dst=${2:?uso: import-jwt-key.sh <repo auth> <directorio de secretos>}
src="$auth_dir/secrets/jwt_public_key"
out="$dst/jwt_public_key"

install -d -m 700 "$dst"
chmod 700 "$dst"

if [[ -f $src ]]; then
  [[ -f $out ]] && cmp -s "$src" "$out" && exit 0
  # Se escribe en el mismo archivo (sin mv): Docker monta cada secreto por inodo.
  (umask 022 && cat "$src" > "$out")
  echo "jwt_public_key"
  exit 0
fi

[[ -f $out ]] && exit 0
openssl version | grep -q '^OpenSSL 3' \
  || { echo "import-jwt-key: se requiere OpenSSL 3 (en macOS: brew install openssl)" >&2; exit 1; }
echo "import-jwt-key: no existe $src; se genera un par de DESARROLLO (los tokens de auth-svc no validarán)" >&2
(umask 077 && openssl genpkey -algorithm ed25519 -out "$dst/dev_jwt_private_key")
(umask 022 && openssl pkey -in "$dst/dev_jwt_private_key" -pubout -out "$out")
echo "jwt_public_key"
