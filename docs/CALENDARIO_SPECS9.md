# Calendário — SPECS 9 (WAVE 14: guarda de alvo por dia/cliente + prioridade-calendário + search)

Continuação de `CALENDARIO_SPECS8.md`. Plano canônico: `CALENDARIO_PLAN.md`. Regra dos 3 docs:
este doc + `back/internal/modules/calendar/AGENT.md` + roadmap `cal-w14-guarda-alvo-search`.

## Problema (print do dono 2026-07-13)

"Pega a tarefa do dia 15 de julho e colocar a Iasmin como responsável" → a IA gerou uma
proposta para editar **"DB002 Visita a clientes"**, que (a) **não é do dia 15** — na verdade
não tem data nenhuma — e (b) **não está no calendário**, só existe no board de tasks. Ou seja:
o modelo alucinou o alvo, e o back aceitou porque o `targetId` era um UUID real do contexto.

Dois problemas somados:
1. O modelo escolhe o alvo por conta própria e erra (não-determinístico); nada garantia que a
   task escolhida fosse do dia pedido.
2. O back confiava em qualquer UUID presente no contexto (a resolução da WAVE 12 só reescrevia
   `targetId` quando vinha como *nome*).

## Decisões do dono

- Quando o alvo não bate o que foi pedido → **barrar e pedir para escolher** (não aplicar no
  item errado). Validar por **data E cliente**.
- A busca de alvo **prioriza o CALENDÁRIO**. Se o item pedido só existe em Tasks (sem evento no
  calendário), a IA **não mexe** — avisa "isso não está no calendário, encontrei em Tasks: X.
  Quer que eu altere lá?" e só age com confirmação.
- Novo: um **search no calendário** (campo de busca que acha o item, leva ao dia e abre o modal).

## O que foi feito

### Back — GUARDA DE ALVO (`chat_target_guard.go`, novo; testes em `chat_target_guard_test.go`)

**Correção reforçada (2026-07-14):** validar o que a IA escolheu NÃO bastava — o modelo chega a
dizer "não há evento no dia 13" tendo um, e escolhe o item errado toda vez. Então o back passou a
**RESOLVER** o alvo, não só validar. Roda no `ChatAsk` **antes** de `resolveProposalTargets` (pode
reescrever o `targetId`):
1. `extractTargetCriteria(question, month, clients)`: extrai o **dia** ("dia 15", "15/07")
   ancorado no mês em foco, e o(s) **cliente(s)** por nome. Sem dia/cliente → não restringe.
2. `guardProposalTargets` (via `calendarMatches` = eventos do calendário que batem dia+cliente),
   por update/delete, com **prioridade-calendário**:
   - **1 evento** no critério → **REESCREVE o `targetId`** para ele (mantém os campos que a IA
     queria mudar, ex.: responsável) e devolve `resolvedTitle`; o answer ganha "Vou alterar: X.
     Confirme no cartão." — o alvo fica SEMPRE certo, independe do modelo;
   - **vários** eventos no dia → barra + lista **só os eventos do calendário** para escolher;
   - **0 no calendário mas há task** → barra + avisa "só existe em Tasks: X, quer alterar lá?".
- `chatDayNumberRe` (`\bdia\s+(\d{1,2})\b`) + `contextClients` (lista lean de clientes do contexto).

**Refinamento TÍTULO-PRIMEIRO (2026-07-14, print "Postagem Bari"):** a frase "na postagem Bari o
cliente deve ser bari..." quebrava a guarda de duas formas: (a) "Bari" era tratado como FILTRO de
cliente atual, mas era o NOME do item + o VALOR a atribuir — e como a "Postagem Bari" estava sem
cliente, o filtro excluía o próprio alvo; (b) com "dia 14" o único match do filtro era a "Gravação
Pérolas" e a guarda FORÇAVA o alvo para ela, por cima do título explícito. Correções:
1. **Título citado vence tudo** (`titleMatchedEvents`): se a pergunta contém o título de um evento
   do calendário (frase inteira, sem acento, ≥4 chars), ESSE é o alvo; match MAXIMAL (título
   contido em outro matched sai — "gravacao" cai quando "gravacao perolas" casou); mesmo título em
   vários dias → o dia citado desempata; ainda ambíguo → barra e lista (com as datas).
2. **Cliente-como-VALOR não filtra** (`dropAssignedClients`): cliente que as propostas ATRIBUEM
   (fields.clientId/clientName) sai do critério — só filtra cliente citado como dono atual.
3. Listas de escolha e `resolvedTitle` agora trazem a DATA ("Postagem Bari (14/07)"; candidatos
   dedup por ID, não por título — títulos iguais aparecem com suas datas).

- 15 testes: os 4 cenários do print (título vence filtro, follow-up curto, match maximal, mesmo
  título em 2 dias) + resolve 1-alvo, barra vários+lista, só-tasks avisa, cliente resolve,
  sem-critério passa, create ignorado, data numérica, e resolução de pessoas (lixo/nome/esquecido/
  id válido).

