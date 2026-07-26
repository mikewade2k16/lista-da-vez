-- 0238_social_publishing_reliability.sql
--
-- Evolucao aditiva da fundacao de social publishing criada na 0237. Mantem a
-- 0237 imutavel para que bancos que ja registraram aquela versao recebam este
-- delta normalmente pelo migrator.
--
-- O migrator executa este arquivo inteiro em uma unica transacao. Assim, o split
-- das lanes e a transferencia dos jobs de analytics sao publicados juntos.
-- Todos os comandos permanecem idempotentes para permitir reexecucao manual
-- segura depois de uma aplicacao concluida.

-- Gravado imediatamente antes de media_publish. Se o efeito externo ficar
-- ambiguo, o worker reconhece a tentativa e nao publica o mesmo post novamente.
alter table social_publishing.posts
    add column if not exists publish_attempted_at timestamptz;

-- O ID imutavel do job passa a ser a chave de dedupe do codigo novo. Snapshots
-- anteriores recebem uma chave deterministica por linha antes do NOT NULL. O
-- default com UUID preserva inserts do binario anterior, que omite job_key.
alter table social_publishing.analytics_snapshots
    add column if not exists job_key text;

update social_publishing.analytics_snapshots
set job_key = 'legacy:' || id::text
where job_key is null or btrim(job_key) = '';

alter table social_publishing.analytics_snapshots
    alter column job_key set default ('legacy:' || gen_random_uuid()::text);

alter table social_publishing.analytics_snapshots
    alter column job_key set not null;

create unique index if not exists social_publishing_analytics_snapshots_job_uidx
    on social_publishing.analytics_snapshots (account_id, post_id, job_key);

-- O binario de rollback usa ON CONFLICT neste alvo. Na instalacao fresh, a
-- constraint da 0237 ja possui este indice; o comando tambem restaura ambientes
-- piloto que tenham executado uma iteracao anterior da 0238 que o removeu.
create unique index if not exists social_publishing_analytics_snapshots_capture_uidx
    on social_publishing.analytics_snapshots (account_id, post_id, captured_at);

-- A lane de publicacao preserva o nome legado para que rollback do binario nao
-- dependa de rollback destrutivo de schema. Reafirmar os indices torna a
-- migration segura mesmo se algum ambiente piloto os removeu manualmente.
create unique index if not exists social_publishing_outbox_account_idempotency_uidx
    on social_publishing.outbox (account_id, idempotency_key);
create index if not exists social_publishing_outbox_ordering_idx
    on social_publishing.outbox (account_id, ordering_key, created_at, id)
    where status in ('pending', 'processing');
create index if not exists social_publishing_outbox_status_run_after_idx
    on social_publishing.outbox (status, run_after);

-- Analytics usa uma lane fisicamente separada, mas repete exatamente o contrato
-- coluna-a-coluna exigido por platform/jobs.
create table if not exists social_publishing.analytics_outbox (
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

create unique index if not exists social_publishing_analytics_outbox_account_idempotency_uidx
    on social_publishing.analytics_outbox (account_id, idempotency_key);
create index if not exists social_publishing_analytics_outbox_ordering_idx
    on social_publishing.analytics_outbox (account_id, ordering_key, created_at, id)
    where status in ('pending', 'processing');
create index if not exists social_publishing_analytics_outbox_status_run_after_idx
    on social_publishing.analytics_outbox (status, run_after);

-- Preserva ID, estado, tentativas, lease e timestamps. O DELETE so remove um job
-- da lane antiga quando a mesma chave idempotente ja existe na lane de analytics.
-- Como o arquivo inteiro roda em uma transacao, nao ha janela com job perdido.
insert into social_publishing.analytics_outbox (
    id,
    account_id,
    ordering_key,
    idempotency_key,
    kind,
    payload,
    status,
    attempts,
    max_attempts,
    run_after,
    locked_at,
    locked_by,
    last_error,
    created_at,
    updated_at
)
select
    id,
    account_id,
    ordering_key,
    idempotency_key,
    kind,
    payload,
    status,
    attempts,
    max_attempts,
    run_after,
    locked_at,
    locked_by,
    last_error,
    created_at,
    updated_at
from social_publishing.outbox
where kind = 'social.analytics.refresh'
on conflict (account_id, idempotency_key) do nothing;

delete from social_publishing.outbox publish_job
where publish_job.kind = 'social.analytics.refresh'
  and exists (
      select 1
      from social_publishing.analytics_outbox analytics_job
      where analytics_job.account_id = publish_job.account_id
        and analytics_job.idempotency_key = publish_job.idempotency_key
  );
