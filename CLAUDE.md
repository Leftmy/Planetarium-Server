# CLAUDE.md

## Project overview

Planetarium Server is the backend for the Planetarium platform: a gamified educational system for building learning paths, tracking progress, and generating AI-powered knowledge checks.

The repository is still in an early stage. The current implementation is a minimal Go HTTP server, and the project is primarily focused on architecture, documentation, and repository setup rather than a full feature set.

## Tech stack

- Language: Go 1.26.5
- Runtime: standard library HTTP server for the current MVP skeleton
- Containerization: Docker and Docker Compose
- CI/CD: GitHub Actions workflow scaffold

## Repository layout

- cmd/api/main.go — application entrypoint
- go.mod — Go module definition and version
- Dockerfile — container build for the API
- docker-compose.yml — local container orchestration
- .github/workflows/deploy.yaml — deployment workflow placeholder
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
go test ./...
```

Run with Docker:

```bash
docker compose up --build
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

The README references environment variables such as:

- PORT
- OPENAI_API_KEY
- DATABASE_URL

If you add configuration behavior, keep it documented and avoid introducing hard-coded secrets.

## CI/CD notes

The deployment workflow in .github/workflows/deploy.yaml is currently a placeholder. Do not assume production deployment is already wired up.

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
