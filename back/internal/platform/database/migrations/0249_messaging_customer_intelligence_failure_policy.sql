-- 0249_messaging_customer_intelligence_failure_policy.sql
--
-- Política operacional, administrável pelo painel, para falhas técnicas do
-- Customer Intelligence. O default preserva o atendimento: tenta novamente de
-- forma limitada pelo jobs engine e, ao esgotar, transfere para humano.
-- Prompts e modelos não podem alterar este gate.

alter table messaging.account_config
    add column if not exists customer_intelligence_failure_policy text
        not null
        default 'retry_then_handoff';

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'messaging_account_config_ci_failure_policy_ck'
          and conrelid = 'messaging.account_config'::regclass
    ) then
        alter table messaging.account_config
            add constraint messaging_account_config_ci_failure_policy_ck
            check (
                customer_intelligence_failure_policy in (
                    'legacy_fallback',
                    'retry_then_handoff',
                    'immediate_handoff'
                )
            );
    end if;
end $$;
