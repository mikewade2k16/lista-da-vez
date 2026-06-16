# Plano — Módulo Bio (Site/Bio)

> Doc canônico do módulo `bio`. Espelhado em `web/app/components/roadmap/roadmap-data.ts` (fase `bio-links`).
> Contrato de saída: `API-INTEGRATION.md` do repo do front bio (Nuxt). **Fonte da verdade do shape é `types/bio.ts` daquele repo** — em caso de dúvida, o type vence.
> Criado em 2026-06-12. Status: B1+B2 ENTREGUES e wiring central APLICADO (2026-06-12, subagentes Opus A/B + integração). Falta B3: aplicar migration 0152 + rebuild api + habilitar módulo + e2e (passos do usuário) e tirar o `hidden` do menu.

---

## 1. Objetivo

Gerir dentro do painel Omni as **páginas de bio** (link-in-bio) servidas pelo front Nuxt separado (VPS, SSR + SWR 300s). O painel é o CRUD; o front bio consome `GET {base}/bio/{slug}` server-to-server e renderiza.

Requisitos do produto:

- **Multitenant**: cada cliente vê e edita **apenas a(s) bio(s) da sua account**. Admin/agência (`platform_admin`) gerencia todas, com **filtro por cliente**.
- Bio "sem cliente fixo": criar uma account normal em `core.accounts` (um "cliente de bio") com só o módulo `bio` habilitado. **Nenhuma modelagem especial** — bio sempre pertence a uma account.
- Menu: dentro de **Site → Bio** (`/site/bio`).
- Design system Omni (tokens `omni-tokens.css`, BEM, sem hex hardcoded).

## 2. Decisões de arquitetura

| Decisão | Escolha | Por quê |
|---|---|---|
| Posição no back | **Módulo próprio `bio`** registrado no Module Registry (padrão `automation`/`meta_ads`), schema Postgres `bio` | O módulo `site` existente NÃO é registrado no registry (drift, ver §9); módulo próprio permite habilitar bio por account sem arrastar leads/produtos |
| Armazenamento do conteúdo | `data_draft jsonb` + `data_published jsonb` por bio + `bio.defaults` global (1 linha jsonb) | O contrato é um JSON profundo que evolui com `types/bio.ts`; jsonb evita drift de schema. Draft/published = editar sem quebrar a bio no ar |
| Merge | Em Go, na hora de servir: `deepMerge(defaults, data_published)` — objetos recursivo, **arrays e primitivos substituição total** (mesma semântica do `server/utils/deepMerge.ts` do front bio) | A API deve devolver o objeto JÁ RESOLVIDO (§2 do contrato) |
| Slug | Único **global** (`lower(slug)`), regex `^[a-z0-9-]+$` | Namespace público `/bio/{slug}` |
| Endpoint público | `GET /v1/public/bio/{slug}` sem JWT, **fora do gating** (registrado como as rotas ingest do site) | O front bio chama server-to-server sem credencial (hoje); base configurável lá via `NUXT_API_BASE` |
| Mídia | Upload local em `UPLOADS_DIR/bio/{accountId}/...`, servido por `GET /uploads/` (file server já existe). No endpoint público, paths `/uploads/...` viram **URLs absolutas** via env nova `PUBLIC_API_BASE_URL` | Contrato exige URL absoluta; painel continua usando relativa |

## 3. Banco — migration `0152_bio_schema.sql`

Idempotente, schema qualificado, **sem `-- +goose Down`** (o migrator roda o arquivo inteiro).

```sql
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
```

Seed opcional do `bio.defaults` com o conteúdo do `_default.json` do front bio fica como passo manual do usuário (PUT /v1/bio/defaults pelo painel), não na migration.

## 4. Backend — `back/internal/modules/bio/`

Arquivos (padrão do módulo Go; máx 450 linhas/arquivo):

| Arquivo | Responsabilidade |
|---|---|
| `module.go` | Adaptador do Registry: ID `bio`, schema `bio`, permissões, role templates, Build + handle (espelhar `automation/module.go`) |
| `model.go` | DTOs: `Bio`, `BioSummary` (lean p/ lista: id, accountId, accountName, slug, name, status, updatedAt, publishedAt), `BioDefaults`, requests |
| `store_postgres.go` | CRUD `bio.bios` + `bio.defaults` + `bio.media`; scan nullable com `*string`/`*time.Time`; queries SEMPRE filtram `account_id` quando escopadas |
| `service.go` | Regras: escopo, validação de slug, publish (draft→published + validação mínima), merge |
| `merge.go` | `deepMerge` (objeto recursivo; array/primitivo substitui) + `absolutizeUploads` (walk no jsonb trocando strings `/uploads/...` por `PUBLIC_API_BASE_URL + path`) |
| `media_storage.go` | Upload local (padrão `feedback/image_storage.go`), **`0o750`/`0o600`** (lint gosec; NÃO copiar o `0o755/0o644` do avatar_storage legado), allowlist de mime, limites (vídeo 50MB, imagem 5MB) |
| `http.go` | Rotas do painel (JWT + X-Account-Id) |
| `http_public.go` | `GET /v1/public/bio/{slug}` sem JWT |
| `service_test.go` | Testes do merge (semântica de array), escopo (404 fora do escopo), publish |
| `AGENT.md` | Doc do módulo |

