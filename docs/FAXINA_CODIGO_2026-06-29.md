# Faxina de Código — 2026-06-29

> Resultado da **revisão tripla** de 2026-06-29 (todos os achados verificados adversarialmente). Segurança vive em [SECURITY_OPTIMIZATION_BACKLOG.md](SECURITY_OPTIMIZATION_BACKLOG.md) (seção "Revisão 2026-06-29", SEC-6 a SEC-14). Este doc cobre os **36 achados de faxina** (qualidade/manutenção): duplicação, código morto, over-engineering, tamanho de arquivo e performance.
>
> **Princípio:** zero remoção de feature. Nada aqui apaga comportamento — só consolida, divide ou enxuga. Refactors grandes ficam na **WAVE 2** (revisão dedicada) para não conflitar com a wave 1 (correções de segurança) nem estourar o orçamento de risco num único deploy.
>
> Status por item: **feito wave 1** (entrou junto das correções de segurança) · **pendente wave 2** (refactor dedicado).
>
> Economia de linhas = estimativa (LOC removidas/consolidadas), não medida exata.

Legenda: 🔴 alto impacto · 🟡 médio · 🟢 baixo.

---

## Atualização — Wave 2 (2026-06-29, commitada)

> Status **corrente** após wave 1 + wave 2. Este bloco SUPERSEDE os rótulos "pendente wave 2" das tabelas abaixo (que refletem o planejamento inicial). Gates da wave 2: `go build/vet/test` verde + `eslint` 0 erros + `vue-tsc` sem erro novo (o único, em `useRealtimeSocket`, corrigido).

**Feito na wave 2 (refactor puro):**
- Helpers consolidados em `back/internal/platform/stringsx` — `FirstNonEmpty` (F-01), `NormalizeIDs` (F-02), `DecodeJSONStringSlice` (F-03) + teste; ~18 call-sites migrados.
- Splits > 450 linhas: `AdminUsersWorkspace.vue` 881→491 (F-18), `AdminRoleMatrixEditor.vue` 633→492 (F-19), `admin_users_repository.go` 592→~420 (F-20); struct órfã `RoleTemplate` removida.
- Extrações: `useRealtimeSocket.ts` (socket WS compartilhado, §6.2), `useInlineEditManager.ts` (6 managers, §6.3), `useFeedbackChat.js` (chat feedback, §6.4).
- `filterPerolaERPWorkspaces` morto removido (task6).

**Adiado de propósito (continua PENDENTE):**
- **F-17 — split do `useTasksPageContext.ts` (3063 linhas):** único closure com refs compartilhados bidirecionais, 4 `watch` com flush post/sync/deep e ordem de `onMounted/onUnmounted` load-bearing. Fazer **incremental, com validação no browser**, começando pelos helpers puros (presence-draft, video-normalize).
- **F-04 — `slugify` (6 variantes):** NÃO consolidado — as variantes geram slugs diferentes (NFD × sem-acento × só-lowercase); unificar mudaria o slug gerado = **decisão de produto** sobre a regra canônica.
- Nuance assumida: `DecodeJSONStringSlice` padroniza `null→[]` em 3 campos de operations/reports (inócuo; documentado em `stringsx/AGENT.md`).

**Pré-existentes a atacar (escopo confirmado — "arrumar o que já existia"):** erros de `vue-tsc` da baseline (`api-client.ts`, `runtime-remote*.ts`, `AppDatePicker.vue`, `useTaskComments/Relations`, `useTasksPageContext`), N+1 do `MeAccounts` (F-22/OPT-2), projeção lean (F-26/OPT-4), os demais 🟡/🟢 abaixo (F-11/13/14/15/16/24/25), e o cleanup do `AUTH_ROLES_SOURCE` no compose (F-08).

---

## Atualização — Wave 3 (2026-06-29, commitada)

> Atacou parte dos pré-existentes acima. Modelos: opus high (tipagem api/runtime, N+1), sonnet high no resto. Todas as partições aprovadas na revisão; build da API verde + healthy.

