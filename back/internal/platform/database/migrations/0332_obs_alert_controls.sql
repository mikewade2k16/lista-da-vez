-- Reconhecer e silenciar alertas (observabilidade, task 1.3). Aditiva e idempotente.
--   acknowledged_*: alerta reconhecido por um platform_admin (nao reavisa).
--   silenced_*: nenhum e-mail do alerta ate silenced_until.
-- Os nomes (acknowledged_by/silenced_by) sao o display_name de core.users no momento da acao.
alter table obs.alerts add column if not exists acknowledged_at timestamptz;
alter table obs.alerts add column if not exists acknowledged_by text;
alter table obs.alerts add column if not exists silenced_until timestamptz;
alter table obs.alerts add column if not exists silenced_by text;
