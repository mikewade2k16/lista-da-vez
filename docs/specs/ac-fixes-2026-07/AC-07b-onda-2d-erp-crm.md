# AC-07b · Onda 2d — Refactor ERP/CRM (5 arquivos, ~5,1k linhas)

> Spec de implementação · Prioridade **P1** · Esforço **L** · Impacto **médio**
> Origem: AC-07 recorte 2 · roadmap `ac-fixes-2026-07` → task `ac-07b-refactor-front-recorte-2`
> Censo 03/07 (re-medir): `app/components/erp/ErpCrmWorkspace.vue` **1.329** ·
> `app/components/crm/CrmConsultantsSection.vue` **912** · `app/components/erp/ErpDataTable.vue` **781** ·
> `app/composables/useErpWorkspace.ts` **605** · `app/components/erp/ErpProductsTable.vue` **469**

## 1. Contexto

A área ERP/CRM concentra workspaces e tabelas gigantes com lógica de exibição misturada
(formatação de moeda/período, colunas configuráveis, filtros locais). Boa parte da formatação já
vive em `app/domain/utils/erp-display.ts` — extração deve REUSAR o que existe lá, não duplicar.
Referência de grade administrativa sem `<table>`: `UsersAccessManager.vue` (padrão do projeto).

## 2. Objetivo e não-objetivos

**Objetivo:** os 5 arquivos ≤450 (cascas), lógica em controllers/subcomponentes ≤450, comportamento
idêntico.
**Não-objetivos (FORA):** `app/stores/erp.ts` (onda 2c); `domain/utils/erp-display.ts` (onda 2f, se
ainda >450 lá); `CrmWorkspace.vue` 456 (2f); mudanças de layout/colunas/regras de CRM.

## 3. Regras de execução

Bloco padrão da rodada (sem git; re-medir/re-ler; casca+barril; ≤450; type-check+vitest via
container; AGENT.md). Extras:
- REUSAR helpers de `erp-display.ts`/`crm-*.ts` existentes; extração NÃO cria segundo formatador.
- Tabelas: colunas configuráveis e ordenação são FEATURES — preservar 1:1 (nada de "simplificar").
- Memória do CRM: `crm-list-usage`/`crm-performance-policy` têm testes — mantê-los verdes sem edição.

## 4. Mudanças (passo a passo)

### 4.1 `ErpCrmWorkspace.vue` (1.329 → ≤450)
Extrair `app/components/erp/crm-workspace/`: `useErpCrmController.ts` (orquestração: fontes de
dados, período, filtros, tabs) + subcomponentes por região do template (header/filtros, cards de
resumo, seções de tabela). Casca compõe.

### 4.2 `CrmConsultantsSection.vue` (912 → ≤450)
Extrair `app/components/crm/consultants/`: controller (métricas por consultor — reusar
`useCrmConsultantMetrics.ts` existente; NÃO duplicar cálculo) + `ConsultantRow/Card` presentacionais.

### 4.3 `ErpDataTable.vue` (781 → ≤450) e `ErpProductsTable.vue` (469 → ≤450)
Padrão comum de tabela: extrair `app/components/erp/table/` com `useErpTableState.ts`
(colunas visíveis, sort, paginação local) + células/linhas presentacionais compartilháveis entre as
duas tabelas QUANDO idênticas (o que aparece 2× vira compartilhado — princípio de componentização;
o que for específico fica específico).

### 4.4 `useErpWorkspace.ts` (605 → ≤450)
Sub-composables em `app/composables/erp-workspace/` (sync/status, filtros, seleção) + orquestrador
casca. Conferir consumidores por grep.

## 5. Critérios de aceite

1. `wc -l` ≤450 nas 5 cascas + novos.
2. `vue-tsc --noEmit` + `vitest run` verdes (testes de crm-* e erp-display intocados).
3. Smoke (dono): /erp carrega tabela com colunas configuráveis/sort; /crm seção de consultores com
   métricas idênticas (comparar 2-3 números antes/depois); export/sync manual ERP funcionando.
4. Zero mudança de import em consumidores.

## 6. Validação

```bash
docker compose run --rm web npx vue-tsc --noEmit
docker compose run --rm web npx vitest run
npm run dev:watch   # smoke /erp e /crm (dono)
```

## 7. Notas de Deploy
Nenhuma migration/env; rebuild web no próximo deploy; rollback = reverter arquivos.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `app/components/erp/ErpCrmWorkspace.vue` + `erp/crm-workspace/*` | editar + criar |
| `app/components/crm/CrmConsultantsSection.vue` + `crm/consultants/*` | editar + criar |
| `app/components/erp/{ErpDataTable,ErpProductsTable}.vue` + `erp/table/*` | editar + criar |
| `app/composables/useErpWorkspace.ts` + `erp-workspace/*` | editar + criar |
| `web/AGENT.md` | editar |

**Conflitos potenciais:** onda 2c (store `erp.ts`) — rodar 2c antes ou depois, nunca em paralelo
com 2d (ambos tocam a cadeia ERP).
