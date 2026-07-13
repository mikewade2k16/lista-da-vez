# CALENDÁRIO — SPECS6 (WAVE 7): CRUD de Anotações e Perfil de Cliente pelo chat

Spec operacional da **WAVE 7**: a IA do chat do Calendário (Crow Assistant) passa a fazer o
CRUD completo das **anotações do mês** e do **perfil estratégico do cliente**, dentro do MESMO
loop propor-e-confirmar já existente (E7/WAVE 5.1). Fonte de verdade: os endpoints reais que já
existem; a IA nunca grava — só propõe; o front executa no confirm pela API autenticada do usuário.

Plano canônico: [CALENDARIO_PLAN.md](CALENDARIO_PLAN.md). Regra dos 3 docs: este SPEC +
`back/internal/modules/calendar/AGENT.md` + roadmap (`cal-w7-chat-crud-notas-perfil`).

## 1. Objetivo

1. IA escreve nas **anotações do mês** (`calendar.notes`, 1 bloco HTML por `YYYY-MM`).
2. IA escreve no **perfil estratégico do cliente** (`calendar.client_profiles`) — enriquece a
   inteligência por cliente a partir de voz/texto.
3. IA **avisa os campos vazios** do perfil e **insiste um pouco** para o dono completar.

Áudio já resolvido: `/chat/transcribe` (Whisper) vira texto do input; nada novo de captura.

## 2. Decisões de produto (dono)

- **Anotações:** ACRESCENTAR por padrão (anexa ao que existe; só reescreve o bloco se pedido
  explícito; `delete` = limpar).
- **Alvo do perfil no escopo "Todos os clientes":** a IA infere o cliente pelo NOME (da lista de
  clientes visíveis do contexto) e o **cartão de confirmação mostra/permite trocar** o cliente.
- **Delete do perfil:** limpa **campos individuais** ("apaga a história") **e** zera o **perfil
  inteiro**, ambos com confirmação no cartão.
- **Campo-a-campo:** todos os campos (9 estáveis + 7 do `extra`) são endereçáveis individualmente,
  tanto para responder ("qual o segmento da Pérola?") quanto para preencher/editar 1 por vez.

## 3. Contrato — 2 novos `kind` de proposta

`ChatProposal.kind` passa a aceitar `note` e `clientProfile` além de `event`/`task`. Os campos
específicos vão em **sub-objetos aninhados** de `ChatProposalFields` (evita achatar ~35 campos):

```
fields.note = {
  month?: "YYYY-MM",          // vazio = mês do contexto (front usa store.focusMonthKey)
  content?: "texto/HTML",     // conteúdo a acrescentar/definir
  mode?: "append" | "replace" // vazio = append; delete ignora
}
fields.profile = {
  segment?, positioning?, description?, history?, siteUrl?, instagram?, address?,
  objectives?, brandVoice?,
  extra?: { audience?, offer?, pillars?, cadence?, restrictions?, performance?, assets? },
  clearFields?: string[],     // campos a esvaziar no delete parcial (ex.: ["history"])
  clearAll?: boolean          // zerar o perfil inteiro
}
```

O cliente-alvo do perfil reusa `fields.clientId` (topo, já existe) — a IA resolve nome→id do
contexto, ou deixa vazio para o dono escolher no cartão (mesma UX do seletor de cliente dos eventos
no escopo "Todos").

### Semântica por ação

- **note** — `create`/`update` = definir conteúdo (`replace` substitui; senão acrescenta ao atual);
  `delete` = limpar a nota do mês.
- **clientProfile** — `create`/`update` = merge dos campos não-vazios (o front faz GET→merge→PUT
  **full-replace**, nunca zera os outros); `delete` = `clearFields` esvazia os listados OU
  `clearAll` zera o perfil.

## 4. Backend Go — `back/internal/modules/calendar/`

