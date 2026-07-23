-- 0234_messaging_ai_credentials_and_roles.sql
--
-- Cofre account-scoped de credenciais nomeadas reutilizaveis por agentes e
-- vinculo versionado da credencial que executa cada funcao de IA.

create table if not exists messaging.ai_credentials (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    name text not null check (char_length(btrim(name)) between 1 and 120),
    provider text not null check (provider in ('openai','gemini','glm')),
    secret_ciphertext text not null check (secret_ciphertext <> ''),
    secret_last4 text not null default '' check (char_length(secret_last4) <= 4),
    created_by uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, id)
);

create unique index if not exists messaging_ai_credentials_name_uidx
    on messaging.ai_credentials (account_id, lower(btrim(name)));

create index if not exists messaging_ai_credentials_provider_idx
    on messaging.ai_credentials (account_id, provider, name);

alter table messaging.ai_agent_versions
    add column if not exists response_credential_id uuid;

do $$
begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_ai_versions_response_credential_fk'
          and conrelid = 'messaging.ai_agent_versions'::regclass
    ) then
        alter table messaging.ai_agent_versions
            add constraint messaging_ai_versions_response_credential_fk
            foreign key (account_id, response_credential_id)
            references messaging.ai_credentials(account_id, id)
            on delete restrict;
    end if;
end
$$;

create index if not exists messaging_ai_versions_response_credential_idx
    on messaging.ai_agent_versions (account_id, response_credential_id)
    where response_credential_id is not null;

alter table messaging.media_analyses
    add column if not exists credential_id uuid;

do $$
begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_media_analyses_credential_fk'
          and conrelid = 'messaging.media_analyses'::regclass
    ) then
        alter table messaging.media_analyses
            add constraint messaging_media_analyses_credential_fk
            foreign key (account_id, credential_id)
            references messaging.ai_credentials(account_id, id)
            on delete restrict;
    end if;
end
$$;

-- Video passa a ter resultado estruturado proprio, sem reutilizar silenciosamente
-- a semantica de imagem estatica.
alter table messaging.media_analyses
    drop constraint if exists media_analyses_analysis_kind_check;

alter table messaging.media_analyses
    add constraint media_analyses_analysis_kind_check
    check (analysis_kind in ('transcription','vision','video_summary','document_text'));
