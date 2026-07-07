# AC-07 — Arquivos gigantes no front (>450 linhas) + components.css monolítico

- **ID canônico:** AC-07 (fatos.json → achados_canonicos.AC-07)
- **Prioridade:** P1 · **Esforço:** L · **Impacto:** médio
- **Escopo desta spec:** apenas o **RECORTE 1** (arquivos >1.000 linhas) + fatiar `components.css`. Recortes 2+ ficam como backlog no fim.

## 1. Contexto (evidência arquivo:linha)

Medição `wc -l` em 2026-07-02 (confirmada nesta sessão):

| Arquivo | Linhas | Tipo | Ação nesta spec |
|---|---|---|---|
| `web/layers/tasks/composables/useTasksPageContext.ts` | 3.029 | composable | dividir por responsabilidade |
| `web/app/components/roadmap/roadmap-data.ts` | 2.614 | dado declarativo | split por array em arquivos de dados |
| `web/app/components/multistore/MultiStoreGoalsSection.vue` | 2.334 | .vue | subcomponentes + composable |
| `web/layers/tasks/stores/tasks.ts` | 2.182 | store setup | módulos importados |
| `web/app/stores/erp.ts` | 1.566 | store setup | módulos importados |
| `web/layers/tasks/pages/tasks.vue` | 1.408 | .vue (page) | subcomponentes + composable |
| `web/app/components/erp/ErpCrmWorkspace.vue` | 1.329 | .vue | subcomponentes + composable |
| `web/layers/tasks/components/AppDatePicker.vue` | 1.320 | .vue | **EXCLUÍDO** (ver §2) |
| `web/app/components/crm/CrmConsultantsSection.vue` | 1.047 | .vue | subcomponentes + composable |
| `web/app/components/omni/OmniEditor.vue` | 1.013 | .vue | subcomponentes + composable |
| `web/app/assets/styles/components.css` | 4.480 | CSS monolítico | fatiar por domínio via `@import` |

**Estrutura confirmada por leitura:**
- Stores (`tasks.ts:829`, `erp.ts:386`) usam **setup-store** (`defineStore('x', () => { ... })`) — a extração é por closures/factories, não por `state/actions` keys.
- `roadmap-data.ts` é puro dado: `ROADMAP_GROUPS` (63), `ROADMAP_PHASES` (152–2139, ~1.988 linhas — o grosso), `ROADMAP_MODULES` (2140), `ROADMAP_RULES` (2403), + labels no fim. Já está no `ignores` do ESLint (`web/eslint.config.mjs:19`) e no `.prettierignore`. **10 consumidores** importam de `~/components/roadmap/roadmap-data` (RoadmapWorkspace, RoadmapTimeline, RoadmapModulesBoard/Card/Form, RoadmapRulesBoard/Card/Form, RoadmapModuleTasksModal, `stores/roadmap.ts:12`).
- `.vue` alvos têm blocos bem separados (ex.: `MultiStoreGoalsSection.vue` — script 1–1177, template 1178–1730, style 1731+).
- `components.css` é carregado 1× em `web/nuxt.config.ts:180` (`'~/assets/styles/components.css'`). Tem só 6 comentários-banner; o split precisa ser por **prefixo de seletor** (ver §4.7).
- ESLint já bloqueia regressão: `max-lines: ['warn', { max: 500, skipBlankLines: true, skipComments: true }]` (`eslint.config.mjs:41`). WARN hoje; a meta dos princípios é 450.

**Regra crítica de concorrência (roadmap-data.ts):** outro agente adiciona uma **fase nova** ao `ROADMAP_PHASES` em paralelo. O executor **deve re-ler `roadmap-data.ts` imediatamente antes de editar** e preservar 100% do conteúdo atual (inclusive a fase recém-adicionada). Nunca copiar o array a partir desta spec.

## 2. Objetivo / não-objetivos

**Objetivo:** reduzir os arquivos do RECORTE 1 para **≤450 linhas cada** (resultado final), com **comportamento e visual IDÊNTICOS**, imports atualizados e sem novo mock/dado inventado.

**Não-objetivos / exclusões (fechadas):**
- **NÃO tocar na layer `web/layers/finance/`** (outro agente atua nela). Nada em `web/server/` finance.
- **EXCLUIR `AppDatePicker.vue` do recorte** — memória do projeto (`project_appdatepicker`): está grande por razão (locale pt-BR, dias via slot, range mode, `formatDisplay`). Não fatiar agora.
- Não mudar nenhum seletor CSS, nenhuma classe usada no template, nenhuma assinatura pública de store/composable/componente.
- Não introduzir lib nova, não mexer em roteamento, não migrar dado para o banco.
- Recortes 2+ (arquivos 500–1.000) são só backlog documentado (§9), não implementar.

