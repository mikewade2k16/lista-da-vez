-- Modulo Bio (link-in-bio) — schema `bio`
-- Plano: docs/bio/PLANO_MODULO_BIO.md secao 3
--
-- Gere as paginas de bio servidas pelo front Nuxt separado. O painel Omni e o
-- CRUD; o front bio consome GET /v1/public/bio/{slug} server-to-server.
-- Idempotente, schema qualificado, sem `-- +goose Down` (o migrator roda o
-- arquivo inteiro).

create schema if not exists bio;

create table if not exists bio.bios (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    slug text not null,
    name text not null,                       -- nome interno no painel
    status text not null default 'draft',     -- draft | published
    data_draft jsonb not null default '{}'::jsonb,
    data_published jsonb,
    published_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
create unique index if not exists bio_bios_slug_uidx on bio.bios (lower(slug));
create index if not exists bio_bios_account_idx on bio.bios (account_id);

create table if not exists bio.defaults (
    id text primary key default 'global',
    data jsonb not null default '{}'::jsonb,
    updated_at timestamptz not null default now()
);
insert into bio.defaults (id, data) values ('global', '{}'::jsonb)
on conflict (id) do nothing;

create table if not exists bio.media (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    bio_id uuid references bio.bios(id) on delete set null,
    kind text not null,                       -- video | poster | logo | favicon | slide | store
    path text not null,                       -- /uploads/bio/{account_id}/{arquivo}
    mime text,
    size_bytes bigint,
    created_at timestamptz not null default now()
);
create index if not exists bio_media_account_idx on bio.media (account_id);
