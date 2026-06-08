-- 0136 — Itens 2 & 3: DROP dos objetos legados em public.*
--
-- Remove as 3 views compat e a tabela-base public.tenants, finalizando a
-- migracao de fonte unica para core.*/queue.*:
--   - public.users       (VIEW sobre core.users)
--   - public.stores       (VIEW sobre queue.stores)
--   - public.consultants  (VIEW sobre queue.consultants)
--   - public.tenants      (TABLE; superset vive em core.accounts desde a 0101,
--                          com core.accounts.id == public.tenants.id, 0 drift)
--
-- Caminho ate aqui:
--   - 0101: seed core.accounts a partir de public.tenants (mesmo id).
--   - 0131/0133: backfill users e papeis legados -> core.
--   - Itens 2&3: todo o codigo Go passou a ler/gravar core.users, queue.stores,
--     queue.consultants e core.accounts (zero ref crua a users/stores/
--     consultants/tenants). Bootstrap e tenants admin gravam SO em core.accounts.
--
-- Como os writers agora gravam apenas em core.accounts, manter as ~27 FKs
-- apontando para public.tenants quebraria insercoes de novas accounts (o
-- tenant_id existiria em core.accounts mas nao em public.tenants). Por isso,
-- ANTES de dropar a tabela, repontamos todas as FKs (tenant_id) de
-- public.tenants -> core.accounts(id). Todas sao ON DELETE CASCADE, coluna
-- unica; integridade garantida pelos 0 drift entre as duas.
--
-- Backup pre-drop: C:\tmp\omni_pre_drop_public.dump (pg_dump -Fc).
-- Idempotente (IF EXISTS / checagem do confrelid).

-- 1) Repontar todas as FKs de public.tenants -> core.accounts.
do $$
declare
    fk record;
begin
    -- Se public.tenants ja nao existe (DB recriado pos-drop), nada a fazer.
    if to_regclass('public.tenants') is null then
        return;
    end if;

    for fk in
        select
            con.conrelid::regclass::text as child_table,
            con.conname                  as constraint_name,
            att.attname                  as fk_column
        from pg_constraint con
        join pg_attribute att
            on att.attrelid = con.conrelid
            and att.attnum = con.conkey[1]
        where con.contype = 'f'
            and con.confrelid = 'public.tenants'::regclass
    loop
        execute format(
            'alter table %s drop constraint %I',
            fk.child_table, fk.constraint_name
        );
        execute format(
            'alter table %s add constraint %I foreign key (%I) '
            || 'references core.accounts(id) on delete cascade',
            fk.child_table, fk.constraint_name, fk.fk_column
        );
    end loop;
end $$;

-- 2) Dropar as views compat (folhas: nada FK a uma view).
drop view if exists public.users;
drop view if exists public.stores;
drop view if exists public.consultants;

-- 3) Dropar a tabela legada (sem mais FKs apontando para ela).
drop table if exists public.tenants;
