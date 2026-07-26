-- Job duravel para transcricao dos audios de atendimento pelo Whisper local.

alter table queue.attendance_recordings
    add column if not exists transcription_requested_at timestamptz,
    add column if not exists transcription_next_attempt_at timestamptz,
    add column if not exists transcription_locked_at timestamptz,
    add column if not exists transcription_locked_by text not null default '',
    add column if not exists transcription_attempt_count integer not null default 0
        check (transcription_attempt_count >= 0);

create index if not exists queue_attendance_recordings_transcription_job_idx
    on queue.attendance_recordings (
        transcription_status,
        transcription_next_attempt_at,
        transcription_requested_at
    )
    where recording_status = 'ready'
      and transcription_requested_at is not null
      and transcription_status in ('pending', 'processing');

comment on column queue.attendance_recordings.transcription_requested_at is
    'Solicitacao duravel da transcricao. Nulo significa audio pronto ainda nao enfileirado.';
comment on column queue.attendance_recordings.transcription_locked_at is
    'Lease do worker; jobs processing com lease expirado podem ser reclamados.';
