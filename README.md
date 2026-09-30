# api

API de items del dashboard. Recibe el JWT de **auth-svc** ([`auth`](https://github.com/JesusMaVe/auth)) como `Authorization: Bearer` y lo valida en cada `GET`/`POST /api/items`. El frontend vive en [`frontend`](https://github.com/JesusMaVe/frontend).

Diseño: [spec en el repo auth](https://github.com/JesusMaVe/auth/blob/main/docs/superpowers/specs/2026-09-24-auth-dashboard-design.md)

## Requisitos

- Docker con Compose v2, GNU Make, bash, OpenSSL 3
- El repo `auth` clonado al lado (`../auth`) y levantado con `make up`: de ahí sale la clave pública del JWT. Otra ruta: `make up AUTH_DIR=/ruta/a/auth`.

## Primeros pasos

```bash
make env     # crea .env con secretos aleatorios (una sola vez)
make up      # postgres (127.0.0.1:5433); importa la clave pública desde ../auth
make test    # corre todos los tests
```

Sin `../auth`, `make up` genera un par de claves de **desarrollo**: la API arranca, pero los tokens reales de auth-svc no validan.

## Endpoints

Todos los de `/api/*` exigen `Authorization: Bearer <jwt de auth-svc>`; sin token o con uno inválido → `401 {"error":"unauthorized"}`.

| Método | Ruta | Respuesta |
|---|---|---|
| GET | `/healthz` | `200 {"status":"ok"}` / `503` si la base no responde |
| GET | `/api/items` | `200 {"items":[{id,title,description,created_by,created_at}]}` (más recientes primero, hasta `ITEMS_PAGE_LIMIT`) |
| POST | `/api/items` | Body `{"title","description"}` → `201` con el item; `created_by` sale del `sub` del token. `400` validación, `413` cuerpo demasiado grande |

```bash
TOKEN=$(curl -s -X POST 127.0.0.1:8081/token -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"<LDAP_SEED_USER_PASSWORD de ../auth/.env>"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')
curl -s 127.0.0.1:8082/api/items -H "Authorization: Bearer $TOKEN"
```