- **`chat.go`**: tipos `ChatProposalNote`/`ChatProposalProfile` (+`ChatProposalProfileExtra`);
  `Note *ChatProposalNote` e `Profile *ChatProposalProfile` em `ChatProposalFields`.
  `sanitizeProposal` ramifica por `kind`:
  - `event`/`task`: regras atuais intactas (update/delete exigem `targetId`; create exige `title`).
  - `note`: `create`/`update` exigem `note.content`; `delete` válido (limpa); `note.month` validado
    (`monthRe`) se vier; `mode` normalizado (append|replace, default append).
  - `clientProfile`: `create`/`update` exigem ≥1 campo de `profile`; `delete` exige `clearAll` OU
    `clearFields` não-vazio; `clientId` OPCIONAL (dono escolhe no cartão).
  `normalizeProposalFields` estende para trim de note/profile + dedupe de `clearFields`.
  `proposalDedupKey` já serializa `Fields` (cobre o aninhado).
- **`runtime_context.go`**: helper `missingProfileFields(planProfile) []string` (só os NOMES dos
  campos estáveis vazios — token-cheap) e `ProfileMissing []string` em `AIContextClientLean`
  (escopo `all`). No escopo `client` o perfil completo já vai no contexto; o nó n8n calcula os
  vazios de `client.profile`.

Sem migration, sem env nova.

## 5. Workflow n8n — `automation/export/workflow-calendar-chat.json`

- **"Montar contexto"** (Code): calcula os campos vazios do perfil em foco (de `ctx.client.profile`)
  e injeta "Campos do perfil ainda vazios: [...]"; estende a INSTRUÇÃO FINAL do schema com os kinds
  `note`/`clientProfile` e seus `fields` aninhados; instrução de **insistência** (mapear a fala para
  campos do perfil, propor `clientProfile`, e no `answer` avisar o que falta e pedir com moderação;
  ideias/pautas → `note` append).
- **"Extrair resposta"** (Code): espelha o sanitizer Go — monta `fields.note`/`fields.profile` para
  os novos kinds, valida por kind, mantém event/task intacto, teto 31.
- **"Respond to Webhook"** e o Go: sem mudança de envelope (`proposals` é genérico).

Doc: [automation/CALENDAR_CHAT_WORKFLOW.md](automation/CALENDAR_CHAT_WORKFLOW.md).

## 6. Frontend — `web/app/`

- **`domain/calendar/calendar-chat-api.ts`**: `kind: 'event'|'task'|'note'|'clientProfile'`;
  `normalizeProposal` aceita os 4; tipos `note?`/`profile?` em `CalendarChatProposalFields`.
- **`composables/useCalendarChat.ts`** — `applyProposal()` ganha 2 ramos, executando pela API do
  usuário e reusando o canônico:
  - `note`: mês = `note.month || store.focusMonthKey`. `delete`→`putNotesForMonth(api,month,'')`;
    `replace`→conteúdo = `note.content`; `append`→`fetchNotesForMonth` + `note.content` (separados
    por `<p>`) → `putNotesForMonth`. Recarrega a nota do mês aberto.
  - `clientProfile`: `clientId` do cartão || `fields.clientId` (sem cliente → erro acionável).
    `fetchClientProfile` → aplica (merge / `clearFields` / `clearAll`) → `putClientProfile`.
- **`components/calendar/CalendarChatMessage.vue`** + **`utils/calendar-chat-proposal-preview.ts`**:
  `kindLabel`/ícones (`note`→"Anotação", `clientProfile`→"Perfil do cliente"); preview dos campos
  propostos (perfil: "Segmento → Luxo", "Limpar: História", "Zerar perfil"; nota: mês + modo +
  trecho); o seletor/popup de cliente do cartão passa a valer também para `kind==='clientProfile'`
  (não só `action==='create'`), cumprindo "inferir + confirmar no card".

## 7. Verificação (dev)

1. Go: `docker compose up -d --build api`; `golangci-lint`/`go build`/`go test ./internal/modules/calendar/...`.
2. Front: rebuild de dev (compose watch).
3. n8n: **re-importar** o workflow calendar-chat + `CALENDAR_CHAT_WEBHOOK_URL` configurado.
4. Browser: perfil por texto e por voz; "o que falta da Pérola?" (lista + insiste); anotação
   append/replace/limpar; "apaga a história da Pérola" e "zera o perfil da Bari".

## 8. Progress log

