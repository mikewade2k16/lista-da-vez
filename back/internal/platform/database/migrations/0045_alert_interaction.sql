-- Adiciona campos de interacao e notificacao externa nos alertas
-- interaction_kind: none (passivo) | reminder (banner sem resposta) | response_required (exige resposta do consultor)
-- interaction_response: still_happening | forgotten (preenchido quando consultor responde)

alter table alert_instances
    add column if not exists interaction_kind varchar(30) not null default 'none',
    add column if not exists interaction_response varchar(30),
    add column if not exists responded_at timestamptz,
    add column if not exists external_notified_at timestamptz;

-- Retroativamente marcar long_open_service existentes como response_required
update alert_instances
set interaction_kind = 'response_required'
where type = 'long_open_service';

-- Rollback:
-- alter table alert_instances
--     drop column if exists interaction_kind,
--     drop column if exists interaction_response,
--     drop column if exists responded_at,
--     drop column if exists external_notified_at;
