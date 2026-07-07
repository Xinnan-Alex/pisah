-- Temporary receipt scan images during owner review (rescan, thumbnail).
create table if not exists scan_sessions (
    id          uuid primary key default gen_random_uuid(),
    owner_id    uuid not null,
    image_url   text not null,
    expires_at  timestamptz not null,
    created_at  timestamptz not null default now()
);
create index if not exists scan_sessions_owner_idx on scan_sessions(owner_id);
create index if not exists scan_sessions_expires_idx on scan_sessions(expires_at);
