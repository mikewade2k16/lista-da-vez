# Plano — Cardápio Online: Gestão / UX do painel (Fase 9)

Doc canônico desta frente. Espelhado em `web/app/components/roadmap/roadmap-data.ts`
(fase `cardapio-online`, tarefas `card-f9-*`) e referenciado nos AGENT.md de
`back/internal/modules/cardapio/` e `web/app/components/cardapio/`.

Foco: **gestão do painel** (Presence), não o site público (TAVOLA). Reaproveita o
máximo do que já existe (`OmniDataTable`/`OmniCollectionFilters`/`useOmniVisibleColumns`,
collapse do bio, RBAC do core). Nada de lib nova de drag-n-drop.

## Objetivo

1. **Duplicar** um cardápio inteiro (restaurante + catálogo + zonas + layout) num clique.
2. **Dados** com blocos colapsáveis (Identidade, Contato, Endereço, Horários, …) — sem scroll infinito.
3. **Categorias** e **Entrega**: drag-n-drop para reordenar + layout de **2 colunas** + edição inline, com a ordem ("o que vem antes") sempre visível.
4. **Produtos**: tabela com **edição inline**, filtros e config de colunas (1 coluna — produto não vira 2 colunas).
5. **Avaliações**: gerir avaliações do **estabelecimento** e de **produto**, com filtro/consciência, e poder usar uma review de produto no estabelecimento.
6. **Pedidos**: telefone do cliente vira **link de WhatsApp** (1 clique abre a conversa). CRM = futuro.
7. **Shell**: menu lateral e header **fixos** (sticky).
8. **Acesso**: faixas operação × config × plataforma.

## Modelo de acesso (reusa permissões existentes — sem migration de RBAC)

**Decisão final 2026-06-22: SEM split operação/config.** Tentamos gatear por
`cardapio.manage`/`cardapio.orders.manage`, mas essas chaves **não estão semeadas nos
papéis** — o `platform_admin` não as carrega (bypassa) e nenhum papel comum
(diretoria/dono/gerente) as tem, então o gating trancava todo usuário não-admin numa
tela vazia. Decisão do dono: deixar **duas faixas** apenas.

| Faixa | Gate | Seções / ações |
|---|---|---|
| **Todos com o módulo** | módulo `cardapio` habilitado (já filtra o acesso ao editor) | Dados, Categorias, Produtos, Avaliações, Pedidos, Entrega, Aparência |
| **Plataforma (Crow)** | `is_platform_admin` (ou `platformView`) | Domínios, aba **Site** (Studio), Seletor de Cliente, **Duplicar**, campo **Custom HTML** |

No front (`CardapioEditorWorkspace.vue`): `SectionGate = 'all' | 'platform'`; só as seções
`platform` checam `isPlatformAdmin`, o resto é sempre visível. Custom HTML já é admin-only
(`CardapioDadosEstatisticas`); Cliente/Duplicar idem.

> Follow-up (se um dia quiserem operador com visão reduzida): semear
> `cardapio.manage`/`cardapio.orders.manage` nos `role_templates` (back) e re-introduzir
> o split por papel. Hoje **não** está ativo — todos com o módulo veem tudo (menos a faixa plataforma).

---

## Onda 0 — Fundação (NÃO paralelizar entre si por arquivo; rodam como 3 agentes sem overlap)

### F1 (back) — Duplicar restaurante
- **Rota**: `POST /v1/cardapio/restaurants/{id}/duplicate` (JWT + gating módulo).
- **Auth**: só `platform_admin` (handler nega não-admin → 403). Source escopado por `scopedAccountID` (404 fora de escopo).
- **Body**: `{ "name": string, "slug": string }` — ambos obrigatórios; `slug` livre globalmente (senão `ErrSlugConflict`/409).
- **Cópia transacional** (uma transação; `account_id` = o do source): restaurante (novo id/slug/name, **`is_active=false`**), categorias, produtos (+`product_variations`/`product_addons`, remapeando `category_id`), `delivery_zones`, `site_layouts` (draft + published).
- **NÃO copiar**: `restaurant_domains` (host único), `reviews` (curadas), `orders`/`order_items`, `events`, `last_order_number` (zera).
- **Resposta**: `201 {restaurant}` (full, sob o novo id). Padrão: espelhar `MoveRestaurantToAccount` (transação + subquery por restaurant_id).
- Arquivos: `service.go` (DuplicateRestaurant), `store_restaurants.go` (cópia transacional), `http.go` (handler + rota), `model.go` (DuplicateRestaurantInput se preciso), `errors.go` (reusar). Teste no `store_fake_test.go`/`service_test.go`.

