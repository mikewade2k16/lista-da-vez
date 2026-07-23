-- 0237_social_publishing_foundation.sql
--
-- Fundacao isolada do modulo de agendamento de postagens. O schema novo e a
-- unica fonte de verdade para conexao Instagram, publicacoes, analytics e jobs.
-- Calendar, Crow Assistant, Meta Ads e Omnichannel nao dependem desta migration.
--
-- Multi-tenant desde a primeira linha: toda entidade e filtravel por account_id,
-- referencias internas usam FKs compostas (account_id, id) e idempotencia nunca
-- e global entre contas.
--
-- Idempotente, aditiva e schema-qualified. SEM `-- +goose Down`: o migrator
-- executa o arquivo inteiro e um bloco Down seria executado como SQL comum. O
-- rollback operacional consiste em desabilitar o modulo e reverter o codigo;
-- as tabelas ficam preservadas para auditoria e retomada segura.

create schema if not exists social_publishing;

-- O catalogo precisa existir antes das permissoes em bancos fresh. O Module
-- Registry faz o mesmo upsert no boot e continua sendo o contrato de runtime.
insert into core.modules (
    id,
    schema_name,
    label,
    description,
    is_core,
    sort_order
)
values (
    'social_publishing',
    'social_publishing',
    'Agendamento de postagens',
    'Conexao Instagram, agendamento/publicacao e analytics organicos.',
    false,
    48
)
on conflict (id) do nothing;

insert into core.permissions (key, module_id, label, description, scope)
values
    (
        'social_publishing.view',
        'social_publishing',
        'Ver postagens',
        'Consultar conexao, rascunhos, agendamentos e publicacoes da conta.',
        'account'
    ),
    (
        'social_publishing.manage',
        'social_publishing',
        'Gerenciar postagens',
        'Criar, editar, agendar e cancelar publicacoes.',
        'account'
    ),
    (
        'social_publishing.connect',
        'social_publishing',
        'Conectar Instagram',
        'Cadastrar, validar, substituir ou revogar a credencial Instagram da conta.',
        'account'
    ),
    (
        'social_publishing.analytics',
        'social_publishing',
        'Consultar analytics de postagens',
        'Consultar e sincronizar metricas organicas das publicacoes.',
        'account'
    )
on conflict (key) do update set
    module_id = excluded.module_id,
    label = excluded.label,
    description = excluded.description,
    scope = excluded.scope,
    deprecated_at = null,
    updated_at = now();

-- Grants iniciais conservadores: owner/admin recebem todas as capacidades;
-- marketing opera conteudo e analytics, mas nao recebe acesso ao cofre de token.
-- Outros papeis e overrides permanecem inalterados e configuraveis pelo RBAC.
with role_grants(role_code, permission_key) as (
    values
        ('platform_admin', 'social_publishing.view'),
        ('platform_admin', 'social_publishing.manage'),
        ('platform_admin', 'social_publishing.connect'),
        ('platform_admin', 'social_publishing.analytics'),
        ('core.platform_admin', 'social_publishing.view'),
        ('core.platform_admin', 'social_publishing.manage'),
        ('core.platform_admin', 'social_publishing.connect'),
        ('core.platform_admin', 'social_publishing.analytics'),
        ('owner', 'social_publishing.view'),
        ('owner', 'social_publishing.manage'),
        ('owner', 'social_publishing.connect'),
        ('owner', 'social_publishing.analytics'),
        ('core.owner', 'social_publishing.view'),
        ('core.owner', 'social_publishing.manage'),
        ('core.owner', 'social_publishing.connect'),
        ('core.owner', 'social_publishing.analytics'),
        ('queue.owner', 'social_publishing.view'),
        ('queue.owner', 'social_publishing.manage'),
        ('queue.owner', 'social_publishing.connect'),
        ('queue.owner', 'social_publishing.analytics'),
        ('marketing', 'social_publishing.view'),
        ('marketing', 'social_publishing.manage'),
        ('marketing', 'social_publishing.analytics'),
        ('core.marketing', 'social_publishing.view'),
        ('core.marketing', 'social_publishing.manage'),
        ('core.marketing', 'social_publishing.analytics'),
        ('queue.marketing', 'social_publishing.view'),
        ('queue.marketing', 'social_publishing.manage'),
        ('queue.marketing', 'social_publishing.analytics')
)
insert into core.role_permissions (role_id, permission_key)
select r.id, grant_row.permission_key
from core.roles r
join role_grants grant_row
    on lower(btrim(r.code)) = grant_row.role_code
