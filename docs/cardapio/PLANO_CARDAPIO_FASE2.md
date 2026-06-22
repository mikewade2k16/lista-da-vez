# Plano — Cardápio Fase 2 (paridade lojatop + identidade visual)

> Continuação de `docs/cardapio/PLANO_MODULO_CARDAPIO.md`. Criado em 2026-06-19.
> **Status (2026-06-19): FASE 2 ENTREGUE — back + painel + TAVOLA + seed.** Migrations reais = **0166** (delivery_zones) e **0167** (restaurant_extra). api rebuildada, migrations aplicadas, seed do Mostarda (dados + 17 categorias + 17 zonas + pagamento + tema) aplicado. TAVOLA: types, checkout com select de bairro (frete por zona), exibição de pagamento, injeção GA/Pixel, theme curado no useRestaurantTheme. **E2E do frete por zona validado via API** (pedido em "Aruana" → frete R$25 aplicado pelo servidor; sem zona → fallback). Falta só a validação VISUAL no browser (painel + TAVOLA). customHeadHtml: campo existe mas NÃO é injetado no TAVOLA (risco XSS, diferido).
> Restaurante de teste/seed: **slug `mk`** (display "Mostarda", account Crow `80caf5d5-...`, módulo `cardapio` habilitado, é o que o TAVOLA aponta via `NUXT_PUBLIC_DEV_SLUG=mk`). O outro `mostarda` (slug mostarda) é o órfão sob a conta Mostarda — abandonado.

---

## 1. Objetivo

Trazer ao nosso módulo cardápio a paridade com o cadastro do sistema antigo (lojatop) que falta, popular os dados reais do **Mostarda Bar Bistrô**, e adicionar a identidade visual por restaurante (mais rica que a "1 cor" do antigo). Tudo sobre a arquitetura existente: painel Omni (CRUD) → API → banco; front público estático TAVOLA consumindo `/v1/public/*`.

## 2. Gap (antigo → nosso) e decisões

| Antigo (lojatop) | Nosso hoje | Frente |
|---|---|---|
| Segmento | ❌ | WS-C |
| Estado/Cidade (texto) | ~ address jsonb | WS-C (mantém texto; UF/cidade opcional) |
| Cor personalizada (1 cor) | theme jsonb sem UI | **WS-D (curado: tema base + acento + fonte + claro/escuro)** |
| Formas de pagamento (dinheiro/débito+bandeiras/crédito+bandeiras/PIX/ticket/outras) | ❌ | **WS-B (informativo no site)** |
| Endereço: nº, complemento, ponto de referência | ❌ | WS-C |
| Facebook, Youtube | ❌ | WS-C |
| Estatísticas: GA ID, FB Pixel, HTML adicional | ❌ | WS-C (⚠️ HTML livre = XSS; ver §WS-C) |
| Bairros + valor de entrega | ❌ | **WS-A (zonas de entrega, ponta a ponta)** |
| Responsável + Login | conta/usuário (core.*) | **fora do escopo** do módulo cardápio |

Decisões do dono (2026-06-19): theming **curado**; pagamento **informativo** (não entra no checkout); zonas de entrega **ponta a ponta**; restaurante = **slug mk**.

## 3. Frentes (independentes → paralelizáveis) e dependências

- **WS-A** Zonas de entrega — back (tabela + CRUD + cálculo de frete) + painel + TAVOLA checkout.
- **WS-B** Formas de pagamento (informativo) — `settings.payment` (jsonb, sem migration) + painel + exibição TAVOLA.
- **WS-C** Campos faltantes + estatísticas — colunas novas no restaurante + painel + injeção TAVOLA.
- **WS-D** Aparência/Theming curado — `theme` jsonb (sem migration) + painel + `useRestaurantTheme` no TAVOLA.
- **WS-E** Seed do Mostarda — dados reais das telas. **Depende de A/B/C/D** (precisa dos campos existirem).

