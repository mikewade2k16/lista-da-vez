# AC-07b · Onda 2a — Refactor da layer tasks (os 3 maiores arquivos do repo)

> Spec de implementação · Prioridade **P1** · Esforço **L** · Impacto **médio**
> Origem: AC-07 recorte 2 · roadmap `ac-fixes-2026-07` → task `ac-07b-refactor-front-recorte-2`
> Censo de 03/07 (re-medir antes de executar): `layers/tasks/composables/useTasksPageContext.ts`
> **3.029** linhas · `layers/tasks/stores/tasks.ts` **2.182** · `layers/tasks/pages/tasks.vue` **1.408**

## 1. Contexto

Os 3 maiores arquivos do frontend vivem na layer tasks e concentram ~6,6 mil linhas. O
`useTasksPageContext` é um "deus-composable" que orquestra workspace, views, campos, drag, modal,
realtime e comentários; o store `tasks.ts` mistura CRUD, filtros, colunas, presença e time
tracking; `tasks.vue` carrega template gigante + lógica de página. Todo agente que toca tasks paga
esse contexto inteiro.

Métodos e exemplos-molde (do AC-07 §3, JÁ APLICADOS no repo):
- composable → sub-composables + orquestrador: `layers/tasks/composables/` já tem
  `useTasksWorkspace`, `useTimeTracking`, `useTaskComments`, `useTaskRelations`, `useTaskPresence`,
  `useTrackingMetrics` — o padrão é ESTE, faltou terminar a extração.
- store setup → factory slices: molde em `web/app/stores/dashboard/runtime/` (`create-*.ts`,
  `state.ts`, `actions/*-actions.ts`).
- .vue → subcomponentes + controller: molde em `web/app/components/operation/finish/`.
- **Regra casca+barril:** o path original PERMANECE e reexporta a API pública idêntica — nenhum
  consumidor muda import; `defineStore('tasks', ...)` mantém o MESMO id.

## 2. Objetivo e não-objetivos

**Objetivo:** os 3 arquivos ≤450 linhas cada (casca) com a lógica distribuída em módulos ≤450,
comportamento IDÊNTICO (zero mudança funcional).

**Não-objetivos (FORA):** `layers/tasks/components/AppDatePicker.vue` (exclusão herdada);
`TasksBoardView.vue`/`TasksTaskModal.vue`/inputs (ficam para a onda 2f); qualquer mudança de
comportamento, naming de API pública, ou migração `clientId` (tasks-refactor-v2).

## 3. Regras de execução (obrigatórias)

- NENHUM comando git. Dev web = `npm run dev:watch`. Sem `npm install` (nenhuma dep nova).
- **RE-MEDIR e RE-LER cada arquivo ANTES de fatiar** (`wc -l`; o censo é de 03/07 e a área é ativa).
- **Regra do espelhamento modal/board:** qualquer ajuste que toque exibição de task no modal deve
  ser replicado no card do board e vice-versa (memória do projeto) — neste refactor NÃO deve haver
  mudança visual nenhuma; a regra existe para o revisor conferir.
- Máx 450 linhas por arquivo (inclusive os novos). TypeScript sem `any`; sem `console.log`.
- API pública congelada: ids de store, exports nomeados, props/emits, InjectionKeys, eventos.
- Validação: type-check + testes via container (`docker compose run --rm web npx vue-tsc --noEmit`
  e `docker compose run --rm web npx vitest run`) — aguardar aprovação do dono para rodar.
- Atualizar `web/AGENT.md` (ou AGENT da layer tasks se existir).

## 4. Mudanças (passo a passo)

### 4.1 `layers/tasks/composables/useTasksPageContext.ts` (3.029 → casca ≤300)

1. Mapear os blocos por responsabilidade (ler o arquivo; blocos esperados: estado da página/refs
   compartilhados, seleção/abertura de task, colunas/agrupamento, drag & drop, filtros/busca,
   sincronização com rota/URL, wiring de realtime/presença, handlers do modal).
