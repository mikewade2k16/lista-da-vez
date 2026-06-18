-- Comissao por atingimento de meta v3 (modelo 2D: gate da loja x faixa propria)
-- Plano: C:/Users/Mike/.claude/plans/vamos-fazer-altera-es-em-purrfect-pony.md
--
-- Mudancas em relacao a v2:
--  - consultant: as faixas passam a ser keyed pela PROPRIA meta do consultor
--    ({80:1, 90:2, 100:3, 120:3.2}) e valem quando a loja >= storeFullPercent.
--  - consultantRules ganha o gate da loja (storeFloorPercent/storeFullPercent/
--    reducedRate/reducedRequiresOwnPercent) e perde o legado minOwnGoalPercent.
--
-- SQL plano, idempotente, schema-qualificado, SEM marcadores goose.

-- 1) Novo DEFAULT v3 (linhas novas).
alter table queue.tenant_operation_core_settings
    alter column crm_goal_payout_policy set default '{
        "consultant":[
            {"threshold":80,"value":1,"mode":"percent"},
            {"threshold":90,"value":2,"mode":"percent"},
            {"threshold":100,"value":3,"mode":"percent"},
            {"threshold":120,"value":3.2,"mode":"percent"}
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
        "consultantRules":{"base":"self","qualityPenaltyPercent":0.1,"storeFloorPercent":50,"storeFullPercent":80,"reducedRate":1.5,"reducedRequiresOwnPercent":100}
    }'::jsonb;

-- 2) Corrige o array consultant que ficou no default v2 ([{50,1.5}]) -> faixas v3
--    keyed pela meta propria. Linhas pre-v2 ja tem {80,90,100,120} (correto p/ v3).
update queue.tenant_operation_core_settings
set crm_goal_payout_policy = jsonb_set(
        crm_goal_payout_policy,
        '{consultant}',
        '[
            {"threshold":80,"value":1,"mode":"percent"},
            {"threshold":90,"value":2,"mode":"percent"},
            {"threshold":100,"value":3,"mode":"percent"},
            {"threshold":120,"value":3.2,"mode":"percent"}
        ]'::jsonb,
        true
    )
where crm_goal_payout_policy is not null
  and crm_goal_payout_policy -> 'consultant'
      = '[{"threshold":50,"value":1.5,"mode":"percent"}]'::jsonb;

-- 3) consultantRules: remove o legado minOwnGoalPercent e garante os campos do
--    gate da loja (preserva base/qualityPenaltyPercent ja configurados). Idempotente.
update queue.tenant_operation_core_settings
set crm_goal_payout_policy = jsonb_set(
        crm_goal_payout_policy,
        '{consultantRules}',
        (coalesce(crm_goal_payout_policy -> 'consultantRules', '{}'::jsonb) - 'minOwnGoalPercent')
        || jsonb_build_object(
            'base', coalesce(crm_goal_payout_policy #>> '{consultantRules,base}', 'self'),
            'qualityPenaltyPercent', coalesce((crm_goal_payout_policy #>> '{consultantRules,qualityPenaltyPercent}')::numeric, 0.1),
            'storeFloorPercent', coalesce((crm_goal_payout_policy #>> '{consultantRules,storeFloorPercent}')::numeric, 50),
            'storeFullPercent', coalesce((crm_goal_payout_policy #>> '{consultantRules,storeFullPercent}')::numeric, 80),
            'reducedRate', coalesce((crm_goal_payout_policy #>> '{consultantRules,reducedRate}')::numeric, 1.5),
            'reducedRequiresOwnPercent', coalesce((crm_goal_payout_policy #>> '{consultantRules,reducedRequiresOwnPercent}')::numeric, 100)
        ),
        true
    )
where crm_goal_payout_policy is not null;