on conflict (role_id, permission_key) do nothing;

-- Mantem os mesmos defaults quando um papel for clonado no futuro.
with template_grants(role_template_id, permission_key) as (
    values
        ('platform_admin', 'social_publishing.view'),
        ('platform_admin', 'social_publishing.manage'),
        ('platform_admin', 'social_publishing.connect'),
        ('platform_admin', 'social_publishing.analytics'),
        ('core.platform_admin', 'social_publishing.view'),
        ('core.platform_admin', 'social_publishing.manage'),
        ('core.platform_admin', 'social_publishing.connect'),
        ('core.platform_admin', 'social_publishing.analytics'),
        ('owner', 'social_publishing.view'),
        ('owner', 'social_publishing.manage'),
        ('owner', 'social_publishing.connect'),
        ('owner', 'social_publishing.analytics'),
        ('core.owner', 'social_publishing.view'),
        ('core.owner', 'social_publishing.manage'),
        ('core.owner', 'social_publishing.connect'),
        ('core.owner', 'social_publishing.analytics'),
        ('queue.owner', 'social_publishing.view'),
        ('queue.owner', 'social_publishing.manage'),
        ('queue.owner', 'social_publishing.connect'),
        ('queue.owner', 'social_publishing.analytics'),
        ('marketing', 'social_publishing.view'),
        ('marketing', 'social_publishing.manage'),
        ('marketing', 'social_publishing.analytics'),
        ('core.marketing', 'social_publishing.view'),
        ('core.marketing', 'social_publishing.manage'),
        ('core.marketing', 'social_publishing.analytics'),
        ('queue.marketing', 'social_publishing.view'),
        ('queue.marketing', 'social_publishing.manage'),
        ('queue.marketing', 'social_publishing.analytics')
)
insert into core.role_template_permissions (role_template_id, permission_key)
select rt.id, grant_row.permission_key
from core.role_templates rt
join template_grants grant_row
    on rt.id = grant_row.role_template_id
on conflict (role_template_id, permission_key) do nothing;

-- Uma conta possui historico de conexoes, mas no maximo uma delas fica ativa.
-- Cada post guarda o connection_id que definiu seu destino; reconectar nunca
-- pode trocar silenciosamente o Instagram de uma publicacao ja agendada. O token
-- guarda somente ciphertext v1: do platform/secretbox e nunca volta ao frontend.
create table if not exists social_publishing.connections (
    id                      uuid primary key default gen_random_uuid(),
    account_id              uuid not null references core.accounts(id) on delete cascade,
    provider                text not null default 'instagram'
        check (provider = 'instagram'),
    ig_user_id              text not null
        check (btrim(ig_user_id) <> ''),
    username                text not null default '',
    account_type            text not null default 'BUSINESS'
        check (account_type in ('BUSINESS', 'CREATOR', 'MEDIA_CREATOR')),
    media_count             bigint not null default 0
        check (media_count >= 0),
    status                  text not null default 'connected'
        check (status in ('connected', 'expired', 'revoked', 'error')),
    access_token_ciphertext text not null
        check (status <> 'connected' or btrim(access_token_ciphertext) <> ''),
    token_last4             text not null default ''
        check (char_length(token_last4) <= 4),
    metadata                jsonb not null default '{}'::jsonb
        check (jsonb_typeof(metadata) = 'object'),
    connected_at            timestamptz not null default now(),
    created_by              uuid references core.users(id) on delete set null,
    updated_by              uuid references core.users(id) on delete set null,
    created_at              timestamptz not null default now(),
    updated_at              timestamptz not null default now(),
    version                 integer not null default 1
        check (version >= 1),
    constraint social_publishing_connections_account_id_uidx unique (account_id, id)
);

-- Remove a constraint da primeira iteracao local da 0237, caso ela tenha sido
-- aplicada durante o piloto antes desta correcao. O indice parcial preserva o
-- historico imutavel e ainda impede duas credenciais ativas para a mesma conta.
alter table social_publishing.connections
    drop constraint if exists social_publishing_connections_account_uidx;

