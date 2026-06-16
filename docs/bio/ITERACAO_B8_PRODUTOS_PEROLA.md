# Iteração B8 — Sincronizar produtos do cliente (Pérola) → site.products

> Specs para subagentes Opus. Doc canônico: docs/bio/PLANO_MODULO_BIO.md.
> Criado 2026-06-13. Status: PLANEJADO — aguardando ok.

A bio lê produtos de `site.products` (nosso banco). Esta iteração POPULA
`site.products` puxando da API do site do próprio cliente. Primeiro cliente =
**Pérola**; arquitetura plugável (outros clientes definidos depois). Conecta com
a B7 (slides com fonte = site.products).

## A API da Pérola (descoberta — read-only, já serve)

- **Online**: `https://perolajoias.com/api/products/` · Local: `C:\xampp\htdocs\painel-perola\api\products\`.
- `GET /api/products/?page=&limit=&q=&category=&campaign=&date_from=&date_to=&with_deleted=`
  → `{ data: [Produto], meta:{page,limit,count,total,has_more} }`. **GET é público** (sem auth); CORS `*`.
- `Produto`: `{ id, name, code, categories (JSON-array em texto), campaigns (JSON-array), image, status('active'|'desactive'), stock, fator, price, created_at, updated_at, deleted_at }`.
- Paginação 0-based; `limit` 1–100. `category`/`campaign` filtram por LIKE no JSON.
- **Aviso de segurança (ao cliente, não-bloqueante)**: a auth de ESCRITA usa chave
  hardcoded `ABC123` em `api/auth.php` (TODO no próprio código). Não afeta nossa
  leitura. Recomendar ao cliente mover para env/segredo se um dia abrirmos escrita.

## Mapeamento Pérola → `site.products` (nosso)

| Pérola | site.products (nosso) |
|---|---|
| name | name |
| code | sku/code (campo existente) |
| categories[] | category (1ª) — ou normalizar; manter o array em campo extra se útil |
| campaigns[] | campaigns[] |
| image | imageUrl (absolutizar com `https://perolajoias.com` se relativa) |
| price | priceCents (price × 100, arredondar) |
| status active/desactive | status (ativo/inativo) |
| stock | stock (se houver coluna; senão ignorar) |
| id (origem) | external_id (chave de upsert por account+source) |

> Confirmar os campos exatos de `site.products` na implementação (model.go). Se
> faltar `external_id`/`source`, adicionar via migration (idempotente).

## Subagente A — Back: fonte externa + sync

Território: `back/internal/modules/site/*` + migration nova.

1. **Config de fonte externa por account** (ex.: `site.product_sources`: account_id,
   type `external_api`, base_url, enabled). Seed/INSERT da Pérola = `https://perolajoias.com/api/products`. (Sem credencial — GET público.)
2. **Cliente HTTP** (`internal`): lê a API paginada (`limit=100`, segue `has_more`),
   com timeout/contexto (noctx-safe), parse do JSON, parse dos JSON-arrays
   (categories/campaigns).
3. **Sync**: mapeia (tabela acima) e faz **upsert** em `site.products` por
   `account_id + external_id` (não duplica; atualiza mudados; marca como
   inativo/removido os `deleted_at`). Sem N+1 (batch).
4. **Endpoint**: `POST /v1/admin/products/sync?accountId=` (ou no site admin) →
   dispara a sync da account; retorna `{inserted, updated, skipped}`. (Agendamento
   periódico fica como fase futura — começa sob demanda/botão.)
5. Testes: parse do payload Pérola (fixture), mapeamento, upsert idempotente.
6. Validação: build/vet/test + golangci-lint 0; AGENT.md do site.

## Subagente B — Front: módulo site/produtos consome o real + botão sincronizar

Território: páginas/componentes de `/site/produtos` no painel. STATUS: ENTREGUE (2026-06-13).

1. Botão **"Sincronizar produtos"** (admin) → chama o endpoint de sync → toast com
   o resultado → recarrega a lista. FEITO — `canSync` = `platform_admin`; chama
   `POST /v1/admin/products/sync?accountId=<activeTenantId>` (header `X-Account-Id`
   também enviado); spinner/disable via `syncing`; toast `useUiStore.success` com
   `{inserted, updated, skipped}`; `fetchProducts()` recarrega ao final.
2. A lista de produtos (já existe) passa a mostrar os produtos puxados (categoria,
   campanhas, imagem com preview, status). Filtro por categoria/campanha. FEITO —
   colunas `image` (type `image`, thumbnail + modal de preview via OmniDataTable),
   `categories` e `campaigns`; filtros select de categoria/campanha populados
   dinamicamente das tags carregadas, via `customPredicate` (match em array).
3. Validação: eslint 0 + vue-tsc limpo. FEITO — eslint 0 erros nos arquivos tocados
   (1 warning pré-existente `no-dynamic-delete` no composable, não introduzido);
   vue-tsc só com os `TS2307` de alias `~/types/*` pré-existentes (mesmo padrão de
   ~10 arquivos do repo), ~231 erros repo-wide pré-existentes.

### Implementação (arquivos)
- `web/app/composables/useProductsManager.ts` — `syncProducts()` + estado `syncing`.
- `web/types/products.ts` — tipo `ProductSyncResult`.
- `web/app/components/site/SiteProductsAdminWorkspace.vue` — botão sync, colunas
  imagem/categorias/campanhas, filtros categoria/campanha.
- `web/app/components/site/SiteProductCreateDialog.vue` — NOVO; modal de criação
  extraído do workspace (manter < 450 linhas).

### Mock/legado encontrado
- `web/app/components/site/SiteProductsWorkspace.vue` (baseado em `useSiteStore`,
  dados mock) está ÓRFÃO — nenhuma rota/componente o referencia. A rota
  `/site/produtos` já usa `SiteProductsAdminWorkspace` (API real). Candidato a
  remoção numa limpeza futura (fora do escopo desta iteração).

## Relação com a B7 (slides com fonte)

- B8 popula `site.products`; B7 (bio) lê `site.products` como fonte dos slides.
- Podem rodar em paralelo (compartilham só a tabela `site.products`, A de B8 é
  dono do schema site; A de B7 só LÊ).
- Confirmado pelo usuário (B7): slide-produto tem **link configurável**
  (produto no site / WhatsApp / sem link); o slideTop escolhe **conteúdo**
  (produtos OU imagens) e **modo** (carrossel OU estático). Atualizar a spec B7
  com essas 3 opções.

## Regras comuns
Ler AGENT_RULES + ENGINEERING_PRINCIPLES + este doc. Sem git. Não migration/rebuild
(devolver). Máx 450 linhas. Sem emoji. Lint zero. AGENT.md. account_id em tudo;
SQL parametrizado/schema-qualificado; 404 fora de escopo.
