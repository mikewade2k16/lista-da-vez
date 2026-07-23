-- 0233_messaging_contact_ai_restrictions.sql
--
-- Bloqueio account-scoped da IA por contato. Uma linha com blocked_until nulo
-- representa bloqueio indefinido; uma data futura representa bloqueio temporario.
-- A regra e independente da ocultacao/privacidade do contato.

create table if not exists messaging.contact_ai_restrictions (
    account_id uuid not null references core.accounts(id) on delete cascade,
    contact_id uuid not null,
    blocked_until timestamptz,
    updated_by_user_id uuid not null references core.users(id) on delete restrict,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (account_id, contact_id),
    constraint messaging_contact_ai_restrictions_contact_fk
        foreign key (account_id, contact_id)
        references messaging.contacts(account_id, id)
        on delete cascade
);

create index if not exists messaging_contact_ai_restrictions_active_idx
    on messaging.contact_ai_restrictions (account_id, blocked_until, contact_id);