Independência: A toca tabela nova + order; B toca `settings` jsonb; C toca colunas novas; D toca `theme` jsonb. Arquivos do back/painel/TAVOLA são separáveis por frente → **4 subagentes A/B/C/D em paralelo**, depois **E** (seed) inline.

Migrations: **0154** (delivery_zones) p/ WS-A, **0155** (colunas do restaurante) p/ WS-C. WS-B e WS-D usam jsonb existente → sem migration.

---

## WS-A — Zonas de entrega (bairros + valor)

### Banco (migration `0154_cardapio_delivery_zones.sql`)
Idempotente, schema-qualificado, sem `-- +goose Down`.
```
cardapio.delivery_zones (
  id uuid pk default gen_random_uuid(),
  account_id uuid not null references core.accounts(id) on delete cascade,
  restaurant_id uuid not null references cardapio.restaurants(id) on delete cascade,
  name text not null,            -- bairro
  fee_cents bigint not null default 0,
  is_active boolean not null default true,
  sort_order integer not null default 0,
  created_at timestamptz not null default now()
)
unique index (restaurant_id, lower(name)); index (account_id); index (restaurant_id, sort_order)
```

### Back (`back/internal/modules/cardapio/`)
- `model.go`: `DeliveryZone {id, restaurantId, name, feeCents, isActive, sortOrder}` (camelCase, centavos `int64`). Input de criação `DeliveryZoneInput {name, feeCents, isActive, sortOrder}`. Update **pointer-based** `UpdateDeliveryZoneInput {Name *string, FeeCents *int64, IsActive *bool, SortOrder *int}` → PATCH parcial de verdade (lição do `project_cardapio_panel_patch_shape`: evitar full-replace que quebra toggles).
- `store_zones.go`: `ListZones(account, restaurant)`, `CreateZone`, `UpdateZone` (COALESCE por campo), `DeleteZone`, `ListPublicZones(restaurant)` (só ativos, `order by sort_order`). Sempre filtra `account_id`.
- `service.go`: `ListDeliveryZones/Create/Update/Delete` (valida posse do restaurante via `GetRestaurant`; account-scoped).
- `http.go`: `GET/POST /v1/cardapio/restaurants/{id}/delivery-zones`, `PATCH/DELETE /v1/cardapio/delivery-zones/{id}` (mesmo `scopedAccountID`/`withScope`).
- `service_public.go` + `model.go`: `PublicMenu` ganha `deliveryZones []DeliveryZone` (ativos). 
- **Pedido**: `PublicOrderInput` ganha `deliveryZoneId string`. `service_orders.go` → `computeDeliveryFee`: se `type=entrega` e `deliveryZoneId` válido (zona ativa do restaurante), `fee = zone.fee_cents`; senão fallback `settings.deliveryFeeCents`. `freeDeliveryAboveCents` continua zerando. Grava o nome do bairro no pedido (campo `delivery_zone_name` no insert OU dentro de `delivery_address`). Zona inválida (não pertence/inativa) → `ErrOptionInvalid`.
- `service_orders_test.go`: frete pela zona; zona inválida; frete grátis acima do limite com zona.
- `AGENT.md`: novas rotas + tabela + regra do frete por zona.

### Painel (`web/app/`)
- `domain/cardapio/types.ts`: `DeliveryZone`.
- `stores/cardapio.ts`: `zones` ref + `reloadZones/createZone/patchZone/deleteZone` (com `withScope`). Carregar no `loadRestaurant` (5ª chamada paralela) ou sob demanda na seção.
- `components/cardapio/sections/CardapioSectionEntrega.vue`: tabela igual ao print (bairro + valor via `CardapioMoneyInput` + toggle ativo + editar/excluir + adicionar). Reordenar opcional. Padrão das outras seções; toggle usa PATCH parcial (pointer-based, já suportado).
- Wire na sidebar do editor (`CardapioEditorWorkspace.vue` SECTIONS) entre "Pedidos" e "Domínios" (ou após "Dados").
- `components/cardapio/AGENT.md`.

