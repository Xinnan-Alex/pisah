-- Temporary receipt images during owner review (rescan, thumbnail). Expired via scan_sessions cleanup.
insert into storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
values ('receipt-scans', 'receipt-scans', true, 12582912, array['image/jpeg'])
on conflict (id) do update set
  public = excluded.public,
  file_size_limit = excluded.file_size_limit,
  allowed_mime_types = excluded.allowed_mime_types;

-- Server fetches via public URL; objects live under {owner_id}/{scan_id}.jpg.
create policy "receipt scans public read"
on storage.objects for select
to public
using (bucket_id = 'receipt-scans');