**Feito:**
- **N+1 do MeAccounts/MeContext eliminado (F-22/OPT-2):** método batch `ListEnabledModuleIDsForAccounts` (`WHERE account_id = ANY($1::uuid[])`) + `loadModulesBatch` no service; nº de queries constante (antes 1 por conta — dói com platform_admin vendo todas). `MeContext` (N=1) segue no singular. `go build/vet/test` verde + imagem da API rebuildou e subiu healthy. **Validar no browser:** switcher de contas com os módulos certos por conta.
- **Tipagem (sem any/@ts-ignore):** `api-client.ts` (`ApiRequestMethod`), `runtime-remote*` (payloads tipados), `sourceValue`/casts em `useTaskComments`/`useTaskRelations`/`useTasksPageContext`, e a prop `creatable` em `OmniSelectInput`/`OmniSelectMenuInput` (cast que destrava `OmniDataTable`/`TasksBoardView`). `prioritySet` confirmado como sentinel de runtime (não é typo).
- **`AUTH_ROLES_SOURCE` (F-08):** removido dos compose/`.env*.example`; `auth/AGENT.md` e `LEGADO.md` corrigidos.

**Realidade do vue-tsc:** total caiu de **348 → 340** erros (zero erro novo introduzido). O baseline do projeto tem **~340 erros de tipo pré-existentes** (ex.: resultados de `apiRequest` sem genérico caindo em `unknown` — `useTaskComments`/`Relations`/`useTasksPageContext`; `AppDatePicker` com tipos do `@internationalized/date`). **"vue-tsc 100% verde" é uma frente dedicada** (centenas de erros), não fechável numa única wave.

---

## 1. Duplicação (helpers repetidos cross-módulo)

> Mesmos helpers reimplementados por módulo. Consolidar num pacote utilitário único (back) sem mudar comportamento — cada call-site continua chamando a mesma assinatura.

| # | Achado | Arquivo:linha (origens) | Economia est. | Status |
|---|--------|--------------------------|---------------|--------|
| F-01 🔴 | `firstNonEmpty` reimplementado **8x** | `back/internal/modules/queue/{reports,operations,analytics}/helpers.go`, `back/internal/modules/crm/erp/service_helpers.go`, `back/internal/modules/cardapio/service_public.go`, `back/internal/modules/queue/alerts/service.go`, `back/internal/modules/site/tracking_service.go`, `back/internal/modules/queue/settings/http.go` | ~60 linhas | pendente wave 2 |
| F-02 🟡 | `normalizeStoreIDs` duplicado cross-módulo (queue × crm/erp × tenants/stores scope) | `back/internal/modules/queue/operations/helpers.go`, `back/internal/modules/crm/erp/service_helpers.go` (+ usos em scope) | ~25 linhas | pendente wave 2 |
| F-03 🟡 | `decodeStringSlice` reimplementado **3x** | 3 ocorrências em `back/internal/modules/*` (jsonb → []string) | ~24 linhas | pendente wave 2 |
| F-04 🟡 | `slugify`/`Slugify` reimplementado **6x** | `back/internal/modules/bio/slug.go`, `back/internal/modules/cardapio/*`, `back/internal/modules/site/*`, `back/internal/modules/users/service.go` (variações) | ~40 linhas | pendente wave 2 |
| F-05 🟢 | Mapeamento de erro `403 vs 404` copiado por módulo (`writeServiceError`/`writeError`) | `*/http.go` + `errors.go` de vários módulos | ~30 linhas | pendente wave 2 (ligado a SEC-2/SEC-9) |
| F-06 🟢 | `clientIP` (parse de XFF/RemoteAddr) provável duplicado entre cardapio e httpapi | `back/internal/modules/cardapio/rate_limit.go:65`, `back/internal/platform/httpapi/*` | ~15 linhas | pendente wave 2 (ligado a SEC-12) |

**Consolidação cross-módulo (WAVE 2 dedicada):** `firstNonEmpty` (8x), `normalizeStoreIDs` (cross-módulo), `decodeStringSlice` (3x), `slugify` (6x) → mover para um pacote `internal/platform/stringsx` (ou similar) com testes; trocar os call-sites por import. Não altera comportamento.

---

## 2. Código morto

> Símbolos/branches sem uso vivo. Remoção segura só após grep zero-usos.

