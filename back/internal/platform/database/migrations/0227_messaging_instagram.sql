-- E8: Instagram professional account, comment moderation and provider metadata.
create table if not exists messaging.instagram_accounts (
    id                    uuid primary key default gen_random_uuid(),
    account_id            uuid not null references core.accounts(id) on delete cascade,
    ig_user_id            text not null,
    username              text,
    display_name          text,
    page_id               text,
    provider_config       jsonb not null default '{}'::jsonb,
    credentials_ciphertext text,
    is_active             boolean not null default true,
    webhook_status        text not null default 'unknown',
    created_at            timestamptz not null default now(),
    updated_at            timestamptz not null default now()
);

create unique index if not exists messaging_instagram_accounts_account_ig_uidx
    on messaging.instagram_accounts (account_id, ig_user_id);
create unique index if not exists messaging_instagram_accounts_account_id_id_uidx
    on messaging.instagram_accounts (account_id, id);
create index if not exists messaging_instagram_accounts_account_active_idx
    on messaging.instagram_accounts (account_id, is_active);

create table if not exists messaging.instagram_comments (
    id                    uuid primary key default gen_random_uuid(),
    account_id            uuid not null references core.accounts(id) on delete cascade,
    instagram_account_id  uuid not null references messaging.instagram_accounts(id) on delete cascade,
    external_comment_id   text not null,
    external_media_id     text,
    parent_comment_id     text,
    contact_id            uuid references messaging.contacts(id) on delete set null,
    author_scoped_id      text not null,
    username              text,
    text                  text not null default '',
    event_kind            text not null default 'comment',
    status                text not null default 'visible',
    is_live               boolean not null default false,
    occurred_at           timestamptz not null,
    metadata              jsonb not null default '{}'::jsonb,
    created_at            timestamptz not null default now(),
    updated_at            timestamptz not null default now(),
    constraint instagram_comments_event_kind_ck check (event_kind in ('comment','mention')),
    constraint instagram_comments_status_ck check (status in ('visible','hidden','deleted','pending_review')),
    constraint instagram_comments_account_account_fk foreign key (account_id, instagram_account_id)
        references messaging.instagram_accounts(account_id, id) on delete cascade
);

create unique index if not exists messaging_instagram_comments_account_external_uidx
    on messaging.instagram_comments (account_id, instagram_account_id, external_comment_id);
create unique index if not exists messaging_instagram_comments_account_id_id_uidx
    on messaging.instagram_comments (account_id, id);
create index if not exists messaging_instagram_comments_account_status_idx
    on messaging.instagram_comments (account_id, instagram_account_id, status, occurred_at desc);

create table if not exists messaging.instagram_comment_actions (
    id                    uuid primary key default gen_random_uuid(),
    account_id            uuid not null references core.accounts(id) on delete cascade,
    comment_id            uuid not null references messaging.instagram_comments(id) on delete cascade,
    action_kind           text not null,
    status                text not null default 'pending_review',
    proposed_text         text,
    approved_text         text,
    ai_run_id             uuid,
    approved_by_user_id   uuid references core.users(id) on delete set null,
    approved_at           timestamptz,
    external_message_id   text,
    idempotency_key       text not null,
    private_reply_expires_at timestamptz,
    last_error            text not null default '',
    created_at            timestamptz not null default now(),
    executed_at           timestamptz,
    constraint instagram_comment_actions_kind_ck check (action_kind in ('public_reply','private_reply','hide','ignore')),
    constraint instagram_comment_actions_status_ck check (status in ('pending_review','approved','rejected','processing','sent','failed','expired','ignored')),
    constraint instagram_comment_actions_account_comment_fk foreign key (account_id, comment_id)
        references messaging.instagram_comments(account_id, id) on delete cascade
);

create unique index if not exists messaging_instagram_comment_actions_account_idempotency_uidx
    on messaging.instagram_comment_actions (account_id, idempotency_key);
create index if not exists messaging_instagram_comment_actions_account_status_idx
    on messaging.instagram_comment_actions (account_id, comment_id, status, created_at desc);
