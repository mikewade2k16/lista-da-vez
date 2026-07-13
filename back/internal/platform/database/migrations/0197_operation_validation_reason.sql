-- 0197: justificativa do encerramento de pendencia (auto-encerramento 2h).
--
-- Quando a gestao encerra um atendimento auto-encerrado (pendencia), registra
-- a JUSTIFICATIVA de por que o consultor nao encerrou na hora. Alimenta as
-- metricas de cobranca (quantos/quais/porques por consultor/gerente/loja).
--
-- SQL plano idempotente, sem goose Down. public.* e VIEW (select *): recriar.

alter table queue.operation_service_history
    add column if not exists validation_reason text not null default '';

create or replace view public.operation_service_history as
    select * from queue.operation_service_history;
