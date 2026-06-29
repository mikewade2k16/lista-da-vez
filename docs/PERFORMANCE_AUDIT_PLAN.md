# Plano de Auditoria de Performance de Navegação — Omni

> Status: **Auditoria feita + 1ª rodada de correções aplicada (2026-06-15); re-auditoria action-first em 2026-06-26 (§17).** Resultados em §14-15; re-auditoria (login + última página bloqueante) em §17. Doc canônico desta frente; espelhado em `web/app/components/roadmap/roadmap-data.ts` (fases `perf-audit` done, `perf-fixes` in_progress, `perf-reaudit-login` in_progress).
>
> **Como medir (setup — anotar):** SEMPRE contra um **build de produção** (o dev compila rota sob demanda no Vite e falseia o número — ver §3.1). Passos: `docker build -t omni-web-prod ./web` → `docker run --rm -d --name omni-web-prod -p 3055:3003 -e NUXT_PUBLIC_API_BASE=http://localhost:9091 -e NUXT_API_INTERNAL_BASE=http://host.docker.internal:9091 omni-web-prod` → `OMNI_QA_EMAIL=.. OMNI_QA_PASSWORD=.. python qa-bot/perf_audit.py --base-url http://localhost:3055`. O container é descartável (`docker stop omni-web-prod` ao terminar). Detalhe em [qa-bot/README.md](../qa-bot/README.md).

## 1. Problema (relato do usuário)

O painel "parece sistema velho e travado": ao clicar num item do menu, demora **vários segundos até a página sequer trocar**, e só então começa o carregamento do conteúdo. Regra básica de UX violada: clicou → tem que ir na hora, e o carregamento acontece já na página nova (com skeleton), não antes dela aparecer.

## 2. Regra que fundamenta (já existe no projeto)

- [docs/ENGINEERING_PRINCIPLES.md](ENGINEERING_PRINCIPLES.md) §6: *"Usuário clicou → sistema responde imediatamente. Se vai demorar, skeleton/loading aparece em < 100ms. Sem tela branca, sem delay perceptível."* + *"Requests da UI são cancelados se o usuário navegar antes de completar (AbortController)."*
- [AGENT_RULES.md](../AGENT_RULES.md) "Pedir só o necessário (otimização + UX de resposta imediata)" e §10.3.
- Roadmap: `fase-7` ("Otimização de performance", `done`) tem como `verifiable` *"navegação entre páginas sem latência perceptível"* — hoje **não se sustenta**; e `fase-9` ("UX de loading") tem `apply-dashboard`/`apply-operacao` ainda `pending`.

Conclusão: a auditoria mede objetivamente onde a regra está sendo quebrada e alimenta a correção.

## 3. Hipótese de causa-raiz (a confirmar pela métrica)

Descartado: `auth.global.ts` chama `await ensureSession()` em toda navegação, **mas** é no-op após a 1ª carga (guard `hydrated` em `web/app/stores/auth.ts:306`). Então o atraso por clique **não** vem daí.

Suspeito nº1: o Nuxt **não pinta a rota nova enquanto o `setup()`/`await` de dados da página não resolve** — cada página trava a troca de rota esperando o fetch dela (sem skeleton, sem resposta imediata). A métrica T1 (abaixo) isola exatamente isso: se T1 é alto, o gargalo é fetch bloqueante no `setup`/middleware da rota; se T1 é baixo e T3 é alto, a troca é rápida mas os dados demoram (aí a correção é skeleton + lazy-load).

## 3.1. Achado crítico (2026-06-15): o dev compila sob demanda — medir contra build de produção

O web local roda em **modo dev** (`docker-compose.yml`: `target: dev` + `npm run dev` + `NODE_ENV=development`). O Vite **compila cada rota na 1ª visita da sessão**. Medido com `curl`:

| Rota | 1ª visita (compila) | 2ª visita (compilada) |
|---|---|---|
| `/auth/login` | **203,15 s** | **0,072 s** |

Ou seja: grande parte do "clico e demora muitos segundos" **no ambiente local** é o compile do Vite — **não** existe em produção (lá o build é pré-compilado). Decisão (2026-06-15): a auditoria mede contra um **build de produção local** (`docker build` do stage prod do `web/Dockerfile`, servido em porta separada `:3010`, API em `localhost:9091`), para isolar o custo real do app do ruído de compilação do dev. O harness tem `--warmup` para o caso de querer medir o dev isolando a 1ª-compilação.

