-- Gravacoes experimentais dos atendimentos da fila.
--
-- O PostgreSQL guarda metadados, escopo e estado autoritativos. Os bytes ficam
-- no storage privado ATTENDANCE_AUDIO_DIR e nunca sob /uploads.

create table if not exists queue.attendance_recordings (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    store_id uuid not null
        references queue.stores(id) on delete cascade,
    service_id text not null,
    consultant_id uuid not null
        references queue.consultants(id) on delete restrict,
    consultant_name text not null,
    client_session_id text not null,
    recording_status text not null default 'recording'
        check (recording_status in ('recording', 'ready', 'interrupted', 'failed')),
    transcription_status text not null default 'pending'
        check (transcription_status in ('pending', 'processing', 'completed', 'failed')),
    mime_type text not null,
    started_at bigint not null,
    ended_at bigint,
    chunk_count integer not null default 0 check (chunk_count >= 0),
    size_bytes bigint not null default 0 check (size_bytes >= 0),
    audio_storage_key text,
    audio_sha256 text not null default '',
    transcript_text text not null default '',
    transcript_error text not null default '',
    created_by uuid not null,
    retention_expires_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, client_session_id),
    unique (account_id, id)
);

comment on column queue.attendance_recordings.account_id is
    'Escopo da account. Sem FK cross-schema por contrato do modulo queue.';
comment on column queue.attendance_recordings.retention_expires_at is
    'Reservado para a politica configuravel de retencao; sem poda automatica neste bloco experimental.';

create index if not exists queue_attendance_recordings_list_idx
    on queue.attendance_recordings (account_id, started_at desc, id desc);

create index if not exists queue_attendance_recordings_store_idx
    on queue.attendance_recordings (account_id, store_id, started_at desc);

create index if not exists queue_attendance_recordings_service_idx
    on queue.attendance_recordings (account_id, store_id, service_id);

create table if not exists queue.attendance_recording_chunks (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    recording_id uuid not null,
    sequence integer not null check (sequence >= 0),
    storage_key text not null,
    mime_type text not null,
    size_bytes bigint not null check (size_bytes > 0),
    sha256 text not null,
    created_at timestamptz not null default now(),
    unique (account_id, recording_id, sequence),
    constraint queue_attendance_recording_chunks_recording_fk
        foreign key (account_id, recording_id)
        references queue.attendance_recordings(account_id, id)
        on delete cascade
);

create index if not exists queue_attendance_recording_chunks_order_idx
    on queue.attendance_recording_chunks (account_id, recording_id, sequence);