### F2 (back) — Avaliações de estabelecimento
- **Migration** (próximo número livre — escanear `back/internal/platform/database/migrations/`, provável `0171`; SQL **plano e idempotente**, sem `+goose Down`):
  - `ALTER TABLE cardapio.reviews ALTER COLUMN product_id DROP NOT NULL;` (review com `product_id NULL` = review do estabelecimento)
  - `ALTER TABLE cardapio.reviews ADD COLUMN IF NOT EXISTS show_on_establishment boolean NOT NULL DEFAULT false;`
- **DTO**: `Review.productId` vira opcional (omitempty); `Review.showOnEstablishment bool`. `ReviewInput` (full-replace) ganha `showOnEstablishment`.
- **Rotas novas** (escopo de restaurante):
  - `GET /v1/cardapio/restaurants/{id}/reviews` → reviews do estabelecimento: `product_id IS NULL OR show_on_establishment = true`, order by sort_order.
  - `POST /v1/cardapio/restaurants/{id}/reviews` → cria review do estabelecimento (`product_id = NULL`).
  - `PATCH/DELETE /v1/cardapio/reviews/{id}` (existentes) passam a aceitar `showOnEstablishment`; servem produto e estabelecimento.
- Público: expor estabelecimento no `/v1/public/restaurants/{slug}` é **follow-up** (TAVOLA), fora desta onda — anotar.
- Arquivos: migration, `model.go` (Review/ReviewInput), `store_catalog.go` (queries de review por restaurante + show_on_establishment), `service.go` (ListEstablishmentReviews/CreateEstablishmentReview), `http_catalog.go` (handlers + rotas), AGENT.md. Teste.

### F3 (front) — Componentes base reutilizáveis (arquivos novos)
- `web/app/components/omni/OmniCollapse.vue` — generalizar `BioCollapsibleItem`: props `title`, `summary?`, `defaultOpen?`; slots `default` + `actions`; chevron; `aria-expanded`; tokens do design system; BEM `.omni-collapse__*`. `v-show` (não remove do DOM).
- `web/app/composables/useSortableList.ts` — drag-n-drop **HTML5 nativo** (sem dep). API genérica: recebe `onReorder(from:number, to:number)`; devolve binders (`itemProps(index)`, handlers dragstart/dragover/drop/dragend) + estado `draggingIndex`. Espelhar o padrão já usado em `OmniTableColumnsConfig.vue`.
- `web/app/utils/whatsapp.ts` — `buildWhatsappLink(phone, text?)` e `openWhatsapp(phone, text?)`: normaliza dígitos, prefixa `55` se faltar DDI (Brasil), `https://wa.me/<num>?text=<enc>`; abre em nova aba `noopener,noreferrer`. Substitui o `wa.me` inline solto do `SiteLeadsAdminWorkspace`.

### F4 (front) — Store + types + acesso (arquivos centrais, consumidos pelas páginas)
- `web/app/domain/cardapio/types.ts`: `Review.productId?: string | null`; `Review.showOnEstablishment: boolean`; ajustar `ReviewInput`.
- `web/app/stores/cardapio.ts`:
  - `duplicateRestaurant(id, { name, slug }, accountId?)` → POST duplicate; refresca a lista lean.
  - `loadEstablishmentReviews(restaurantId)`, `createEstablishmentReview(restaurantId, input)`; `patchReview` aceita `showOnEstablishment`; `setReviewOnEstablishment(reviewId, value)` (conveniência, full-replace).
- Acesso: helper de leitura via `usePermission()`/core store consumido pelo shell (sem mudar back).

> **Contrato congelado aqui**: as páginas da Onda 1 só **consomem** store/types/base. Nenhum agente de página edita `cardapio.ts`/`types.ts`/`http*.go` — se faltar algo, reporta.

---

## Onda 1 — Páginas (paralelo real, 1 agente por arquivo, zero overlap)

