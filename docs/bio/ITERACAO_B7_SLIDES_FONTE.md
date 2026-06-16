# Iteração B7 — Slides com fonte de produtos + collapse + carrossel

> Specs para subagentes Opus (paralelo). Doc canônico: docs/bio/PLANO_MODULO_BIO.md.
> Criado 2026-06-13. Status: PLANEJADO — aguardando ok para disparar.

Pacote pedido pelo usuário sobre o editor de bio (após B6). Os ajustes rápidos
(regra de collapse, preview de imagem no BioMediaField, select de cliente
maior/voltar-p/-agência) já foram feitos fora deste pacote. Aqui ficam as 3
frentes maiores.

## Decisões de design (validar)

- **D1 — Fonte plugável por bio**: o `slideTop` ganha um modo. `slideTop.source.type`:
  `manual` (slides à mão, como hoje) OU `site_products` (1ª fonte real). Arquitetura
  com interface `ProductSource` — ERP e API externa do cliente entram depois sem
  mexer no resto.
- **D2 — MVP = `site.products`**: já tem `category`, `campaigns[]`, `tipo`, `status`,
  `imageUrl`, `name` por account. A fonte filtra por categoria/campanha/tipo.
- **D3 — Resolução no BACK**: quando `source.type != manual`, o endpoint público
  `GET /v1/public/bio/{slug}` RESOLVE os produtos da fonte e injeta em `slideTop.slides`
  já prontos (src=imageUrl, title=name, href/whatsapp). O crow-nuxt continua
  recebendo `slides[]` — só ganha o **botão** abaixo do carrossel.
- **D4 — Quantidade**: `limit` 5 / 10 / 0(todos).
- **D5 — Botão abaixo do carrossel**: `slideTop.button { text, href }` (1 botão, ex.:
  "Ver toda a coleção" → link da categoria no site do cliente). Opcional.
- **D6 — Link do slide-produto (CONFIRMADO, configurável)**: 3 opções escolhíveis
  no editor — (a) **link do produto no site** do cliente, (b) **WhatsApp** (número
  da bio/lightbox), (c) **sem link**. Padrão a definir depois com uso.
- **D7 — Conteúdo do slide (CONFIRMADO)**: a fonte do conteúdo pode ser **produtos**
  (da fonte) OU **imagens** (avulsas, manuais). Seletor no editor.
- **D8 — Modo de exibição (CONFIRMADO)**: **carrossel** (slide) OU **imagem(ns)
  estática(s)**. Seletor no editor (`slideTop.mode: 'carousel' | 'static'`).
- **D9 — Fonte externa**: além de `site_products` (nosso banco), `site.products` é
  populado pela sync da API do cliente (ver docs/bio/ITERACAO_B8_PRODUTOS_PEROLA.md,
  Pérola como 1º exemplo). A bio sempre lê do nosso `site.products`.

## Contrato (back ↔ front)

| Verbo | Path | Efeito |
|---|---|---|
| GET | `/v1/bio/sources` | Fontes disponíveis para a account (`[{type, label, available}]`); MVP devolve `site_products` |
| GET | `/v1/bio/sources/site_products/facets` | `{categories[], campaigns[], tipos[]}` distintos de site.products da account — popula os selects do editor |
| GET | `/v1/public/bio/{slug}` | Se `slideTop.source.type != manual`, resolve a fonte → injeta `slides[]`; senão usa os slides manuais |

`slideTop` no BioData passa a ter (tudo opcional, retrocompatível):
```
slideTop: {
  active, engine, carousel{...}, slides[]  // (manual, como hoje)
  source?: { type: 'manual'|'site_products', category?, campaigns?[], tipo?, limit? }
  button?: { text, href }
}
```

---

## Subagente A — Back: fonte de produtos plugável

Território: `back/internal/modules/bio/*` (+ ler site.products via adapter).

