-- Modulo cardapio (Fase 3 / site builder, Opcao B): layout de secoes do site por
-- restaurante, com rascunho (draft) e publicado (published) separados + version
-- para concorrencia otimista (ETag/If-Match). O GET publico serve so o published;
-- o painel grava o draft e promove no publish. Idempotente, schema-qualificado,
-- sem -- +goose Down. Plano: docs/cardapio/PLANO_CARDAPIO_SITE_BUILDER.md (Fase 1).

create table if not exists cardapio.site_layouts (
    id            uuid        primary key default gen_random_uuid(),
    account_id    uuid        not null references core.accounts(id) on delete cascade,
    restaurant_id uuid        not null references cardapio.restaurants(id) on delete cascade,
    draft         jsonb       not null default '{}'::jsonb,
    published     jsonb       not null default '{}'::jsonb,
    version       bigint      not null default 0,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);
create unique index if not exists cardapio_site_layouts_restaurant_uidx
    on cardapio.site_layouts (restaurant_id);
create index if not exists cardapio_site_layouts_account_idx
    on cardapio.site_layouts (account_id);
