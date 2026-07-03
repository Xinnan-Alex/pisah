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

## Cursor Cloud specific instructions

The dev environment is pre-provisioned in the VM snapshot (persists across sessions):
Go 1.23 (`/usr/bin/go` is symlinked to it), a local PostgreSQL 16 cluster, and a
gitignored `/workspace/.env` wired for local dev. The startup update script only runs
`go mod download`. Standard commands live in the Makefile/README (`make run`,
`make test`); the notes below are the non-obvious caveats.

- **Local Postgres, not Supabase.** There is no Supabase CLI/Docker here. A native
  Postgres 16 cluster holds a `pisah` database owned by role `pisah` (password
  `pisah`) with `schema.sql` already applied. The cluster is usually not running on a
  fresh boot — start it with `sudo pg_ctlcluster 16 main start` (or
  `sudo service postgresql start`) before `make run`/DB work. `DATABASE_URL` in `.env`
  points at `postgresql://pisah:pisah@127.0.0.1:5432/pisah`.
- **Owner auth uses HS256, no live Supabase.** `.env` sets `SUPABASE_JWT_SECRET` (and
  leaves `SUPABASE_URL` empty), so the server verifies owner tokens as legacy HS256
  JWTs. There is no way to sign in through the `/signin` web page (that proxies a real
  Supabase Auth instance). To exercise owner-only APIs (`/api/splits`, `/api/me/*`,
  track), mint an HS256 JWT signed with `local-dev-secret` containing a `sub` (any
  UUID) and an `exp`, and send it as `Authorization: Bearer <jwt>`.
- **Testing owner + friend flows end to end:** create a split via `POST /api/splits`
  with an owner JWT to get a `slug`, then drive the friend flow (join → claims →
  share → paid) via the web UI at `/r/<slug>` (no account needed) or the
  `/api/splits/<slug>/*` endpoints. The web UI loads htmx/Alpine/fonts from public
  CDNs, so the browser needs internet.
- **Optional, unconfigured features:** AWS Textract OCR (`/scan`, `/api/receipts/scan`)
  and Supabase Storage DuitNow-QR upload are intentionally not set up; those endpoints
  fail but everything else works. Create splits with explicit item JSON instead of OCR.
- `make test` is DB-free (pure `share/` math + broker + tokens). Lint is `gofmt` +
  `go vet`; note `web.go` and `web_handlers.go` are not gofmt-clean in the current
  tree (pre-existing).
