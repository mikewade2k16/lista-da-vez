# AC-07b · Onda 2f — Cauda por área (spec-molde para os ~55 arquivos restantes)

> Spec de implementação (MOLDE parametrizado — executa-se em LOTES, 1 agente por lote)
> Prioridade **P1** (cauda P2 na prática) · Esforço **L** (somado) · Impacto **médio**
> Origem: AC-07 recorte 2 · roadmap `ac-fixes-2026-07` → task `ac-07b-refactor-front-recorte-2`

## 1. Contexto

Depois das ondas 2a–2e restam ~55 arquivos entre 450–900 linhas espalhados por todas as áreas. Um
único agente não deve tentar tudo (contexto estoura e a qualidade cai) — esta spec define o MÉTODO
e os LOTES; cada lote é despachado como uma execução independente desta mesma spec, com o
parâmetro `LOTE=<id>`.

## 2. Objetivo e não-objetivos

**Objetivo:** zerar o censo >450 nas áreas em escopo, lote a lote, sem mudança de comportamento.
**Não-objetivos (FORA em TODOS os lotes):** área calendário (`**calendar**` — desenvolvimento
ativo); `layers/tasks/components/AppDatePicker.vue`; layer `finance/**` (frente própria);
`web-reference/`; arquivos de TESTE; qualquer arquivo que o re-censo mostre ≤450.

## 3. Regras de execução (todas as do bloco padrão da rodada, mais:)

1. **RE-CENSO OBRIGATÓRIO no início de cada lote** (números abaixo são de 03/07):

```powershell
Get-ChildItem web/app, web/layers -Recurse -Include *.vue,*.ts,*.js |
  Where-Object { $_.FullName -notmatch 'node_modules|\.nuxt|\.output|dist|test|spec|calendar|AppDatePicker|web-reference|layers\\finance' } |
  ForEach-Object { [pscustomobject]@{ Lines = (Get-Content $_.FullName | Measure-Object -Line).Lines; Path = $_.FullName } } |
  Where-Object { $_.Lines -gt 450 } | Sort-Object Lines -Descending | Format-Table -AutoSize
```

2. Métodos por tipo (AC-07 §3, moldes vivos): `.vue` → subcomponentes + `use<X>Controller`
   (`components/operation/finish/`); store → factory slices (`stores/dashboard/runtime/`);
   composable → sub-composables; dado/types → barril (`roadmap-data.ts`). SEMPRE casca+barril.
3. Um lote por vez; `vue-tsc --noEmit` + `vitest run` (container) verdes ao fim de CADA lote.
4. Arquivo em uso por outra frente ativa (conferir `git status` do dia) → PULAR e registrar.

## 4. Lotes (executar na ordem; linhas = censo 03/07)

### Lote 2f-1 — tasks components (~3,5k) — NÃO paralelizar com 2a
`layers/tasks/components/TasksBoardView.vue` 883 · `TasksTaskModal.vue` 790 ·
`inputs/OmniSelectInput.vue` 713 · `inputs/OmniSelectMenuInput.vue` 561 ·
`omni/table/OmniDataTable.vue` 526. **Regra do espelhamento modal/board vale dobrado aqui.**

### Lote 2f-2 — operação restante + dashboard shell (~2,8k) — NÃO paralelizar com 2b
`app/components/operation/OperationQueueColumns.vue` 666 · `OperationOverviewBoard.vue` 516 ·
`OperationSidePanel.vue` 475 · `app/components/dashboard/DashboardUnifiedHeader.vue` 671 ·
`DashboardSidebarNav.vue` 498. (Cronômetro ancorado no servidor — não tocar na âncora.)

### Lote 2f-3 — stores remanescentes + motor runtime (~3,4k) — após 2c
`app/stores/dashboard/runtime/state.ts` 841 · `runtime/actions/settings-actions.ts` 578 ·
`app/stores/cardapio.ts` 654 · `feedback.ts` 488 · `users.ts` 486 · `meta-ads.ts` 483.
(state.ts se divide por domínio de estado; ids de store imutáveis.)

