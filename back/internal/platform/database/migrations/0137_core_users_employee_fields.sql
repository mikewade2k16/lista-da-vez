-- 0137 — core.users ganha employee_code/job_title (regressao do view-swap manual).
--
-- POR QUE: as colunas employee_code/job_title existiam so em public.users (criadas
-- pela 0015a para bootstrap/consultores). O codigo Go da Fila passou a ler/gravar
-- core.users (ex.: consultantSelectQuery -> coalesce(u.employee_code,'') em
-- queue/consultants/store_postgres.go). Em ambiente onde core.users foi populado
-- so pelas migrations (0101/0131) sem o script manual unify_users_view.sql, essas
-- 2 colunas nunca chegaram em core.users -> GET /v1/consultants quebrava com 500
-- ("Erro ao processar o consultor") e travava o login no front.
--
-- O script manual back/internal/platform/database/manual/unify_users_view.sql fazia
-- esse ALTER + backfill, mas e' fora de migrations/ (roda deliberado, nao no boot).
-- Esta migration traz o passo ESTRUTURAL (colunas + backfill) para o caminho
-- automatico, que e' o que a VPS roda. ADITIVO e idempotente — nao apaga nada.
--
-- Backfill: copia de public.users SE ela ainda for tabela-base com a coluna (caso
-- da VPS). Se public.users ja for VIEW sobre core.users (manual ja aplicado), o
-- backfill e' pulado porque as colunas ja vivem em core.users.

alter table core.users add column if not exists employee_code text not null default '';
alter table core.users add column if not exists job_title    text not null default '';

do $$
begin
    if exists (
        select 1 from information_schema.tables
        where table_schema = 'public' and table_name = 'users' and table_type = 'BASE TABLE'
    ) and exists (
        select 1 from information_schema.columns
        where table_schema = 'public' and table_name = 'users' and column_name = 'employee_code'
    ) then
        update core.users c
        set employee_code = coalesce(b.employee_code, ''),
            job_title     = coalesce(b.job_title, '')
        from public.users b
        where b.id = c.id;
    end if;
end $$;
