create table if not exists storage.multipart_uploads (
    id text primary key,
    object_id text not null references storage.objects(id) on delete cascade,
    account_id uuid not null,
    source_module text not null,
    provider_upload_id text not null default '',
    part_size_bytes bigint not null check (part_size_bytes >= 5242880),
    part_count integer not null check (part_count between 1 and 10000),
    status text not null default 'creating' check (status in ('creating','uploading','completing','completed','failed','aborted')),
    completion_attempts integer not null default 0,
    created_by uuid not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    expires_at timestamptz not null default (now() + interval '7 days'),
    unique (object_id)
);

create table if not exists storage.multipart_parts (
    upload_id text not null references storage.multipart_uploads(id) on delete cascade,
    part_number integer not null check (part_number between 1 and 10000),
    etag text not null default '',
    size_bytes bigint not null default 0,
    attempts integer not null default 0,
    uploaded_at timestamptz,
    primary key (upload_id, part_number)
);

create index if not exists storage_multipart_uploads_scope_idx
    on storage.multipart_uploads (account_id, source_module, status, updated_at desc);
