# Pisah

Go server for the Pisah bill-splitter. Serves both the JSON API and the
server-rendered web UI (owner flow + friend web view).

- **Language:** Go, stdlib `net/http` (Go 1.22+ routing) — no framework
- **DB:** Supabase Postgres via `pgx`
- **Auth:** Supabase JWT (owner) · random bearer token (anonymous friend)
- **OCR:** AWS Textract `AnalyzeExpense`
- **Live updates:** Server-Sent Events
- **Money:** integer sen everywhere (1 sen = RM 0.01); never floats

## Run

```bash
cp .env.example .env          # fill in DATABASE_URL, SUPABASE_JWT_SECRET, AWS creds
psql "$DATABASE_URL" -f schema.sql   # or paste schema.sql into the Supabase SQL editor
make run
```

```bash
make test                     # share math + SSE broker + token gen (no DB needed)
```

## Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/receipts/scan` | owner | image bytes → parsed receipt (Textract) |
| POST | `/api/splits` | owner | create split from reviewed items → share link |
| GET  | `/api/splits/{slug}` | public | friend landing: split + items + claim status |
| POST | `/api/splits/{slug}/join` | public | `{name}` → participant + bearer token |
| POST | `/api/splits/{slug}/claims` | participant | `{itemIds}` → recompute, return my share |
| GET  | `/api/splits/{slug}/share` | participant | my breakdown lines + owner DuitNow QR |
| POST | `/api/splits/{slug}/paid` | participant | mark paid, push SSE event |
| GET  | `/api/me/splits` | owner | recent splits with collected progress |
| DELETE | `/api/splits/{slug}` | owner | permanently delete a split and its items/participants |
| GET  | `/api/splits/{slug}/track` | owner | collected total + per-participant status |
| GET  | `/api/splits/{slug}/events` | public | SSE stream of payment events |

Owner calls send `Authorization: Bearer <supabase-jwt>`. Friend calls after join send
`Authorization: Bearer <participant-token>`.

## Split math

`share/share.go` (pure, stdlib-only, tested offline). Each item is split equally
among the participants who claim it; tax + service is distributed proportionally to
each participant's claimed subtotal. Reproduces the prototype's figures exactly
(RM 14.50 for one nasi lemak; RM 24.71 for three items incl. a 3-way share).

## Deliberate simplifications (`ponytail:` in code)

- **Owner uploads their own DuitNow QR** (stored as a Supabase Storage URL on the
  split) — we don't *generate* dynamic DuitNow QRs. Real per-payment DuitNow QR
  needs PayNet/acquirer merchant onboarding; not worth it pre-launch.
- **SSE broker is in-memory, single-process.** Fine for one container. To scale
  horizontally, back it with Postgres `LISTEN/NOTIFY` or subscribe clients to
  Supabase Realtime directly.
- **Authorization enforced in app code**, not Postgres RLS (the service connects as
  the DB user). Enable RLS before letting clients hit PostgREST/Realtime directly.
- **CORS is open (`*`).** Lock to your web-view domain in production.
- **Per-participant rounding** can drift a sen or two from the printed total; the
  owner's bill total stays authoritative. Upgrade path noted in `share/share.go`.

## Not built yet

- Push notification to owner on friend payment (the SSE event is the hook point)
- Editing a split after creation
