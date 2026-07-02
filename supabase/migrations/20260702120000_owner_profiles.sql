-- Owner payment settings (DuitNow QR + preferences), keyed by auth user id.
create table if not exists owner_profiles (
    owner_id          uuid primary key,
    owner_qr_url      text,
    auto_fill_amount  boolean not null default true,
    updated_at        timestamptz not null default now()
);
