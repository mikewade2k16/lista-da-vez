# Specs dos subagentes — Performance (página online + correções)

> Frente derivada de [PERFORMANCE_AUDIT_PLAN.md §14](PERFORMANCE_AUDIT_PLAN.md). Roadmap: fase `perf-fixes` (pending).
> **Eu monto, você dispara.** Nenhum agente roda `git` (só você). Cada agente: respeita design tokens, usa `AdminPageHeader`, classes BEM, `page-workspace`/overflow; máx 450 linhas/arquivo; valida local no browser; ao final atualiza AGENT.md do módulo + a task correspondente em `roadmap-data.ts`.

Tracks A–D são **independentes** (arquivos quase disjuntos). Único ponto de atenção: todos podem tocar `roadmap-data.ts` no fim (marcar a própria task) e cada um seu AGENT.md — sem conflito real se cada um mexe só na sua linha. Só o Track A toca os arquivos de navegação.

---

## Track A — Página dedicada `/performance` (front)

**Objetivo:** página no menu lateral (platform_admin) que mostra os resultados da auditoria: tabela por rota com T1/T2/T3 (média), nos modos in-app e cold, ranking das mais lentas, e um bloco explicando o warm-up de dev.

**Fonte de dados:** o `qa-bot/perf_audit.py` passa a **emitir** `web/app/components/performance/perf-data.ts` (módulo TS tipado) ao final do run, a partir dos mesmos dados do relatório. Seed inicial: portar os números de `qa-bot/artifacts/perf-20260615-133516.csv` para esse arquivo, para a página já funcionar. Re-rodar a auditoria atualiza a página.

**Arquivos:**
- `qa-bot/perf_audit.py` — adicionar `write_perf_data_ts(rows, ...)` (chamado junto de `write_reports`) que gera `web/app/components/performance/perf-data.ts` (`export interface PerfRow { path; mode; t1; t2; t3; capped }` + `export const PERF_RUN = { stamp, baseUrl }` + `export const PERF_ROWS: PerfRow[]`).
- `web/app/pages/performance.vue` — `definePageMeta({ layout:'dashboard', workspaceId:'performance', pageLabel:'Performance' })`, usa `AdminPageHeader`, raiz `.page-workspace`.
- `web/app/components/performance/PerformanceWorkspace.vue` (+ sub-cards se passar de 450 linhas) — tabela/ranking/explicação do warm-up, tokens do design system, classes `.performance-workspace__…`.
- **Wiring do menu (regra dos 3 arquivos — ver ENGINEERING_PRINCIPLES registro 2026-05-29):**
  1. `web/app/utils/workspaces.ts` → entry no `WORKSPACES` (id `performance`, label, icon, path `/performance`).
  2. `web/app/domain/utils/permissions.ts` → entry em `WORKSPACE_ACCESS_DEFINITIONS` + adicionar `performance` em `ROLE_WORKSPACES.platform_admin`.
  3. `web/layers/queue/nav.config.ts` → item de menu (provável na seção `manage` ou `indicators`) com `workspaceId:'performance'`, `path:'/performance'`.

**Aceite:** logado como platform_admin, "Performance" aparece no menu e abre; a tabela mostra T1/T2/T3 por rota (in-app+cold) + ranking; respeita tema (toggle de PAGE HEADERS desliga o header); re-rodar `perf_audit.py` regenera `perf-data.ts` e a página reflete.

---

## Track B — Skeletons (`apply-operacao` + `apply-dashboard` da fase-9)

**Objetivo:** "responde na hora" nas duas telas que hoje pintam conteúdo só quando o dado chega. Skeleton < 100ms no clique, sem tela vazia.

**Contexto:** `CoreSkeleton.vue` (layer core) já existe com variantes (`card/table-row/text/avatar/block`). `AppEntityGrid` já usa para tabelas. Falta aplicar em:
- `/operacao` (web/app/pages/operacao/index.vue + workspace da operação): grid de lojas + faixa/fila **enquanto o realtime conecta** (T3 in-app ~3,4s, cold ~4,5s — é stream, mas hoje não há skeleton no meio).
- Dashboard `/` (web/app/pages/index.vue): skeleton dos cards iniciais.

**Restrições:** NÃO remover/alterar comportamento existente (realtime, faixa de consultores). Skeleton é aditivo: aparece no estado loading e some quando os dados chegam. Espelhar modal/board onde aplicável.

**Aceite:** ao abrir `/operacao` e `/` (sem cache), skeleton aparece imediatamente e é trocado pelo conteúdo real; nenhuma regressão na operação/realtime.

---

## Track C — `/tasks`: board que não settla (pior caso)

**Objetivo:** o board (247 cards, render progressivo) é a pior rota (cold T3 15s+, nunca quieta). Reduzir o custo de montagem sem quebrar o board já usável.

**Contexto/roadmap:** já anotado em `roadmap-data.ts` (fase tasks): "montar os selects pesados do card só ao clicar (hoje cada card monta vários `OmniSelectMenuInput` de uma vez) e/ou windowing real por viewport".

**Direção:** (1) montar os editores pesados do card (`OmniSelectMenuInput` etc.) **só ao interagir** (clique/focus), renderizando um placeholder leve antes; (2) avaliar windowing por viewport (render só dos cards visíveis). Medir antes/depois com `perf_audit.py --only /tasks --base-url http://localhost:3055`.

**Restrições:** board está EM USO REAL (Crow 247 tasks, Duby) — não quebrar drag, edição inline, realtime, tracking. Mudança é de performance de render, não de regra. Modal e board card espelhados.

**Aceite:** `/tasks` cold T3 cai para < 3s OU mostra skeleton/placeholder imediato e fica interativo rápido; nenhuma funcionalidade do board removida; re-medição registrada.

---

## Track D — `/erp` + `/manage/users` (listas pesadas)

**Objetivo:** as duas listas mais lentas depois das realtime (in-app ~1,8s / cold ~2,4–2,5s). Trazer para < 1,5s.

**Direção (espelha AGENT_RULES "pedir só o necessário" + §10.3):** projeção lean (só os campos que a tela usa, não o objeto inteiro); paginação server-side (cursor onde a lista cresce); gatear o fetch por permissão ANTES de disparar; sem N+1 (`WHERE id = ANY($1)`). Back: `back/internal/modules/crm/erp/` e o endpoint de usuários admin. Front: `web/app/pages/erp.vue` + `web/app/pages/manage/users.vue` (UsersWorkspace).

**Restrições:** isolamento multi-tenant intacto (escopo validado no service, nunca do client). Não mudar contrato sem registrar. Rebuild do back após alterar Go (`docker compose up -d --build api`).

**Aceite:** `/erp` e `/manage/users` < 1,5s (re-medir com `perf_audit.py`); mesmos dados na tela; sem vazamento cross-tenant; AGENT.md dos módulos atualizado.

---

## Como disparar

Frentes independentes → podem rodar em paralelo (Codex×Codex, Codex×Claude, ou subagentes). Sugestão de divisão: A (front isolado) + B (skeletons) + C (tasks) + D (erp/users). Mínimo viável: A + B primeiro (menor risco, cobre "responde na hora"), C e D depois. Antes de mover qualquer um, o build de prod precisa estar no ar para re-medição: `docker run --rm -d --name omni-web-prod -p 3055:3003 -e NUXT_PUBLIC_API_BASE=http://localhost:9091 -e NUXT_API_INTERNAL_BASE=http://host.docker.internal:9091 omni-web-prod`.
