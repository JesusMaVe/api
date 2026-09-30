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