**"APLICOU MAS NÃO SALVOU responsável/cliente" — looksLikeUUID v4-estrito (2026-07-14):** o card
da "Campanha multi-dia teste" aplicava tipo/prioridade/descrição mas responsável (Mike) e cliente
(Pérola) não persistiam. Investigação: a proposta trazia os UUIDs REAIS (Mike `cccccccc-...c005`,
Pérola `aaaaaaaa-...` — ids de SEED); um PATCH direto na API com os mesmos valores persistia
(back OK). A raiz era o `looksLikeUUID` do store de tasks (front): exigia UUID **v4 estrito**
(`[1-5]`+`[89ab]`) e rejeitava os seeds → `responsibleUserId`=null e `clientAccountId`=null
(limpando o cliente); o UUID cru ia para o LABEL `ui_metadata.responsible`. A Iasmin é v4 por
acaso — por isso parte dos testes passava. Correções:
1. `looksLikeUUID` valida FORMATO (qualquer versão/variante) — conserta chat E board.
2. Store aceita `responsibleUserId` explícito no patch/payload (update+create); o chat passa o
   id na coluna e o NOME no label (`responsible`/`involved`/`clientName` legíveis via
   `store.people`/`chatScope.clients`).
3. `foldChatLabel` trata hífen/underscore como espaço — o título "Campanha multi-dia teste"
   agora casa com a fala "campanha multi dia teste" (título-primeiro resolve).
4. `resolveClientsInProposals` (back): clientId desconhecido → resolve por clientName/cliente
   citado na pergunta; irrecuperável → limpo (id inventado nunca chega ao PATCH); id válido
   ganha o nome no label. 23 testes Go verdes.

**BUSCA AMPLA — mês alvo primeiro, depois QUALQUER mês/ano (2026-07-14, achado do dono):** a IA
"não encontrava" a Postagem Bari porque a TELA estava em julho/2025 — o contexto é montado do mês
em foco, e o item (07/2026) nem chegava ao modelo. Hipótese do dono CONFIRMADA em teste controlado
(month=2025-07 → "não encontrei"; month=2026-07 → acha). Fix (`appendWideTitleMatches`, chat_targets.go):
após montar o contexto do mês em foco, se a pergunta cita um TÍTULO que não está nos eventos desse
mês, busca numa janela ampla (±24 meses, mesmas queries scoped: `ListEventsLean`/`ForClients`,
limit 1000) e ANEXA os matches (máx. 8) ao contexto ANTES do LLM — o modelo passa a ver o item e o
fluxo normal (proposta + guarda título-primeiro) segue. O destaque e as listas mostram o ANO
quando o item é de outro ano que o foco ("Postagem Bari (14/07/2026)"), via `crit.focusYear`.
Validado: month=2025-07 → "Vou alterar: Postagem Bari (14/07/2026)" com target/responsável
corretos; 2026-01 e 2026-07 idem (sem ano, mesmo ano do foco). O toggle de "pesquisa em tasks"
chegou a ser iniciado e foi CANCELADO pelo dono ao achar a causa real (revertido; tasks seguem no
contexto). ARMADILHA repetida: `up -d --build api` reusou cache e não embutiu o binário novo —
`build --no-cache` resolveu (2ª ocorrência; ver memória docker-build-cache).

### n8n (`workflow-calendar-chat.json`, nó "Montar contexto")

Regra **PRIORIDADE DO CALENDÁRIO** no prompt (reforça a guarda, reduz retry): procurar primeiro
em `context.events` com `date` = dia pedido; vários no dia → perguntar; item só em `context.tasks`
→ avisar que não está no calendário e pedir confirmação; nunca alterar item fora do calendário sem
isso. Reimportado.

### Front — SEARCH DO CALENDÁRIO (`CalendarSearch.vue`, novo)

Lupa nos controles do calendário abre um campo de busca; digitar (≥2 letras) filtra os itens do
calendário por título/cliente; clicar num resultado **navega ao mês/dia e abre o modal** de edição
(`onSearchOpen`: `setFocusMonth` + `onSelectDay` + abre `CalendarEventForm`). Busca numa janela
ampla (-6/+12 meses do foco) via `fetchEventsInRange`, então acha itens de outros meses. Fecha no
clique-fora/Esc (regra de popover). Injetado no `CalendarControls` via slot `search`. O store
passou a expor `events` (lista plana) — na verdade o search busca própria janela, sem depender do
estado renderizado.

## Validação

- `go build`/`vet`/`test` verdes (7 testes novos da guarda).
- eslint 0 erros (warnings de max-lines pré-existentes).
- E2E browser: reproduzir "tarefa do dia 15" e conferir que alvo errado é barrado com aviso; search
  acha item e abre o modal.

## Notas de Deploy

- Rebuild api (guarda) + web (search). SEM migration. SEM env nova.
- Reimportar `calendar-chat` no n8n (regra de prioridade-calendário no prompt).
