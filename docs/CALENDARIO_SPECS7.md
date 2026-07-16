# Calendário — SPECS 7 (WAVE 12: chat resolve tudo por NOME + alvo garantido no card)

Continuação de `CALENDARIO_SPECS6.md`. Plano canônico: `CALENDARIO_PLAN.md`. Regra dos 3 docs:
este doc + `back/internal/modules/calendar/AGENT.md` + roadmap `cal-w12-chat-alvo-por-nome`.

## Problema (real, print do dono 2026-07-13)

"reel de dona evania vamos adicionar um responsável, que vai ser iasmin" → a IA respondia
"Preciso do ID da responsável Iasmin no sistema" em loop, mesmo com o dono insistindo
("sem id, pelo nome iasmin mesmo"). Causa raiz: o contexto enviado à IA **não tinha a lista
de pessoas da equipe** (id+nome) e o prompt mandava "não inventar IDs" — sem a lista, o
modelo não tinha como resolver o nome e ficava exigindo ID (que o usuário nunca conhece).

## O que foi feito (2026-07-13)

### Back (`chat_targets.go`, novo — testes em `chat_targets_test.go`)

1. **Pessoas no contexto**: `People []Member` (id+nome) nos DOIS escopos do chat
   (`calendarChatContext.People` e `AIContextAll.People`), populado por `chatPeopleContext`
   → `ListResponsibles` (subconjunto configurado ou todos os membros — a MESMA fonte do
   GET /responsibles que o front usa). Falha ao listar = lista vazia (chat não quebra).
2. **Alvo resolvido server-side** (`resolveProposalTargets`, roda no `ChatAsk` após a
   sanitização): `targetId` que veio como TÍTULO (a IA escorrega) é reescrito para o id
   REAL cruzando com o contexto autoritativo — match ÚNICO por título (sem acento,
   igual/contido) entre `context.events` + `context.tasks`; ambíguo não mexe (o front ainda
   tem a rede de segurança própria).
3. **Snapshot do alvo no card**: cada update/delete ganha o snapshot do item alvo anexado
   aos `calendar_items` da mensagem (`mergeCalendarItems`, sem duplicar) — task pura vira
   `AIContextEvent` sintetizado (`aiContextEventFromTask`, `TaskID=ID`; data/hora do dueDate
   pela MESMA heurística de fuso do mirror: meia-noite UTC = sem hora, hora real converte
   p/ SP — `taskDueDateParts`/`mirrorEventDateTime`). O front já resolvia título/"antes"
   pelos `calendarItems` e já esconde alvos da seção "Calendário" (filtro por targetId) —
   com o snapshot, o card SEMPRE mostra o título e o antes→depois, inclusive de task sem
   evento vinculado.
4. `chatAskTimeout` 60s → **120s**: resposta longa (ex.: listar tasks sem data com contexto
   de 100 tasks) + retries do n8n (4×2,5s) estouravam a janela e viravam "IA fora do ar".

### n8n (`workflow-calendar-chat.json`, nós "Montar contexto" + "Extrair resposta")

- Contexto ganha a linha `Pessoas da equipe (responsaveis/envolvidos possiveis...)` com
  `ctx.people`.
- Regras novas no prompt: **RESPONSAVEL/ENVOLVIDOS** (resolver nome→id pela lista; nome
  fora da lista viaja como NOME — o sistema resolve ao aplicar; **NUNCA exigir ID** do
  usuário; só perguntar se DUAS pessoas empatarem) e **ALVO POR NOME** (achar o item pelo
  título em events/tasks e usar o id em targetId; com 1 candidato claro, gerar a proposta
  direto — o cartão já é a confirmação).
- Extrator: aliases `responsible/responsavel/responsibleName` (responsibleId) e
  `envolvidos` (involvedIds).
- Patch via script Node (mexe só nos literais jsCode; parse validado com `new Function`).
  Reimportado com `npm run n8n:import:chat`.

### Front

- `CalendarChatMessage.vue`: `proposalTitle` de update/delete prioriza o TÍTULO DO ALVO
  (o título novo, quando houver, aparece no diff); create com cliente já resolvido mostra
  **rótulo fixo + botão "Trocar"** (`showClientLabel`/`pickerRequested`) — o select só abre
  se pedido; sem cliente resolvido, select direto (comportamento anterior). Update/delete
  seguem SEM select (herdam o cliente do alvo).
- `TasksBoardView.vue`: card do board mostra a **PRIMEIRA mídia** da task
  (`firstMediaByTask` — calendarMedia espelhada primeiro, senão vídeos próprios; badge +N
  para o restante). A ordem vem do drag-and-drop de mídia da WAVE 11 (uploader do
  calendário) → espelho `calendarMedia` → 1ª do card.

## Correções extras achadas PELO TESTE de browser (2026-07-13)

