-- E10: rascunhos de resposta do modo assist.
--
-- O Go/PostgreSQL continuam autoritativos: a IA apenas propoe texto. O envio so acontece
-- pelo POST humano existente, que registra uso/edicao na mesma transacao da mensagem+outbox.

create unique index if not exists messaging_ai_dispatches_account_id_uidx
    on messaging.ai_dispatches (account_id, id);

create table if not exists messaging.ai_reply_drafts (
    id                 uuid primary key default gen_random_uuid(),
    account_id         uuid not null references core.accounts(id) on delete cascade,
    conversation_id    uuid not null,
    dispatch_id        uuid not null,
    generation         bigint not null,
    content            text not null,
    status             text not null default 'pending',
    used_message_id    uuid,
    decided_by_user_id uuid references core.users(id) on delete set null,
    decision_reason    text not null default '',
    edited             boolean not null default false,
    created_at         timestamptz not null default now(),
    decided_at         timestamptz,
    updated_at         timestamptz not null default now(),
    constraint messaging_ai_reply_drafts_conversation_fk
        foreign key (account_id, conversation_id)
        references messaging.conversations(account_id, id) on delete cascade,
    constraint messaging_ai_reply_drafts_dispatch_fk
        foreign key (account_id, dispatch_id)
        references messaging.ai_dispatches(account_id, id) on delete cascade,
    constraint messaging_ai_reply_drafts_generation_ck check (generation >= 0),
    constraint messaging_ai_reply_drafts_content_ck check (
        char_length(btrim(content)) between 1 and 4000
    ),
    constraint messaging_ai_reply_drafts_status_ck check (
        status in ('pending','used','dismissed','expired')
    ),
    constraint messaging_ai_reply_drafts_decision_ck check (
        (status = 'pending' and decided_at is null and decided_by_user_id is null)
        or (status <> 'pending' and decided_at is not null)
    ),
    constraint messaging_ai_reply_drafts_reason_ck check (
        char_length(decision_reason) <= 500
    )
);

create unique index if not exists messaging_ai_reply_drafts_dispatch_uidx
    on messaging.ai_reply_drafts (account_id, dispatch_id);

create unique index if not exists messaging_ai_reply_drafts_pending_conversation_uidx
    on messaging.ai_reply_drafts (account_id, conversation_id)
    where status = 'pending';

create index if not exists messaging_ai_reply_drafts_account_created_idx
    on messaging.ai_reply_drafts (account_id, created_at desc, id desc);
