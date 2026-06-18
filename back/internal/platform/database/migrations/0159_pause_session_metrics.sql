-- 0159: metricas de pausa nas sessoes de status
--
-- A duracao das pausas ja era gravada em queue.operation_status_sessions
-- (status='paused', started_at/ended_at/duration_ms). Faltavam o MOTIVO e o
-- TIPO da pausa (pause/assignment), que viviam so em operation_paused_consultants
-- e eram apagados no resume. Aqui adicionamos reason/kind (nullable) para
-- metrificar pausas por consultor/loja/dia.
--
-- Idempotente e schema-qualificada. SQL plano (sem blocos goose Down).

alter table queue.operation_status_sessions
    add column if not exists reason text;

alter table queue.operation_status_sessions
    add column if not exists kind text;

-- Recria a view publica para expor as colunas novas: uma view "select *" congela
-- a lista de colunas no momento da criacao, entao precisa ser recriada para
-- incluir reason/kind. CREATE OR REPLACE aceita adicionar colunas ao final.
create or replace view public.operation_status_sessions as
    select * from queue.operation_status_sessions;
