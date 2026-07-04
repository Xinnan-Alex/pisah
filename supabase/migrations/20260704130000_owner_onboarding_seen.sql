-- Track when an owner dismisses the first-time welcome walkthrough.
alter table owner_profiles add column if not exists onboarding_seen_at timestamptz;
