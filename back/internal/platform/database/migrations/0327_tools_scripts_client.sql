-- 0327 — Roteiro sobre um cliente.
-- O roteiro continua pertencendo à conta que o criou (ex.: a conta-agência). client_account_id
-- marca sobre qual conta-cliente da MESMA organização ele fala; client_visible (padrão true)
-- libera a leitura para os usuários dessa conta-cliente. A API só aceita cliente não-agência,
-- ativo e da mesma organização de uma conta-agência; a leitura repete a checagem de organização.
-- Idempotente; não altera dados existentes (roteiros atuais ficam sem cliente).
alter table tools.scripts
    add column if not exists client_account_id uuid references core.accounts(id) on delete set null;
alter table tools.scripts
    add column if not exists client_visible boolean not null default true;
create index if not exists tools_scripts_client_idx
    on tools.scripts (client_account_id)
    where client_account_id is not null;
