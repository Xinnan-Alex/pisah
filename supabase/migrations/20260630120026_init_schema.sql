-- Pisah schema. All money is integer sen (1 sen = RM 0.01).
-- The Go service connects with the Postgres connection string and enforces
-- authorization in app code. RLS is enabled deny-all (no policies) as defense in
-- depth: the direct Postgres connection bypasses RLS, but nothing leaks if a
-- table is ever exposed via the Data API. gen_random_uuid() is built into PG13+.

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
    position        int  not null default 0
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

-- Deny-all RLS: no policies, so anon/authenticated get nothing via the Data API.
-- The backend's direct Postgres connection (owner role) bypasses RLS.
alter table splits       enable row level security;
alter table items        enable row level security;
alter table participants enable row level security;
alter table claims       enable row level security;
