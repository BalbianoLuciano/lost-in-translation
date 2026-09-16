# Lost in Translation

Un profesor de inglés personal: diagnostica, enseña, hace practicar, evalúa y
explica en castellano cuando algo no cierra.

- **Qué y por qué:** [`PLAN.md`](./PLAN.md)
- **Cómo se ve:** [`design.md`](./design.md)
- **Deploy:** [`docs/deploy.md`](./docs/deploy.md)

## Estructura

```
apps/web/        SvelteKit (SPA) → Vercel
services/api/    Go + Postgres  → Railway
docs/            deploy y notas
```

## Desarrollo local

Requisitos: Docker, Go 1.27+, Node 24 + pnpm 10, [sqlc](https://sqlc.dev).

```sh
# 1. Postgres (puerto 54329)
docker compose up -d --wait

# 2. API en :8080, con login falso de desarrollo
cd services/api
cp .env.example .env
set -a && source .env && set +a
go run ./cmd/api

# 3. Web en :5173 (otra terminal)
cd apps/web
pnpm install
pnpm dev
```

Sin variables de Firebase, la web muestra **Enter · dev mode** y la API
(`AUTH_MODE=dev`) acepta ese usuario. En producción ese modo no arranca.

## Tests

```sh
# API (el test de integración usa el Postgres de docker compose)
cd services/api
TEST_DATABASE_URL="postgres://lit:lit@localhost:54329/lit?sslmode=disable" go test ./...

# Web: tipos, tests (incluye el contraste de los colores) y build
cd apps/web
pnpm check && pnpm test && pnpm build
```

## Cambiar el esquema

1. Nueva migración en `services/api/migrations/` (`goose create <nombre> sql`).
2. Queries en `services/api/internal/store/queries/`.
3. `sqlc generate`. CI falla si el código generado quedó desactualizado.