| # | Achado | Arquivo:linha | Economia est. | Status |
|---|--------|---------------|---------------|--------|
| F-07 🟡 | Fallback legado de roles no auth (dead code pós-drop das `user_*_roles`) | `back/internal/modules/auth/*` (resolveLegacyAuthRoleScope etc.) | ~120 linhas | **feito wave 1** (ver roadmap `lc-auth-fallback`) |
| F-08 🟡 | `AUTH_ROLES_SOURCE` virou no-op (flag morta) | `back/internal/modules/auth/*` + compose | ~10 linhas | **feito wave 1** (remover do compose no deploy-cleanup) |
| F-09 🟢 | Permissões `cardapio.view`/`cardapio.manage` declaradas mas nunca exigidas (decorativas) | `back/internal/modules/cardapio/module.go` (registry) × `http.go:17` | 0 (passam a ser usadas) | **feito wave 1** (deixaram de ser mortas via SEC-6) |
| F-10 🟢 | SQL manual obsoleto `unify_users_view.sql` (+ pasta `manual/`) nunca rodado | `manual/unify_users_view.sql` | arquivo inteiro | **feito wave 1** (ver roadmap `lc-manual-sql`) |
| F-11 🟢 | `imageScale: 'none'` grava valor inerte em `block.props` (lixo no jsonb) | TAVOLA `StudioBlockEditor` / `SectionRenderer` | trivial | pendente wave 2 (limpar p/ undefined) |
| F-12 🟢 | `PubAnnounceBar` hardcoded substituída pela barra do Studio (resíduo) | TAVOLA `public.vue` | ~10 linhas | **feito wave 1** (removida na fase 12) |

---

## 3. Over-engineering

> Abstração/configuração além do necessário hoje. Não remover capacidade real — só não pagar custo de manutenção por flexibilidade não usada.

| # | Achado | Arquivo:linha | Economia est. | Status |
|---|--------|---------------|---------------|--------|
| F-13 🟡 | `allow` é wrapper fino de `allowN(…,1,…)` — uma das duas pode ser inlined | `back/internal/modules/cardapio/rate_limit.go:29-31` | ~4 linhas | pendente wave 2 (manter ambas se houver call-sites distintos) |
| F-14 🟡 | Rate-limiter por IP em memória reimplementa o que o httpapi já faz por usuário (2 limitadores paralelos) | `back/internal/modules/cardapio/rate_limit.go` × `httpapi/ratelimit.go` | — | pendente wave 2 (avaliar, NÃO unificar sem manter rota pública sem-JWT) |
| F-15 🟢 | `sections/families/*` e `components.ts` gerados por `.work/gen-registry.cjs` mas editados à mão (gerador diverge) | TAVOLA `.work/defs/*` × gerados | — | pendente wave 2 (portar à-mão p/ `.work/defs` antes de rodar o gerador) |
| F-16 🟢 | Camadas de fetch redundantes no front quando o store já injeta escopo | composables de listagem (admin) | ~20 linhas | pendente wave 2 |

> ⚠️ **Não simplificar (feature, não over-engineering):** o limitador público do cardápio existe **de propósito** porque cobre rotas públicas sem JWT que o rate-limit por-usuário do httpapi não alcança (`rate_limit.go:13`). Coexistência mantida — só endurecer a chave (SEC-11) e o `clientIP` (SEC-12).

---

## 4. Tamanho de arquivo (> limite de 450 linhas — princípio de engenharia)

> Arquivos acima do teto de 450 linhas. Divisão por responsabilidade, sem mudar comportamento público. **Todos WAVE 2** (refactor dedicado, alto risco de conflito).

| # | Arquivo:linha | Linhas | Alvo da divisão | Status |
|---|---------------|--------|-----------------|--------|
| F-17 🔴 | `web/layers/tasks/composables/useTasksPageContext.ts` | **3063** | splittar por domínio (estado/board/realtime/inline-edit/presence) | pendente wave 2 |
| F-18 🟡 | `web/app/components/admin/AdminUsersWorkspace.vue` | **881** | extrair tabela + drawer + filtros em subcomponentes | pendente wave 2 |
| F-19 🟡 | `web/app/components/admin/users/AdminRoleMatrixEditor.vue` | **633** | extrair matriz + linha de papel + estado | pendente wave 2 |
| F-20 🟡 | `back/internal/modules/core/admin_users_repository.go` | **592** | separar loaders/agregados batch do CRUD básico | pendente wave 2 |
| F-21 🟢 | `web/app/components/roadmap/roadmap-data.ts` | grande (dados) | aceitável (arquivo de dados, não lógica) — não dividir | pendente wave 2 (avaliar) |