create unique index if not exists social_publishing_connections_active_uidx
    on social_publishing.connections (account_id)
    where status = 'connected';

create index if not exists social_publishing_connections_status_idx
    on social_publishing.connections (account_id, status);

-- Posts sao o agregado autoritativo. schedule_revision identifica a revisao do
-- job agendado; version sustenta optimistic locking de qualquer mutacao.
create table if not exists social_publishing.posts (
    id                   uuid primary key default gen_random_uuid(),
    account_id           uuid not null references core.accounts(id) on delete cascade,
    connection_id        uuid,
    caption              text not null default ''
        check (char_length(caption) <= 2200),
    media_url            text not null default ''
        check (media_url = '' or media_url ~* '^https://'),
    alt_text             text not null default ''
        check (char_length(alt_text) <= 1000),
    status               text not null default 'draft'
        check (status in ('draft', 'scheduled', 'publishing', 'published', 'failed', 'cancelled')),
    scheduled_for        timestamptz,
    timezone             text not null default 'America/Sao_Paulo'
        check (btrim(timezone) <> ''),
    schedule_revision    integer not null default 0
        check (schedule_revision >= 0),
    version              integer not null default 1
        check (version >= 1),
    source_type          text not null default 'manual'
        check (source_type in ('manual', 'calendar', 'crow_assistant')),
    source_ref           text
        check (source_ref is null or btrim(source_ref) <> ''),
    external_creation_id text not null default '',
    external_media_id    text not null default '',
    permalink            text not null default '',
    last_error_code      text not null default '',
    last_error_message   text not null default ''
        check (char_length(last_error_message) <= 1000),
    published_at         timestamptz,
    created_by           uuid references core.users(id) on delete set null,
    updated_by           uuid references core.users(id) on delete set null,
    created_at           timestamptz not null default now(),
    updated_at           timestamptz not null default now(),
    constraint social_publishing_posts_account_id_uidx unique (account_id, id),
    constraint social_publishing_posts_connection_tenant_fk
        foreign key (account_id, connection_id)
        references social_publishing.connections(account_id, id)
        on delete restrict,
    constraint social_publishing_posts_integrated_source_ck
        check (source_type = 'manual' or source_ref is not null),
    constraint social_publishing_posts_scheduled_at_ck
        check (status <> 'scheduled' or scheduled_for is not null),
    constraint social_publishing_posts_connection_required_ck
        check (status not in ('scheduled', 'publishing', 'published') or connection_id is not null),
    constraint social_publishing_posts_media_required_ck
        check (status not in ('scheduled', 'publishing', 'published') or media_url <> ''),
    constraint social_publishing_posts_published_ck
        check (
            status <> 'published'
            or (btrim(external_media_id) <> '' and published_at is not null)
        )
);

create unique index if not exists social_publishing_posts_source_uidx
    on social_publishing.posts (account_id, source_type, source_ref)
    where source_ref is not null;
create unique index if not exists social_publishing_posts_external_media_uidx
    on social_publishing.posts (account_id, external_media_id)
    where external_media_id <> '';
create index if not exists social_publishing_posts_schedule_idx
    on social_publishing.posts (account_id, scheduled_for, id)
    where status = 'scheduled';
create index if not exists social_publishing_posts_status_updated_idx
    on social_publishing.posts (account_id, status, updated_at desc);
create index if not exists social_publishing_posts_created_idx
    on social_publishing.posts (account_id, created_at desc);

-- Projecao corrente, uma linha por post. O payload da Graph nao e persistido:
-- somente metricas normalizadas e o instante de captura.
create table if not exists social_publishing.post_analytics (
    account_id         uuid not null references core.accounts(id) on delete cascade,
    post_id            uuid not null,
    views              bigint not null default 0 check (views >= 0),
    reach              bigint not null default 0 check (reach >= 0),
    likes              bigint not null default 0 check (likes >= 0),
    comments           bigint not null default 0 check (comments >= 0),
    saved              bigint not null default 0 check (saved >= 0),
    shares             bigint not null default 0 check (shares >= 0),
    total_interactions bigint not null default 0 check (total_interactions >= 0),
    captured_at        timestamptz not null,
    updated_at         timestamptz not null default now(),
    primary key (account_id, post_id),
    constraint social_publishing_post_analytics_post_tenant_fk
        foreign key (account_id, post_id)
        references social_publishing.posts(account_id, id)
        on delete cascade
);