## 3. Método replicável por tipo (decisões fechadas)

Aplicar o método do tipo correspondente; sempre manter o arquivo "casca" reexportando a API pública para **zero mudança nos consumidores**.

- **composable** → extrair sub-composables por responsabilidade num diretório-irmão; o composable original vira orquestrador que compõe e reexpõe o mesmo objeto de retorno.
- **.vue** → extrair (a) **subcomponentes presentacionais** (template + estilo scoped locais, recebem props/emits) e (b) **um composable de lógica** (estado + handlers). O `.vue` original mantém o mesmo nome, props, emits e classes de template.
- **store (setup-store)** → extrair grupos coesos de estado+ações para **factories** (`createXxxSlice(deps)`) em arquivos-módulo; a store principal chama as factories e faz spread no `return`. Mantém `defineStore('id', ...)` e o mesmo `id` e a mesma superfície pública.
- **dado declarativo** → mover cada array grande para um arquivo de dados próprio; o arquivo original vira **barril** que reexporta tudo (types + arrays + labels), preservando os mesmos nomes exportados.
- **CSS** → recortar por domínio em `assets/styles/components/*.css` e transformar `components.css` num **manifesto de `@import`** na mesma ordem-fonte (cascata preservada). Zero alteração de seletor/propriedade.

## 4. Mudanças passo a passo (arquivos exatos)

> Ordem sugerida: 4.7 (CSS, isolado e de baixo risco) → 4.2 (roadmap-data) → stores → composable → .vue. Cada item é independente; se algum precisar parar, os demais seguem.

### 4.1 Regra geral de "casca + barril"
Para todo arquivo dividido, o path original **permanece** e reexporta a API pública idêntica. Consumidores não mudam de import salvo onde esta spec disser explicitamente. Todo arquivo novo criado deve, ele próprio, ficar ≤450 linhas.

### 4.2 `roadmap-data.ts` (dado) — **RE-LER ANTES DE EDITAR**
1. **Re-ler o arquivo inteiro agora** (o outro agente pode ter adicionado uma fase). Trabalhar sobre o conteúdo lido, nunca sobre trechos desta spec.
2. Criar `web/app/components/roadmap/data/`:
   - `phases.ts` → `export const ROADMAP_PHASES: RoadmapPhase[] = [...]` (o array atual de `ROADMAP_PHASES`, ~1.988 linhas → se ainda >450, dividir em `phases-part1.ts`, `phases-part2.ts`… e concatenar em `phases.ts` via spread; cada part ≤450).
   - `modules.ts` → `ROADMAP_MODULES`.
   - `rules.ts` → `ROADMAP_RULES`.
   - `groups.ts` → `ROADMAP_GROUPS`.
   - `labels.ts` → `ROADMAP_TITLE`, `ROADMAP_SUBTITLE`, `ROADMAP_MODULE_STATUS_LABEL`, `ROADMAP_PRIORITY_LABEL`, `ROADMAP_RULE_CATEGORY_LABEL`.
   - `types.ts` → `PhaseStatus`, `RoadmapTask`, `RoadmapPhase`, `RoadmapGroup`, `ModuleStatus`, `ModulePriority`, `RoadmapModule`, `RuleCategory`, `RoadmapRule` (mover as interfaces/types).
3. `roadmap-data.ts` vira **barril**: `export * from './data/types'` + `export * from './data/groups'` + `…/phases` + `…/modules` + `…/rules` + `…/labels`. Todos os 10 consumidores continuam importando de `~/components/roadmap/roadmap-data` sem alteração.
4. Ajustar `web/eslint.config.mjs` `ignores` (linha 19): manter `roadmap-data.ts` e **adicionar** `'app/components/roadmap/data/**'`. Ajustar `web/.prettierignore` do mesmo modo (é dado gerado/volumoso).
5. Decisão fechada: `ROADMAP_PHASES` é dado, não código — dividir por tamanho de linha é aceitável desde que a **ordem das fases** seja idêntica à do arquivo lido.

### 4.3 `web/layers/tasks/stores/tasks.ts` (store)
1. Criar `web/layers/tasks/stores/tasks/` com módulos-factory que recebem deps (refs, apiRequest, accountId) e retornam `{ state, actions }`:
   - `board-loading.ts` → `listBoardTasks`, `ensureBoardTasksLoaded`, `loadArchivedForBoard`, `replaceBoardTasks`, sets `loadedBoardIds/archivedBoardIds` e fetches-em-voo (lógica dos comentários em `tasks.ts:848–1141`).
   - `account-sync.ts` → o `watch(accountId, …)` de troca de conta (`tasks.ts:1285–1304+`) e auto-create guardado (`1249–1285`).
   - `mappers.ts` → helpers puros de mapeamento `BackendTask -> item` e o bloco de `clientId/clientAccountId` legado (`tasks.ts:734–828`, incluindo os comentários MOCK — **preservar comentários**).
