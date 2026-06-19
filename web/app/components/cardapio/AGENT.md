# AGENTS

## Escopo

Estas instrucoes valem para `web/app/components/cardapio/` (painel `/cardapio` — modulo Cardapio Online).

## Responsabilidade

Painel de gestao dos cardapios online (restaurantes). CRUD de restaurante, categorias, produtos
(variacoes/adicionais/galeria), avaliacoes, dominios e pedidos. O front PUBLICO (site estatico que o
visitante acessa) e outro repo: este painel so administra os dados via API do painel.

Doc canonico do modulo: `docs/cardapio/PLANO_MODULO_CARDAPIO.md` (contrato em §4, front em §5).

## Arquitetura

- **Pages** (`pages/cardapio/`): orquestradores finos.
  - `index.vue` -> `CardapioListWorkspace`.
  - `[id].vue` -> `CardapioEditorWorkspace` (recebe `restaurantId` e `accountId` da rota; `?account=`
    so para `platform_admin` abrindo cardapio de OUTRO cliente).
- **Store** (`stores/cardapio.ts`): unica fonte de dados. Lista lean, restaurante ativo + catalogo +
  dominios, pedidos paginados. `X-Account-Id` e injetado automaticamente pelo `createApiRequest`
  (account ativa) — nao passar manualmente. Para `platform_admin` abrindo restaurante de OUTRA
  account, o `accountId` vai na query de TODAS as chamadas do editor: `loadRestaurant(id, accountId)`
  seta `scopeAccountId` e `withScope()` anexa `?accountId=`; o backend prioriza o query sobre o header
  (`scopedAccountID`). Vazio = account ativa (nao-admin ou agencia).
- **Types** (`domain/cardapio/types.ts`): port EXATO do contrato (camelCase). Dinheiro SEMPRE em
  centavos inteiros (`...Cents`). Helpers `formatCurrency`/`formatCents`/`parseCents`/`slugify` e os
  labels pt-BR de status/tipo de pedido vivem aqui.
- **Composables**:
  - `useCardapioEditor.ts`: estado + dirty-check + salvamento da secao Dados, e upload de midia.
  - `useCardapioProductForm.ts`: conversao Product <-> form e montagem do payload PATCH do produto.

## Componentes

| Componente                               | Papel                                                                                                                                                                                 |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `CardapioListWorkspace.vue`              | Tabela de restaurantes (nome/slug/cliente/dominio/ativo/atualizado) + busca + filtro por cliente (so `platform_admin`, via `useTenantsStore`) + criar. Clique na linha abre o editor. |
| `CardapioCreateModal.vue`                | Nome + slug (auto a partir do nome) + select de cliente (so admin; default "Agencia" = propria account ativa, espelha a bio).                                                         |
| `CardapioEditorWorkspace.vue`            | Shell: breadcrumb, barra de status (ativo/inativo + link publico + publicar/despublicar), sidebar de secoes + painel ativo.                                                           |
| `CardapioMoneyInput.vue`                 | Input de moeda reutilizavel: v-model em CENTAVOS, exibe `R$ 1.234,56`.                                                                                                                |
| `sections/CardapioSectionDados.vue`      | Identidade, contato, endereco, horarios, settings de entrega (centavos com mascara), tema JSON. Salva via `useCardapioEditor` (dirty-check).                                          |
| `sections/CardapioSectionCategorias.vue` | CRUD + reordenar (subir/descer via troca de `sortOrder`) + ativar/desativar.                                                                                                          |
| `sections/CardapioSectionProdutos.vue`   | Lista agrupada por categoria + toggle disponibilidade + remover. Abre `product/CardapioProductModal`.                                                                                 |
| `product/CardapioProductModal.vue`       | Editor de produto (campos do contrato) + variacoes + adicionais + galeria. PATCH faz replace-all de variations/addons.                                                                |
| `product/CardapioProductVariations.vue`  | Lista editavel de variacoes (delta de preco, pode ser negativo).                                                                                                                      |
| `product/CardapioProductAddons.vue`      | Lista editavel de adicionais (preco >= 0).                                                                                                                                            |
| `product/CardapioProductGallery.vue`     | Grade de imagens + upload (delega `upload` ao pai).                                                                                                                                   |
| `sections/CardapioSectionAvaliacoes.vue` | Seleciona produto -> CRUD de reviews (rating 1-5, destaque).                                                                                                                          |
| `sections/CardapioSectionPedidos.vue`    | Filtro por status + paginacao + select de status com feedback imediato (toast). Linha expansivel com itens.                                                                           |
| `sections/CardapioSectionDominios.vue`   | CRUD de dominios proprios + explicacao do subdominio por convencao.                                                                                                                   |

## Regras

- Centavos no modelo, `R$` na tela — sempre via `CardapioMoneyInput`/`formatCurrency`/`parseCents`.
- Tokens do design system (`omni-tokens.css`); nunca hex. BEM `.cardapio-x__el--mod`.
- Raiz de pagina rola: `flex:1; min-height:0; overflow-y:auto`.
- Acoes com spinner/disable + toast (`useUiStore().success/error`); confirmacao destrutiva via
  `useUiStore().confirm`. Estados vazios orientativos.
- Filtro por cliente e o `accountId` na criacao so existem para `platform_admin`; demais papeis
  operam na propria account (o backend resolve o escopo).
- **Shape do body no PATCH** (`ReadJSON` usa `DisallowUnknownFields`): (1) NUNCA enviar campo que o
  Input do back nao tem — `slug` no PATCH de restaurante (imutavel) e `id` em variacao/adicional
  (replace-all) dao `400 "Body invalido"`. (2) `CategoryInput`/`ProductInput`/`ReviewInput` sao
  full-replace (nao-ponteiro): todo PATCH (inclusive toggles de `isActive`/`isAvailable`/`isHighlight`
  e reordenacao) manda o OBJETO COMPLETO + o campo alterado, senao zera o resto e da `400 "Dados
invalidos"`. O toggle de disponibilidade do produto busca o produto completo antes (lista e lean).
  So `UpdateRestaurantInput` e parcial (pointer-based).
- Admin abrindo restaurante de outro cliente: o editor leva o `accountId` na query (lista/criacao ->
  `/cardapio/{id}?account=<accountId>`). Sem isso o GET cai na account ativa (`X-Account-Id`) e da 404
  quando o restaurante e de outra account. O default "Agencia" no modal mantem o caso comum na account
  ativa, onde nem precisa do `?account=`.

## Pendente (fora do MVP)

- Notificacao realtime de pedido novo (hoje: refresh manual / re-fetch ao trocar filtro).
- Dashboard de analytics dos eventos (a aba de eventos do contrato nao tem UI ainda).
- O wiring de menu ja foi aplicado (workspace `cardapio_web` em `workspaces.ts` + `permissions.ts`;
  gating em `module-enabled.global.ts`); a pagina aparece no menu. Nao existe `nav.config.ts` neste
  repo — o nav deriva de `workspaces.ts`/`permissions.ts`.