create index if not exists social_publishing_post_analytics_captured_idx
    on social_publishing.post_analytics (account_id, captured_at desc);

-- Historico append-only das metricas. A unique de captura torna retry do mesmo
-- refresh idempotente sem impedir snapshots posteriores.
create table if not exists social_publishing.analytics_snapshots (
    id                 uuid primary key default gen_random_uuid(),
    account_id         uuid not null references core.accounts(id) on delete cascade,
    post_id            uuid not null,
    source             text not null default 'instagram'
        check (source = 'instagram'),
    views              bigint not null default 0 check (views >= 0),
    reach              bigint not null default 0 check (reach >= 0),
    likes              bigint not null default 0 check (likes >= 0),
    comments           bigint not null default 0 check (comments >= 0),
    saved              bigint not null default 0 check (saved >= 0),
    shares             bigint not null default 0 check (shares >= 0),
    total_interactions bigint not null default 0 check (total_interactions >= 0),
    captured_at        timestamptz not null default now(),
    constraint social_publishing_analytics_snapshots_post_tenant_fk
        foreign key (account_id, post_id)
        references social_publishing.posts(account_id, id)
        on delete cascade,
    constraint social_publishing_analytics_snapshots_capture_uidx
        unique (account_id, post_id, captured_at)
);

create index if not exists social_publishing_analytics_snapshots_account_captured_idx
    on social_publishing.analytics_snapshots (account_id, captured_at desc);
create index if not exists social_publishing_analytics_snapshots_post_captured_idx
    on social_publishing.analytics_snapshots (account_id, post_id, captured_at desc);

-- Contrato coluna-a-coluna de platform/jobs. A tabela pertence ao modulo; nao
-- reutiliza messaging.outbox nem cria dependencia com o Omnichannel.
create table if not exists social_publishing.outbox (
    id              uuid primary key default gen_random_uuid(),
    account_id      uuid not null references core.accounts(id) on delete cascade,
    ordering_key    text not null,
    idempotency_key text not null,
    kind            text not null,
    payload         jsonb not null default '{}'::jsonb,
    status          text not null default 'pending'
        check (status in ('pending', 'processing', 'done', 'failed', 'dead')),
    attempts        int not null default 0,
    max_attempts    int not null default 3,
    run_after       timestamptz not null default now(),
    locked_at       timestamptz,
    locked_by       text not null default '',
    last_error      text not null default '',
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now()
);

create unique index if not exists social_publishing_outbox_account_idempotency_uidx
    on social_publishing.outbox (account_id, idempotency_key);
create index if not exists social_publishing_outbox_ordering_idx
    on social_publishing.outbox (account_id, ordering_key, created_at, id)
    where status in ('pending', 'processing');
create index if not exists social_publishing_outbox_status_run_after_idx
    on social_publishing.outbox (status, run_after);

-- O backend do Roadmap usa roadmap.modules; fases/grupos/tasks continuam no
-- declarativo estatico do frontend e nao ganham uma segunda fonte sem consumidor.
-- account_id e omitido de proposito: este e um registro global.
insert into roadmap.modules (
    source_id,
    label,
    route,
    status,
    priority,
    category,
    description,
    scope,
    depends_on,
    sort_order
)
select
    'social_publishing',
    'Agendamento de postagens',
    '/postagens',
    'in_progress',
    'P1',
    'operacao-comercial',
    'Workspace isolado para conectar Instagram profissional, agendar publicacoes e consultar analytics. Calendar e Crow Assistant permanecem bloqueados ate homologacao.',
    '[
      "Conexao Instagram cifrada por conta",
      "Rascunho, agendamento e publicacao idempotente",
      "Outbox PostgreSQL com retry e dead letter",
      "Analytics corrente e snapshots historicos"
    ]'::jsonb,
    '[]'::jsonb,
    130
where not exists (
    select 1
    from roadmap.modules existing
    where existing.source_id = 'social_publishing'
      and existing.account_id is null
);

-- Nao ha DROP automatico. Dados de publicacao e analytics sao historicos
-- operacionais; qualquer remocao futura exige migration explicita e autorizada.