### TAVOLA (`apps/web/app/`)
- `types.ts`: `DeliveryZone` + `MenuPayload.deliveryZones`.
- `components/public/CartDrawer.vue`: para `entrega`, trocar o input livre de bairro por um `<select>` das `deliveryZones` (do menu) → ao escolher, `deliveryFeeCents = zona.feeCents` (com freeDeliveryAbove) e `totalCents` recalcula; o body do pedido manda `deliveryZoneId`. Manter rua/nº/complemento como texto.

---

## WS-B — Formas de pagamento (informativo)

Sem migration: estende `settings` jsonb.

### Back
- `model.go` `Settings` ganha `Payment` (struct): `{ cash bool, debit {accepted bool, brands []string}, credit {accepted bool, brands []string}, pix bool, ticket bool, other string }` — tags camelCase. Sai no menu público (settings já é serializado). `UpdateRestaurantInput.Settings` (pointer) já cobre — sem mudança de rota.
- `AGENT.md`: documentar o sub-objeto `settings.payment`.

### Painel
- `domain/cardapio/types.ts`: `RestaurantSettings.payment`.
- Nova sub-seção "Pagamento" em `CardapioSectionDados.vue` (ou seção própria): toggles dinheiro/débito/crédito/PIX/ticket; quando débito/crédito ativos, input de bandeiras (lista por vírgula → array); campo "outras". Salva no `settings` (full body do save de Dados — já é o padrão).

### TAVOLA
- Exibir "Formas de pagamento" no cardápio/checkout (informativo): ícones/labels de PIX, dinheiro, bandeiras aceitas. Componente novo em `sections/` (ex.: `PaymentMethods.vue`), alimentado por `restaurant.settings.payment`. NÃO altera o fluxo do pedido.

---

## WS-C — Campos faltantes + estatísticas

### Banco (migration `0155_cardapio_restaurant_extra.sql`)
`alter table cardapio.restaurants add column if not exists` para: `segment text not null default ''`, `facebook text not null default ''`, `youtube text not null default ''`, `google_analytics_id text not null default ''`, `facebook_pixel_id text not null default ''`, `custom_head_html text not null default ''`. (Endereço: nº/complemento/ponto de referência entram no `address` jsonb — sem migration.)

### Back
- `model.go`: `Restaurant` e `UpdateRestaurantInput` ganham `segment, facebook, youtube, googleAnalyticsId, facebookPixelId, customHeadHtml`. `Address` (jsonb) ganha `number, complement, reference` (opcionais).
- `store_restaurants.go`: incluir as colunas novas no `restaurantColumns`, no INSERT (defaults) e no UPDATE (COALESCE, pointer).
- ⚠️ **Segurança (`custom_head_html`)**: HTML livre injetado no site público = XSS. Regra: o campo só é **editável por `platform_admin`** (gate no painel) e exibe aviso "HTML é injetado no site — risco; só admin". GA/Pixel são só IDs (renderizados por template conhecido no TAVOLA, sem HTML livre) → seguros para o lojista. Documentar como legado/risco no `docs/LEGADO.md`. Alternativa preferível: na 1ª entrega, **só GA/Pixel**; `custom_head_html` fica para depois com sanitização.
- `AGENT.md`.

### Painel
- `domain/cardapio/types.ts`: campos novos.
- `CardapioSectionDados.vue`: segmento; endereço completo (nº/complemento/referência); Facebook/Youtube.
- Sub-seção "Estatísticas": GA ID, Pixel ID (+ `custom_head_html` só com badge/admin-gate).

### TAVOLA
- `useHead`/plugin: injeta GA (gtag) e Pixel (fbq) quando os IDs existem (templates conhecidos). `customHeadHtml` só se vier preenchido (renderização controlada).
- Exibir Facebook/Youtube no footer/header junto do Instagram.

---

## WS-D — Aparência/Theming (curado) — alvo: site público TAVOLA

Sem migration: usa o `theme` jsonb do restaurante. Contrato do `theme`:
```
{ base: "tavola" | "brasa" | <tema registrado>, accent: "#hex", font: "<familia>", mode: "dark" | "light" }
```