### Endpoints do painel (gating `/v1/bio` → módulo `bio`)

| Verbo | Path | Notas |
|---|---|---|
| GET | `/v1/bio/bios?accountId=&status=&q=` | Lista lean. `accountId` é **filtro dentro do permitido**: não-admin só vê a própria account (membership via Principal); pedir outra → `404` |
| POST | `/v1/bio/bios` | `{accountId?, slug, name}`; não-admin ignora `accountId` e usa o do contexto |
| GET | `/v1/bio/bios/{id}` | Draft + published + meta. Fora do escopo → `404` |
| PATCH | `/v1/bio/bios/{id}` | `{name?, slug?, dataDraft?}` (dataDraft substitui o draft inteiro) |
| POST | `/v1/bio/bios/{id}/publish` | Copia draft→published; valida mínimos (`branding.logo.srcMobile`, `video.bgVideo`); seta status/published_at |
| POST | `/v1/bio/bios/{id}/unpublish` | Volta para draft (público passa a 404) |
| DELETE | `/v1/bio/bios/{id}` | |
| GET | `/v1/bio/bios/{id}/preview` | JSON mesclado do **draft** (para prévia no painel) |
| POST | `/v1/bio/bios/{id}/media` | multipart (`kind` + arquivo) → `{url: "/uploads/bio/..."}` |
| GET/PUT | `/v1/bio/defaults` | Só `platform_admin` (equivalente ao `_default.json`) |

### Endpoint público (sem JWT, sem gating)

`GET /v1/public/bio/{slug}`:

1. Valida `^[a-z0-9-]+$` (normaliza lowercase).
2. Busca bio `status='published'`; account ativa; módulo `bio` habilitado em `core.account_modules` para a account. Qualquer falha → `404 not_found` (sem vazar existência).
3. Responde `deepMerge(defaults, data_published)` com mídias absolutizadas. `Cache-Control: public, max-age=60` (o front bio já faz SWR 300s).
4. Se env `BIO_PUBLIC_TOKEN` estiver setada, exige `Authorization: Bearer <token>` (extensão prevista no §6 do contrato; default desligado).

### Registro no app.go (feito na INTEGRAÇÃO, não pelo subagente)

- `registry.MustRegister(bio.New())` no bloco CoreV2 (junto de automation/meta_ads).
- `moduleGatingRules()`: `{Prefix: "/v1/bio", ModuleID: "bio"}` (a rota pública usa `/v1/public/bio`, prefixo diferente — fora do gate).
- `app.go` é compartilhado com o módulo `cardapio` (docs/cardapio/PLANO_MODULO_CARDAPIO.md), que roda em paralelo — por isso o wiring central é aplicado pelo agente principal na fase B3/C3, nunca pelos subagentes.

### Permissões e role templates

- `bio.view` / `bio.manage` / `bio.publish` (scope `account`).
- Templates: `bio.manager` (as 3), `bio.editor` (view+manage), `bio.viewer` (view).

### Segurança/perf (checklist da auditoria §10)

- `account_id` nunca vem do body para escopo — vem do Principal/X-Account-Id; `accountId` de query é filtro validado.
- Fora do escopo → `404`, não `403`.
- SQL parametrizado; índices em `account_id` e `lower(slug)`.
- Lista lean (sem jsonb na listagem); jsonb só no GET por id.
- Endpoint público: payload único, 1 query (join account_modules), cache header.

## 5. Frontend — Site → Bio

### Wiring (os 4 lugares — falha silenciosa se faltar um; feito na INTEGRAÇÃO, não pelo subagente)