- **2026-07-10 — IMPLEMENTADO (backend + n8n + frontend + docs).** Validação estática limpa:
  - Go: `go build` + `go vet` + `golangci-lint` (0 issues) + `go test ./internal/modules/calendar/...` OK.
    Novos: `chat_proposals_crud.go` (tipos + sanitizers por kind); `sanitizeProposal` despacha por kind;
    `missingProfileFields` + `ProfileMissing` no lean (`runtime_context.go`).
  - n8n: `workflow-calendar-chat.json` — nós "Montar contexto" (schema note/clientProfile + linha de campos
    vazios + insistência) e "Extrair resposta" (sanitizer espelhando o Go, aliases pt) atualizados via patch
    Python (só os 2 literais jsCode; resto do arquivo intacto). JSON válido + `node --check` OK; simulação do
    extractor cobriu perfil (com aliases pt), nota append/replace/delete/vazia, clearAll/clearFields (filtra
    chave inválida), evento intacto.
  - Front: tipos (4 kinds + note/profile) em `calendar-chat-api.ts`; execução em `calendar-chat-crud.ts`
    (extraído do `useCalendarChat.ts` para não estourar mais o limite de linhas); preview + seletor de cliente
    generalizado (`needsClient`, `showBefore`, `kindIcon`) em `CalendarChatMessage.vue` +
    `calendar-chat-proposal-preview.ts`. eslint 0 erros; vue-tsc limpo nos arquivos tocados.
- **2026-07-10 — ajuste de prompt (validação do dono):** no teste, "quero só fazer a anotação do mês"
  virou proposta de **Tarefa** (sintoma do schema ANTIGO). Confirmado por `strings ~/.n8n/database.sqlite |
  grep -c canonicalKind` = **0** → o workflow novo NÃO estava importado no n8n local. Reforcei o nó "Montar
  contexto": ANOTACAO DO MES (kind:note) = "o QUADRO de ANOTACOES da ESQUERDA da tela", gatilhos explícitos
  (anotar/anota/nota do mês/…), SEMPRE kind:note (nunca event/task/clientProfile) + disambiguação
  perfil≠anotação. JS revalidado (`node --check`). Continua PENDENTE o re-import.
- **2026-07-10 — workflow RE-IMPORTADO no n8n local + AUTOMATIZADO.** O dono pediu para não depender de
  lembrar do re-import. Novo script `scripts/dev/n8n-import.ps1` + `npm run n8n:import` / `n8n:import:chat`
  (copia p/ o container → `n8n import:workflow` atualiza pelo id → reativa → `docker compose restart n8n`).
  Rodado: "Successfully imported 1 workflow" → reativado `calendarchat0001` → n8n reiniciado. Verificado por
  `n8n export:workflow` (WAL-aware): canonicalKind + "ANOTACAO DO MES" presentes, `active:true`, healthz 200.
  Regra gravada: EU re-importo sempre que mexer no workflow (memória feedback_n8n_import_on_change).
- **2026-07-10 — fix do CLIENTE na edição de perfil (validação do dono).** No print, ao editar a
  descrição da Pérola (escopo "Todos"), o card mostrava "Sem cliente"/seletor pedindo para escolher/trocar
  — sem sentido numa edição de perfil (o cliente é a IDENTIDADE do perfil). Correções:
  - **Front** (`CalendarChatMessage.vue`): `resolvedClientId` resolve pelo NOME (`resolveClientIdByName`
    contra a lista de clientes visíveis) quando a IA manda `clientName` mas não o id; `showClientPicker`
    só exibe o seletor para clientProfile quando o cliente NÃO foi resolvido (resgate) — resolvido, o
    título já mostra "Perfil de X" e não há dropdown de trocar.
  - **n8n** (`Montar contexto` + `Extrair resposta`): o prompt manda resolver `fields.clientId` (id real
    de `context.clients`) + `fields.clientName`, NÃO emitir proposta sem cliente identificado (pergunta no
    answer), não transformar edição direta em lista de perguntas; o extractor passou a capturar
    `clientName`. Re-importado via `npm run n8n:import:chat` + web rebuildado; verificado (extractor +
    prompt no workflow, active=true, web 200).
