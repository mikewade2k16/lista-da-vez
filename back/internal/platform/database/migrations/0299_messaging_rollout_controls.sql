-- E10: rollout tenant-scoped e auditavel do Omnichannel.
--
-- O JSON generico de core.account_modules nao oferece constraint, revisao otimista nem
-- trilha before/after. Esta tabela pequena e a fonte autoritativa do kill switch e dos
-- gates de IA. Ausencia de linha preserva o comportamento legado (`active`) ate que a
-- conta grave sua primeira configuracao; a API deixa esse fallback explicito.

create table if not exists messaging.rollout_configs (
    account_id                    uuid primary key references core.accounts(id) on delete cascade,
    mode                          text not null default 'active',
    allowed_instance_ids          uuid[] not null default '{}'::uuid[],
    allowed_instagram_account_ids uuid[] not null default '{}'::uuid[],
    allowed_queue_ids             uuid[] not null default '{}'::uuid[],
    auto_reply_percent            integer not null default 100,
    allowed_hours                 jsonb not null default '{"timezone":"America/Sao_Paulo","windows":[]}'::jsonb,
    excluded_tags                 text[] not null default '{}'::text[],
    max_daily_auto_replies        integer not null default 0,
    kill_switch_reason            text,
    revision                      bigint not null default 1,
    updated_by_user_id            uuid references core.users(id) on delete set null,
    created_at                    timestamptz not null default now(),
    updated_at                    timestamptz not null default now(),
    constraint messaging_rollout_configs_mode_ck check (
        mode in ('off','observe','shadow','assist','auto_pilot','active','paused')
    ),
    constraint messaging_rollout_configs_percent_ck check (
        auto_reply_percent between 0 and 100
    ),
    constraint messaging_rollout_configs_daily_ck check (max_daily_auto_replies >= 0),
    constraint messaging_rollout_configs_revision_ck check (revision >= 1),
    constraint messaging_rollout_configs_pause_reason_ck check (
        mode <> 'paused' or nullif(btrim(kill_switch_reason),'') is not null
    )
);

create table if not exists messaging.rollout_changes (
    id            uuid primary key default gen_random_uuid(),
    account_id    uuid not null references core.accounts(id) on delete cascade,
    actor_user_id uuid references core.users(id) on delete set null,
    before_config jsonb not null,
    after_config  jsonb not null,
    reason        text not null,
    created_at    timestamptz not null default now(),
    constraint messaging_rollout_changes_reason_ck check (
        char_length(btrim(reason)) between 3 and 500
    )
);

create index if not exists messaging_rollout_changes_account_created_idx
    on messaging.rollout_changes (account_id, created_at desc, id desc);