## 4. O que medir — 3 marcos por página

Por página medimos **três** tempos (não dois), porque "demora pra abrir" e "demora pra carregar" são problemas diferentes com correções diferentes:

| Marco | Definição técnica | Frase do usuário |
|---|---|---|
| **T1 — clique → troca de rota** | do clique (ou `router.push`) até a rota efetivamente mudar (`router.afterEach`/`page:start`). Mede fetch bloqueante de `setup`/middleware. | *"clico pra ir pra página X e demora pra ir"* |
| **T2 — clique → primeira pintura** | do clique até o conteúdo da nova rota aparecer em tela (FCP no modo cold; 1º frame com o container de conteúdo visível no modo in-app). | *"quanto tempo até a página aparecer em tela"* |
| **T3 — clique → carregamento final** | do clique até estabilizar: network idle (sem request por 500ms) **e** sem skeleton (`.core-skeleton` ausente) **e** overlay global zerado (`useCoreLoading` em 0). | *"do início do carregamento até o final"* |

Sempre T1 ≤ T2 ≤ T3.

## 5. Dois modos de medição (decisão do usuário: medir os dois)

### 5.1. In-app (navegação SPA) — o que se sente no dia a dia
App já aberto e logado. Para cada rota: navegar por clique no menu, medir T1/T2/T3 via marcas `performance.now()` injetadas em hooks do router e observers de DOM/rede. Cache de rede desligado (CDP `Network.setCacheDisabled(true)`); entre medições volta-se a uma rota neutra para não reaproveitar estado já hidratado. Sessão mantida.

### 5.2. Cold load (1ª visita / F5) — carga do zero
Por rota: cache 100% off, `page.goto(url)` direto (cookies de sessão mantidos). Marcos via Navigation Timing / Paint Timing API:
- T1 ≈ `responseEnd`/`domInteractive` (HTML pronto);
- T2 = `first-contentful-paint` (PerformancePaintTiming);
- T3 = network idle + skeleton sumiu.

## 6. Metodologia

- **Perfil:** `platform_admin` (vê todas as rotas numa rodada). Credenciais fornecidas pelo usuário na hora de rodar — nunca inventadas/commitadas.
- **Repetições:** 3 rodadas por página por modo, **sem cache** em todas. Mostrar os 3 tempos + média (T1, T2, T3).
- **Ambiente:** local (web em `:3003`, api em `:9091`), Chromium headless via Playwright. Máquina e build registrados no cabeçalho do relatório (resultado é relativo ao ambiente).
- **Ordem:** medir cada página isoladamente; descartar a 1ª amostra de aquecimento do navegador antes da rodada 1 (warm-up do processo, não da página).

## 7. Escopo — todas as rotas (perfil platform_admin)

Fonte: `web/layers/queue/nav.config.ts` + `web/app/pages/**`. Inclui itens `hidden:true` (admin acessa direto).

**Estáticas (dashboard):** `/` · `/operacao` · `/operacao/usuarios` · `/operacao/clientes` · `/tasks` · `/tracking` · `/editor` · `/automation` · `/omnichannel` · `/consultor` · `/ranking` · `/dados` · `/inteligencia` · `/relatorios` · `/bi` · `/crm` · `/erp` · `/finance` · `/monitoramento` · `/multiloja` · `/configuracoes` · `/alertas` · `/feedback` · `/campanhas` · `/meta-ads` · `/cardapio` · `/site/leads` · `/site/produtos` · `/site/tracking` · `/site/bio` · `/manage/clientes` · `/manage/clientes-web` · `/manage/produtos-web` · `/manage/leads-web` · `/manage/users` · `/manage/organizations` · `/manage/auditoria` · `/manage/integracoes` · `/themes` · `/banco` · `/roadmap` · `/perfil` · `/meus-feedbacks` · `/usuarios` · `/clientes`

**Dinâmicas (precisam de id/param real):**

| Rota | Param | Estratégia |
|---|---|---|
| `/tools/[tool]` | `qr-code`, `encurtador-de-link`, `scripts` | valores fixos conhecidos do nav |
| `/team/[area]` | `equipe`, `escalas` | valores fixos conhecidos do nav |
| `/site/[area]` | a enumerar na página | mapear no harness; senão pular com nota |
| `/site/bio/[id]` | id de uma bio real | harness pega o 1º id da lista `/site/bio`; vazio → pula com nota |
| `/cardapio/[id]` | id de um cardápio real | harness pega o 1º id da lista `/cardapio`; vazio → pula com nota |

