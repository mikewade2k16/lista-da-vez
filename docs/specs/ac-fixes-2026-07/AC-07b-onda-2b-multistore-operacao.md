# AC-07b · Onda 2b — Refactor multistore + operação (4 arquivos, ~6,2k linhas)

> Spec de implementação · Prioridade **P1** · Esforço **L** · Impacto **médio**
> Origem: AC-07 recorte 2 · roadmap `ac-fixes-2026-07` → task `ac-07b-refactor-front-recorte-2`
> Censo 03/07 (re-medir): `app/components/multistore/MultiStoreGoalsSection.vue` **2.334** ·
> `app/components/operation/finish/useFinishModalController.js` **2.117** ·
> `app/components/operation/OperationProductPicker.vue` **883** ·
> `app/components/multistore/MultiStoreLojasSection.vue` **865**

## 1. Contexto

Dois hot-spots operacionais: a seção de METAS do multiloja (o maior .vue do repo) e o controller do
modal de encerrar atendimento — que era o EXEMPLO do recorte 1 e voltou a estourar (2.117 linhas).
São os arquivos com maior churn da operação; cada bug/feature ali paga contexto gigante.

Moldes: subcomponentes+controller em `app/components/operation/finish/` (steps já existem:
`FinishStepClient/Notes/Outcome/Product.vue`); slices de store em `app/stores/dashboard/runtime/`.

**Regras de domínio a preservar (memórias do projeto):**
- Modal de encerrar: o lookup por serviceId varre `storeSnapshots` (visão integrada do admin:
  `integratedStoreId ≠ activeStoreId`) — NÃO simplificar esse caminho.
- Cronômetros ancorados no servidor (`serverTime + performance.now()` — nunca `Date.now()` de parede).
- Nada de remover funcionalidade para "simplificar" (features coexistem).

## 2. Objetivo e não-objetivos

**Objetivo:** os 4 arquivos ≤450 (cascas) com lógica em módulos ≤450; comportamento idêntico.
**Não-objetivos (FORA):** `OperationQueueColumns.vue`/`OperationOverviewBoard.vue`/`OperationSidePanel.vue`
(onda 2f); stores `operations.ts`/`multistore.ts` (onda 2c); qualquer mudança visual/funcional.

## 3. Regras de execução (obrigatórias)

Mesmo bloco da onda 2a: sem git; re-medir/re-ler antes; casca+barril (API pública congelada);
≤450 em tudo; type-check+vitest via container; `web/AGENT.md`. Extra desta onda:
- `useFinishModalController.js` é **JS** — manter `.js` na casca (não converter para TS nesta onda;
  conversão muda superfície de erro e mistura refactor com migração).
- A visão integrada multi-loja da operação tem testes manuais sensíveis — o smoke com o dono é
  OBRIGATÓRIO antes de dar por concluído.

## 4. Mudanças (passo a passo)

### 4.1 `MultiStoreGoalsSection.vue` (2.334 → casca ≤450)

1. Ler e mapear: o arquivo mistura grade de metas por loja, edição inline, comparativos e regras de
   penalidade/aviso (princípio 5: avisos inline de meta ausente — PRESERVAR).
2. Extrair para `app/components/multistore/goals/`:
   - `useGoalsSectionController.ts` (lógica: carregar/salvar metas, drafts com `touched` — atenção
     à regra "draft re-hidrata do back"; estado de linhas, validações);
   - subcomponentes presentacionais: `GoalsStoreRow.vue`, `GoalsEditorCell.vue`,
     `GoalsSummaryHeader.vue` (nomes conforme a leitura; ≤450 cada).
3. A casca mantém o nome/props/emits e compõe controller + filhos.

### 4.2 `operation/finish/useFinishModalController.js` (2.117 → casca ≤300)

1. Criar `app/components/operation/finish/controller/` com módulos JS por responsabilidade —
   esperado pela leitura: `state.js` (refs/estado do fluxo), `steps.js` (navegação entre steps),
   `product.js` (seleção de produto), `outcome.js` (resultado/motivos), `submit.js`
   (montagem do payload + chamada + revalidação do snapshot), `snapshot-lookup.js`
   (**o lookup por serviceId varrendo storeSnapshots — mover INTEIRO, sem mexer na lógica**).
2. `useFinishModalController.js` vira orquestrador: importa os módulos, monta o mesmo objeto de
   retorno (mesmas chaves — os `FinishStep*.vue` consomem esse contrato).
3. Zero mudança nos `FinishStep*.vue` nesta onda.

### 4.3 `OperationProductPicker.vue` (883 → ≤450)

Extrair `app/components/operation/product-picker/`: `useProductPicker.ts` (busca/filtragem/estado)
+ `ProductPickerList.vue`/`ProductPickerItem.vue` conforme o template. Casca mantém props/emits.

### 4.4 `MultiStoreLojasSection.vue` (865 → ≤450)

Extrair `app/components/multistore/lojas/`: controller (drafts por linha com `touched`/`rowBusy` —
**preservar a regra do registro de falhas nº 9: draft NUNCA vence o dado autoritativo**) +
subcomponentes de linha/células. Casca idêntica por fora.

### 4.5 Conferência de consumidores

`grep -rn "MultiStoreGoalsSection\|useFinishModalController\|OperationProductPicker\|MultiStoreLojasSection" web/`
→ zero mudanças fora dos arquivos tocados.

## 5. Critérios de aceite

1. `wc -l` ≤450 nas 4 cascas e em todos os novos.
2. `vue-tsc --noEmit` + `vitest run` verdes (testes existentes intocados — `multistore.test.ts` e
   `operations.test.ts` são os detectores de contrato).
3. Smoke com o dono: (a) admin na visão integrada consegue ABRIR e CONCLUIR o modal de encerrar em
   loja integrada (o caso `integratedStoreId ≠ activeStoreId`); (b) editar meta no multiloja, salvar,
   RECARREGAR → valor persiste (regra do draft); (c) picker de produto busca e seleciona.
4. Cronômetros da operação continuam ancorados no servidor (nenhum `Date.now()` novo).

## 6. Validação

```bash
docker compose run --rm web npx vue-tsc --noEmit
docker compose run --rm web npx vitest run
npm run dev:watch   # smoke: /operacao (modal encerrar na visão integrada) + /multiloja (metas)
```

## 7. Notas de Deploy

Nenhuma migration/env. Rebuild web no próximo deploy. Rollback: reverter arquivos.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `app/components/multistore/MultiStoreGoalsSection.vue` | editar (casca) |
| `app/components/multistore/goals/*` | criar |
| `app/components/operation/finish/useFinishModalController.js` | editar (casca) |
| `app/components/operation/finish/controller/*.js` | criar (~6) |
| `app/components/operation/OperationProductPicker.vue` | editar (casca) |
| `app/components/operation/product-picker/*` | criar |
| `app/components/multistore/MultiStoreLojasSection.vue` | editar (casca) |
| `app/components/multistore/lojas/*` | criar |
| `web/AGENT.md` | editar |

**Conflitos potenciais:** onda 2c (stores `operations.ts`/`multistore.ts`) — pode rodar em paralelo
POR ÁREA DE ARQUIVO diferente, mas recomenda-se sequencial (2b → 2c) porque os controllers novos
importam esses stores. Onda 2f toca outros arquivos de operação — não paralelizar 2b‖2f-operacao.
