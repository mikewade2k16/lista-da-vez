-- Multi-Tenant Completion (C1.1)
-- Branch: refactor/multi-tenant-complete
-- Plano: docs/MULTITENANT_COMPLETION_PLAN.md secao C1
--
-- Adiciona em core.accounts as colunas reais de billing/contact/webhook que
-- ate hoje so existiam no BFF Nitro mock (web/server/utils/clients-repository.ts).
-- Quando o painel admin real (C10) substituir o mock, ele grava direto aqui.
--
-- Todas as colunas sao IF NOT EXISTS para permitir re-execucao segura.

alter table core.accounts
    add column if not exists billing_mode text not null default 'single',
    add column if not exists monthly_payment_amount numeric(12, 2) not null default 0,
    add column if not exists payment_due_day smallint,
    add column if not exists webhook_enabled boolean not null default false,
    add column if not exists webhook_key text,
    add column if not exists contact_phone text,
    add column if not exists contact_site text,
    add column if not exists contact_address text,
    add column if not exists logo_path text,
    add column if not exists require_user_store_link boolean not null default true,
    add column if not exists require_user_registration boolean not null default true;

-- Constraints idempotentes via DO block (CHECK nao suporta IF NOT EXISTS direto).

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'core_accounts_billing_mode_check'
    ) then
        alter table core.accounts
            add constraint core_accounts_billing_mode_check
            check (billing_mode in ('single', 'per_store'));
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conname = 'core_accounts_payment_due_day_check'
    ) then
        alter table core.accounts
            add constraint core_accounts_payment_due_day_check
            check (payment_due_day is null or (payment_due_day between 1 and 31));
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conname = 'core_accounts_monthly_payment_amount_check'
    ) then
        alter table core.accounts
            add constraint core_accounts_monthly_payment_amount_check
            check (monthly_payment_amount >= 0);
    end if;
end $$;