**Auth (sem sessão, medidas à parte):** `/auth/login` · `/auth/esqueceu-senha`.

## 8. Design do harness (estender o qa-bot já existente)

Base: `qa-bot/` (Python + Playwright já instalado — `qa_bot/runner.py`). Novo script `qa-bot/perf_audit.py`:

1. Lê a lista de rotas (constante no script, espelhando a seção 7) e as credenciais por env/CLI (`OMNI_QA_EMAIL`/`OMNI_QA_PASSWORD`).
2. Faz login uma vez; descobre os ids dinâmicos navegando as listas.
3. Para cada rota × modo × 3 rodadas: injeta um script de instrumentação (`performance.mark`/observers; hooks no `window.$nuxt`/router) que devolve `{t1,t2,t3}`; cache desligado via CDP.
4. Agrega em média e gera:
   - `qa-bot/artifacts/perf-<timestamp>.csv` (linha por rota×modo×rodada);
   - `qa-bot/artifacts/perf-<timestamp>.md` (tabela por página + média + ranking das piores).
5. Screenshot de cada página no fim da 3ª rodada (evidência de que renderizou de verdade).

Instrumentação por marco (resumo):
- **T1:** `useRouter().beforeEach` marca `t0` no clique; `afterEach` marca troca de rota.
- **T2 (in-app):** 1º `requestAnimationFrame` após o container de conteúdo da rota (`.page-workspace`/`.module-workspace-full > *`) existir no DOM. **(cold):** `first-contentful-paint`.
- **T3:** `MutationObserver` + contador de rede ocioso por 500ms + ausência de `.core-skeleton` + store de loading global em 0.

## 9. Formato de saída (exemplo)

```
Página: /operacao            (in-app)
  T1 (clique→troca):   run1 2.41s  run2 2.38s  run3 2.55s  | média 2.45s
  T2 (clique→pintura): run1 2.62s  run2 2.50s  run3 2.71s  | média 2.61s
  T3 (clique→final):   run1 3.90s  run2 3.71s  run3 4.05s  | média 3.89s
```
+ tabela-resumo ordenada da página mais lenta para a mais rápida (por T1 e por T3), separando os dois modos.

## 10. Fases de execução (após aprovação)

| Fase | Entrega |
|---|---|
| **E1** | `perf_audit.py`: login + instrumentação dos 3 marcos + 1 rota de prova (`/operacao`), valida que os números batem com o cronômetro manual. |
| **E2** | Loop por todas as rotas estáticas, modo in-app, 3×, com CSV+MD. |
| **E3** | Modo cold (Navigation/Paint Timing) para as mesmas rotas. |
| **E4** | Rotas dinâmicas (descoberta de id) + rotas de auth. |
| **E5** | Relatório consolidado: ranking, médias, e diagnóstico por página (T1 alto = fetch bloqueante; T3 alto = falta lazy-load/skeleton). Vira backlog de correção. |

## 11. Critério de aceite do plano

Relatório com, para **cada** rota do escopo, os 3 tempos × 3 rodadas + média, nos dois modos, e um ranking que aponte as páginas que violam a regra (T1 perceptível / T2 > ~300ms / sem skeleton). A correção em si é frente seguinte (provável reabertura de `fase-7`/`fase-9`).

## 12. Notas de deploy / dependências

- Ferramenta de QA **local**, não vai pra produção; nenhuma mudança em `.env`/`docker-compose` da app.
- Depende de Playwright (já no `qa-bot/.venv`); pode exigir `python -m playwright install chromium`.
- App precisa estar de pé local (`web :3003`, `api :9091`) durante a medição.
- Credenciais do `platform_admin` passadas por env/CLI no momento da execução; nunca versionadas.

## 13. Decisões em aberto (resolver antes do E4)

- Ids reais para `/site/bio/[id]` e `/cardapio/[id]`: por descoberta automática (1º da lista) ou ids específicos que o usuário queira medir.
- `/site/[area]`: enumerar os `area` válidos (ler a página) ou marcar como fora do escopo desta rodada.

## 14. Resultados — 1ª rodada (2026-06-15, build de produção em `:3010/3055`)