### Lote 2f-4 — cardápio (~2,7k)
`app/components/cardapio/sections/CardapioSectionCategorias.vue` 601 ·
`CardapioSectionSite.vue` 531 · `CardapioSectionAvaliacoes.vue` 502 ·
`product/CardapioProductModal.vue` 476 · `app/domain/cardapio/types.ts` 600 (barril de types).
**PATCH do painel cardápio manda body COMPLETO (memória) — não tocar shapes de submit.**

### Lote 2f-5 — workspaces grandes A (~3,9k)
`app/components/campaigns/CampaignWorkspace.vue` 842 · `banco/BancoSettingsSchema.vue` 795 ·
`alerts/AlertsWorkspace.vue` 765 · `tenants/TenantsWorkspace.vue` 754 ·
`reports/ReportsWorkspace.vue` 709.

### Lote 2f-6 — workspaces grandes B + feedback (~3,4k)
`app/components/feedback/UserFeedbackWorkspace.vue` 722 · `FeedbackFormModal.vue` 536 ·
`consultant/ConsultantRecentAttendancesTable.vue` 683 · `ConsultantDetailsDrawer.vue` 579 ·
`ConsultantPlayerCard.vue` 471 · `tenants/ClientsAdminWorkspace.vue` 583.
(Feedback: unread_count/preview vêm do LIST — não reintroduzir fan-out de mensagens.)

### Lote 2f-7 — roadmap (~3,8k)
`app/components/roadmap/RoadmapTimeline.vue` 775 · `RoadmapDatabaseSchema.vue` 696 ·
`RoadmapDatabaseDiagram.vue` 664 · `RoadmapModuleCard.vue` 652 · `RoadmapModulesBoard.vue` 549 ·
`RoadmapModuleTasksModal.vue` 481.

### Lote 2f-8 — site/bio/theme + avulsos (~5,6k; pode dividir em 2 se estourar contexto)
`app/components/site/SiteProductsWorkspace.vue` 642 · `SiteTrackingAdminWorkspace.vue` 595 ·
`SiteProductsAdminWorkspace.vue` 530 · `bio/BioEditorWorkspace.vue` 503 ·
`layers/core/composables/useThemeStudio.ts` 621 · `useOmniTheme.ts` 462 ·
`theme/studio/ThemeStudioSimplePanel.vue` 452 · `app/components/ui/AppEntityGrid.vue` 650 ·
`bi/BiWorkspace.vue` 592 · `multistore/MultiStoreUserAccessCard.vue` 559 ·
`users/UsersRoleMatrixManager.vue` 499 · `admin/AdminUsersWorkspace.vue` 456 ·
`ranking/RankingDetailsDrawer.vue` 455 · `automation/AutomationKnowledgeCard.vue` 454 ·
`crm/CrmWorkspace.vue` 456 · `app/domain/utils/erp-display.ts` 580 (barril).

## 5. Critérios de aceite (POR LOTE)

1. Todos os arquivos do lote ≤450 (originais viram casca; novos ≤450).
2. `vue-tsc --noEmit` + `vitest run` verdes sem editar testes existentes.
3. Grep de consumidores: zero mudança de import fora do lote.
4. Smoke da(s) página(s) da área com o dono (1 fluxo principal por arquivo tocado).
5. Registro no README da rodada: lote concluído + re-censo (quantos >450 restam).

## 6. Validação

Igual às ondas anteriores (type-check + vitest via container + smoke). Ao fim do ÚLTIMO lote, rodar
o re-censo completo e colar o resultado (esperado: só exclusões declaradas acima de 450).

## 7. Notas de Deploy

Nenhuma migration/env; rebuild web por lote deployado; rollback = reverter arquivos do lote.

## 8. Arquivos tocados

Por lote (tabela acima). AGENT.md: `web/AGENT.md` ao fim de cada lote.

**Conflitos potenciais:** 2f-1↔2a, 2f-2↔2b, 2f-3↔2c (respeitar a ordem/serialização indicada);
lotes diferentes de 2f podem rodar em paralelo se as áreas não se cruzarem (ex.: 2f-4 ‖ 2f-7 ok).