2. `tasks.ts` mantém `defineStore('tasks', () => { … })`, compõe as factories e faz spread no return. **Não mudar o `id` 'tasks'** nem nomes de ações/getters (consumidores e devtools dependem).
3. **Espelhamento modal/board (feedback_modal_board_mirror):** se qualquer ajuste tocar comportamento de card/modal, replicar no par correspondente. Aqui é refactor sem mudança de comportamento → nenhuma divergência deve surgir; verificar no aceite.

### 4.4 `web/app/stores/erp.ts` (store)
1. Ler o arquivo e agrupar por domínio (ex.: filtros/seleção de loja, carga de dados ERP, consultores/metas, CRM). Criar `web/app/stores/erp/` com factories análogas ao 4.3.
2. `erp.ts` mantém `defineStore('erp', () => …)`, mesmo `id`, spread no return. Superfície pública inalterada.

### 4.5 `useTasksPageContext.ts` (composable, 3.029)
1. O composable já importa muitos sub-composables (`useTasksWorkspace`, `useTimeTracking`, `useTaskComments`, etc. — `:6–31`). Extrair mais responsabilidades para `web/layers/tasks/composables/page-context/`:
   - `use-task-draft.ts` → estado do `taskDraft`, autosave (`TASK_AUTOSAVE_DELAY_MS`, timers, assinaturas `lastSaved*`, hidratação) — bloco `:222–261+`.
   - `use-task-video-draft.ts` → `taskVideoDrafts`, `taskVideoSaving/Error`, normalização (usa os helpers `sharedNormalizeTaskVideoItem*`).
   - `use-board-editing.ts` → `columnDraft`, `projectSettingsDraft`, `creatingCards`, drafts de campo (`draftAddedFields/draftMenuOpen/draftFieldOpen`), drag (`dragKind/dropTarget/draggingTaskId`).
   - `use-filters-view.ts` → `filters`, `viewMode`, `tableSelectedRows`, `tableFocusCell`, `FIELD_DEFS/filterSwitchDefs/cardFieldSwitchDefs`, `BOARD_GROUP_OPTIONS`, `PRIORITY_OPTIONS`, `COLUMN_COLOR_OPTIONS`.
2. `useTasksPageContext` vira orquestrador: instancia os sub-composables, injeta deps compartilhadas, e **mantém idêntico** o objeto de `return {...}` (`:527`) e a `InjectionKey TASKS_PAGE_CONTEXT_KEY` (`:47`). Nada muda para quem faz `inject(TASKS_PAGE_CONTEXT_KEY)`.
3. Constantes puras (`ORDER_STEP`, `PRIORITY_OPTIONS`, `DEFAULT_FILTERS`, defs) podem ir para `page-context/constants.ts` e ser importadas.

### 4.6 Componentes `.vue` (MultiStoreGoalsSection, tasks.vue, ErpCrmWorkspace, CrmConsultantsSection, OmniEditor)
Para cada um, na sua própria pasta:
1. **Composable de lógica** `use<Nome>.ts` no diretório-irmão (ex.: `web/app/components/multistore/use-multistore-goals.ts`) com o estado + handlers do `<script setup>`.
2. **Subcomponentes presentacionais** para blocos repetidos/coesos do `<template>` (ex.: linha de meta, header, tabela) recebendo props/emits. Estilo scoped daquele bloco migra junto.
3. O `.vue` original mantém **nome, props, emits e classes de template** (para não quebrar seletores de `components.css`). Ele passa a compor o composable + subcomponentes.
4. Alvos e limites de blocos confirmados:
   - `MultiStoreGoalsSection.vue`: script 1–1177 / template 1178–1730 / style 1731–2334.
   - `ErpCrmWorkspace.vue`: script 1–422 / template 423–823 / style 824–1329.
   - `CrmConsultantsSection.vue`: script 1–449 / template 450–708 / style 709–1047. (Reaproveitar `.consultant-*` de `components.css:2045`.)
   - `OmniEditor.vue`: script 1–606 / template 607–858 / style 859–1013. **Cuidado:** `OmniEditor` é lazy real (`utils/lazy-component.ts`) — manter default export do componente e o nome.
   - `tasks.vue` (page): script 1–40 / template 41–124 / style 125–1408. O grosso é `<style>` — extrair para `web/layers/tasks/pages/tasks.page.css` importado no `<style>` via `@import`, OU mover regras para arquivos de estilo da layer; template já é pequeno.
