# Módulo Tools — Encurtador de Link + Gerador de QR Code

Fonte de verdade do módulo `tools`. Espelhado no painel (`roadmap-data.ts`, fase `tools-module`)
e no `back/internal/modules/tools/AGENT.md`. Ao concluir um item, sincronizar os três.

## Contexto e decisão

O projeto antigo (`web-reference/`) tinha um "Encurtador de Link" e um "Gerador de QR Code" em
`app/pages/admin/tools/`, mas **eram mocks** — estado em memória (`globalThis`) servido pelo BFF
Nitro (`web/server/*`), que foi **eliminado** (ver [[project_bff_nitro_temp]]). Não havia banco.

Este módulo reconstrói as duas ferramentas **de verdade**: back Go real + Postgres, isolado por
`account_id`, consumindo o design system que já existe no painel (`OmniDataTable`,
`OmniCollectionFilters`, `AdminPageHeader`, tokens `rgb(var(--…))`).

Decisões do dono (2026-07-09):

1. **QR rastreado.** O QR não codifica a URL final; codifica um redirect do backend
   `GET /q/{slug}` que faz 302 para o destino, incrementa `scan_count` e respeita `is_active`.
   Resultado: o contador de scans e o toggle "Ativo" funcionam de verdade (desativar o QR o
   derruba). Sem campo cosmético.
2. **Dono escolhido no modal (cross-conta para admin).** O modal tem dropdown de conta.
   `platform_admin` cria/vê links de qualquer conta (tabela cross-conta, coluna Cliente visível só
   para admin). Usuário comum fica travado na conta ativa. O `accountId` do body é **validado
   contra o Principal** (não confiado do client) — ver Isolamento.

## Tabelas (schema `tools`, migration 0197)

Idempotente, schema qualificado, sem `-- +goose Down` (ver [[project_migration_goose_bug]]).

### `tools.short_links`
| coluna | tipo | nota |
|---|---|---|
| id | uuid pk `gen_random_uuid()` | |
| account_id | uuid not null → core.accounts(id) on delete cascade | conta dona (escolhida no modal) |
| slug | text not null **unique** | resolvido no redirect sem contexto de conta → único global |
| target_url | text not null | destino do 302 |
| hits | bigint not null default 0 | cliques (`/s/{slug}`) |
| created_at / updated_at | timestamptz | |

### `tools.qr_codes`
| coluna | tipo | nota |
|---|---|---|
| id | uuid pk `gen_random_uuid()` | |
| account_id | uuid not null → core.accounts(id) on delete cascade | conta dona |
| slug | text not null **unique** | resolvido no redirect `/q/{slug}` → único global |
| target_url | text not null | destino final do 302 |
| fill_color | text not null default '#000000' | cor dos módulos do QR (customização) |
| back_color | text not null default '#ffffff' | cor de fundo |
| size | int not null default 220 | px (120–1000) |
| is_active | boolean not null default true | desativado → `/q/{slug}` responde 404 |
| scan_count | bigint not null default 0 | scans (`/q/{slug}`) |
| last_scanned_at | timestamptz null | último scan |
| created_at / updated_at | timestamptz | |

Índices: `unique(slug)` em cada tabela + `(account_id, created_at desc)`.

Slug único global por tabela: como o redirect não tem X-Account-Id, o slug precisa ser único na
tabela. Colisão no create → o service adiciona sufixo `-2`, `-3`, … (igual ao mock antigo).
`/s` e `/q` são namespaces separados (tabelas diferentes), então o mesmo slug pode existir nos dois.

## Contrato de API

Envelope padrão do projeto: `{ status: "success", data, meta? }`. `ReadJSON` usa
`DisallowUnknownFields` — o front manda **exatamente** os campos abaixo (sem extras).

### Painel (autenticado, gateado por módulo `tools`)
- `GET  /v1/tools/short-links?q=&page=&limit=` → `{ data: ShortLinkItem[], meta }`
- `POST /v1/tools/short-links` body `{ targetUrl, slug?, accountId? }` → `{ data: ShortLinkItem }`
- `PATCH /v1/tools/short-links/{id}` body parcial `{ slug?, targetUrl? }` (todos opcionais; `nil` = não
  mexe) → `{ data: ShortLinkItem }`. Alimenta a **edição inline** da tabela. Trocar `slug` mantém o
  unique global (mesmo loop de sufixo do create) e muda o `shortUrl` — links já divulgados param.
- `DELETE /v1/tools/short-links/{id}` → `{ status:"success" }`
- `GET  /v1/tools/qr-codes?q=&status=&page=&limit=` → `{ data: QrCodeItem[], meta }`
- `POST /v1/tools/qr-codes` body `{ targetUrl, slug?, fillColor?, backColor?, size?, isActive?, accountId? }` → `{ data: QrCodeItem }`
- `PATCH /v1/tools/qr-codes/{id}` body parcial (mesmos campos, todos opcionais) → `{ data: QrCodeItem }`
- `DELETE /v1/tools/qr-codes/{id}` → `{ status:"success" }`

### Público (sem auth, fora do gating — prefixos `/s` e `/q` não estão em `moduleGatingRules`)
- `GET /s/{slug}` → 302 `target_url`; `hits++`. 404 se não achar / conta inativa / módulo off.
- `GET /q/{slug}` → 302 `target_url`; `scan_count++`, `last_scanned_at=now()`. 404 se não achar /
  `is_active=false` / conta inativa / módulo `tools` off na conta.

