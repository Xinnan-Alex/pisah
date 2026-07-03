# Pisah — environments

Two environments, one schema (`supabase/migrations/`):

- **Local** — `supabase start` (Docker) for day-to-day testing.
- **Remote** — a hosted Supabase project for prod.

The Go server connects directly to Postgres and verifies owner tokens via the
project's JWKS (ES256). It serves the owner web UI and the friend web page at
`/r/<slug>`.

---

## Local (already set up)

Ports are shifted to the **5442x** range (config.toml) to avoid clashing with
other local Supabase projects.

```bash
# from repo root
supabase start -x analytics,edge-runtime,functions,imgproxy,inbucket,meta,realtime,rest,storage,studio,vector --ignore-health-check
#   (analytics/storage health checks fail under Colima; we don't need them yet)

# server (reads .env — already filled for local)
make run

# stop everything
supabase stop          # add --project-id pisah if you have multiple
```

Local endpoints: API `http://127.0.0.1:54421` · DB `postgresql://postgres:postgres@127.0.0.1:54422/postgres`
Backend: `http://localhost:8080` · friend page: `http://localhost:8080/r/<slug>`

**Test accounts** (password for all: `password123`):

| Email | Notes |
|-------|-------|
| `owner@test.com` | Primary test owner |
| `test@pisah.local` | Second test owner |

Seeded automatically on `supabase db reset` via `supabase/seed.sql`.

Open `http://localhost:8080` and sign in with a test account
(`owner@test.com` / `password123`) to reach the owner flow; friends open
`http://localhost:8080/r/<slug>`.

---

## Remote (prod) — steps you run

These need your Supabase login, so run them yourself:

```bash
# 1. authenticate the CLI (opens browser)
supabase login

# 2. create a project (or make one in the dashboard and skip to link)
supabase projects create pisah --org-id <your-org> --region ap-southeast-1 --db-password '<pick-one>'

# 3. link this repo to it
supabase link --project-ref <project-ref>

# 4. push the schema (applies supabase/migrations/* to remote)
supabase db push
```

Then configure the server for prod from `.env.prod.example`
(`DATABASE_URL` pooler string, `SUPABASE_URL`, `PUBLIC_BASE_URL`, AWS keys) and
deploy it (see `infra/` for the Terraform + `deploy.sh` workflow).

### Google sign-in (prod Supabase)

1. **Google Cloud Console** → APIs & Services → Credentials → Create **OAuth client ID** → **Web application**.
   - Authorized redirect URI: `https://<project-ref>.supabase.co/auth/v1/callback`
2. **Supabase Dashboard** → Authentication → Providers → **Google** → enable, paste Web client ID + secret.
3. **Supabase Dashboard** → Authentication → URL Configuration → add redirect URL: `pisah://auth/callback`
4. Deploy the server (includes `GET /api/auth/oauth/google`), then use **Continue with Google** on the web sign-in page.

Local Google auth also needs `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` in your shell when running `supabase start` (see `supabase/config.toml`).

---

## Changing the schema

Never hand-edit applied migrations. Iterate with `execute_sql` / `psql`, then:

```bash
supabase migration new <name>      # creates a timestamped file
# write SQL into it
supabase db reset                  # local: replays all migrations from scratch
supabase db push                   # remote: applies new migrations
```
