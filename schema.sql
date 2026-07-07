-- Pisah database schema. Run in the Supabase SQL editor (or `psql $DATABASE_URL -f schema.sql`).
-- All money is integer sen (1 sen = RM 0.01).
--
-- The Go service connects with the Postgres connection string (service role) and
-- enforces authorization in application code, so Row Level Security is not relied
-- on here. If you ever let clients hit PostgREST / Supabase Realtime directly,
-- enable RLS on these tables first.

create extension if not exists "pgcrypto"; -- gen_random_uuid()

create table if not exists splits (
    id            uuid primary key default gen_random_uuid(),
    owner_id      uuid not null,               -- auth.users.id of the bill owner
    slug          text not null unique,        -- short code in split.my/r/<slug>
    merchant      text not null default '',
    owner_name    text not null default '',
    owner_qr_url  text,                         -- owner's DuitNow QR (Supabase Storage URL)
    captured_at   timestamptz,
    subtotal_sen  bigint not null default 0,
    sst_sen       bigint not null default 0,
    service_sen   bigint not null default 0,
    rounding_sen  bigint not null default 0,
    total_sen     bigint not null default 0,
    created_at    timestamptz not null default now()
);

create table if not exists items (
    id              uuid primary key default gen_random_uuid(),
    split_id        uuid not null references splits(id) on delete cascade,
    name            text not null,
    qty             int  not null default 1,
    unit_price_sen  bigint not null default 0,
    line_total_sen  bigint not null default 0,
    position        int  not null default 0,
    included_in_split boolean not null default true
);
create index if not exists items_split_idx on items(split_id);

create table if not exists participants (
    id          uuid primary key default gen_random_uuid(),
    split_id    uuid not null references splits(id) on delete cascade,
    name        text not null,
    is_owner    boolean not null default false,
    token       text unique,                    -- null for owner; random bearer for friends
    owed_sen    bigint not null default 0,       -- derived; recomputed on claim changes
    paid        boolean not null default false,
    paid_at     timestamptz,
    created_at  timestamptz not null default now()
);
create index if not exists participants_split_idx on participants(split_id);

create table if not exists claims (
    participant_id uuid not null references participants(id) on delete cascade,
    item_id        uuid not null references items(id) on delete cascade,
    primary key (participant_id, item_id)
);
create index if not exists claims_item_idx on claims(item_id);

create table if not exists owner_profiles (
    owner_id            uuid primary key,        -- auth.users.id
    owner_qr_url        text,                    -- Supabase Storage public URL
    auto_fill_amount    boolean not null default true,
    onboarding_seen_at  timestamptz,             -- first-time welcome dismissed
    updated_at          timestamptz not null default now()
);

alter table owner_profiles add column if not exists onboarding_seen_at timestamptz;

create table if not exists scan_sessions (
    id          uuid primary key default gen_random_uuid(),
    owner_id    uuid not null,
    image_url   text not null,
    expires_at  timestamptz not null,
    created_at  timestamptz not null default now()
);
create index if not exists scan_sessions_owner_idx on scan_sessions(owner_id);
create index if not exists scan_sessions_expires_idx on scan_sessions(expires_at);