### DTOs (camelCase)
```
ShortLinkItem { id, slug, targetUrl, shortUrl, hits, createdAt, accountId, clientName }
QrCodeItem    { id, slug, targetUrl, qrUrl, fillColor, backColor, size, isActive,
                scanCount, lastScannedAt, createdAt, accountId, clientName }
```
`shortUrl` = `{publicBase}/s/{slug}`; `qrUrl` = `{publicBase}/q/{slug}` (o que o QR codifica).
`publicBase` vem de `TOOLS_PUBLIC_BASE_URL` (override) ou `PUBLIC_API_BASE_URL` (padrão, já usado
por bio/cardapio), com fallback para o host do request no redirect. `clientName` = `core.accounts.name`.
A **imagem** do QR é gerada no cliente (lib `qrcode`) a partir de `qrUrl` + cores + size — o
backend não renderiza nem armazena PNG.

## Isolamento multi-tenant (princípio inegociável)

`RequireAuthWithAccount` rejeita `platform_admin` (sem linha em `core.account_users`), por isso as
rotas usam `RequireAuth` + validação de membership **no handler**:
- `platform_admin`: confia no `X-Account-Id` (troca de conta legítima); `X-Account-Id` vazio =
  platform view = lista **todas** as contas. No create pode mirar qualquer `accountId` do body.
- Usuário comum: valida `IsMember(X-Account-Id, userID)` contra `core.account_users`; escopo =
  essa conta; `accountId` do body é **ignorado** (nunca escreve em conta alheia). Delete/patch
  filtram `id = $1 and account_id = $2` → linha de outra conta vira 404 (não 403; não vaza existência).

Defesa em profundidade: toda query do store filtra por `account_id` mesmo após a validação no handler.

## Permissões (declaradas em `module.go`, sync automático no boot)
- `tools.shortlinks.view` / `tools.shortlinks.manage`
- `tools.qr.view` / `tools.qr.manage`

Role templates: `tools.manager` (todas) e `tools.viewer` (só `*.view`). A visibilidade do menu
"Tools" continua governada pela permissão de página `workspace.tools.view` (módulo core).

## Frontend
- Páginas estáticas `web/app/pages/tools/qr-code.vue` e `.../encurtador-de-link.vue` (vencem a rota
  dinâmica `[tool].vue`), `definePageMeta({ layout:'dashboard', workspaceId:'tools', pageLabel })`.
- Composables `useShortLinksManager` / `useQrcodesManager` via `createApiRequest` (X-Account-Id
  automático pelo bridge). Dropdown de conta = `useTenantsStore` (`/v1/tenants` = `core.accounts`),
  só para quem pode escolher cliente.
- Remove os mocks `tools/qr-code` e `tools/encurtador-de-link` do `demo-pages.ts` e liga as entradas
  do menu (`hidden:false`) em `web/layers/queue/nav.config.ts`.
- Dep nova: `qrcode` (+ `@types/qrcode`) no `web/package.json`.

## Notas de Deploy (ordem)
1. **Migration** `0197_tools_module.sql` — roda automática no boot da api (schema `tools`).
2. **Rebuild api**: `docker compose up -d --build api` (código Go novo; restart não basta — ver
   [[feedback_backend_rebuild]]).
3. **Env var** `TOOLS_PUBLIC_BASE_URL` — base absoluta dos links `/s` e `/q`. Se ausente, usa
   `PUBLIC_API_BASE_URL` (= `https://omni.crowvisuals.com.br` em prod); se ambos ausentes, cai no
   host do request. **Para os links saírem como `crowvisuals.com.br/s/{slug}`** (raiz, sem o
   subdomínio `omni.`): setar `TOOLS_PUBLIC_BASE_URL=https://crowvisuals.com.br` no `.env.production`.
4. **Caddy (VPS)** precisa rotear `/s/*` e `/q/*` para a api no host escolhido, senão o link clicado
   cai no web (Nuxt) e dá 404. No bloco do host servir:
   `handle /s/* { reverse_proxy lista-api:8080 }` e `handle /q/* { reverse_proxy lista-api:8080 }`.
   **Ressalva:** se os links usam a raiz `crowvisuals.com.br`, esse host precisa existir no Caddy da
   VPS e apontar pra este stack (hoje só `omni.crowvisuals.com.br` está configurado — confirmar antes).
5. **Dep npm** `qrcode` — regenerar lock via docker (lock cross-platform, ver
   [[project_web_npm_lockfile_cross_platform]]), não `npm install` no host.
6. **Rebuild web** de prod só após aprovação (ver [[feedback_no_npm_until_approved]]).

## Status (2026-07-09)
- [x] Migration 0197 (aplicada; schema tools + short_links + qr_codes)
- [x] Módulo Go (model/store/service/http/http_public/module/errors) — build+vet+golangci-lint limpos
- [x] Registro em app.go + gating (/v1/tools) + front MODULE_PATH_GUARDS
- [x] AGENT.md do módulo
- [x] Dep qrcode + tipos + composables + páginas + workspaces + nav (hidden:false, moduleId:'tools')
- [x] Redirect público validado por smoke-test: /s e /q dão 302, hits/scan contam, is_active e
      módulo-off dão 404, slug desconhecido 404 (dados de teste removidos)
- [x] Front compila e renderiza (HTTP 200, eslint limpo)
- [x] Edição inline do encurtador (slug/targetUrl): `PATCH /v1/tools/short-links/{id}` + `updateShortLink`
      via `useInlineEditManager` (debounce + otimista + re-hidrata do banco); larguras de coluna ajustadas
- [ ] **Pendente do dono**: habilitar o módulo `tools` nas contas que vão usá-lo (senão o menu
      some e os redirects dão 404). platform_admin usa via platform view. Validar create/edit no
      browser logado.
