-- Omnichannel E5: snapshot imutavel da policy que determinou o handoff.
-- A policy continua editavel para novos atendimentos; o handoff preserva a
-- referencia e o conteudo avaliado no momento da decisao.

alter table messaging.handoffs
    add column if not exists policy_id uuid,
    add column if not exists policy_snapshot jsonb not null default '{}'::jsonb;

do $$
begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_handoffs_policy_fk'
          and conrelid = 'messaging.handoffs'::regclass
    ) then
        alter table messaging.handoffs
            add constraint messaging_handoffs_policy_fk
            foreign key (policy_id)
            references messaging.handoff_policies(id)
            on delete set null;
    end if;
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_handoffs_policy_snapshot_object_ck'
          and conrelid = 'messaging.handoffs'::regclass
    ) then
        alter table messaging.handoffs
            add constraint messaging_handoffs_policy_snapshot_object_ck
            check (jsonb_typeof(policy_snapshot) = 'object');
    end if;
end $$;

create index if not exists messaging_handoffs_policy_idx
    on messaging.handoffs (account_id, policy_id, created_at desc)
    where policy_id is not null;