2. Extrair cada bloco para `layers/tasks/composables/page-context/` como sub-composable puro que
   recebe dependências por parâmetro (NADA de estado module-level novo):
   `usePageSelection.ts`, `usePageColumns.ts`, `usePageDnd.ts`, `usePageFilters.ts`,
   `usePageRouteSync.ts`, `usePageModal.ts` (nomes ajustáveis ao que a leitura revelar; ≤450 cada).
3. `useTasksPageContext.ts` vira o ORQUESTRADOR: instancia os sub-composables na ordem certa,
   monta e retorna o MESMO objeto de contexto público (mesmas chaves, mesmos nomes).
4. Se houver um InjectionKey/provide: intacto, exportado do mesmo path.

### 4.2 `layers/tasks/stores/tasks.ts` (2.182 → casca ≤300)

1. Criar `layers/tasks/stores/tasks/` no molde de `app/stores/dashboard/runtime/`:
   `state.ts` (estado + tipos), e slices por domínio conforme a leitura — esperado:
   `crud-slice.ts`, `filters-slice.ts`, `columns-slice.ts`, `presence-slice.ts`,
   `tracking-slice.ts` — cada um `createXxxSlice(deps)` retornando o pedaço de
   state/getters/actions.
2. `tasks.ts` mantém `defineStore('tasks', () => { ... })` (ID IMUTÁVEL — realtime e persist
   dependem dele), monta os slices e faz spread no return, preservando TODOS os nomes exportados.
3. `layers/tasks/stores/tasks.test.ts` existente DEVE continuar verde sem edição (é o detector de
   quebra de API pública). Se um teste falhar, o refactor quebrou contrato — corrigir o refactor,
   NUNCA o teste.

### 4.3 `layers/tasks/pages/tasks.vue` (1.408 → casca ≤450)

1. Template: extrair regiões auto-contidas para subcomponentes presentacionais em
   `layers/tasks/components/page/` (esperado: toolbar/header de views, região de board/tabela,
   empty-states) — props/emits explícitos, sem acessar store direto quando o dado já vem do contexto.
2. Script: o que sobrar de lógica vai para os sub-composables do 4.1 (não criar um segundo
   controller paralelo).
3. `definePageMeta`/rotas/guards intactos.

### 4.4 Conferência de consumidores

`grep -rn "useTasksPageContext\|stores/tasks\|useTasksStore" web/ --include="*.vue" --include="*.ts"`
→ nenhum consumidor deve precisar de mudança. Se precisar, o refactor violou a regra casca+barril.

## 5. Critérios de aceite

1. `wc -l` ≤450 nos 3 originais e em TODO arquivo novo.
2. `docker compose run --rm web npx vue-tsc --noEmit` limpo; `npx vitest run` verde SEM editar
   testes existentes.
3. `/tasks` no browser: board, tabela, modal, drag, filtros, comentários, presença e time tracking
   funcionando como antes (checklist manual com o dono).
4. Nenhuma mudança de import em consumidores fora da layer.
5. Realtime de tasks segue funcionando (id do store inalterado; conferir `useTasksRealtime.test.ts` verde).

## 6. Validação

```bash
docker compose run --rm web npx vue-tsc --noEmit
docker compose run --rm web npx vitest run
npm run dev:watch   # smoke manual em /tasks (dono)
```

## 7. Notas de Deploy

Nenhuma migration/env. Rebuild web no próximo deploy (front mudou). Rollback: reverter os arquivos
(refactor puro, sem dado).

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `layers/tasks/composables/useTasksPageContext.ts` | editar (vira orquestrador) |
| `layers/tasks/composables/page-context/*.ts` | criar (~6 arquivos) |
| `layers/tasks/stores/tasks.ts` | editar (vira casca do store) |
| `layers/tasks/stores/tasks/*.ts` | criar (state + slices) |
| `layers/tasks/pages/tasks.vue` | editar |
| `layers/tasks/components/page/*.vue` | criar (2-4 arquivos) |
| `web/AGENT.md` (ou AGENT da layer) | editar |

**Conflitos potenciais:** onda 2f toca `TasksBoardView/TasksTaskModal` — NÃO executar 2a e 2f-tasks
em paralelo. AC-15b não toca tasks (sem conflito).