1. Interface `ProductSource` (em bio): `Facets(ctx, accountID)` → categorias/campanhas/tipos; `Resolve(ctx, accountID, filtro, limit)` → `[]ResolvedSlide{src,title,href}`.
2. `SiteProductsSource`: lê `site.products` (schema `site`) por account, filtrando por category/campaigns/tipo e `status` ativo, `limit`. Query parametrizada, schema-qualificado, sem N+1. (Cross-schema com o mesmo pool — documentar no AGENT.md; não acoplar ao módulo site, só ler a tabela.)
3. Endpoints painel: `GET /v1/bio/sources` e `GET /v1/bio/sources/site_products/facets` (JWT + escopo da account).
4. `service.Public`: se `slideTop.source.type` ∈ fontes, resolve e injeta em `slideTop.slides` ANTES do merge/absolutize; mídia dos produtos absolutizada como o resto.
5. model.go: tipos do `source`/`button`/facets. Testes: resolve (filtro+limit), facets, fallback manual.
6. Validação: build/vet/test + golangci-lint 0; AGENT.md.

## Subagente B — Front: seção Slides (fonte + carrossel-antes + botão)

Território: `web/app/components/bio/sections/BioSectionSlides.vue` + (se precisar) um sub-componente próprio + `web/app/domain/bio/types.ts` (campos novos) + `web/app/stores/bio.ts` (fetch de sources/facets — método novo, não conflita com B6).

1. **Config do carrossel PRIMEIRO** (no topo da seção), depois o conteúdo.
2. Seletor de **Fonte**: "Manual" (slides à mão, como hoje) ou "Produtos do site".
3. Modo "Produtos do site": carregar facets (`GET .../facets`) → selects de **categoria / campanha / tipo** + **quantidade** (5/10/todos). Prévia textual de quantos produtos casam (opcional).
4. **Botão abaixo do carrossel**: campos `texto` + `link` (editáveis), opcional.
5. Modo "Manual": os slides à mão (como hoje) — mas com collapse por slide (coordenar com C; B mantém a edição, C dá o invólucro colapsável OU B usa o componente do C).
6. Validação: eslint 0 + vue-tsc limpo nos arquivos bio.

## Subagente C — Front: collapse nas listas/blocos

Território: novo `web/app/components/bio/BioCollapsibleItem.vue` + `BioLinkListEditor.vue` + `sections/BioSectionLinks.vue` + `sections/BioSectionStores.vue` (+ aplicar no Slides manual em coordenação com B). NÃO tocar BioSectionSlides a fundo (é do B) — só fornecer o `BioCollapsibleItem` reutilizável.

1. `BioCollapsibleItem`: cabeçalho (título + resumo, ex.: "Menu do topo — 5 itens" / "Loja — Pérola Riomar") + botão expandir/recolher; slot do conteúdo. Lembra estado aberto/fechado.
2. Aplicar em: blocos "Menu do topo (header)" e "Links" (BioSectionLinks/BioLinkListEditor), cada **loja** (BioSectionStores), cada **item de menu/link**. Recolhido por padrão quando há vários.
3. Seguir a regra "Blocos de edicao colapsaveis" do AGENT_RULES (já adicionada).
4. Validação: eslint 0 + vue-tsc limpo.

## Subagente D — crow-nuxt: botão abaixo do carrossel

Território: `C:\Users\Mike\Documents\Projects\crow-nuxt`.

1. `types/bio.ts`: `slideTop` ganha `button?: { text, href }`.
2. Renderizar o botão abaixo do carrossel de slides (componente do slideTop), quando `button.text` existir. Estilo coerente com o tema.
3. Não reiniciar/rebuildar — devolver ao usuário.

## Regras comuns
Ler AGENT_RULES + ENGINEERING_PRINCIPLES + este doc. Sem git. Não migration/rebuild.
Máx 450 linhas/arquivo; sem emoji; lint zero; AGENT.md. Layout compacto + collapse.
Territórios exclusivos. Contrato congelado acima.

## Integração (agente principal)
Wiring (se houver app.go — provável que não, rotas via handle), rebuild api,
reiniciar crow-nuxt, e2e (fonte site_products → selects → prévia → publicar →
ver produtos no slide + botão), sync 3 docs + roadmap.

## Definições que ainda faltam (perguntar)
- Para onde o slide de um PRODUTO leva ao clicar (whatsapp padrão? link do produto
  no site? nada)? — D6 é um chute; confirmar.
- Botão é 1 por carrossel (link da categoria) — confirmado pelo usuário.