5. **Espelhamento modal/board:** `tasks.vue` e componentes de card/modal de tasks — qualquer mudança em um deve refletir no espelho. Refactor não deve alterar comportamento; validar no aceite.

### 4.7 `components.css` (4.480) → manifesto de `@import`
1. Criar `web/app/assets/styles/components/` e recortar por **domínio de prefixo** (contagem confirmada). Sugestão de arquivos (fechada; cada um ≤450 linhas — se um domínio estourar, sufixar `-1/-2`):
   - `product.css` (99 seletores `.product*`, inclui "Product Pick widget" `:3338`)
   - `admin.css` (`.admin*`), `settings.css` (`.settings*`)
   - `operation.css` (`.operation*`, `.finish*`, `.queue*`, `.service*`, `.employee*`)
   - `reports.css` (`.report*`, `.dist*`, `.chart*`, `.ranking*`, `.metric*`, `.summary*`, `.insight*`, `.intel*`, "Report chart grids/Distribution/Hourly SVG" `:1959–2044`)
   - `consultant.css` (`.consultant*`, "Consultant goal rows" `:2045`)
   - `multistore.css` (`.multistore*`), `campaign.css` (`.campaign*`), `meeting.css` (`.meeting*`)
   - `misc.css` (`.ui*`, `.option*`, `.progress*`, `.profile*`, `.workspace*`, `.modal*`, `.brand*`, `.alert*`, `.simulator*`, `.catalog*`, `.dashboard*`, `.column*`, `.team*`, `.loading*`, `.auth*`, `.omni*`, `.app*` e o que sobrar)
2. `components.css` vira **apenas** uma lista de `@import './components/xxx.css';` **na mesma ordem em que os blocos apareciam no arquivo original** (a cascata do CSS depende da ordem — preservar é obrigatório para zero mudança visual). Recorte por **corte contíguo de linhas** (mover blocos inteiros), não por reescrita de seletores.
3. **Não** registrar os novos arquivos em `nuxt.config.ts` — só `components.css` continua no array `css` (`:180`); os `@import` puxam o resto. Isso garante ordem e evita mudança de bundling.
4. Método seguro: mover blocos por faixa de linhas (extrair a faixa, colar no arquivo-domínio) e, no fim, `components.css` conter só os `@import`. Conferir que a soma de linhas dos arquivos-domínio ≈ 4.480 (nada perdido/duplicado). Como `@import` respeita a ordem de declaração, blocos que dependem de override tardio (ex.: um seletor sobrescrito mais abaixo) devem cair em arquivos cujo `@import` venha depois — por isso o corte é por faixa contígua, não por reordenar por domínio.

## 5. Critérios de aceite (verificáveis)

1. **Nenhum** dos arquivos do RECORTE 1 (exceto `AppDatePicker.vue`, excluído) tem >450 linhas: `wc -l` de cada path original **e** de cada arquivo novo criado ≤450.
2. `components.css` contém **somente** linhas `@import`/comentário; soma das linhas dos `components/*.css` ≈ total original (sem seletor perdido). `grep -cE '^\.' components.css` = 0 (nenhum seletor solto no manifesto).
3. Todos os imports de consumidores continuam resolvendo sem alteração de path para: `roadmap-data`, store `tasks`, store `erp`, `useTasksPageContext`/`TASKS_PAGE_CONTEXT_KEY`, e os 5 componentes `.vue` (mesmo nome/props/emits).
4. `id` das stores inalterado (`'tasks'`, `'erp'`); superfície pública (ações/getters/state expostos) idêntica.
5. `ROADMAP_PHASES` preserva ordem e conteúdo do arquivo **relido** (incluindo a fase que o outro agente adicionou).
6. `web/eslint.config.mjs` e `.prettierignore` cobrem `app/components/roadmap/data/**`.
7. Comportamento **idêntico** (validação §6): páginas Tasks, ERP/CRM, Multistore Goals, Roadmap e OmniEditor renderizam e operam como antes; visual sem diff.
8. AGENT.md dos módulos tocados atualizados (§8).
9. Zero mock/dado novo; nenhuma feature removida; layer `finance` intocada.

## 6. Validação (comandos — rodar só após aprovação do usuário p/ npm)

Front é `npm` → **não rodar sem o usuário aprovar** (memória `feedback_no_npm_until_approved`). Listar para aprovação:

