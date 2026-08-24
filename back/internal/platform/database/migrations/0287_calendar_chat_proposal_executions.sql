-- Assistente 360: idempotencia de /ask e execucao autoritativa das propostas
-- Calendar. Os cards continuam projetados em calendar.chat_messages.proposals,
-- mas esta tabela passa a ser a fonte de execucao e do recibo do efeito.

create table if not exists calendar.chat_proposal_executions (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    conversation_id uuid not null references calendar.chat_conversations(id) on delete cascade,
    message_id uuid not null references calendar.chat_messages(id) on delete cascade,
    proposal_id text not null,
    kind text not null check (kind in ('event', 'task', 'taskItem', 'note', 'clientProfile')),
    action text not null check (action in ('create', 'update', 'delete')),
    status text not null default 'pending'
        check (status in ('pending', 'executing', 'succeeded', 'failed', 'unknown', 'rejected')),
    proposal_snapshot jsonb not null check (jsonb_typeof(proposal_snapshot) = 'object'),
    proposal_hash bytea not null check (octet_length(proposal_hash) = 32),
    editable_fields jsonb not null default '{}'::jsonb
        check (jsonb_typeof(editable_fields) = 'object'),
    confirmation_key text,
    confirmation_request_hash bytea,
    storage_account_id uuid references core.accounts(id) on delete restrict,
    target_id uuid,
    expected_version integer,
    before_snapshot jsonb not null default '{}'::jsonb
        check (jsonb_typeof(before_snapshot) = 'object'),
    before_hash bytea,
    result_snapshot jsonb not null default '{}'::jsonb
        check (jsonb_typeof(result_snapshot) = 'object'),
    error_code text not null default '',
    error_message text not null default '',
    actor_user_id uuid references core.users(id) on delete set null,
    attempted_at timestamptz,
    rejected_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint calendar_chat_proposal_execution_source_unique
        unique (account_id, message_id, proposal_id),
    constraint calendar_chat_proposal_execution_confirmation_complete check (
        (confirmation_key is null and confirmation_request_hash is null)
        or
        (confirmation_key is not null and confirmation_request_hash is not null
            and octet_length(confirmation_request_hash) = 32)
    ),
    constraint calendar_chat_proposal_execution_before_hash_length check (
        before_hash is null or octet_length(before_hash) = 32
    ),
    constraint calendar_chat_proposal_execution_limits check (
        length(proposal_id) between 1 and 64
        and (confirmation_key is null or length(confirmation_key) between 8 and 200)
        and octet_length(proposal_snapshot::text) <= 65536
        and octet_length(editable_fields::text) <= 65536
        and octet_length(before_snapshot::text) <= 65536
        and octet_length(result_snapshot::text) <= 65536
        and length(error_code) <= 100
        and length(error_message) <= 1000
    )
);

create unique index if not exists calendar_chat_conversations_account_id_uidx
    on calendar.chat_conversations (account_id, id);

create unique index if not exists calendar_chat_messages_source_uidx
    on calendar.chat_messages (account_id, conversation_id, id);

do $$
begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'calendar_chat_proposal_execution_conversation_fk'
          and conrelid = 'calendar.chat_proposal_executions'::regclass
    ) then
        alter table calendar.chat_proposal_executions
            add constraint calendar_chat_proposal_execution_conversation_fk
            foreign key (account_id, conversation_id)
            references calendar.chat_conversations(account_id, id)
            on delete cascade;
    end if;
    if not exists (
        select 1 from pg_constraint
        where conname = 'calendar_chat_proposal_execution_message_fk'
          and conrelid = 'calendar.chat_proposal_executions'::regclass
    ) then
        alter table calendar.chat_proposal_executions
            add constraint calendar_chat_proposal_execution_message_fk
            foreign key (account_id, conversation_id, message_id)
            references calendar.chat_messages(account_id, conversation_id, id)
            on delete cascade;
    end if;
end $$;

create unique index if not exists calendar_chat_proposal_execution_confirmation_uidx
    on calendar.chat_proposal_executions (account_id, actor_user_id, confirmation_key)
    where confirmation_key is not null;

create index if not exists calendar_chat_proposal_execution_source_idx
    on calendar.chat_proposal_executions (account_id, conversation_id, message_id);

create table if not exists calendar.chat_ask_requests (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    actor_user_id uuid not null references core.users(id) on delete cascade,
    idempotency_key text not null check (length(idempotency_key) between 8 and 200),
    request_hash bytea not null check (octet_length(request_hash) = 32),
    status text not null default 'executing'
        check (status in ('executing', 'succeeded', 'failed', 'unknown')),
    -- IDs deliberadamente sem FK: o recibo da chave sobrevive a remocao futura da
    -- conversa e impede que a mesma operacao seja executada de novo apos o delete.
    requested_conversation_id uuid,
    conversation_id uuid,
    entry_surface text not null check (entry_surface in ('calendar', 'meta_ads', 'global')),
    scope_mode text not null check (scope_mode in ('client', 'all')),
    scope_client_id uuid,
    response_snapshot jsonb not null default '{}'::jsonb
        check (jsonb_typeof(response_snapshot) = 'object'),
    error_code text not null default '' check (length(error_code) <= 100),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    completed_at timestamptz,
    constraint calendar_chat_ask_requests_key_unique
        unique (account_id, actor_user_id, idempotency_key),
    constraint calendar_chat_ask_requests_response_limit
        check (octet_length(response_snapshot::text) <= 131072)
);

create index if not exists calendar_chat_ask_requests_conversation_idx
    on calendar.chat_ask_requests (account_id, conversation_id, created_at desc);