### Back
- Nenhuma mudança de schema (theme é jsonb passthrough). Opcional: validação leve do shape no `UpdateRestaurantInput` (ignorar chaves desconhecidas? hoje é RawMessage livre — manter livre).

### Painel
- Nova seção "Aparência": seletor de **tema base** (lista dos temas registrados), **cor de acento** (color picker), **fonte de destaque** (select de uma lista curada de famílias), **modo** claro/escuro. Preview opcional (iframe/box). Salva em `restaurant.theme`. (Logo/Capa já existem em Dados.)
- `domain/cardapio/types.ts`: tipar o `theme` curado.

### TAVOLA
- `composables/useRestaurantTheme.ts`: já aplica `theme.name`/`accent`/`fontDisplay`. Estender p/ o contrato curado (`base`→data-theme-name, `accent`→token de acento, `font`→`--serif`/família, `mode`→dark/light). Garantir que as famílias da lista curada estão registradas em `nuxt.config` (`fonts`) e que os temas base existem em `themes/`.
- Lista curada de fontes e temas base = definir no spec da WS-D (ex.: 4-6 fontes Google + temas tavola/brasa).

---

## WS-E — Seed do Mostarda (depende de A/B/C/D)

SQL idempotente (UPDATE no restaurante slug `mk`) com os dados das telas:
- Geral: name "Mostarda Bar Bistrô", description (do print), segment "Alimentação", slug mantido `mk` (imutável).
- Endereço: CEP 49020-200, nº 228, bairro "Treze de Julho", rua "Rua José Ramos da Silva", complemento "LOJA". Estado SE / cidade Aracaju.
- Hours: "Seg a Dom 11h–16h" + "Ter a Sáb 18h–23h".
- Contato: whatsapp (79) 99110-6000, email Suellen.s3@icloud.com, instagram (link do print), facebook/youtube vazios.
- Pagamento: minOrder 5000 (R$50), dinheiro sim, débito Visa/Mastercard/Elo/Banese, crédito Visa/Mastercard/Elo/Hiper/Banese/Diners/Amex, PIX sim, ticket não.
- Aparência: base/mode dark, accent a partir de #0a0a0a (ou cor da marca).
- Pickup: sim.
- **Zonas de entrega** (do 1º print): inserir os bairros + valores (13 de Julho 700, 18 do Forte 1800, Aeroporto 1500, Aruana 2500, Atalaia 1600, Capucho 1800, Centro 900, Cirurgia 1500, Getúlio Vargas 1400, Grageru 1200, Jabotiana 1500, Jardins 1500, Luzia 1000, Pereira Lobo 1500, Ponto Novo 1200, Salgado Filho 1200, Siqueira Campos 1400, ... resto do print) em `cardapio.delivery_zones`.

## WS-F — Campos opcionais de catálogo (contrato TAVOLA)

> Extensão pós-Fase 2 (2026-06-20). O contrato da API do TAVOLA
> (`TAVOLA/docs/api-contract.md`, "Campos opcionais novos no cardápio") declara
> três campos que o Omni ainda não refletia. São **opcionais**: o site funciona
> sem eles (deriva/placeholder), mas as seções da biblioteca Lego os aproveitam.
> Auditoria do gap: ver memória `project_tavola_omni_layout_gap`.

### Campos
- **`Category.imageUrl`** — foto representativa da categoria (URL absoluta ou
  `/uploads/*` absolutizado no público).
- **`Category.productCount`** — contagem de produtos disponíveis na categoria.
  **Derivada no servidor** (sem coluna): o `PublicMenu` conta os produtos por
  `categoryId`. Sai com `omitempty` (0 = ausente → o front deriva localmente).
- **`Product.compareAtPriceCents`** — preço "cheio" para exibição riscada
  (promoção). `omitempty` (0 = sem preço riscado). Nunca usado como preço real.

