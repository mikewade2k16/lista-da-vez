-- Comissao por atingimento de meta v2 (Trilha A)
-- Plano: C:/Users/Mike/.claude/plans/vamos-fazer-altera-es-em-purrfect-pony.md
--
-- 1) Atualiza o DEFAULT da coluna crm_goal_payout_policy para o shape v2.
-- 2) Backfill IDEMPOTENTE das linhas existentes: faz merge JSONB SO onde a chave
--    falta — cria managerShopping/managerBairro a partir de "manager" (legado)
--    quando ausentes; adiciona consultantRules quando ausente. Nao toca em
--    chaves que ja existem (preserva a config do usuario, inclusive faixas vazias).
--
-- SQL plano, idempotente, schema-qualificado, SEM marcadores goose.
-- A tabela vive em queue.*; public.* e uma view de compat.

alter table queue.tenant_operation_core_settings
    alter column crm_goal_payout_policy set default '{
        "consultant":[
            {"threshold":50,"value":1.5,"mode":"percent"}
        ],
        "managerShopping":[
            {"threshold":80,"value":0.8,"mode":"percent"},
            {"threshold":90,"value":0.9,"mode":"percent"},
            {"threshold":100,"value":1,"mode":"percent"},
            {"threshold":120,"value":1.2,"mode":"percent"}
        ],
        "managerBairro":[
            {"threshold":80,"value":1,"mode":"percent"},
            {"threshold":100,"value":1.7,"mode":"percent"},
            {"threshold":120,"value":2,"mode":"percent"}
        ],
        "support":[
            {"threshold":80,"value":80,"mode":"amount"},
            {"threshold":90,"value":90,"mode":"amount"},
            {"threshold":100,"value":100,"mode":"amount"},
            {"threshold":120,"value":120,"mode":"amount"}
        ],
        "consultantRules":{"base":"self","minOwnGoalPercent":100,"qualityPenaltyPercent":0.1}
    }'::jsonb;

-- managerShopping ausente: semeia a partir de "manager" (legado) se existir,
-- senao do default v2.
update queue.tenant_operation_core_settings
set crm_goal_payout_policy = jsonb_set(
        crm_goal_payout_policy,
        '{managerShopping}',
        coalesce(
            crm_goal_payout_policy -> 'manager',
            '[
                {"threshold":80,"value":0.8,"mode":"percent"},
                {"threshold":90,"value":0.9,"mode":"percent"},
                {"threshold":100,"value":1,"mode":"percent"},
                {"threshold":120,"value":1.2,"mode":"percent"}
            ]'::jsonb
        ),
        true
    )
where crm_goal_payout_policy is not null
  and not (crm_goal_payout_policy ? 'managerShopping');

-- managerBairro ausente: mesma logica.
update queue.tenant_operation_core_settings
set crm_goal_payout_policy = jsonb_set(
        crm_goal_payout_policy,
        '{managerBairro}',
        coalesce(
            crm_goal_payout_policy -> 'manager',
            '[
                {"threshold":80,"value":1,"mode":"percent"},
                {"threshold":100,"value":1.7,"mode":"percent"},
                {"threshold":120,"value":2,"mode":"percent"}
            ]'::jsonb
        ),
        true
    )
where crm_goal_payout_policy is not null
  and not (crm_goal_payout_policy ? 'managerBairro');

-- consultantRules ausente: adiciona o default.
update queue.tenant_operation_core_settings
set crm_goal_payout_policy = jsonb_set(
        crm_goal_payout_policy,
        '{consultantRules}',
        '{"base":"self","minOwnGoalPercent":100,"qualityPenaltyPercent":0.1}'::jsonb,
        true
    )
where crm_goal_payout_policy is not null
  and not (crm_goal_payout_policy ? 'consultantRules');

-- Recria a view de compat (idempotente).
create or replace view public.tenant_operation_core_settings as
    select * from queue.tenant_operation_core_settings;