1. `web/app/utils/workspaces.ts`: `{ id: 'site_bio_web', label: 'Bio', icon: 'dashboard_customize', path: '/site/bio' }`.
2. `web/app/domain/utils/permissions.ts`: entry em `WORKSPACE_ACCESS_DEFINITIONS` (mesmo padrão dos `site_*_web`: viewPermission/editPermission `''`) + `site_bio_web` em cada `ROLE_WORKSPACES[role]` que hoje tem `site_tracking_web` (platform_admin, owner, marketing, director, manager...).
3. `web/layers/queue/nav.config.ts`: child do `site-menu`: `{ id: 'site-bio', label: 'Bio', icon: 'page', path: '/site/bio', workspaceId: 'site_bio_web', moduleId: 'bio', hidden: true }` (**hidden até validar**, padrão meta-ads).
4. `web/app/middleware/module-enabled.global.ts`: `{ prefix: '/site/bio', moduleId: 'bio' }`.

### Páginas e componentes

| Arquivo | Conteúdo |
|---|---|
| `pages/site/bio/index.vue` | Orquestra `BioListWorkspace` |
| `pages/site/bio/[id].vue` | Orquestra `BioEditorWorkspace` |
| `components/bio/BioListWorkspace.vue` | Lista (tabela: nome, slug, cliente, status, atualizado em) + busca + **filtro por cliente (só admin; accounts via `useTenantsStore` → `/v1/tenants`)** + botão criar |
| `components/bio/BioCreateModal.vue` | Nome + slug (+ select de cliente, só admin) |
| `components/bio/BioEditorWorkspace.vue` | Shell do editor: sidebar de seções + painel ativo + barra de status/publicar (padrão visual do AutomationWorkspace M6) |
| `components/bio/sections/BioSectionMeta.vue` | meta (title, favicon, gtmId, lang) |
| `components/bio/sections/BioSectionBranding.vue` | branding (logo, nome, rodapé) |
| `components/bio/sections/BioSectionVideo.vue` | vídeo bg + overlay + layout (alignItems, template, toggles) |
| `components/bio/sections/BioSectionLinks.vue` | `links[]` + `headerMenu[]` via `BioLinkListEditor` reutilizável (ordem do array = ordem de exibição; reordenar com botões subir/descer) |
| `components/bio/sections/BioSectionSlides.vue` | slideTop (toggle, slides, config do carrossel) |
| `components/bio/sections/BioSectionStores.vue` | storeLocator (lojas, bounds, openOnQuery) + lightbox |
| `components/bio/BioPublishBar.vue` | Status draft/published, Publicar/Despublicar, link da bio pública (`NUXT_PUBLIC_BIO_FRONT_URL + /bio/ + slug`, se configurada), prévia JSON |
| `components/bio/AGENT.md` | Doc da área |
| `stores/bio.ts` (Pinia) | Lista + bio ativa + defaults; chamadas via fetch autenticado padrão do projeto |
| `composables/useBioEditor.ts` | Estado do draft, dirty-check, save (PATCH dataDraft), upload de mídia |
| `domain/bio/types.ts` | Port do `BioData` (§3 do API-INTEGRATION.md): `BioMeta`, `BioBranding`, `BioLayout`, `BioVideo`, `BioMenuItem`, `BioLink`, `BioSlideTop`, `BioStoreLocator`, `BioLightbox` |

### Regras de UI

- Raiz da página com `flex: 1; min-height: 0; overflow-y: auto` (ou `.page-workspace`) — senão corta sem scroll.
- Tokens do design system (`rgb(var(--primary))`, `var(--text-muted)`, `var(--line-soft)`...); BEM `.bio-editor__secao--ativa`; sem emoji; máx 450 linhas/arquivo (seções separadas garantem isso).
- Feedback imediato: salvar desabilita botão + spinner; toasts de sucesso/erro; estados vazios orientativos.
- Upload: `<input type=file>` por campo de mídia, mostra thumbnail/nome após upload, grava a URL retornada no draft.

## 6. Notas de Deploy

Ordem exata:

1. Migration `0152_bio_schema.sql` — roda no deploy padrão (verificar `DATABASE_URL` aponta `:5433` no local).
2. **Rebuild da api** (mudou Go): `docker compose up -d --build api`.
3. Envs novas em `.env.production` **E** `docker-compose.prod.yml` (environment):
   - `PUBLIC_API_BASE_URL` (ex.: `https://api.omni.../`) — absolutiza mídia no endpoint público. Sem ela, mídia sai relativa (quebra no front bio).
   - `BIO_PUBLIC_TOKEN` (opcional, default vazio = endpoint aberto).
4. Front painel: `NUXT_PUBLIC_BIO_FRONT_URL` (opcional, link "ver bio").
5. Repo do front bio (separado): `NUXT_API_BASE=https://<api>/v1/public` → ele chama `GET {base}/bio/{slug}`.
6. Checklist de aceite §6 do API-INTEGRATION.md (200 no slug, 404 em inexistente, fallback local ao derrubar a API, SWR ~5min).