- **WAVE 8 (busca web) DOCUMENTADA no roadmap** (`cal-w8-chat-busca-web`, pending): sob demanda + Tavily.
- **2026-07-10 — 2 bugs das ANOTAÇÕES do mês corrigidos (validação do dono):**
  - **Nota não persistia (perda de dado):** no banco TODA nota estava `<p></p>` (vazio), inclusive meses
    nunca tocados. Raiz no `OmniEditor.vue`: o guarda `shouldIgnorePassiveEmptyEmission` só ignorava
    emissão vazia quando o `modelValue` ATUAL tinha conteúdo. No reload a nota carrega async → o editor
    monta vazio → emite `<p></p>` (não focado, prop ainda '') → guarda NÃO ignora → agenda PUT vazio
    (debounce 800ms) → a nota real chega, mas o PUT vazio dispara depois e a apaga. Fix: ignorar TODA
    emissão vazia quando o editor não está focado (limpar de verdade é com foco). Verificado por logs
    (PUTs 200) + `psql` no `calendar.notes` (content=`<p></p>`).
  - **Conteúdo por cima da toolbar ao rolar:** em `notes-drawer.css` a toolbar do editor estava
    `background: transparent` (para não virar um quadrado sólido sobre o vidro) → o conteúdo rolado
    vazava por cima. Fix: fundo FROSTED (`rgb(var(--surface)/0.85)` + `backdrop-filter: blur`).