### Banco (migration `0168_cardapio_catalog_optional_fields.sql`)
Idempotente, schema-qualificado, sem `-- +goose Down`:
```
alter table cardapio.categories add column if not exists image_url text not null default '';
alter table cardapio.products   add column if not exists compare_at_price_cents bigint not null default 0;
```
`productCount` **não** tem coluna (derivado no `service_public`).

### Back (`back/internal/modules/cardapio/`)
- `model.go`: `Category` ganha `ImageURL` (`imageUrl`) e `ProductCount`
  (`productCount,omitempty`); `CategoryInput` ganha `ImageURL`. `Product` e
  `ProductInput` ganham `CompareAtPriceCents` (`compareAtPriceCents`/omitempty no DTO).
- `store_catalog.go`: `image_url` em `categoryColumns` + scan + INSERT/UPDATE de
  categoria; `compare_at_price_cents` ao FIM de `productColumns` + scan +
  INSERT/UPDATE de produto (`productUpdateArgs`). `productColumns`/`scanProduct`
  são compartilhados → cobre menu público, prato e GET do painel.
- `service_public.go` `PublicMenu`: deriva `ProductCount` por categoria (conta os
  produtos disponíveis já carregados, sem query extra) e absolutiza
  `Category.ImageURL` (como já faz com produto/restaurante).

### Painel (`web/app/`)
- `domain/cardapio/types.ts`: `Category.imageUrl`/`productCount?`,
  `Product.compareAtPriceCents?`.
- `useCardapioProductForm.ts` + `CardapioProductModal.vue`: campo "Preço cheio
  (riscado)" via `CardapioMoneyInput`, ao lado do preço base.
- `CardapioSectionCategorias.vue`: imagem por categoria (URL + upload reusando
  `store.uploadMedia` + `resolveMediaUrl`), no modo de edição; `categoryBody`
  passa `imageUrl` (full-replace — sem ele, zera).

### TAVOLA
- Já preparado: `types.ts` declara `Category.imageUrl`/`productCount?` e
  `Product.compareAtPriceCents?`; o `SectionRenderer` deriva `productCount`
  quando ausente. Sem mudança necessária no site para estes campos.

---

## WS-G — Código do pedido (referência grandes redes)

> Extensão pós-Fase 2 (2026-06-20). O cliente precisa de um **código** como
> confirmação ("fiz o pedido X"), para não haver "fiz e não recebi". Referência
> McDonald's/BK/iFood: código **curto e legível** (não um número longo aleatório,
> ruim de ditar). O `order_number` sequencial continua para uso interno do painel.

### Banco (migration `0169_cardapio_order_code.sql`)
`alter table cardapio.orders add column if not exists code text not null default ''`
+ `create unique index if not exists ... on cardapio.orders (restaurant_id, code) where code <> ''`
(unique **parcial** ignora pedidos antigos com `code=''`).

### Back (`back/internal/modules/cardapio/`)
- `model_order.go`: `Order.Code` (`code`).
- `store_orders.go`: `generateOrderCode` (base32 Crockford `0123456789ABCDEFGHJKMNPQRSTVWXYZ`,
  6 chars, modulo sem viés) + `uniqueOrderCode` (gera e checa na MESMA tx; o unique
  index parcial é o backstop contra corrida). `CreateOrder` gera o code e o insere;
  `orderColumns`/`scanOrder` ganham `code` no fim. `service_orders.go` inalterado.
- `store_orders_code_test.go`: testa formato/alfabeto/colisão do gerador.

### Painel (`web/app/`)
- `domain/cardapio/types.ts`: `Order.code`.
- `CardapioSectionPedidos.vue`: mostra o `code` como identificador principal do
  pedido, com `#orderNumber` + tipo na linha de baixo (o atendente casa o código
  que o cliente informar).

### TAVOLA (`apps/web/app/`)
- `types.ts`: `Order.code`.
- `CartDrawer.vue`: tela de confirmação com status "Pedido recebido", o **código
  em destaque** (mono), instrução "guarde este código" e botão WhatsApp.
- `utils/whatsapp.ts`: a mensagem usa o `code` no cabeçalho do pedido.

