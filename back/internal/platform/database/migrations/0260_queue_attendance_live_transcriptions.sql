-- Segmentos duraveis para transcricao quase ao vivo dos atendimentos.
--
-- Os blocos de transporte continuam com 5 segundos. A cada cinco blocos
-- contiguos nasce uma janela de 25 segundos; depois da primeira, a janela
-- inclui 2,5 segundos de sobreposicao para proteger palavras nas bordas.

alter table queue.attendance_recordings
    add column if not exists live_transcript_text text not null default '',
    add column if not exists live_transcript_updated_at timestamptz;

create table if not exists queue.attendance_live_transcription_segments (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    recording_id uuid not null,
    segment_index integer not null check (segment_index >= 0),
    start_sequence integer not null check (start_sequence >= 0),
    end_sequence integer not null check (end_sequence >= start_sequence),
    trim_start_ms integer not null default 0 check (trim_start_ms between 0 and 5000),
    status text not null default 'pending'
        check (status in ('pending', 'processing', 'completed', 'failed')),
    transcript_text text not null default '',
    error_message text not null default '',
    attempt_count integer not null default 0 check (attempt_count >= 0),
    next_attempt_at timestamptz not null default now(),
    locked_at timestamptz,
    locked_by text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, recording_id, segment_index),
    constraint queue_attendance_live_segments_recording_fk
        foreign key (account_id, recording_id)
        references queue.attendance_recordings(account_id, id)
        on delete cascade
);

create index if not exists queue_attendance_live_segments_job_idx
    on queue.attendance_live_transcription_segments (
        status,
        next_attempt_at,
        recording_id,
        segment_index
    )
    where status in ('pending', 'processing');

comment on table queue.attendance_live_transcription_segments is
    'Janelas limitadas e duraveis do Whisper quase ao vivo; a passagem integral final permanece autoritativa.';
comment on column queue.attendance_live_transcription_segments.trim_start_ms is
    'Inicio descartado da janela bruta para manter 2,5 segundos de sobreposicao util.';
