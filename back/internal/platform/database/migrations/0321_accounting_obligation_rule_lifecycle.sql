-- CT-15 pacote 7 (P15-16): ciclo autoritativo das regras de obrigação.
-- Versões de regra ficam imutáveis; ativação/aposentadoria é histórico append-only.
-- Nenhuma regra real é inserida ou ativada por esta migration.
create or replace function accounting.reject_obligation_rule_update() returns trigger
language plpgsql as $$
begin
    raise exception 'accounting obligation rule versions are immutable';
end;
$$;

drop trigger if exists obligation_rules_immutable on accounting.obligation_rules;
create trigger obligation_rules_immutable
    before update on accounting.obligation_rules
    for each row execute function accounting.reject_obligation_rule_update();

create table if not exists accounting.obligation_rule_events (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    issuer_id uuid not null,
    rule_id uuid not null,
    action text not null check (action in ('activate', 'retire')),
    actor_id uuid not null references core.users(id),
    reason text not null check (length(reason) between 3 and 500),
    created_at timestamptz not null default clock_timestamp(),
    foreign key (account_id, issuer_id, rule_id)
        references accounting.obligation_rules(account_id, issuer_id, id) on delete cascade
);

create index if not exists accounting_obligation_rule_events_rule_idx
    on accounting.obligation_rule_events(account_id, issuer_id, rule_id, created_at desc, id desc);

drop trigger if exists obligation_rule_events_immutable on accounting.obligation_rule_events;
create trigger obligation_rule_events_immutable
    before update on accounting.obligation_rule_events
    for each row execute function accounting.reject_obligation_rule_update();
