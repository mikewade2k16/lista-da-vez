-- Configuracao e job duravel de interpretacao das transcricoes da fila.
-- O audio continua privado no modulo Queue; o n8n recebe apenas texto e contexto.

create table if not exists queue.attendance_analysis_configs (
    account_id uuid primary key,
    enabled boolean not null default true,
    transcription_provider text not null default 'local'
        check (transcription_provider = 'local'),
    transcription_model text not null default 'Systran/faster-whisper-base',
    transcription_language text not null default 'pt',
    provider text not null default 'gemini'
        check (provider in ('gemini', 'openai')),
    model text not null default 'gemini-2.5-flash',
    system_prompt text not null default '',
    temperature numeric(3,2) not null default 0.20
        check (temperature between 0 and 1),
    updated_by uuid,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

comment on column queue.attendance_analysis_configs.account_id is
    'Escopo autoritativo da conta, sem FK cross-schema por contrato do modulo Queue.';
comment on column queue.attendance_analysis_configs.system_prompt is
    'Politica soberana para interpretar, normalizar nomes e resumir o atendimento.';

create table if not exists queue.attendance_analysis_secrets (
    account_id uuid primary key,
    api_key_ciphertext text not null,
    api_key_last4 text not null default '',
    updated_by uuid,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

comment on column queue.attendance_analysis_secrets.api_key_ciphertext is
    'Chave do provider cifrada por platform/secretbox; nunca devolvida ao frontend.';

alter table queue.attendance_recordings
    add column if not exists analysis_status text not null default 'not_requested'
        check (analysis_status in ('not_requested', 'pending', 'processing', 'completed', 'failed')),
    add column if not exists summary_text text not null default '',
    add column if not exists analysis_report jsonb not null default '{}'::jsonb,
    add column if not exists analysis_error text not null default '',
    add column if not exists analysis_requested_at timestamptz,
    add column if not exists analysis_next_attempt_at timestamptz,
    add column if not exists analysis_locked_at timestamptz,
    add column if not exists analysis_locked_by text not null default '',
    add column if not exists analysis_attempt_count integer not null default 0
        check (analysis_attempt_count >= 0),
    add column if not exists analysis_config_snapshot jsonb not null default '{}'::jsonb;

create index if not exists queue_attendance_recordings_analysis_job_idx
    on queue.attendance_recordings (
        analysis_status,
        analysis_next_attempt_at,
        analysis_requested_at
    )
    where transcription_status = 'completed'
      and analysis_requested_at is not null
      and analysis_status in ('pending', 'processing');
