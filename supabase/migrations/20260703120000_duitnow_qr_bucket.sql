-- Public bucket for owner DuitNow QR images (served to friends at pay time).
insert into storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
values ('duitnow-qr', 'duitnow-qr', true, 5242880, array['image/jpeg', 'image/png'])
on conflict (id) do update set
  public = excluded.public,
  file_size_limit = excluded.file_size_limit,
  allowed_mime_types = excluded.allowed_mime_types;

-- Anyone can read QR images (they are shown to friends to scan and pay).
create policy "duitnow qr public read"
on storage.objects for select
to public
using (bucket_id = 'duitnow-qr');