Medido como `platform_admin`, ~50 rotas, 3 rodadas, in-app + cold. Relatório bruto: `qa-bot/artifacts/perf-20260615-133516.{md,csv}`.

**Conclusão headline:** no build de produção, a navegação é **rápida em TODAS as rotas** — o "clico e demora muitos segundos" **não reproduz**. Confirma que a dor no dev local é o **compile sob demanda do Vite** (203s→0,07s), não o app.

| Marco | In-app (faixa) | Cold (faixa) | Leitura |
|---|---|---|---|
| **T1** clique→troca de rota | 0,00–0,02 s | 0,01–0,29 s | Troca de rota é **instantânea**. O gargalo do "demora pra ir" não está no app. |
| **T2** clique→primeira pintura | 0,08–0,29 s | 0,13–0,60 s | Página aparece rápido. Cold um pouco maior (custo do `ssr:false`, mas sub-segundo). |
| **T3** clique→carregamento final | mediana ~0,9 s | mediana ~1,2 s | Maioria saudável (~1 s). Cauda lenta isolada abaixo. |

**Páginas que concentram o custo real (T3) — alvos de otimização:**

| Rota | T3 in-app | T3 cold | Diagnóstico |
|---|---|---|---|
| `/tasks` | 4,57 s | **15+ s (cap)** | Board pesado (247 cards, render progressivo). Skeleton persiste / DOM nunca quieta. Alvo nº1. Casa com fase-tasks "perf >500 cards". |
| `/operacao`, `/` | 3,35 s | 4,52 s | **Realtime** (stream ao vivo) — pintura inicial é instantânea (T2 ~0,1 s); T3 reflete o stream. Falta skeleton (fase-9 `apply-operacao` pendente). |
| `/erp` | 1,83 s | 2,46 s | Lista/dados mais pesados. |
| `/manage/users` | 1,78 s | 2,43 s | Idem. |
| `/usuarios`, `/manage/clientes-web` | 1,1–1,2 s | 1,7–1,8 s | Levemente acima da média. |

Demais ~40 rotas: T3 in-app ~0,9 s / cold ~1,2 s — **saudáveis**.

**Caveats/erros desta rodada:**
- `/editor` (in-app): `Execution context was destroyed` — a página faz reload/navegação dura que destrói o contexto JS; medir só em cold (cold = 1,07 s, ok).
- `/cardapio/[id]`: sem registro no ambiente → pulada. `/site/bio/[id]` **medida** (id real via clique na 1ª linha): T3 in-app ~0,93 s / cold ~1,19 s — **saudável**.
- T3 de páginas realtime (`/operacao`, `/`, `/tasks`) usa fallback de 4 s (DOM nunca fica "quiet"); é teto, não carga pura.

**Backlog que sai daqui:**
1. **Dev (sua dor diária):** o lento é o Vite compilando rota na 1ª visita. Alavanca: pré-aquecer rotas (warm-up) ou aceitar que é dev-only. Não é bug do app.
2. **Produto (prod):** otimizar `/tasks` (board); aplicar skeleton em `/operacao` (fase-9 `apply-operacao`) e dashboard (`apply-dashboard`); olhar `/erp` e `/manage/users`. O resto está bom.

## 15. Pós-correções — 4 tracks em paralelo (2026-06-15)

Implementado por 4 subagentes (specs em [PERF_FIXES_SUBAGENT_SPECS.md](PERF_FIXES_SUBAGENT_SPECS.md)). Re-medição contra o build de prod rebuildado: relatório `qa-bot/artifacts/perf-20260615-155749.{md,csv}`.

| Rota | Modo | Antes (T3) | Depois (T3) | Veredito |
|---|---|---|---|---|
| `/erp` | in-app | 1,83 s | **1,27 s** | ✅ N+1 eliminado (10→2 queries no `/erp/status`) |
| `/manage/users` | in-app | 1,78 s | **1,64 s** | ✅ paginação server-side (antes baixava todas as páginas) |
| `/manage/users` | cold | 2,43 s | **2,25 s** | ✅ + ganho estrutural: não degrada quando o nº de usuários cresce |
| `/manage/clientes-web` | cold | 1,82 s | **1,55 s** | ✅ de brinde |
| `/tasks` | cold | 15,37 s | 15,39 s | ⚠️ T3 inalterado — ver nota |
| `/operacao` | in-app/cold | 3,4 / 4,5 s | 4,2 / 4,6 s | ~ruído de realtime |

