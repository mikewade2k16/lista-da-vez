# Organização global do menu (Header × Sidebar)

> Doc canônico da feature. Espelhado em `web/app/components/roadmap/roadmap-data.ts`
> (fase `menu-layout`, code `MENU`) e em `back/internal/modules/core/AGENT.md`.
> Criado 2026-06-16.

## Problema

Header e sidebar renderizam **os mesmos itens**: `useDashboardNav.ts` calcula
`visibleSections` (sidebar) e o header é só um `flatMap` disso (`headerItems`). Não há
conceito de "isto vai só no header" / "só no sidebar". Para `platform_admin` (que vê todos
os workspaces + `platformView` revela tudo) o header estoura — não responsivo, bagunçado.

## Decisões de produto

1. **Escopo: global da plataforma** — o `platform_admin` define UMA organização do menu que
   vale para todos os usuários; cada um ainda é filtrado pelas próprias permissões/módulos.
   Não é per-user nem per-tenant.
2. **Controle por item:** posição (`header` / `sidebar` / `both` / `hidden`) + **reordenar**
   itens e seções (drag-and-drop).
3. **Header responsivo** está no escopo: excedente colapsa num popover "Mais".
4. **Sem surpresa:** sem layout salvo, tudo é `both` (= comportamento atual). O editor oferece
   "Sugerir layout enxuto" que pré-preenche um split curado para o admin revisar e salvar.

## Modelo de dados / contrato (congelado)

Persistência: tabela **platform-global** singleton `core.platform_settings` (KV jsonb).
Chave do menu: `menu_layout`.

```
GET   /v1/platform/menu-layout    → RequireAuth (TODOS os usuários leem p/ renderizar o menu)
PATCH /v1/platform/menu-layout    → RequireAuth + requirePlatformAdmin (só admin escreve)
```

Request do PATCH = `{ "layout": <Layout> }`. Response do GET e do PATCH:
`{ "layout": <Layout>, "updatedAt": "<RFC3339|null>", "updatedBy": "<userId|null>" }`.

```jsonc
// <Layout>
{
  "version": 1,
  "sections": [ { "id": "service", "order": 0 }, { "id": "manage", "order": 1 } ],
  "items": {
    // chave = id estável do nó de nav.config.ts (inclui filhos aninhados)
    "fila":       { "placement": "header",  "order": 0 },
    "tasks":      { "placement": "both",    "order": 1 },
    "tools-menu": { "placement": "sidebar", "order": 0 },
    "banco":      { "placement": "hidden",  "order": 9 }
  }
}
```

- `placement ∈ { "header", "sidebar", "both", "hidden" }`.
- Item **ausente** do mapa → default `placement: "both"`, ordem = ordem declarada em
  `nav.config.ts`. Nada muda até o admin salvar.
- `placement: "hidden"` = ocultado pelo admin. É **distinto** do flag `hidden: true` do dev em
  `nav.config.ts` (esse esconde de forma incondicional e continua valendo).

## Backend (módulo `core`)

- Migration `0160_core_platform_settings.sql` — `core.platform_settings (key text pk, config
  jsonb not null default '{}', updated_at timestamptz, updated_by uuid → core.users)`.
  Tabela **platform-global** (exceção consciente à regra de `account_id` por ser config de
  plataforma, não de tenant). Idempotente, SQL plano (ver memória `project-migration-goose-bug`).
- `platform_settings_{model,repository,service,http}.go` — repo `GetByKey`/`Upsert`; service
  valida placement ∈ enum; rotas em `RegisterPlatformSettingsRoutes` (GET sem guard de admin,
  PATCH com `requirePlatformAdmin` de `admin_http.go`). Wire no `module.go`.

## Frontend

- Store `web/app/stores/menuLayout.ts` — `load()` (GET, 1x após auth, disparado no
  `layouts/dashboard.vue`), `save()` (PATCH otimista + toast), `placementOf/orderOf/sectionOrder`.
- `useDashboardNav.ts` — `visibleSections` (sidebar) mantém placement ∈ {sidebar, both},
  ordenado; `headerItems` mantém placement ∈ {header, both}, ordenado; `hidden` some dos dois.
- Tela `/manage/menu-layout` (`pages/manage/menu-layout.vue`) — editor com placement + drag
  (HTML5 nativo, reusado de `OmniTableColumnsConfig.vue`) + preview. Wiring de 3 arquivos
  (`workspaces.ts`, `permissions.ts` → só `platform_admin`, `nav.config.ts` seção `manage`).
  Rota fica em `/manage/*` (não `/configuracoes/*`, que é gated por `queue` no
  `module-enabled.global.ts` e cuja `configuracoes.vue` viraria rota-pai e engoliria a filha).
- `DashboardHeader.vue` — overflow "Mais" via `ResizeObserver`.

## Notas de Deploy

1. Aplicar migration `0160_core_platform_settings.sql`.
2. `docker compose up -d --build api` (mudança em `back/`).
3. Sem env var nova. Sem mudança em `docker-compose.prod.yml`.
