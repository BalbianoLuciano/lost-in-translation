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
content/         banco de ejercicios en YAML (fuente de verdad)
tools/           Python: valida content/ y compila el bundle que embebe la API
docs/            deploy y notas
```

## Desarrollo local

Requisitos: Docker, Go 1.27+, Node 24 + pnpm 10, [sqlc](https://sqlc.dev), [uv](https://docs.astral.sh/uv/).

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

# Contenido
cd tools && uv run pytest

# Web: tipos, tests (incluye el contraste de los colores) y build
cd apps/web
pnpm check && pnpm test && pnpm build

# Punta a punta: levanta la API (auth dev) y la web, y hace el diagnóstico entero
cd apps/web
pnpm test:e2e
```

`pnpm test:e2e` usa el Postgres de `docker compose`. Con otro Postgres:
`E2E_DATABASE_URL="postgres://localhost:5432/lit_test?sslmode=disable" pnpm test:e2e`.

## Contenido

Los ejercicios viven en `content/items/<parte>/<habilidad>.yaml` y el glosario en
`content/glossary/`. Los formatos de referencia están comentados en
`content/items/tenses/present-perfect-result-experience.yaml` y en
`content/glossary/cheatsheets/conditionals.yaml`.

```sh
cd tools
uv run lit-tools validate   # valida todo content/
uv run lit-tools build      # compila services/api/internal/content/bundle.json
uv run lit-tools stats      # ítems por habilidad y tipo
```

El bundle se commitea: la API lo embebe, así que publicar contenido es un push.
CI falla si `content/` y el bundle no coinciden, y un test de Go corrige cada
ítem con su propia respuesta.

## Cambiar el esquema

1. Nueva migración en `services/api/migrations/` (`goose create <nombre> sql`).
2. Queries en `services/api/internal/store/queries/`.
3. `sqlc generate`. CI falla si el código generado quedó desactualizado.