**Nota honesta — o que o T3 NÃO mede:**
- **`/tasks` (lazy-mount, Track C):** o T3 mede "DOM parou de mudar"; com 247 cards em render progressivo o DOM muda até o fim, independente de cada card montar editor pesado ou placeholder leve. O lazy-mount reduz **trabalho na thread principal / jank** (validado por 15/15 testes + menos `USelectMenu` montados), mas isso é **outra dimensão** — precisa de Total Blocking Time / long-tasks para provar, não T3.
- **Skeletons (`/operacao`, `/`, Track B):** o ganho é **perceptual** (skeleton pinta < 100ms no lugar de tela vazia); o T3 (settle) não muda porque o realtime nunca fica quiet. Confirmar visualmente no browser.

**Próximo passo de medição:** adicionar um marco de **Total Blocking Time / long tasks** ao harness para capturar o ganho de responsividade do `/tasks` (o T3 atual é cego pra isso).

## 16. Estratégia de cache / Redis — ANOTADO (decisão: NÃO implementar agora)

> Registrado em 2026-06-15 a pedido. É uma OPÇÃO documentada, não trabalho ativo. Nenhuma das páginas críticas vai ser tocada por isso por enquanto.

**Princípio honesto:** o que a auditoria mede é latência de **um** usuário num stack local rápido. **Redis quase não move esse número** — Redis é alavanca de **escala/concorrência** (muitos usuários, múltiplas instâncias da API), não de latência de página individual. Já existe `omni-redis-1` no stack (profile automation), então infra não seria nova.

| Página | Gargalo real | Cache/Redis ajuda? |
|---|---|---|
| `/tasks` | **Front** (render de 247 cards) | ❌ Não. Redis é backend; o custo é no navegador. Lever = front (lazy-mount feito; windowing; payload menor; medir TBT). |
| `/operação` | Realtime — T3 é o stream ao vivo, não query lenta | △ Não acelera o stream. Cache + realtime **brigam** (cache deixa stale). Redis ajuda só a **escalar** (pub/sub entre instâncias) e a cachear o snapshot inicial por poucos segundos. |
| `/erp` | Agregação de status (lida a cada load, muda só no sync) | ✅ Melhor candidato: cache (memória ou Redis) com **invalidação por evento** no sync. Ganho aparece sob carga. |
| `/manage/users` | Lista que muda; já paginada | △ Marginal. Para "checar se mudou" barato: **ETag/304**, não cache. |

**Regra (se um dia implementar):** cache SÓ com **invalidação por evento**, nunca brigando com realtime (não servir dado stale onde o usuário espera ao vivo); polling → trocar por **push (WebSocket/evento)** ou **GET condicional (ETag/`If-None-Match` → 304)**; Redis entra como **backplane** quando houver múltiplas instâncias (pub/sub realtime + `PrincipalCache` compartilhado — já previsto na fase-7 item `redis-cache`, deferido).

**Levers reais por página (quando focarmos):** `/tasks` → front (TBT/windowing/payload); realtime → push/invalidação + Redis só ao escalar; `/erp` → cache de status com invalidação no sync; `/manage/users` → ETag/304 (paginação já feita).

## 17. Re-auditoria action-first (2026-06-26) — login + última página bloqueante

> Relato novo do usuário: "ainda trava ao trocar de página (~1s parado e só então vai) e no login (o botão de carregando nem sempre aparece / para de carregar e continua na tela de login)". Re-varredura das 41 páginas (10 agentes em paralelo) + deep-dive do login. Espelhado no roadmap (`perf-reaudit-login`) e na nota da página `/performance` (`PerformanceWarmupNote.vue`).

### 17.1. Navegação entre páginas — confirma o §14
- Das **41 páginas**, **40 já são action-first**: o fetch sai em `onMounted`/`watch` assíncrono (não suspende a rota), e/ou há skeleton imediato. 9 delas nem buscam rede (estáticas/runtime em memória).
- **1 violação real (vale em PROD, não é Vite):** `/usuarios`. `web/app/components/users/UsersAccessManager.vue:19` faz `const ctx = await useUsersAccessManager({ mode })` — **await de topo** no `<script setup>`. Sem `<Suspense>` com fallback no caminho, o Suspense implícito da rota **segura a troca**: ao clicar em `/usuarios` o conteúdo fica na página anterior até `/v1/users` + `/v1/auth/roles` responderem. Ironia: o `AppEntityGrid` teria skeleton via `:loading="usersStore.pending"`, mas ele nunca aparece porque o componente não monta antes do await terminar. (Só atinge `/usuarios` → `UsersWorkspace mode="queue"`; `/manage/users` usa `AdminUsersWorkspace`, que já é action-first.)
- **Demais "travadas ~1s" no dev local = compile sob demanda do Vite** (§3.1/§14): em build de produção a troca de rota é instantânea. Levers de dev: `qa-bot/warmup_dev.py` (pré-compila as rotas) ou rodar o stage prod do `web/Dockerfile` local no dia a dia. Não é bug do app.

