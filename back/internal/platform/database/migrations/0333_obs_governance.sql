-- Observabilidade, etapa 2 (governança pelo painel): configurações editáveis e
-- achados como fila de trabalho. Aditiva. Reversão:
-- observability/migrations/0333_obs_governance.down.sql.

-- Configurações da observabilidade editadas no painel (alertas, e-mail, relatório).
-- Variáveis de ambiente continuam como valor inicial quando a chave não existe.
create table if not exists obs.settings (
  key text primary key,
  value jsonb not null default '{}'::jsonb,
  updated_by text not null default '',
  updated_at timestamptz not null default now()
);

-- Achados da auditoria com dono, prazo e nota (fila de trabalho).
alter table obs.findings add column if not exists owner text not null default '';
alter table obs.findings add column if not exists due_date date;
alter table obs.findings add column if not exists note text not null default '';
alter table obs.findings add column if not exists updated_by text not null default '';