- **2026-07-10 — IA "mentindo" que fez (crítico, validação do dono):** no print a IA disse "Vou
  adicionar" e depois "Adicionei a descrição" ao perfil da Pérola — mas NÃO gerou cartão e NADA foi
  aplicado (campo vazio). O LLM narrou a ação como concluída em vez de emitir a proposta (armadilha de
  QUALQUER modelo — ver [[project_calendar_ai_proposal_flow]]; CORREÇÃO 2026-07-11: o provider real era
  OpenAI gpt-4o-mini configurado no painel, não gemini — nunca supor provider, checar calendar.config).
  Regra do dono: a IA só PROPÕE, toda alteração
  passa pela aprovação no cartão; a IA NUNCA pode dizer que fez. Correções (2 camadas):
  - **Prompt (n8n `Montar contexto`):** REGRA DE OURO no FIM (recência pesa mais) — "você NUNCA executa/
    aplica/salva nada; toda alteração só acontece se o usuário aprovar o CARTÃO; sempre coloque a mudança
    em proposals[]; NUNCA diga que fez/adicionou/criou/editou/salvou (seria mentira); se faltar dado,
    pergunte". + o bullet do answer curto proíbe afirmar que a ação foi feita. Re-importado via `npm run
    n8n:import:chat`.
  - **Front (guarda determinístico, `CalendarChatMessage.vue`):** se uma msg da IA usa verbo de ação em
    1ª pessoa no passado ("adicionei/criei/editei/…") E não tem cartão nem resultado, mostra aviso âmbar
    "nada foi aplicado — nenhuma alteração é salva sem você aprovar num cartão". Backstop caso o LLM
    escorregue.
- **2026-07-10 — WAVE 9 (edit inline nos cards) + WAVE 10 (tempo real do perfil):**
  - **Edit inline (WAVE 9):** cada proposta pendente ganhou botão Editar (lápis) que abre inline os campos
    QUE a IA propôs (alcance escolhido pelo dono); ao Aplicar usa os valores editados. Helpers puros em
    `utils/calendar-chat-proposal-edit.ts` (editableFields + get/setFieldByPath); estado (editingIds/edits)
    + wrappers no `CalendarChatMessage.vue`; `confirmSelectedProposals`/`applyProposal` aceitam os fields
    editados (override sobre os da IA). Edição é local (antes de aprovar); toda alteração continua exigindo
    aprovação no cartão.
  - **Tempo real do perfil (WAVE 10) — reuso do WebSocket, SEM SSE:** back publica
    `calendar.client_profile_updated` (resourceId=clientId) no `PutClientProfile` (publisher.go +
    realtime/model.go const espelho); `useCalendarRealtime` roteia o evento; `ConfigClientProfiles` assina
    (`onClientProfileUpdated`) e refaz o fetch do índice + recarrega o perfil aberto (respeitando `touched`).
    Aprovar um card de perfil atualiza a aba Clientes na hora — pro dono (o próprio WS recebe o broadcast) e
    pra quem estiver junto — sem reload. Decisão: SSE seria 2º sistema paralelo ao WS existente = mais
    pesado; o que dá leveza é a invalidação enxuta (dica pequena + refetch), independente do transporte.
  - Estático: eslint 0 erros (2 warnings max-lines pré-existentes/incrementais); vue-tsc limpo nos meus
    arquivos (só o erro pré-existente de `ui.confirm`); go vet/build OK. api+web rebuildados.
- **2026-07-10 — 3 fixes do cruzamento task↔calendário↔chat (validação do dono):** editar uma task pelo
  chat atualizava a task mas o **evento do calendário não refletia a DESCRIÇÃO** (título/data/etc. já
  sincronizavam) e uma 2ª edição falhava com "1 falharam" sem motivo. Correções:
  - **Descrição task→evento (assimetria):** `applyTaskSyncToEvent` nunca setava `in.Description` (o
    `eventToInput` preservava a antiga = vazia); o sentido evento→task já levava a descrição. Adicionei
    `ContentHTML` ao `TaskSyncSnapshot` (platform) + populei em `tasks/service.go` + em `task_sync.go`
    converto o HTML da task em texto simples (`htmlToPlainText`, inverso do `descToHTML`) e seto a
    descrição do evento **só quando a task tem corpo** (guarda anti-clobber). Terminal nos 2 sentidos =
    sem loop. go build/test OK.
  - **Erro engolido → motivo real:** `confirmSelectedProposals` mostrava só "N falharam"; agora acumula
    o `lastError` do `applyProposal` e exibe "Apliquei X de Y. Z falhou. Motivo: …".
  - **Conflito de versão (provável causa da 2ª falha):** quando a task sincroniza pro evento no back, o
    store do calendário fica com a `version` defasada → o próximo update do chat dava 409. `applyProposal`
    agora, no conflito, faz `refetchWindow` + tenta 1× com a versão fresca antes de pedir "recarregue".
  - api **force-recreate** (o `up --build` não recriava) + web rebuild; eslint 0 erros.
- **2026-07-10 — causa raiz da 2ª edição de task falhar (diagnóstico no banco):** com o "Motivo" agora
  visível, o erro era "Não encontrei essa task no board configurado". Query em `calendar.chat_messages`
  mostrou: a 1ª proposta (OK) tinha `targetId = <UUID da task>`; as que falhavam tinham
  `targetId = "Brasil Gamers"` (o NOME antigo!). Ou seja: a IA às vezes manda o NOME no `targetId` em vez
  do id (o contexto do chat é fresco do banco — NÃO é problema de websocket). Fixes:
  - **Prompt (n8n `Montar contexto`):** TASK update/delete → `targetId` TEM que ser o UUID de
    context.tasks/context.events, NUNCA o nome/título nem id de memória; procurar a task pelo nome em
    context.tasks e usar o `id`. Re-importado + n8n reiniciado (regra confirmada ativa).
  - **Rede de segurança (front, `applyTaskTarget`):** se não achar a task pelo id, tenta casar pelo
    TÍTULO atual (match único) — cobre o escorregão de nome atual.
- **2026-07-11 — auditoria "n8n sem modelo/prompt próprio" (cobrança do dono):** verificado no banco
  (`calendar.config->ai`: provider=**openai**, model=**gpt-4o-mini**, temp 0.7, systemPrompt = o prompt
  do painel) e no workflow (o nó "Montar contexto" lê `ai.provider/model/baseUrl/temperature/systemPrompt/
  apiKey` do `body.ai` que o Go monta da config do painel). **SEM regressão**: os `|| 'gemini'` etc. são
  fallback SÓ para campo vazio (design "vazio = default do provider"); o que o n8n anexa ao prompt do
  painel é apenas o CONTRATO técnico (contexto + envelope JSON das proposals), sem persona própria. Meu
  erro foi chamar o modelo de "gemini" por suposição — o targetId-com-NOME aconteceu no gpt-4o-mini.
  Regra gravada ([[feedback_ai_config_from_panel]]): nunca supor provider (checar o banco); robustez de
  parsing (targetId por nome → fuzzy match) mora no front/sanitizer, não em empilhar prompt.
- **2026-07-11 — 2 ajustes (pedido do dono):**
  - **"Criar task" do evento leva TUDO:** `createLinkedTask` (usado pelo botão do badge "evento sem
    task" E pela criação com toggle) já levava título/descrição/prazo/cliente/responsável(id)/coluna;
    agora tem PARIDADE com o `syncTaskFromEvent`: + `Priority`, + `ui_metadata.type` (tipo do evento),
    + `ui_metadata.responsible` (NOME do responsável cacheado) e + `ui_metadata.calendarMedia` (mídia do
    evento espelhada read-only via `eventMediaForTask`). go build/test OK; api recriada.
  - **Busca no seletor de conversas do chat:** campo "Buscar conversa..." no dropdown (foco automático ao
    abrir), filtro client-side por TÍTULO e AUTOR (sem acento/caixa), aviso "nenhuma conversa combina";
    limpa ao fechar. `CalendarChatConversations.vue` + `chat-conversations.css`; web recriado.
- **2026-07-11 — aba IA da config reordenada (pedido do dono):** todos os collapses FECHADOS por
  padrão (removido o `open` das Chaves) e nova ordem: **Prompt do sistema PRIMEIRO** (a lei da IA) →
  Provedor e modelo → Transcrição → Escopo por cliente → **Chaves de API por ÚLTIMO**. `ConfigAi.vue`;
  web recriado. + **Checklist de conferência do dono REGISTRADA no roadmap**
  (`cal-w11-backlog-conferencia`): mídias como tarefas especiais (sem título na visão geral; título =
  nome do arquivo na task), 1ª mídia por tarefa, drag-and-drop de ordem, assistant na aba Tasks, tasks
  sem data, tarefas multi-dia (com ocultar), modo expandido do chat (bug de quebra de linha), bug visual
  de anotações, conferir IA editar tarefas campo a campo — especificar/priorizar antes de implementar.
- **2026-07-12 — WAVE 11 IMPLEMENTADA ("todas as tarefas", decisões do dono na conversa):**
  - **A1 Horário em task via chat:** `applyTaskTarget` agora compõe `dueDate+T+time` (create e update;
    time sem data nova reusa a data atual da task) — `toOptionalDateTime` converte hora local→ISO. O
    prompt lista `time` nos campos de task.
  - **A2 Tasks sem data:** já entravam no contexto (ListTasks sem filtro); prompt ganhou instrução
    explícita ("dueDate vazio = sem data; liste exatamente essas"). Re-importado.
  - **A3 Modo expandido do chat:** teto tipográfico da mensagem 46rem→72rem (min(94%,72rem)) — o texto
    usa a largura da janela expandida/fullscreen.
  - **B Mídias:** (i) upload avulso no dia VIRA ITEM ESPECIAL — `EventInput.IsMediaItem` (json
    `mediaItem`) → server seta `source='media'` (nunca aceita 'task' do body); DayDrawer cria evento
    título=nome do arquivo + `createTask` (task nasce com o nome do arquivo); falha → anexo preservado
    no day_media; EventChip com source='media' esconde o TÍTULO (só ícone de mídia). (ii) fundo do dia =
    **1ª mídia POR evento** (dayBackgroundUrls; a ordem define a 1ª). (iii) **drag-and-drop** reordena
    as mídias no uploader (HTML5 nativo; persiste pelo mesmo update:modelValue).
  - **C Barra multi-dia (estilo Google):** util novo `calendar-task-spans.ts` (spans de tasks com
    dueDate→dueEndDate; lanes greedy, teto 3); MonthGrid posiciona DayCell explicitamente no grid e
    renderiza as barras na MESMA row (colunas start..end, align-self:end, empilhadas por lane); cor do
    cliente; clique abre o card no board (deep-link WAVE 5); toggle mostrar/ocultar nos controls
    (localStorage `omni.calendar.spans.show`); tasks carregadas lazy do board configurado; respeita o
    filtro de cliente. (WeekView fica para depois — anotado.)
  - **D Assistant em /tasks:** FAB fixo (canto inferior direito) na página de tasks abre o MESMO
    Crow Assistant (painel lazy via defineLazyComponent; `calendarStore.init()` guardado + openPanel).
  - Estático: go build/test OK; eslint 0 erros; vue-tsc limpo nos arquivos tocados (só o pré-existente
    de `ui.confirm`); api+web force-recreate (200).
- **2026-07-12 (tarde) — atalhos por CAPTURA + combos + reorg + cores do Manage:**
  - **Captura de tecla (não digitação):** o campo de atalho virou botão "clique e pressione"; grava a
    tecla/combinação REAL. **Modificadores + combinações** (Shift/Alt/Ctrl/Meta) via `event.code`
    (estável a layout/Shift): `shortcutComboFromEvent` gera 'shift+t', 'ctrl+shift+k', etc. — a MESMA
    função no runtime, na captura e espelhada no back (`canonicalShortcut`/`sanitizeShortcuts`). × desliga.
  - **Aba Aparência reorganizada:** todos os blocos viraram COLLAPSES fechados (padrão da aba IA);
    atalhos em linhas compactas.
  - **Cores do Manage (fora do escopo do calendário, mas registrado):** Banco/Roadmap/Auditoria usavam
    cor escura FIXA ignorando os tokens (cards escuros + texto lavado no tema claro). 202 trocas →
    tokens. Botões da Fila achatados (gradiente→sólido + texto branco). Detalhe no roadmap
    `cal-w11-cores-tokens`. Pendente: Usuários (classes Tailwind fixas) é refactor à parte.
- **2026-07-12 — bug visual das ANOTAÇÕES (print do dono) + MAPA DE ATALHOS configurável:**
  - **Bug visual (fix ESTRUTURAL):** o texto rolado atravessava a toolbar do editor (o frosted não
    segurou no browser dele). `OmniEditor.vue` reestruturado: a TOOLBAR saiu da área de rolagem — o
    root virou flex-column `overflow:hidden`, e o scroll mora SÓ no `.omni-editor__content`
    (flex:1 + overflow-y:auto). Sobreposição impossível em qualquer tema/browser; vale para todos os
    usos do editor (notas, tasks). O frosted do notes-drawer ficou (cosmético).
  - **Atalhos (WAVE 11, pedido + sugestões aceitas):** mapa `{ ação: tecla }` na config por conta
    (`config.shortcuts`, jsonb; `sanitizeShortcuts` no PUT — ações whitelist, tecla 1-char/especial,
    vazio = desligado; ausente = default). UI na aba Aparência (grupos Assistente/Página + restaurar
    padrão). Execução: composable `useCalendarShortcuts` (keydown global; ignora campos editáveis
    exceto `force`; ignora modificadores). Defaults: **chat** C abrir/fechar, A gravar, Enter parar
    (force), Esc fechar (force, "mesmo sem input em foco"); **calendário** T hoje, M mês, W semana,
    N novo, S recolher/mostrar anotações, B barras multi-dia, ←/→ navegar. Registrados na página
    (index.vue) e no CalendarChatPanel (gravação via guarda de estado do onMic).
- **PENDENTE:** validar no browser TODA a leva (nota persiste; IA gera CARTÃO; perfil resolve cliente;
  edit inline; tempo real do perfil; fuzzy+descrição task; criar task completa; busca de conversas; aba
  IA na ordem nova; horário em task pelo chat; tasks sem data; modo expandido; upload avulso vira item
  especial; 1ª mídia por item; drag-and-drop; barras multi-dia + toggle; FAB em /tasks; **toolbar das
  anotações sem sobreposição; atalhos + config de atalhos**).
- **PRÓXIMO (decidido, a especificar):** busca na web SOB DEMANDA (só quando o usuário pedir OU a IA
  perguntar "quer que eu pesquise?" e ele confirmar — controla custo), provedor **Tavily** (free tier);
  resultado alimenta a resposta com fonte e pode virar proposta de preencher o perfil (WAVE 7). Precisa de
  plano/specs (doc-first) + chave Tavily.