### 17.2. Login — o gargalo de app que ainda sobra (vale em PROD)
Sequência exata do clique em "Entrar" até a rota destino pintar (`login.vue:43` → `auth.ts:385`):
1. `POST /v1/auth/login` (REDE #1) → seta token.
2. `await fetchContext()` → `GET /v1/me/context` (REDE #2) → popula `user/principal`. **`homePath` já está resolvido aqui** (deriva só de `role`/`permissionKeys`/`permissionsResolved`).
3. ...mas `fetchContext` ainda `await syncRuntimeAccess()` **com `pending=true`**: `fetchAccounts` (`/v2/me/accounts` + `/v2/me/context`, REDE #3-#4) + `hydrateRuntimeStoreContext` (`/v1/settings` + `/v1/consultants` + `/v1/operations/snapshot`, REDE #6-#8). **Mínimo de 4 round-trips sequenciais** antes de o `navigateTo` sequer começar — e **nada disso é necessário para rotear**.
4. `pending` zera no `finally` de `login()` (`auth.ts:409`) **antes** do `navigateTo` (`login.vue:67`). Resultado: o botão volta de "Entrando..." → "Entrar" e os inputs saem do `readonly` **enquanto o usuário ainda está na tela de login** = exatamente o "para de carregar e continua travado".

**Correção (action-first):**
- **P1 — adiar runtime:** `fetchContext({ deferRuntime: true })` no caminho do login dispara `syncRuntimeAccess()` **sem await** (já é best-effort, `try/catch` que degrada sem derrubar sessão). Navega logo após REDE #2. `ensureSession`/bootstrap por reload seguem com await completo (sem mudança).
- **P2 — segurar o loading:** ref local `submitting` em `login.vue`; botão/inputs gateados por `auth.pending || submitting`; solta num `finally` **após** o `navigateTo`. (Menor risco — não altera o contrato de `pending` do store.)
- **P3 — anti-corrida:** guard de in-flight em `account.ts fetchAccounts` (memoizar a promise como em `ensureSession`) para o `await fetchAccounts()` do `auth.global.ts` não duplicar `/v2/me/accounts`.
- **Validado:** `homePath` é puro/síncrono a partir de `principal` (REDE #2) — o defer é seguro; nenhum input de `homePath` vem da parte adiada. O gating de módulo no destino é fail-open durante o hidrate (já documentado em `module-enabled.global.ts`), então navegar cedo não barra a rota.

### 17.3. Notas de deploy / dependências
- **Só front.** Sem migration, sem env var, sem mudança de contrato de API. Arquivos: `web/app/stores/auth.ts`, `web/app/pages/auth/login.vue`, `web/layers/core/stores/account.ts`, `web/app/components/users/UsersAccessManager.vue` (+ `web/app/composables/useUsersAccessManager.js`).
- Validar **no browser** (mudança de UX, type-check não cobre): login com rede lenta (DevTools throttling) — botão fica "Entrando..." até a rota destino pintar; `/usuarios` troca na hora e mostra skeleton.
- `apply-tables`/skeleton já existem (`AppEntityGrid`); o fix de `/usuarios` só destrava a montagem para o skeleton aparecer.

### 17.4. Honestidade — o que isto contradiz
- O `verifiable` da **fase-7** ("Login < 500ms; navegação sem latência perceptível", `done`) não se sustenta enquanto o login encadear 4+ chamadas e `/usuarios` travar. Anotado em `perf-reaudit-login` até as correções fecharem.
- **Login não é medido** pelo `perf_audit.py` (só rotas pós-login). Para a página `/performance` refletir login seria preciso instrumentar o login no harness e re-emitir `perf-data.ts` — fora do escopo desta rodada; por isso a nota da página diz isso explicitamente.
