-- Device owner display name for no-login onboarding (name + DuitNow QR).
alter table owner_profiles add column if not exists display_name text not null default '';
