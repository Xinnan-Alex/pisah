-- Local dev test accounts. Loaded on `supabase db reset` (see config.toml [db.seed]).
-- Password for all seeded accounts: password123

create extension if not exists pgcrypto;

-- owner@test.com (primary test owner)
-- GoTrue scans token columns into Go strings; they must be '' not NULL.
insert into auth.users (
  instance_id, id, aud, role, email, encrypted_password,
  email_confirmed_at, raw_app_meta_data, raw_user_meta_data, created_at, updated_at,
  confirmation_token, email_change, email_change_token_new, email_change_token_current,
  recovery_token, phone_change, phone_change_token
) values (
  '00000000-0000-0000-0000-000000000000',
  '31e936fa-5e43-4335-bb8d-5dd0c0b4dcd9',
  'authenticated', 'authenticated', 'owner@test.com',
  crypt('password123', gen_salt('bf')),
  now(), '{"provider":"email","providers":["email"]}', '{}', now(), now(),
  '', '', '', '', '', '', ''
) on conflict (id) do nothing;

insert into auth.identities (
  id, user_id, identity_data, provider, provider_id, last_sign_in_at, created_at, updated_at
) values (
  'de14c872-2d06-45df-a04f-ab853f4a5c4e',
  '31e936fa-5e43-4335-bb8d-5dd0c0b4dcd9',
  '{"sub": "31e936fa-5e43-4335-bb8d-5dd0c0b4dcd9", "email": "owner@test.com", "email_verified": true, "phone_verified": false}',
  'email', '31e936fa-5e43-4335-bb8d-5dd0c0b4dcd9', now(), now(), now()
) on conflict do nothing;

-- test@pisah.local (second test owner)
insert into auth.users (
  instance_id, id, aud, role, email, encrypted_password,
  email_confirmed_at, raw_app_meta_data, raw_user_meta_data, created_at, updated_at,
  confirmation_token, email_change, email_change_token_new, email_change_token_current,
  recovery_token, phone_change, phone_change_token
) values (
  '00000000-0000-0000-0000-000000000000',
  'a8f3c2e1-4b5d-6e7f-8091-a2b3c4d5e6f7',
  'authenticated', 'authenticated', 'test@pisah.local',
  crypt('password123', gen_salt('bf')),
  now(), '{"provider":"email","providers":["email"]}', '{}', now(), now(),
  '', '', '', '', '', '', ''
) on conflict (id) do nothing;

insert into auth.identities (
  id, user_id, identity_data, provider, provider_id, last_sign_in_at, created_at, updated_at
) values (
  'b9e4d3f2-5c6a-7b8c-9012-d3e4f5a6b7c8',
  'a8f3c2e1-4b5d-6e7f-8091-a2b3c4d5e6f7',
  '{"sub": "a8f3c2e1-4b5d-6e7f-8091-a2b3c4d5e6f7", "email": "test@pisah.local", "email_verified": true, "phone_verified": false}',
  'email', 'a8f3c2e1-4b5d-6e7f-8091-a2b3c4d5e6f7', now(), now(), now()
) on conflict do nothing;