### Comportamento que previne o "pedido fantasma"
Pedido só ganha `code` quando é **gravado no banco** (resposta 201); e o checkout
**não finaliza** endereço sem zona de entrega (ver `TAVOLA/docs/checkout-entrega.md`).

---

## 4. Notas de Deploy
1. Migrations **0166** (delivery_zones) e **0167** (restaurant_extra). Aplicar local (`:5432`) e portar p/ produção.
1b. Migration **0168** (WS-F: `image_url` em categories, `compare_at_price_cents` em products). Aplicar local e portar p/ produção; **rebuild api**.
1c. Migration **0169** (WS-G: `code` em orders + unique parcial). Aplicar local e portar p/ produção; **rebuild api**.
2. **Rebuild api**: `docker compose up -d --build api` (mudou Go).
3. TAVOLA: mudanças de front (sem rebuild; HMR + hard-reload; mudança de `nuxt.config`/fontes exige reiniciar o dev server).
4. Envs: nenhuma nova (GA/Pixel são dado por-restaurante, não env).
5. **IMAGENS DO IMPORT DO MOSTARDA (2026-06-19).** As 203 fotos vieram do scraper em `.avif`, foram convertidas p/ `.jpg` (ffmpeg) e copiadas (via `tar` pipe) pro **volume nomeado `api_uploads`** em `/app/data/uploads/cardapio/{accountId-Crow}/mostarda/<slug>.jpg`. Os produtos foram inseridos por **SQL direto** (não pela API) com `image_url = /uploads/cardapio/.../mostarda/<slug>.jpg`. **Para PRODUÇÃO**, o volume de uploads NÃO sobe junto com a imagem da api — é preciso: (a) recriar os mesmos arquivos no `api_uploads` da VPS (re-rodar a conversão+`tar` pipe contra o container de prod, OU dar `docker cp`), e (b) re-rodar o import SQL dos produtos no banco de prod (o `image_url` é relativo e idempotente). Scripts locais: `C:/tmp/import_mostarda.py` (gera SQL + converte), `C:/tmp/import_mostarda.sql`. As imagens convertidas ficaram em `C:/tmp/mostarda_jpg/`.
6. **Mídia relativa no painel.** O painel (web :3003) e a api (:9091) rodam em hosts diferentes; URLs `/uploads/*` são RELATIVAS. O front absolutiza com `web/app/utils/media.ts` (`resolveMediaUrl(url, apiBase)`) nas seções que renderizam mídia (Produtos, Dados logo/banner, Galeria, Modal). O público (TAVOLA) já recebe URL absoluta (o back absolutiza via `PUBLIC_API_BASE_URL`). Não confundir as duas absolutizações (painel = front; público = back).

## 5. Regras comuns dos subagentes (idênticas ao plano §8 do bio/cardapio)
Ler `AGENT_RULES.md` + `docs/ENGINEERING_PRINCIPLES.md` + este plano. **Nenhum comando git.** NÃO aplicar migration/rebuild/portas (passos do dono). Máx 450 linhas/arquivo; sem emoji; lint zero (Go: perms 0o750/0o600, switch, SQL schema-qualificado; TS: sem `any`/`console.log`). Atualizar AGENT.md do módulo tocado. Multi-tenant: `account_id` do Principal, filtro no repo, 404 fora do escopo. PATCH parcial = pointer-based; toggles não mandam body inteiro. Centavos `int64`/camelCase exato. NÃO rodar `npm install` no host do web Omni; TAVOLA pode.

**Independência (4 agentes simultâneos):** A=tabela+order, B=settings.payment, C=colunas+estatísticas, D=theme. PROIBIDO 2 agentes editarem o MESMO arquivo. Conflitos prováveis: `model.go` (B e C e A mexem) e `CardapioSectionDados.vue` (B e C). Resolver: ou serializar B/C, ou dividir model.go/Dados por blocos bem demarcados, ou a integração final (agente principal) costura. Definir no momento do disparo.
