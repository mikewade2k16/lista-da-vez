-- Modulo Tools — encurtador de link + gerador de QR Code.
-- Reconstroi como back Go real as duas ferramentas que no projeto antigo eram
-- mock (globalThis no BFF Nitro eliminado). Plano: docs/tools/PLANO_MODULO_TOOLS.md.
--
-- Multi-tenant dia 1: account_id uuid not null (conta dona, escolhida no modal).
-- slug e UNIQUE por tabela: o redirect publico (/s/{slug}, /q/{slug}) resolve sem
-- X-Account-Id, entao o slug precisa ser unico globalmente na tabela (colisao no
-- create vira sufixo -2/-3 no service). /s e /q sao tabelas distintas: o mesmo
-- slug pode existir nas duas.
--
-- Idempotente, schema qualificado, SEM `-- +goose Down` (o migrator roda o arquivo
-- inteiro; um Down aqui se auto-destruiria). Ver 0187_finance_module.sql.

create schema if not exists tools;

create table if not exists tools.short_links (
    id         uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    slug       text not null,
    target_url text not null,
    hits       bigint not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
create unique index if not exists tools_short_links_slug_key on tools.short_links (slug);
create index if not exists tools_short_links_account_created_idx
    on tools.short_links (account_id, created_at desc);

create table if not exists tools.qr_codes (
    id              uuid primary key default gen_random_uuid(),
    account_id      uuid not null references core.accounts(id) on delete cascade,
    slug            text not null,
    target_url      text not null,
    fill_color      text not null default '#000000',
    back_color      text not null default '#ffffff',
    size            int not null default 220,
    is_active       boolean not null default true,
    scan_count      bigint not null default 0,
    last_scanned_at timestamptz,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now()
);
create unique index if not exists tools_qr_codes_slug_key on tools.qr_codes (slug);
create index if not exists tools_qr_codes_account_created_idx
    on tools.qr_codes (account_id, created_at desc);
