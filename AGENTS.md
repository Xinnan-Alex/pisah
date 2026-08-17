# Repository Guidelines

## Project Structure & Module Organization

Pisah is a single Go module at the repository root. The server combines JSON API handlers, Supabase/Postgres storage, JWT or bearer-token auth, OCR, and server-rendered web pages.

- Root `*.go`: server, API, auth, storage, scanning, and shared web handlers.
- `share/`: pure bill-splitting math and its unit tests.
- `web/templates/`, `web/static/`: HTML templates, CSS, and JavaScript assets.
- `supabase/migrations/`, `supabase/seed.sql`: database migrations and local seed data.
- `infra/terraform/`, `infra/deploy.sh`: AWS/ECS deployment infrastructure.

## Build, Test, and Development Commands

Run commands from the repository root:

- `cp .env.example .env` then `make run`: start the local server on port 8080.
- `make test`: run the complete Go test suite; tests are DB-free by default.
- `go build ./...`: compile all packages.
- `gofmt -w <files>` and `go vet ./...`: format and statically check Go changes.
- `make supabase-start` / `make supabase-stop`: start or stop local Supabase services.
- `supabase db reset`: replay local migrations and seed data.

## Coding Style & Naming Conventions

Use idiomatic Go, lowercase package names, descriptive feature-oriented filenames, and standard `gofmt` formatting. Keep monetary values as integer sen; never use floating-point arithmetic for bill calculations. Prefer small, focused handlers and pure functions that are easy to test.

## Testing Guidelines

Tests use Go’s standard `testing` package. Name files `*_test.go` and functions `TestXxx`; use table-driven tests for repeated cases. Add or update tests for changes to split math, auth, scanning, handlers, templates, or persistence behavior. Run `make test` before submitting.

## Commit & Pull Request Guidelines

Use Conventional Commits with a lowercase imperative subject under 50 characters, such as `feat(web): add split summary` or `fix: reject invalid token`. Keep each commit focused. PRs should explain the change, note API/schema/environment impact, and include screenshots or sample requests for web or endpoint changes.

## Security & Configuration Tips

Copy `.env.example` for local configuration and never commit secrets, credentials, or `terraform.tfvars`. Review auth, CORS, storage, and IAM changes carefully. Treat Supabase migrations as append-only: create a new migration, test with `supabase db reset`, then apply remotely with `supabase db push`.