## 7. Fases (espelhadas no roadmap-data.ts, grupo `bio`)

- **B1 — Banco + módulo Go `bio`** (subagente A): migration 0152 + módulo completo + testes + AGENT.md. **Sem `app.go`.**
- **B2 — Painel Site/Bio** (subagente B, em paralelo): types + store + composable + páginas + componentes + AGENT.md. **Sem wiring compartilhado.**
- **B3 — Integração e validação** (agente principal + usuário): wiring central (app.go registro/gating + 4 arquivos do front, junto com o do cardápio), aplicar migration, rebuild api, habilitar módulo numa account de teste, e2e (criar bio → editar → publicar → GET público), tirar `hidden`, sync dos 3 docs + panorama HTML.

## 8. Specs dos subagentes Opus

Regras comuns (colar em ambos os prompts):

- Ler ANTES: `AGENT_RULES.md`, `docs/ENGINEERING_PRINCIPLES.md`, `docs/bio/PLANO_MODULO_BIO.md` (este arquivo) e o `API-INTEGRATION.md` (conteúdo do contrato está resumido no §3-§5 deste plano).
- **NENHUM comando git** (somente o usuário roda git).
- NÃO aplicar migration, NÃO rebuildar containers, NÃO mexer em portas (api=9091, web=3003, postgres=5432) — devolver comandos ao usuário.
- Máx 450 linhas/arquivo; sem emoji; lint zero antes de encerrar.
- Atualizar o AGENT.md da área tocada.
- **Regra de paralelismo (4 agentes simultâneos — bio A/B + cardapio C/D):** PROIBIDO tocar em `app.go`, `nav.config.ts`, `workspaces.ts`, `permissions.ts`, `module-enabled.global.ts`. O wiring central dos dois módulos é aplicado pelo agente principal na integração (B3/C3).

**Subagente A — back+banco (B1).** Entregáveis: `0152_bio_schema.sql` (§3); módulo `back/internal/modules/bio/` (§4, todos os arquivos); `service_test.go` (merge/escopo/publish). NÃO tocar `app.go` — o registro/gating é da integração. Validação: `go build ./... && go vet ./...` + `golangci-lint run` no pacote = 0 issues; `gofmt`. Padrões: IDs `string` (sem uuid externo), scan nullable `*string`, SQL schema-qualificado, 404 fora de escopo, permissões via catálogo do módulo (não hardcode).

**Subagente B — front painel (B2).** Entregáveis: §5 (types, store, composable, 2 páginas, componentes, AGENT.md) — SEM os 4 arquivos de wiring (integração). Contrato da API = §4 (endpoints e DTOs) — não esperar o back pronto. Validação: `eslint` 0 erros + `vue-tsc --noEmit` limpo (exceto ruído repo-wide pré-existente de `~/types`, se houver). NÃO rodar `npm install` no host (lockfile cross-platform via container).

Independência: A (back/modules/bio) × B (web bio) × C (back/modules/cardapio) × D (web cardapio) — nenhum arquivo em comum entre os 4. Contratos congelados nos dois docs canônicos. Conflito zero.

## 9. Avisos (drift/legado encontrado no mapeamento)

- **`site` não é registrado no Module Registry**: `site/module.go` existe com catálogo de permissões, mas `app.go` monta as rotas legacy-style e nunca chama `registry.MustRegister(site.New())` — logo `core.modules` pode não conter `site`, e o front gateia `moduleId: 'site'` em `/site/leads|produtos|tracking`. Para contas não-admin isso pode estar bloqueando o menu Site inteiro. O módulo `bio` NÃO herda esse drift (registry desde o dia 1). Corrigir o `site` fica fora deste escopo — anotado aqui e a decidir.
- `auth/avatar_storage.go` usa `0o755`/`0o644` (padrão antigo); o lint atual exige `0o750`/`0o600` — o `media_storage.go` do bio segue o padrão novo.
- `bio.defaults` nasce `{}` — até o admin colar o conteúdo do `_default.json` do front bio via painel, o merge devolve só o que estiver na bio.

## 10. Pendências combináveis depois (do contrato)

- Webhook/purge de cache no front bio ao publicar (hoje: SWR ~5min).
- `GET /bios` de listagem pública (se o front bio um dia tiver índice).
- Auth da chamada pública por token (já previsto via `BIO_PUBLIC_TOKEN`, exige ajuste no front bio).