1. **REGRESSÃO calendarMedia (cruzamento A morto no front):** `normalizeTaskUiMetadata`
   (store de tasks) faz whitelist de chaves do ui_metadata e `calendarMedia` NÃO estava
   nela — o espelho de mídia do calendário era descartado ao mapear a task (card e modal
   sem mídia, apesar de o back mandar). Fix: `mapTaskToStoreItem` lê `calendarMedia`
   DIRETO de `task.uiMetadata` (dado server-populated read-only; não passa pelo pipeline
   de patch local, que é de ESCRITA).
2. **FAB do assistant coberto em /tasks:** o FAB (bottom 1.25rem/right 1.25rem) ficava
   embaixo do botão de feedback do dashboard (fixed bottom 2rem/right 2rem) — o feedback
   interceptava o clique e o assistant "não abria". Fix: FAB sobe para bottom 5.6rem/right
   2rem (acima do feedback).
3. **Task multi-dia nascia com início errado pelo chat:** "começa 21/07 e termina 24/07"
   virava dueDate=24/07 (a IA trata 'termina' como prazo) e a barra não aparecia (início
   = fim). Fix duplo: prompt ganha a regra TASK MULTI-DIA (startDate = início,
   dueEndDate = fim; dueDate só para prazo de dia único) e o front (`useCalendarChat`
   create/update) usa `startDate` como início quando há `dueEndDate`.

## Validação

- `go build`/`go vet`/`go test ./internal/modules/calendar/` verdes (testes novos de
  resolução de alvo, título com acento, fuso e merge).
- E2E API: "no Reels Pérola adiciona a responsável Iasmin" → proposta `update` com
  `responsibleId` da Iasmin resolvido POR NOME + `targetId` real + snapshot "Reels Pérola"
  nos calendarItems.
- E2E browser (Playwright, headless, login real, 4 partes — screenshots em
  `qa-bot/artifacts/chat-crud/`):
  - Parte 1 (CRUD pelo chat): login ✓; chips de mídia sem título ✓; card mostra o título
    do alvo ✓; IA não exigiu ID ✓; update sem select de cliente ✓; aplicar edição ✓;
    multi-tarefa 3 propostas ✓; cliente como rótulo+Trocar ✓; criação em lote ✓; anotação
    do mês ✓; exclusão por nome com título no card ✓.
  - Parte 2/3: tasks sem data respondidas ✓ (após timeout 120s); board com a 1ª mídia nos
    cards ✓ (após fix da regressão); task criada pelo chat no board ✓; excluída sumiu ✓;
    FAB do assistant clicável em /tasks ✓ (após fix de sobreposição); assistant abre ✓;
    **WS tempo real nos DOIS sentidos** — edição pelo chat aparece no board aberto SEM
    reload ✓ e edição no board aparece no chip do calendário SEM reload ✓.
  - Parte 4: barra multi-dia renderiza ✓; toggle ocultar/mostrar (atalho B) ✓; task
    multi-dia criada PELO CHAT com início 27/07 → fim 30/07 persistidos certos e barra
    atravessando os dias (2 segmentos, cruza semana) ✓.
  - Parte 5 (drag-and-drop de mídia, gesto REAL): 2 anexos semeados no dia 22/07;
    arrastar a 2ª miniatura p/ a 1ª posição inverteu a ordem na hora ✓ e PERSISTIU após
    reload ✓; e a cadeia ordem→task provada via API: evento com mídia [A,B] espelha
    calendarMedia [A,B] na task, reordenar para [B,A] (o que o drag emite no save)
    re-espelha [B,A] — ou seja, a 1ª mídia do card do board segue a ordem escolhida ✓.
    ARMADILHA de teste: o calendário é scroll contínuo de meses — clicar "dia 22" sem
    escopar a seção do mês abre o 22 de OUTRO mês (usar o botão Hoje + escopo "Julho").
  - Nota de variância do modelo: num dos runs o gpt-4o-mini colocou a pessoa em
    `involvedIds` em vez de `responsibleId` ("adiciona a responsável Iasmin") — o card
    mostra exatamente o que vai mudar (Envolvidos: Iasmin) e o dono pode editar inline
    antes de aplicar; comportamento aceito (confirmação explícita é a rede).
- Incidente na sessão: Docker Desktop travou (daemon unresponsive, `docker ps` pendurado,
  portas mortas) após builds paralelos + compose watch; cura = matar processos Docker
  Desktop + `wsl --shutdown` + reabrir. Containers voltam sozinhos (restart policy).

## Notas de Deploy

- Rebuild da api (`docker compose up -d --build api`) — mudanças em Go (contexto/targets/
  timeout). SEM migration nova. SEM env nova.
- Re-import do workflow `calendar-chat` no n8n (deploy:fast:prod já força o import).