| # | Página | Arquivo | Entrega | Depende de |
|---|---|---|---|---|
| P1 | Lista | `CardapioListWorkspace.vue` | Botão **Duplicar** nas ações (só platform_admin) → modal nome/slug (slug pré "copia-de-…") → `store.duplicateRestaurant` → abre editor novo | F1, F4 |
| P2 | Shell | `CardapioEditorWorkspace.vue` | Sidebar + header **sticky**; filtrar seções por faixa de acesso; seção ativa inicial = 1ª visível | F4 |
| P3 | Dados | `CardapioSectionDados.vue` | Blocos em **OmniCollapse** (Identidade/Contato/Endereço/Horários/Entrega-Retirada/Pagamento/Estatísticas); footer salvar continua | F3 |
| P4 | Categorias | `CardapioSectionCategorias.vue` | **drag-n-drop** (`useSortableList`) + **2 colunas** + badge de ordem (#1,#2…) + inline; persiste `sortOrder` (CategoryInput full-replace) só nos itens alterados | F3 |
| P5 | Produtos | `CardapioSectionProdutos.vue` | **OmniDataTable** inline (nome/categoria/preço/compareAt/disponível/destaque) + filtros (categoria, disponibilidade) + config de colunas; modal segue p/ variações/adicionais/galeria. Inline = carrega produto full, mescla campo, PATCH (replace-all) | OmniDataTable |
| P6 | Entrega | `CardapioSectionEntrega.vue` | Mesmo padrão de Categorias: drag-n-drop + 2 colunas + inline; PATCH de zona é parcial | F3 |
| P7 | Avaliações | `CardapioSectionAvaliacoes.vue` | Seletor **Estabelecimento × Produto** (Estabelecimento no topo); CRUD nos 2 escopos; ação "usar no estabelecimento" (`showOnEstablishment`); badge de origem | F2, F4 |
| P8 | Pedidos | `CardapioSectionPedidos.vue` | `customerPhone` vira **link WhatsApp** (`openWhatsapp`); mantém filtro/expand/paginação | F3 |

Regras transversais (todas as páginas): tokens do design system (nunca hex), BEM `.cardapio-x__el--mod`, ações com spinner/disable + toast, confirmação destrutiva via `ui.confirm`, ≤450 linhas/arquivo (dividir por responsabilidade), respeitar o shape de PATCH (full-replace vs parcial) já documentado no AGENT.md.

## Ordem de execução

1. Doc + roadmap + AGENT.md (este passo). 2. Onda 0 (3 agentes, sem overlap). 3. Validar Onda 0 (build/lint). 4. Onda 1 (8 agentes em paralelo). 5. Sync 3 docs + panorama + validação no browser.

## Notas de Deploy

1. **Migration** nova (F2, próximo número livre, provável `0171`): `product_id` nullable + `show_on_establishment` em `cardapio.reviews`. SQL plano/idempotente.
2. **Rebuild da api** (mudou Go — F1 + F2): `docker compose up -d --build api`. Restart não basta.
3. Sem env nova. Sem dep nova no front (drag-n-drop é HTML5 nativo).
4. Validação visual no browser obrigatória (mudança de UI) antes de propor commit.

## Estado (CÓDIGO ENTREGUE 2026-06-22, local — falta migration/rebuild/browser)

- [x] Passo 1 — doc + roadmap + AGENT.md
- [x] Onda 0 — F1 duplicar (back) — gofmt/build/vet/test PASS
- [x] Onda 0 — F2 reviews estabelecimento (back + migration **0171**) — testes PASS
- [x] Onda 0 — F3 base front (OmniCollapse, useSortableList, whatsapp) — eslint PASS
- [x] Onda 0 — F4 store/types/acesso — eslint PASS
- [x] Onda 1 — P1..P8 — eslint PASS; vue-tsc sem erro novo no cardapio
- [x] Sync roadmap + AGENT.md (back/front) + doc
- [ ] **Usuário**: aplicar migration `0171` + `docker compose up -d --build api` + validação visual no browser

### Pendências/Follow-ups conhecidos
- **compareAtPriceCents na lista lean** (P5): `ProductLean`/`ListProductsLean` (back) e `ProductListItem` (front) não trazem o campo; a coluna "Preço comparativo" fica vazia até o produto ser editado (a edição funciona via load-full). Fix opcional: adicionar `compare_at_price_cents` ao `ListProductsLean` + DTO + type.
- **Avaliações de estabelecimento no site público** (TAVOLA): F2 expôs só no painel; a renderização no `/v1/public/*` é follow-up.
- **Tamanho de arquivo**: `CardapioSectionCategorias.vue` (505) e `CardapioSectionAvaliacoes.vue` (556) passaram de 450 (quase tudo CSS coeso); aceitável, mas candidatos a extrair `<style>` se crescerem.
