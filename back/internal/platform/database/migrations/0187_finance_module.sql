-- Modulo Finance — planilhas financeiras mensais por cliente (AC-12).
-- Substitui o mock BFF Nitro (web/server/*) por back Go real (ADR 0002).
-- Plano: docs/finance/PLANO_MODULO_FINANCE.md.
--
-- Multi-tenant dia 1: account_id uuid not null em TODAS as tabelas raiz
-- (sheets/categories/fixed_accounts/recurring_entries/config_state). coreTenantId
-- e um filtro DENTRO da account (cliente escolhido no painel; '' = escopo padrao).
-- Idempotente, schema qualificado, SEM `-- +goose Down` (o migrator roda o arquivo
-- inteiro; um Down aqui se auto-destruiria).
--
-- fixed_account_id/category_id sao TEXT (nao FK): o front usa ids determinísticos
-- de recorrencia (finance-ids.ts) e o mock trata como snapshot textual. SEM unique
-- de lower(name) em categories e SEM unique em recurring_entries: PUT de config e
-- full-replace e o autosave nao valida duplicata.

create schema if not exists finance;

create table if not exists finance.sheets (
    id             uuid primary key default gen_random_uuid(),
    account_id     uuid not null references core.accounts(id) on delete cascade,
    core_tenant_id text not null default '',
    title          text not null default '',
    period         text not null default '',
    status         text not null default 'aberta',
    notes          text not null default '',
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now()
);
create index if not exists finance_sheets_account_scope_idx
    on finance.sheets (account_id, core_tenant_id, period);
create index if not exists finance_sheets_account_updated_idx
    on finance.sheets (account_id, updated_at desc);

create table if not exists finance.lines (
    id                uuid primary key default gen_random_uuid(),
    sheet_id          uuid not null references finance.sheets(id) on delete cascade,
    kind              text not null check (kind in ('entrada','saida')),
    description       text not null default '',
    category          text not null default '',
    effective         boolean not null default false,
    effective_date    date,
    amount            numeric(14,2) not null default 0,
    adjustment_amount numeric(14,2) not null default 0,
    fixed_account_id  text not null default '',
    details           text not null default '',
    position          int not null default 0
);
create index if not exists finance_lines_sheet_idx on finance.lines (sheet_id, kind, position);

create table if not exists finance.line_adjustments (
    id       uuid primary key default gen_random_uuid(),
    line_id  uuid not null references finance.lines(id) on delete cascade,
    amount   numeric(14,2) not null default 0,
    note     text not null default '',
    date     date,
    position int not null default 0
);
create index if not exists finance_line_adjustments_line_idx
    on finance.line_adjustments (line_id, position);

create table if not exists finance.categories (
    id             uuid primary key default gen_random_uuid(),
    account_id     uuid not null references core.accounts(id) on delete cascade,
    core_tenant_id text not null default '',
    name           text not null default '',
    kind           text not null default 'ambas' check (kind in ('entrada','saida','ambas')),
    description    text not null default '',
    position       int not null default 0
);
create index if not exists finance_categories_scope_idx
    on finance.categories (account_id, core_tenant_id);

create table if not exists finance.fixed_accounts (
    id             uuid primary key default gen_random_uuid(),
    account_id     uuid not null references core.accounts(id) on delete cascade,
    core_tenant_id text not null default '',
    name           text not null default '',
    kind           text not null default 'ambas' check (kind in ('entrada','saida','ambas')),
    category_id    text not null default '',
    default_amount numeric(14,2) not null default 0,
    notes          text not null default '',
    position       int not null default 0
);
create index if not exists finance_fixed_accounts_scope_idx
    on finance.fixed_accounts (account_id, core_tenant_id);

create table if not exists finance.fixed_account_members (
    id               uuid primary key default gen_random_uuid(),
    fixed_account_id uuid not null references finance.fixed_accounts(id) on delete cascade,
    name             text not null default '',
    amount           numeric(14,2) not null default 0,
    position         int not null default 0
);
create index if not exists finance_fixed_account_members_idx
    on finance.fixed_account_members (fixed_account_id, position);

create table if not exists finance.recurring_entries (
    id                     uuid primary key default gen_random_uuid(),
    account_id             uuid not null references core.accounts(id) on delete cascade,
    core_tenant_id         text not null default '',
    source_core_tenant_id  text not null default '',
    adjustment_amount      numeric(14,2) not null default 0,
    notes                  text not null default ''
);
create index if not exists finance_recurring_entries_scope_idx
    on finance.recurring_entries (account_id, core_tenant_id);

create table if not exists finance.config_state (
    account_id     uuid not null references core.accounts(id) on delete cascade,
    core_tenant_id text not null default '',
    updated_at     timestamptz not null default now(),
    primary key (account_id, core_tenant_id)
);
