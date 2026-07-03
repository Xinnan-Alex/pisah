# Repository Guidelines

## Project Structure & Module Organization
The Go server is a single module at the repo root (`go.mod`). It serves both the
JSON API and the server-rendered web UI (owner + friend flows).
- `main.go`, `handlers.go`, `store.go`, `storage.go`, `ocr.go`, `jwks.go`: HTTP API, storage, auth, and OCR wiring.
- `web.go`, `web_handlers.go`, `web_auth.go`: server-rendered web UI handlers.
- `web/templates/`, `web/static/`: HTML templates and CSS/JS assets (embedded via `go:embed`).
- `share/`: pure split-math logic with unit tests.
- `schema.sql`: database schema for Supabase Postgres.
- `.env.example`: local configuration template.
- `infra/`: Terraform + deploy script for AWS ECS/ECR.

## Build, Test, and Development Commands
Run commands from the repo root:
- `make run` starts the server locally (loads `.env`).
- `make test` runs all tests, including the pure math package.
- `psql "$DATABASE_URL" -f schema.sql` applies the schema to a local or Supabase database.

## Coding Style & Naming Conventions
Use standard Go formatting and idioms:
- Format code with `gofmt` before committing.
- Keep package names short and lowercase.
- Use descriptive file names that match the package or feature (`handlers.go`, `share_test.go`).
- Money values stay in integer sen; do not introduce floats for bill math.

## Testing Guidelines
Tests use Go’s built-in `testing` package.
- Prefer small, table-driven tests for pure logic.
- Keep deterministic logic in `share/` covered by unit tests.
- Name test files with `_test.go` and test functions `TestXxx`.

## Commit & Pull Request Guidelines
This repository currently has no Git commit history, so no repo-specific commit convention is established yet. Use short, imperative commit messages such as `Add split validation`.
PRs should include:
- A clear summary of the change.
- Notes on any schema, env, or API impact.
- Screenshots or sample requests when web UI (`web/`) or endpoint behavior changes.

## Security & Configuration Tips
Copy `.env.example` to `.env` and fill in `DATABASE_URL`, `SUPABASE_JWT_SECRET`, and AWS credentials before running locally.
Do not commit secrets. Open CORS and in-memory SSE behavior are deliberate simplifications; tighten them before production deployment.
