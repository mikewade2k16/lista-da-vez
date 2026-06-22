#!/usr/bin/env bash
# Aplica as migrations do cardápio que faltam na VPS (0168/0169/0170) e registra no
# tracking. Idempotente (if not exists / insert where not exists) — pode rodar de novo.
# Rode da máquina LOCAL no Git Bash:  bash scripts/deploy/apply-cardapio-migrations-prod.sh
#
# Por que: a VPS está na migration 0167. O api NOVO (do deploy) usa `orders.code`
# (0169) → sem a coluna o PEDIDO quebra; 0170 cria `cardapio.site_layouts` (salvar/
# publicar no Studio); 0168 = foto de categoria + preço riscado. O migrator do api
# deveria aplicar no startup, mas na VPS isso não vinha acontecendo — este script
# garante. Ver docs/cardapio/PLANO_CARDAPIO_SITE_BUILDER.md.
set -euo pipefail

KEY="$HOME/.ssh/gh_actions_omnichannel_vps"
SSH="/c/Windows/System32/OpenSSH/ssh.exe -i $KEY -o StrictHostKeyChecking=accept-new -o BatchMode=yes -o ConnectTimeout=30"
TARGET="deploy@85.31.62.33"
REMOTE_PSQL="cd /home/deploy/lista-atendimento && docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres sh -lc 'psql -U \$POSTGRES_USER -d \$POSTGRES_DB --single-transaction -v ON_ERROR_STOP=1 -tA'"

[ -f "$KEY" ] || { echo "ERRO: chave SSH nao encontrada em $KEY"; exit 1; }

echo "==> Aplicando 0168/0169/0170 no prod (atômico, idempotente)"
$SSH "$TARGET" "$REMOTE_PSQL" <<'SQL'
-- 0168 — campos opcionais do catálogo
alter table cardapio.categories add column if not exists image_url text not null default '';
alter table cardapio.products   add column if not exists compare_at_price_cents bigint not null default 0;

-- 0169 — código do pedido (unique parcial: ignora code vazio dos antigos)
alter table cardapio.orders add column if not exists code text not null default '';
create unique index if not exists cardapio_orders_code_uidx on cardapio.orders (restaurant_id, code) where code <> '';

-- 0170 — layout do site (draft/published + version)
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
create unique index if not exists cardapio_site_layouts_restaurant_uidx on cardapio.site_layouts (restaurant_id);
create index if not exists cardapio_site_layouts_account_idx on cardapio.site_layouts (account_id);

-- registra no tracking (version = name = nome do arquivo)
insert into public.schema_migrations (version, name)
select v, v from (values
  ('0168_cardapio_catalog_optional_fields.sql'),
  ('0169_cardapio_order_code.sql'),
  ('0170_cardapio_site_layouts.sql')
) as t(v)
where not exists (select 1 from public.schema_migrations m where m.version = t.v);

\echo == VERIFICAÇÃO ==
select 'site_layouts=' || coalesce(to_regclass('cardapio.site_layouts')::text, 'AUSENTE');
select 'orders.code=' || count(*) from information_schema.columns where table_schema='cardapio' and table_name='orders' and column_name='code';
select 'categories.image_url=' || count(*) from information_schema.columns where table_schema='cardapio' and table_name='categories' and column_name='image_url';
select 'migrations: ' || string_agg(version, ', ' order by version) from public.schema_migrations where version like '0168%' or version like '0169%' or version like '0170%';
SQL

echo "==> OK. Se as 3 verificações acima mostrarem site_layouts=cardapio.site_layouts, orders.code=1, categories.image_url=1 e as 3 migrations, está pronto."