```bash
# type-check + lint (WARN não bloqueia; olhar se max-lines caiu)
cd web && npm run lint
cd web && npx nuxi typecheck    # ou npm run typecheck se existir

# testes existentes (17 .test.ts) — nenhum deve quebrar
cd web && npm run test

# build de produção (confirma que @import do CSS e barris resolvem)
cd web && npm run build
```

Verificações manuais (sem build): `wc -l` nos paths (aceite 1/2), `grep` dos imports (aceite 3), abrir as páginas no dev e comparar visual (aceite 7). Não há mudança de backend nesta spec → sem `docker compose`.

## 7. Notas de Deploy

- **Sem migration, sem mudança de env/Dockerfile/deps.** Puramente refactor de front.
- Nenhuma mudança de rota, de contrato de API ou de dado.
- Risco de deploy: baixo, contido no bundle web. Um `npm run build` verde é a garantia. Reversível por git (arquivos originais preservados como casca).

## 8. Arquivos tocados

**Modificados (casca/barril/manifesto — mesma API):**
- `web/layers/tasks/composables/useTasksPageContext.ts`
- `web/app/components/roadmap/roadmap-data.ts`
- `web/app/components/multistore/MultiStoreGoalsSection.vue`
- `web/layers/tasks/stores/tasks.ts`
- `web/app/stores/erp.ts`
- `web/layers/tasks/pages/tasks.vue`
- `web/app/components/erp/ErpCrmWorkspace.vue`
- `web/app/components/crm/CrmConsultantsSection.vue`
- `web/app/components/omni/OmniEditor.vue`
- `web/app/assets/styles/components.css`
- `web/eslint.config.mjs`, `web/.prettierignore`

**Criados (todos ≤450 linhas):** `web/app/components/roadmap/data/*`; `web/layers/tasks/stores/tasks/*`; `web/app/stores/erp/*`; `web/layers/tasks/composables/page-context/*`; composables `use<Nome>.ts` + subcomponentes por `.vue`; `web/app/assets/styles/components/*.css`; `web/layers/tasks/pages/tasks.page.css`.

**AGENT.md a atualizar (feedback_agent_md):**
- `web/app/components/AGENT.md` (novos subcomponentes/estrutura; registrar a fatia de `components.css` e o manifesto de `@import`)
- `web/layers/tasks/AGENT.md` (split de store/composable/page)
- `web/app/components/erp/AGENT.md`, `web/app/components/crm/AGENT.md`, `web/app/components/omni/AGENT.md`
- Não existe AGENT.md em `multistore/` nem `roadmap/` — registrar essas mudanças no AGENT.md pai (`web/app/components/AGENT.md`), sem criar novos AGENT.md só por isso.

**NÃO tocar:** `web/layers/finance/**`, `web/server/**` (finance), `web/layers/tasks/components/AppDatePicker.vue`.

## 9. Backlog (Recortes 2+ — NÃO implementar aqui)

Documentar como pendência: os ~95 arquivos restantes entre 450 e 1.000 linhas (fatos.json: 105 no total). Priorizar por próxima onda stores/composables antes de `.vue`. Meta futura: virar `max-lines` de `warn`→`error` (450) no ESLint depois que o recorte estiver limpo (hoje `eslint.config.mjs:41` está em `warn`/500).

## 10. Regras de execução (obrigatórias)

- **Sem git** (nem add/commit/push): entregar as mudanças no working tree; o usuário roda git.
- **Sem npm/build sem aprovação:** implementar e parar; listar os comandos de §6 para o usuário aprovar. Validação de front é responsabilidade do usuário até o "aprovei".
- **Não há back nesta spec** → o orquestrador não precisa de `docker compose up -d --build api`. (Regra registrada por padrão; aqui é no-op.)
- **Máx 450 linhas por arquivo** — inclusive todo arquivo novo criado no split.
- **Migrations:** nenhuma nesta spec. (Se algum dia houver: SQL plano idempotente, sem `-- +goose Down`, próxima numeração ≥ 0187.)
- **Não remover funcionalidade** (feedback_no_remove_features): features coexistentes permanecem; refactor não altera comportamento.
- **Zero mock novo** (feedback_no_legacy_flag_mocks): não inventar dado; preservar comentários MOCK/legado existentes em `tasks.ts`.
- **Espelhamento modal/board** (feedback_modal_board_mirror): qualquer toque em card/modal de tasks replicado no par.
- **roadmap-data.ts:** RE-LER antes de editar (fase nova sendo adicionada por outro agente em paralelo). Nunca reconstruir o array a partir desta spec.
- **Não tocar na layer `finance`** (outro agente atua nela).
- **Atualizar AGENT.md** dos módulos tocados (§8).
