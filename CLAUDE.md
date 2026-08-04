# CLAUDE.md

## Project overview

Planetarium Server is the backend for the Planetarium platform: a gamified educational system for building learning paths, tracking progress, and generating AI-powered knowledge checks.

The repository is still in an early stage. The current implementation is a minimal Go HTTP server, and the project is primarily focused on architecture, documentation, and repository setup rather than a full feature set.

## Tech stack

- Language: Go 1.26.5
- Runtime: standard library HTTP server for the current MVP skeleton
- Containerization: Docker and Docker Compose
- Database: PostgreSQL 16+ (citext, pg_trgm); migrations with goose, driver pgx v5
- CI/CD: GitHub Actions workflow scaffold

## Repository layout

- cmd/api/main.go — application entrypoint
- cmd/migrate/main.go — migration runner, a separate binary from the API
- internal/ — vertical slices (user/register, user/login), domain, adapters, transport
- internal/transport/httperr — the single error shape the API returns; a package
  of its own because the slices import it and transport/http imports the slices
- pkg/ — shared helpers: httpserver, logger, render, hash (argon2id), token (JWT
  and refresh tokens)
- config/ — application configuration, composed from adapter configs
- migrations/ — goose SQL migrations, embedded via embed.FS
- docs/ — DB schema (db-schema.md), its review, the dbml diagram, and the API
  specification (openapi.yaml) embedded into the binary via embed.go
- Swagger UI is served by the API itself at `/docs`, reading `/api/v1/openapi.yaml`.
  The spec is hand-written and the code follows it, so update the spec in the same
  change as the handler — nothing regenerates it.
- go.mod — Go module definition and version
- Dockerfile — container build for the API
- docker-compose.yml — local orchestration: API plus PostgreSQL 16
- .github/workflows/pipeline.yaml — lint, test, build; deploy step is a placeholder
- README.md — product overview and run instructions
- CONTRIBUTING.md — contribution and workflow guidance

## Current development status

- The project is in planning / architecture design mode.
- Core business features are not implemented yet.
- Prefer small, deliberate changes that improve the foundation rather than large abstractions.

## Local development commands

Run the API locally:

```bash
go run ./cmd/api/main.go
```

Format Go code:

```bash
go fmt ./...
```

Run tests:

```bash
go test ./...          # data-layer tests start a throwaway Postgres via Docker
go test -short ./...   # skips them, for a machine without a Docker daemon
```

Tests that touch the database call `postgres.NewTestDB(t)`. It applies the real
embedded migrations, starts the container **once per package** and empties every
table before each test, so tests do not depend on the order they run in. Setting
`TEST_DATABASE_URL` points the harness at an existing database instead of
starting a container — that is what CI does with a service container.

Run with Docker (starts PostgreSQL and waits for it to become healthy):

```bash
docker compose up --build
```

Apply migrations (a separate step on purpose — several API replicas starting at
once must not race to migrate the same database):

```bash
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/migrate down
```

Build the container manually:

```bash
docker build -t planetarium-server .
```

## Coding guidelines

- Follow standard Go conventions and keep code simple and explicit.
- Prefer small functions and clear naming over premature abstraction.
- Handle errors directly and avoid silent failures.
- Keep HTTP handlers lightweight; move business logic into separate functions or future packages as the app grows.
- Maintain consistency with the architecture direction described in the README: clean / hexagonal-style backend structure.
- When adding dependencies, justify them in the change and keep the repository easy to understand.

## Environment and configuration

`.env.example` is the source of truth for what the code actually reads: `APP_PORT`,
`DB_USER` / `DB_PASSWORD` / `DB_HOST` / `DB_PORT` / `DB_NAME`, and
`JWT_SECRET` / `ACCESS_TTL` / `REFRESH_TTL`. The README still mentions `PORT`,
`DATABASE_URL` and `OPENAI_API_KEY` — the first two are outdated names and the
third is not wired up yet.

An unset `JWT_SECRET` falls back to a value published in `config/config.go` and
logs a warning at startup. That is deliberate — a fresh checkout should run — but
it means anyone can mint valid access tokens until a real secret is set.

Under docker compose the API reaches the database as host `db`; `.env.example`
keeps `localhost` because that is the correct value when running the API on the
host with `go run`.

If a PostgreSQL is already installed on the developer's machine it will own port
5432, and `localhost` then resolves to it rather than to the container — the
symptom is `database "planetarium_db" does not exist` against a database that
plainly exists in Docker. Both sides read `DB_PORT`, so the fix needs no code
change: `DB_PORT=5433 docker compose up -d db` and the same variable for
`go run`.

`config.InitConfig` is the single place these variables are read. Both entrypoints
call it; do not reach for `os.Getenv` elsewhere.

If you add configuration behavior, keep it documented and avoid introducing hard-coded secrets.

## CI/CD notes

The workflow lives in .github/workflows/pipeline.yaml and runs lint, test and build.
Its final `deploy` job is still an `echo` placeholder — do not assume production
deployment is wired up.

If you modify deployment automation:

- keep the workflow safe and explicit
- avoid introducing live deployment steps without required secrets and review
- preserve the existing placeholder structure unless the repo is ready for a full deployment implementation

## Contribution expectations

- Keep changes aligned with the project goals described in README.md and CONTRIBUTING.md.
- If you introduce a significant architectural change, document it clearly.
- When implementing new features, add or update relevant documentation alongside the code.

## Notes for future agents

- This repository is still early-stage, so documentation and architecture decisions matter a lot.
- Prefer incremental improvements over large rewrites.
- If you need to add new backend modules, keep the entrypoint in cmd/api/main.go simple and move logic into focused packages.
- Add tests as features become real; the current repository does not yet have a mature test suite.