---

## 5. Performance

> Round-trips e padrões caros. Os N+1 críticos já foram tratados em fases anteriores; aqui ficam os resíduos/atenções.

| # | Achado | Arquivo:linha | Economia est. | Status |
|---|--------|---------------|---------------|--------|
| F-22 🟡 | `MeAccounts`/`ListAccountsForUser` ampliou o N+1 conhecido (platform_admin agora vê todas as contas) | `back/internal/modules/core/store_postgres.go` (ListEnabledModuleIDs por account) | — | pendente wave 2 (batch `WHERE account_id = ANY($1)`, ver OPT-2) |
| F-23 🟡 | Selects de feedback resolvendo escopo só na app (sem `tenant_id` na query) → leituras mais largas que o necessário | `back/internal/modules/queue/feedback/store_postgres.go:138,157,217` | — | **feito wave 1** (predicado `tenant_id`, ver SEC-10) |
| F-24 🟡 | Cards de tasks montam vários `OmniSelectMenuInput` pesados de uma vez | TAVOLA/tasks `TasksBoardView` (card) | — | pendente wave 2 (montar editores só ao clicar; ver roadmap `tasks-board-render-improve`) |
| F-25 🟢 | Rate-limit por IP varre o bucket inteiro a cada request (O(n) na janela) | `back/internal/modules/cardapio/rate_limit.go:47-53` | — | pendente wave 2 (aceitável no volume atual) |
| F-26 🟢 | Listagens devolvem objeto inteiro quando a tela usa poucos campos | repos de listagem (users/accounts/leads) | — | pendente wave 2 (projeção lean, ver OPT-4) |

---

## 6. Refactors grandes — WAVE 2 (revisão dedicada)

> Agrupados aqui porque exigem uma onda própria: tocam arquivos quentes, têm alto risco de conflito com a wave 1 e precisam de revisão/teste dedicados. **Nenhum remove feature.**

1. **Split `useTasksPageContext.ts` (3063 linhas)** → quebrar por responsabilidade mantendo a API do composable estável (F-17).
2. **Extrair socket compartilhado** de `useTasksRealtime.ts` (298 linhas) e `useTaskPresence.ts` — hoje cada um gerencia conexão/lifecycle WS por conta própria; unificar num composable de socket reusável.
3. **Extrair `useInlineEditManager`** — consolidar os ~6 "managers" de edição inline (espalhados no contexto de tasks) num único composable parametrizável.
4. **Extrair composable de chat do feedback** — isolar o estado/polling do chat de feedback num composable dedicado (prepara a troca por WebSocket, fase feedback-realtime).
5. **Consolidar helpers cross-módulo** (F-01..F-04): `firstNonEmpty` (8x), `normalizeStoreIDs` (cross-módulo), `decodeStringSlice` (3x), `slugify` (6x) → pacote utilitário único + testes.
6. **Split de arquivos > 450 linhas** (F-18..F-20): `AdminUsersWorkspace.vue` (881), `AdminRoleMatrixEditor.vue` (633), `admin_users_repository.go` (592).

> Ordem sugerida: helpers cross-módulo (item 5, baixo risco, destrava os outros) → splits de arquivo (itens 1/6, mecânicos) → socket/inline-edit/chat (itens 2/3/4, exigem teste de realtime). Cada refactor num PR próprio, com gates verdes (go build/vet/test + eslint/vue-tsc).

---

## Resumo

- **9 achados de segurança** → [SECURITY_OPTIMIZATION_BACKLOG.md](SECURITY_OPTIMIZATION_BACKLOG.md) §"Revisão 2026-06-29" (SEC-6..SEC-14): maioria **feita/em correção na wave 1**.
- **36 achados de faxina** (este doc, F-01..F-26 + 6 refactors agrupados): wave 1 fechou os mortos do auth/manual, o gating decorativo virou real e o predicado de tenant no feedback; **wave 2 fechou helpers (stringsx), splits de admin/core, socket/inline-edit/chat e o task6**. Continuam pendentes só o split do `useTasksPageContext.ts` (F-17) e o `slugify` (F-04, decisão de produto) — ver §"Atualização — Wave 2" no topo.
- **Economia estimada total da faxina:** ~400+ linhas em duplicação/código morto + redução substancial nos 4 arquivos > 450 linhas após os splits.
