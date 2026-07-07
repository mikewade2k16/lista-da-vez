# Calendário — Plano (front preview → back)

> Doc canônico do módulo Calendário (`/calendario`). Fonte de verdade da visão e do
> escopo do back. Espelhar mudanças aqui + em `web/app/components/AGENT.md` (seção
> `calendar`) + em `roadmap-data.ts` (regra dos 3 docs).

## 1. Visão / objetivo

Agenda de **conteúdo por cliente** da agência. Cada dia carrega o que vai ter naquele
dia: qual **cliente**, quais **postagens** (post/story/reels/reunião/gravação/evento),
com **status / prioridade / responsável** e anotações. Objetivos:

- **Cliente pode ver** o que está agendado (o que será entregue e até quando).
- **Equipe se programa** — sabe o que precisa entregar até qual data.
- Anotações por dia/mês sobre o que vai rolar.

## 2. Estado atual

- Página `/calendario`: visões **Mês** e **Semana**, scroll + **CSS Scroll Snap**
  (foco centralizado com peek), rail lateral `M / S1..Sn`, notas por mês, drawer do dia.
- **✅ Fase 1 (CRUD) LIGADA**: EVENTOS e NOTAS vêm do **back real** (`/v1/calendar/*`,
  módulo Go `back/internal/modules/calendar`, migration 0181). Front: `stores/calendar.ts`
  busca eventos por janela (`from`/`to`), notas por mês (save debounced), e faz CRUD via
  `CalendarEventForm` + Editar/Excluir no drawer. **CLIENTES = reais** (`useTenantsStore`).
- **✅ Fase 2 LIGADA (dados 100% reais, zero mock)**:
  - **Responsáveis = usuários reais** da conta (`GET /v1/calendar/responsibles`, subconjunto
    configurável). `store.people` = responsáveis; `useCalendarData` (mock) **removido**.
  - **Feriados/datas comemorativas** computados no back (`GET /v1/calendar/holidays?from=&to=`,
    conjuntos BR nacional / Sergipe / Aracaju / luxo internacional, móveis via Páscoa) e
    renderizados em `DayCell`/`WeekView`.
  - **Modal de configuração** na página (engrenagem): escolhe quais usuários aparecem como
    responsáveis + liga/desliga cada conjunto de feriados. Persistido por conta em
    `calendar.config` (jsonb) via `GET/PUT /v1/calendar/config` (migration 0182).
- Gate de API `/v1/calendar` existe (platform_admin bypassa); gate de rota/nav no front ainda
  é preview.
- **✅ Fases 3c–6 IMPLEMENTADAS no código (2026-07-02, orquestração multi-agente — specs e
  progress log em [CALENDARIO_SPECS.md](CALENDARIO_SPECS.md))**: viewer de mídia + poster de
  vídeo via canvas + fundo do dia em grade; perfil estratégico do cliente (migration 0185);
  página `/calendario/config` (modal migrado e deletado) com config v2 (cores/semana/white-label)
  + config da IA; endpoints de plano de IA (migration 0186) + workflow n8n autorado.
  Migrations aplicadas, api rebuildada, golangci/eslint limpos. **PENDENTE: validação no
  dev/browser + import do workflow no n8n** (regra: implementar e parar pra revisão).

## 3. Back — escopo (várias fases)

### 3.1 Dados (troca o mock)
- `calendar_events`: id, **account_id** (tenant, do Principal — nunca do body), date,
  time, client_id, type, title, status, priority, responsible_id, involved[], media[],
  description, timestamps.
- `calendar_notes`: account_id, month_key (`YYYY-MM`), content, updated_by, updated_at.
- Endpoints: `GET/POST/PUT/DELETE /v1/calendar/events` (update = PUT full-replace),
  `GET/PUT /v1/calendar/notes/{month}`.
- Isolamento multi-tenant e projeção lean (só a janela pedida).

### 3.2 Datas comemorativas & feriados — ✅ FEITO
Marcados no dia (faixa/badge em `--accent-warning`), cada conjunto **ativável/desativável**:
- **Nacionais (BR)** — fixos + móveis calculados (Carnaval/Sexta-feira Santa/Corpus Christi via
  Páscoa/Meeus; Dia das Mães/Pais por n-ésimo domingo).
- **Estaduais (Sergipe)** e **municipais (Aracaju)**.
- **Internacionais p/ marcas de luxo** — Valentine's, Dia da Mulher, Halloween, Natal, Réveillon,
  Black Friday + Cyber Monday (móveis).
- Implementação: **sem tabela de seed** — `back/.../calendar/holidays.go` gera as datas do range
  sob demanda (`HolidaysInRange`) e filtra pelos conjuntos ligados na config da conta.

### 3.3 Configuração / White-label (modal na página)
Modal de config (`CalendarConfigModal.vue`, engrenagem nos controles), persistido por conta em
`calendar.config` (jsonb, migration 0182):
- ✅ **Responsáveis**: quais usuários reais da conta aparecem na lista (vazio = todos).
- ✅ **Feriados**: liga/desliga cada conjunto (nacional, SE, Aracaju, luxo internacional).
- ⏳ **Cores**: por cliente e/ou por tipo de evento; opção **"sem cor"**.
- ⏳ Início da semana (domingo/segunda).
- ⏳ **White-label**: aparência do painel/visão exposta ao cliente (logo/cores) — configurável
  direto no painel, não no código.

### 3.4 Visão compartilhável (cliente vê)
- Visão read-only do calendário do cliente (o que será entregue até quando), respeitando
  o white-label.

### 3.5 Perfil do cliente (insumo p/ a IA de conteúdo)

Para o botão **"Gerar plano/sugestões"** (IA monta o plano estratégico do mês por cliente),
cada cliente tem um **perfil estratégico**. **Decisão (2026-07-02): tabela dedicada 1:1 por
account** (não coluna em `core.accounts`, que é enxuta de identidade/billing) — migration
**0185**, chave `account_id`, com colunas para os campos estáveis (segmento, posicionamento,
site, instagram, endereço, objetivos, tom de voz) + um `jsonb` para os campos flexíveis
abaixo. Endpoints `GET/PUT /v1/calendar/client-profile?clientId=`.
Campos que alimentam a IA:

- **Identidade**: segmento/nicho, descrição do negócio, posicionamento (luxo/popular/…), região.
- **Público-alvo**: persona (idade/gênero/interesses/dor-desejo/classe).
- **Voz da marca**: tom de voz, do's & don'ts, palavras/hashtags fixas, emojis sim/não.
- **Oferta**: produtos/serviços, carro-chefe, lançamentos, faixa de preço, promoções recorrentes.
- **Estratégia**: objetivos do mês, pilares de conteúdo (+ proporção), cadência, canais ativos,
  datas fixas do cliente (aniversário da loja, coleções).
- **Restrições**: temas proibidos, compliance (farmácia/bebida/saúde etc.).
- **Histórico**: posts que performaram (métricas) + feedbacks — a IA aprende com o tempo.
- **Assets**: logo, paleta, moodboard.

Mínimo para começar a gerar algo bom: **segmento, descrição, tom de voz, público, produtos,
objetivos, pilares, cadência**. A IA monta o prompt com **perfil + mês + datas comemorativas +
histórico** → devolve **pilares + ideias por dia + rascunho de copy**. Dá pra gerar por **um
cliente OU vários** de uma vez. **A orquestração e os providers estão no §3.7** (n8n "Calendar
Omni" + seletor de provider: Claude + IAs chinesas). O resultado é **salvo** (§3.7): vira
rascunho e pode ser aplicado como notas do mês e/ou eventos nos dias.

### 3.6 Anexos / mídia (imagens e vídeos) — Fase 3 (em andamento)

Anexos em **dois lugares** (decisão do dono):
- **No evento/post** (`calendar.events.media` — jsonb já existe): a criação daquele post.
- **No dia** (avulso): referências/moodboard do dia, sem vínculo com um evento.

**MediaItem** (item de mídia, jsonb):
`{ id, url, name, type: "image"|"video", contentType, sizeBytes }`. `url` sempre aponta para
`/uploads/calendar/{accountId}/{arquivo}` (nunca URL externa).

**Fluxo de upload (stateless, espelha cardápio/tasks):**
1. `POST /v1/calendar/media` (multipart, campo `file`) — valida mime (imagem: jpg/png/webp/gif;
   vídeo: mp4/webm/mov) + tamanho, grava em `uploads/calendar/{accountId}/` e devolve o MediaItem.
2. O front adiciona o item ao array de mídia do evento (ou do dia) e **salva** (full replace, padrão
   do módulo). Assim o `PUT` do evento/dia não precisa saber de upload.

**Limite de tamanho — definido na plataforma (global, editável por platform_admin):**
- Guardado em `core.platform_settings`, chave `media_limits`:
  `{ imageMaxBytes (default 10MB), videoMaxBytes (default 300MB) }`.
- `GET /v1/calendar/media-limits` (qualquer autenticado — o front mostra o limite e valida no
  cliente antes de subir) · `PUT /v1/calendar/media-limits` (só platform_admin).
- O handler de upload lê o limite no ato; vídeo acima do teto → 400 `invalid_media`.

**Anexos do dia:** tabela `calendar.day_media (account_id, event_date, media jsonb)` —
`GET /v1/calendar/day-media?from=&to=` (janela, igual eventos) + `PUT /v1/calendar/day-media/{date}`
(full replace da lista daquele dia).

**Orfãos:** remover um item do array NÃO apaga o arquivo do disco (v1 tolera; GC/limpeza fica p/
depois). **Multi-tenant:** account do Principal (nunca do body); arquivos por `uploads/calendar/{account}/`.

**Mídia rica (Fase 3c — NOVO):**
- **Clicar para abrir** (lightbox/player): todo anexo (do evento e do dia) abre num visualizador
  — imagem em tela cheia, vídeo com player (play/pause). Componente novo `CalendarMediaViewer.vue`
  (não há lightbox reutilizável no app hoje). Navegação entre os itens do dia (próximo/anterior).
- **Poster do vídeo** — **decisão (2026-07-02): capturado no FRONT** via `<canvas>` no momento do
  upload (1 frame) e enviado como imagem-poster pelo mesmo `POST /v1/calendar/media`. O `MediaItem`
  ganha `posterUrl` (opcional). **Sem dependência nova no servidor** (não instala ffmpeg) e sem
  migration (o campo mora no jsonb do `MediaItem`). Fallback: se a captura falhar, usa `<video>`.
- **Fundo do dia = a postagem daquele dia** (`DayCell`/`WeekView`): a célula do dia recebe as
  mídias do(s) evento(s) daquele dia como **background**. Regra de grade: **1 conteúdo = fundo
  inteiro; 2 = fundo dividido em 2; 3-4 = grid 2×2; N > 4 = grid + "+N"**. Vídeo entra pelo
  `posterUrl`. **Legibilidade:** overlay/gradiente por cima garante contraste do número do dia,
  feriados e chips (nunca ilegível). Preferência: mídia do **evento**; anexos avulsos do dia
  entram só se não houver evento com mídia (evita poluição). Respeita o filtro de cliente ativo.

### 3.7 IA de Calendário — plano estratégico do mês (n8n "Calendar Omni") — NOVO

Botão de IA na página (nas notas do mês e/ou nos controles): escolhe **1 ou vários clientes** +
o **mês** → a IA lê **perfil (§3.5) + mês + feriados/datas + histórico** → devolve um **plano
estratégico**: pilares de conteúdo, ideias por dia, rascunho de copy e sugestão de tipos
(post/story/reels). O resultado é **salvo** (rascunho) e pode ser **aplicado**: joga o texto nas
**notas do mês** e/ou cria **eventos** nos dias.

- **Orquestração no n8n** (workflow **"Calendar Omni"**), no mesmo padrão da automação: o n8n
  **não fala com o Postgres direto** — bate na **API Go** (token de serviço por account) para ler
  perfil/feriados e para **gravar** o plano. O back expõe o gatilho (`POST /v1/calendar/ai/plan`,
  body = `clientIds[]` + `month`) que chama o n8n (webhook) e persiste a resposta.
- **Provider plugável — decisão (2026-07-02): SELETOR desde a v1.** Claude é o padrão; **IAs
  chinesas via API compatível com OpenAI** entram como opção: **DeepSeek**, **Qwen/DashScope
  (Alibaba)**, **Moonshot/Kimi**, **Zhipu GLM**. No n8n é o nó *OpenAI* com *Base URL* trocado (ou
  *HTTP Request* genérico). O provider/modelo/prompt escolhidos vêm da **config** (§3.8); as
  **chaves de API ficam server-side** (env var / `core.platform_settings`), **nunca** no jsonb
  lido pelo front.
- **Persistência do plano** — tabela `calendar.ai_plans` (account_id, month_key, client_ids jsonb,
  status `draft|applied`, content jsonb, model/provider, created_by, timestamps) OU rascunho nas
  próprias notas com marcação. A definir na fase (ver §4).

### 3.8 Página de configuração do Calendário + IA — NOVO

Hoje a config é um **modal** (só responsáveis + feriados). Evoluir para uma **página** de config
(`/calendario/config` ou aba), com gate de rota próprio, reunindo:
- Tudo do modal atual (responsáveis + conjuntos de feriados).
- **Cores** por cliente e/ou por tipo de evento (+ "sem cor") · **início da semana** ·
  **white-label** (logo/cores da visão exposta ao cliente) — itens já previstos no §3.3.
- **Config da IA de calendário**: provider ativo + modelo + *Base URL* (por provider) + **prompts
  editáveis** (system prompt do plano) + limites (tamanho do plano, temperatura). Chaves de API
  **server-side**. Espelha o padrão "config-driven pelo painel" da automação.
  - **Modelo = SELECT do provedor (Opção C, 2026-07-07)**: o campo Modelo deixou de ser texto livre. Ao
    escolher o provedor, o painel lista os modelos REAIS via `GET /v1/calendar/ai/models?provider=` (o back
    resolve a chave server-side e bate no `/models` do provedor no endpoint **canônico** — a *Base URL* do
    cliente **não** é usada na listagem, para não abrir SSRF; segue valendo no dispatch). Isso elimina a
    armadilha `provider=OpenAI` + `model=gemini-*`. Decisão: **select sempre, sem digitação**; sem
    conseguir listar (sem chave / provedor sem `/models` / API falhou) o campo fica **desabilitado com
    aviso + "Tentar novamente"** (o provedor precisa expor `/models`). Front: `ConfigAiModelSelect.vue`;
    back: `ai_models.go`/`http_ai_models.go`.
- **Criação MULTI-TAREFA pelo chat (WAVE 5.1, 2026-07-07)**: o chat deixou de propor 1 item por resposta —
  agora devolve `proposals[]` (lista) e o usuário aprova em LOTE. Decisão: **enumerar cada item** (a IA lista
  uma proposta por data; "uma por dia" = N itens, teto 31) + **seleção com criar em lote** (checkbox por item,
  `Criar N selecionadas` / `Recusar todas` / `×` individual). Persistência: `calendar.chat_messages.proposals`
  jsonb (migration 0195), cada proposta com `status` próprio + `action` (`create`|`update`|`delete`)
  usado pelo **CRUD pelo chat (2026-07-07)**: `action=update` (editar evento existente por `targetId` = id de
  `context.events`, full-replace mesclando campos não-vazios) e `action=delete` (excluir por `targetId`), além de
  `create`. Front `applyProposal` (era `createFromProposal`) faz o switch via `store.getEventById`/`updateEvent`/
  `deleteEvent`; o card mostra a ação (Criar/Editar/**Excluir** com acento de perigo) e some o seletor de cliente
  no delete; `sanitizeProposal` (Go) valida por ação (update/delete exigem `targetId`; delete dispensa título).
  **Limite atual:** update/delete só em **EVENTOS** (estão no contexto). Editar/excluir **tasks do board** pelo
  chat depende do chat LER as tasks (roadmap) — por ora responde "em breve". n8n `workflow-calendar-chat` reimportado (prompt
  → `proposals[]`, parse/respond em array). Front: `CalendarChatMessage.vue` (lista) + `useCalendarChat.ts`
  (`confirmSelectedProposals`/`rejectSelectedProposals`). **Roadmap encadeado (pedido do dono):** (a) CRUD
  completo pelo chat (pegar tarefa → editar/excluir, com confirmação — o schema `action`/`targetId` já está
  pronto); (b) chat LER todas as tasks (não só as espelhadas no calendário) — exige o contexto puxar do módulo
  `tasks`; (c) levar o chat para a **página de Tasks** (o composable é singleton, é wiring de montagem).
- **Cliente na criação + cards colapsáveis (WAVE 5.2, 2026-07-07, só front)**: escopo **cliente** → tudo
  criado já vai para ele (clientId forçado, sem perguntar); escopo **todos** → seletor de cliente por
  proposta (editar uma a uma) + popup **[Continuar sem cliente]/[Escolher cliente]** (aplica um para todas)
  quando falta cliente em algum selecionado. Os cards de propostas viraram **colapsáveis** (minimizar).
  `CalendarChatMessage.vue` recebe `clients`/`scopeMode`/`scopeClientId`; `createFromProposal(proposal,
  clientId)`. Sem back/n8n/migration — o clientId entra na criação autenticada existente. **Mesma regra no
  BOTÃO** (`CalendarEventForm.vue`): criar evento novo pré-seleciona o cliente do filtro ativo do calendário
  (`store.selectedClientId`); editar mantém o do evento; "todos" = sem cliente (o usuário escolhe no select).
- **Limites de mídia** (imagem/vídeo) — hoje só via `PUT /media-limits` (platform_admin); expor na
  página para o admin.

### 3.9 Config em drawer lateral com abas (redesign da /calendario/config) — WAVE 2

A página `/calendario/config` (Fase 5) ficou **desorganizada**: 6 cards heterogêneos num grid
`auto-fit` (layout "esburacado"), **3 modelos de salvar coexistindo** na mesma tela (o footer
"Salvar configurações" NÃO salva perfil de cliente nem limites de mídia), dirty-guard
inconsistente (`window.confirm` fora do padrão `ui.confirm`) e o "Voltar" descarta edição sem
avisar. **Decisão (2026-07-04): a config volta a ser um drawer lateral, agora organizado por
abas**, aberto na engrenagem SEM sair do calendário (preserva scroll/mês em foco):

- **Casca canônica**: `OmniEntityDrawer.vue` (mesma do modal de tasks) — teleporta pro body
  (resolve o `overflow:hidden` da `.calendar-page`), modo side redimensionável.
- **Abas por categoria**: Responsáveis · Feriados · Aparência (semana + cores + white-label) ·
  IA (config do plano + chat) · Cliente (perfis estratégicos) · Integrações (tasks, §3.11) ·
  Mídia (limites, platform_admin).
- **1 aba = 1 modelo de salvar**, sem ambiguidade: as abas que editam o `CalendarConfig`
  compartilham o draft + botão do footer do drawer; Cliente e Mídia salvam por conta própria
  com botão claro DENTRO da aba. Dirty-guard único via `ui.confirm` ao fechar/trocar de aba.
- **Deep-link**: `/calendario?config=<aba>`; a rota `/calendario/config` passa a redirecionar
  (links antigos continuam funcionando). A página é deletada após a migração (sem código morto).
- Os componentes `config/Config*.vue` são REAPROVEITADOS dentro das abas (não reescrever).

### 3.10 Chat de IA do calendário + voz (Whisper) — WAVE 2

Botão flutuante (FAB, Teleport pro body) na página do calendário abre um **chat sobreposto**
(painel no canto inferior direito); o mesmo painel pode ser aberto pela aba IA do drawer.
Assistente conversacional com o contexto do calendário/cliente — termina de configurar o n8n
e serve de cockpit de conteúdo.

- **Topologia travada (igual Omni Chat)**: browser → `POST /v1/calendar/chat/ask` (API Go) →
  webhook n8n `calendar-chat` (workflow novo) → resposta síncrona. O navegador NUNCA fala com
  o n8n; chaves de API SÓ nas credentials do n8n.
- **Contexto injetado pelo Go** (fonte = banco): config da IA da conta (provider/modelo/
  baseUrl/prompt/temperature), perfil estratégico do cliente selecionado, mês em foco,
  feriados do mês, nota do mês e resumo dos eventos. `sessionKey = accountId|userId|
  conversationId` (memória por conversa, "nova conversa" zera).
- **Memória no Redis** (Redis Chat Memory do n8n, redis do profile automation já existe) —
  sobrevive a restart do n8n, ao contrário do buffer do Omni Chat.
- **Provider Gemini para teste grátis**: entra como provider de 1ª classe no seletor
  (`gemini`, free tier do Google AI Studio via endpoint OpenAI-compatible) — enum no Go/TS +
  mapa de baseUrl no workflow. Claude continua padrão.
- **Voz (Whisper)**: botão de microfone no chat grava (MediaRecorder) → `POST
  /v1/calendar/chat/transcribe` (Go encaminha ao mini-workflow n8n `calendar-transcribe`, nó
  OpenAI transcribe `whisper-1` — o MESMO nó já comprovado no bot de WhatsApp) → texto volta
  pro input para revisar e enviar. Alternativa barata (Groq whisper free tier via HTTP)
  documentada no workflow.
- **IAs se conhecem (conhecimento compartilhado)**: novo endpoint runtime `GET
  /v1/runtime/calendar/context?accountId=&clientId=` (Bearer `AUTOMATION_RUNTIME_TOKEN`)
  agrega perfil + eventos próximos + nota + planos — QUALQUER workflow (WhatsApp, Omni Chat,
  Calendar Chat) pode consumir o mesmo contexto pela API Go (n8n nunca no Postgres). Memória
  longa compartilhada reusa `GET/PUT /v1/runtime/automation/memory` (migration 0145).
  **Escopo da wave 2**: entrega a CAPACIDADE (endpoint + doc); ligar os workflows JÁ
  existentes (bot WhatsApp, Omni Chat) a esse contexto é etapa futura do plano de automação.

### 3.11 Integração Calendário ↔ Tasks — WAVE 2

Evento agendado no calendário (ex.: gravação) vira/vincula **task no board** — a equipe vê o
trabalho na página de tasks sem redigitar.

- **Vínculo pela infra existente**: `tasks.task_relations` (module=`calendar`,
  resource_type=`event`, resource_id=eventId) + `RelationResolver` do calendar registrado no
  registry (o modal da task mostra label/link do evento via `relations:expand`).
- **Criação**: toggle "Criar task no board" no `CalendarEventForm` (sugerido ligado para
  gravação/reunião/evento). Task nasce com title/data/cliente/responsável do evento +
  `uiMetadata.source='calendar'`. Requer config `tasks: { boardId, defaultColumnId }` no
  `CalendarConfig` (aba Integrações do drawer); sem config → aviso acionável inline.
- **Wiring no back**: expor o service de tasks por accessor exportado no módulo
  (`tasks.Module.Service()`) e injetar como provider lazy no calendar — a criação passa por
  `ResolveAccessContext` + `CreateTask` oficiais (ganha WS/audit/notificação de graça).
- `EventView` ganha `taskId` (LEFT JOIN em task_relations) → badge/link no drawer do dia
  (o `EventChip` NÃO muda — decisão anti-poluição visual). Excluir evento remove o vínculo
  (novo `RemoveRelation` + evento `task.relation_removed`); a task NÃO é arquivada junto.
- Sem sincronização bidirecional de status na v1 (futuro; documentado).

### 3.12 Realtime, presença e concorrência — WAVE 2

Edição colaborativa: outros usuários da conta veem mudanças do calendário ao vivo, com
presença estilo Google Docs. **Presença em TASKS já existe** (avatares, "Fulano editando X",
lock de campo, draft ao vivo — módulo `realtime`, canais tasks/presence); a wave ESTENDE o
mesmo padrão pro calendário.

- **Canal novo no módulo realtime existente** (hub genérico, padrão já exercitado 2×):
  tópico `calendar:account:{accountId}`, rota `GET /v1/realtime/calendar` (ticket efêmero
  `POST /v1/ws/ticket`), autorização por permissão `calendar.view` (espelho de
  `authorizeTasksAccount`). Eventos de INVALIDAÇÃO (padrão da casa, sem patch local):
  `calendar.event_created/updated/deleted`, `calendar.note_updated`,
  `calendar.day_media_updated`, `calendar.config_updated`, `calendar.plan_updated`.
- **Publisher** no módulo calendar (espelho de `tasks.Publisher`), injetado no boot.
- **Front**: `useCalendarRealtime` (molde `useTasksRealtime` sobre `useRealtimeSocket`, com a
  cadeia de conta v2 `accountStore.activeAccountId` — nunca `auth.activeTenantId` como fonte
  única) → refetch debounced da janela; nota só re-hidrata se NÃO houver edição pendente.
- **Concorrência (Postgres/Go nativos)**: optimistic locking — migration **0188** adiciona
  `version int` em `calendar.events`; `PUT /events/{id}` passa a aceitar `If-Match: version`
  (409 `version_conflict`, espelho de tasks) e o front resolve o conflito de forma acionável
  (mostra aviso + recarrega). Notas do mês: last-writer-wins + presença ("Fulano está
  editando as notas") — CRDT fora de escopo na v1.
- **Presença**: tópico `presence:calendar:{accountId}` reusando o `PresenceStore`
  (heartbeat/TTL prontos); front mostra avatares de quem está no calendário + badge de quem
  edita nota/evento (field_focus/blur).
- Premissa single-instance mantida (hub em memória); multi-réplica → broker externo (futuro
  documentado no AGENT.md do realtime).

## 4. Fases sugeridas
1. **✅ Back CRUD** de eventos + notas (feito: migration 0181, módulo Go, front integrado).
   Falta o gate de rota/nav próprio no front (hoje preview).
2. **✅ Feriados/datas comemorativas** (cálculo no back + toggles na config) + render no calendário
   + **responsáveis reais** + **modal de config** (migration 0182). Mock 100% removido.
3. **Anexos/mídia** (§3.6):
   - **3a ✅ back** — upload + `calendar.day_media` (0183) + `media-limits` global.
   - **3b ✅ front** — `CalendarMediaUploader` (progresso/preview/remover) no evento e no dia
     (falta validar no dev).
   - **3c ✅ mídia rica (código pronto; validar no dev)** — lightbox/player ao clicar
     (`CalendarMediaViewer`) + poster do vídeo no front (canvas → `posterUrl`) + **fundo do dia**
     com grid dividido (1/2/2×2) + overlay de legibilidade.
4. **✅ Perfil estratégico do cliente (§3.5; código pronto; validar no dev)** — migration 0185
   (`calendar.client_profiles`) + `GET/PUT client-profile` + índice lean + form na página de config.
5. **✅ Página de configuração completa (§3.8; código pronto; validar no dev)** — modal migrado →
   `/calendario/config` (modal deletado); cores por cliente/tipo aplicadas nos chips, início da
   semana no viewport, white-label + **config da IA** (provider/modelo/baseUrl/prompt/temperature)
   + limites de mídia (edição só platform_admin).
6. **✅ IA de Calendário + n8n "Calendar Omni" (§3.7; código pronto; PENDENTE import/teste no
   n8n)** — migration 0186 + `POST /ai/plan` (dispatch em goroutine) + callback público com
   service token + botão multi-cliente no front (polling, aplicar em notas/eventos) + workflow
   `automation/export/workflow-calendar-omni.json` (seletor Claude + chinesas OpenAI-compatible).
7. **⏳ Aprovação via WhatsApp** (n8n/WAHA) + **visão compartilhável** read-only para o cliente
   (respeitando white-label).

**WAVE 2 (planejada 2026-07-04 — specs atômicas em [CALENDARIO_SPECS2.md](CALENDARIO_SPECS2.md)):**

8. **⏳ Config em drawer lateral com abas (§3.9)** — `CalendarConfigDrawer` sobre
   `OmniEntityDrawer`, abas por categoria, 1 aba = 1 modelo de salvar, dirty-guard único,
   deep-link `?config=<aba>`, página `/calendario/config` vira redirect e é deletada.
9. **⏳ Chat de IA + voz (§3.10)** — FAB + painel de chat sobreposto; `POST /v1/calendar/chat/ask`
   → workflow n8n `calendar-chat` (memória Redis); provider **gemini** (free tier p/ teste) no
   seletor; mic → `POST /chat/transcribe` → mini-workflow `calendar-transcribe` (whisper-1);
   contexto compartilhado entre IAs via `GET /v1/runtime/calendar/context`.
10. **⏳ Integração Tasks (§3.11)** — toggle "criar task" no form de evento; vínculo via
    `tasks.task_relations` + RelationResolver; `EventView.taskId` (badge/link); config
    board/coluna destino na aba Integrações; `RemoveRelation` ao excluir evento.
11. **⏳ Realtime + presença + concorrência (§3.12)** — canal `GET /v1/realtime/calendar` +
    tópico `calendar:account:{id}` + publisher no módulo; `useCalendarRealtime` (refetch
    debounced); presença `presence:calendar:{id}` (avatares + "editando X"); optimistic
    locking em `calendar.events` (migration **0188**, `If-Match: version`, 409 acionável).

Ordem recomendada: **8 → 9** (o chat ancora no drawer) e **10 → 11** (o realtime propaga o
`taskId`); as duplas (8+9) e (10+11) podem correr em paralelo por lanes (arquivos disjuntos,
exceto `stores/calendar.ts` — ver "Ordem/Dependências" no SPECS2).

## 5. Notas de deploy
- Migrations aplicadas: **0181** (`calendar.events` + `calendar.notes`), **0182** (`calendar.config`
  jsonb por conta). Ordem: 0181 → 0182.
- Fase 3 (anexos): **0183** (`calendar.day_media`). Limite de vídeo mora em `core.platform_settings`
  (chave `media_limits`, sem migration — linha criada no 1º PUT; default 300MB no código).
- Feriados **não** têm tabela/seed (computados em `holidays.go`); nada a migrar por ano.
- **Upload de 300MB**: o proxy da frente (Caddy/nginx) precisa aceitar body grande
  (`client_max_body_size`/limite de request) e o volume de `UPLOADS_DIR` (`data/uploads`) precisa de
  espaço. Em dev (direto na :9091) o Go aceita. Arquivos servidos em `/uploads/...` (rota não gateada).
- **Fase 3c (mídia rica)**: poster do vídeo é capturado no **front** (canvas) e sobe pelo mesmo
  `POST /media` — **sem ffmpeg no servidor** e **sem migration** (`posterUrl` mora no jsonb do
  `MediaItem`; só ajustar o struct Go + tipo TS). Sem impacto de deploy além do que já existe.
- **Fase 4 (perfil do cliente)**: migration **0185** (`calendar.client_profiles`) — **✅ aplicada
  em dev 2026-07-02** (rebuild da api feito). Ordem: …0183 → 0184 (erp) → **0185** → **0186**.
- **Fase 5 (página de config)**: nova rota `/calendario/config` criada (preview,
  `workspaceId:''` igual à página principal); gate de rota/nav próprio segue pendente p/ o
  módulo inteiro.
- **Fase 6 (IA + n8n)** — implementada; para LIGAR o fluxo:
  1. Migration **0186** (`calendar.ai_plans`) — **✅ aplicada em dev 2026-07-02**.
  2. **Envs da api** (compose/env): `CALENDAR_AI_WEBHOOK_URL` (webhook do n8n),
     `CALENDAR_AI_SERVICE_TOKEN` (segredo do callback), `CALENDAR_AI_CALLBACK_BASE` (base pública
     da api p/ o n8n chamar de volta). Sem webhook configurado o `POST /ai/plan` responde 503
     `ai_not_configured` (aviso acionável no front).
  3. **n8n**: importar `automation/export/workflow-calendar-omni.json` + criar as credentials
     `calendar-ai-{claude,deepseek,qwen,kimi,glm}` (chaves de API ficam SÓ no n8n) — passo a
     passo em [automation/CALENDAR_OMNI_WORKFLOW.md](automation/CALENDAR_OMNI_WORKFLOW.md).
  4. Callback público: `POST /v1/public/calendar-ai/plans/{id}/result` (fora do gate de módulo,
     autenticado por `X-Service-Token` constant-time).
  Provider/modelo/prompt/temperature por conta em `calendar.config` (página de config).
- Rebuild da api ao mexer no back: `docker compose up -d --build api` (feito a cada fase de back).
- **WAVE 2 (fases 8-11)** — pendências de deploy quando implementar:
  1. Migration **0188** (`calendar.events.version` p/ optimistic locking) — próximo número
     livre confirmado em 2026-07-04 (0187 = finance).
  2. **Envs novos da api**: `CALENDAR_CHAT_WEBHOOK_URL` (webhook `calendar-chat`) e
     `CALENDAR_TRANSCRIBE_WEBHOOK_URL` (webhook `calendar-transcribe`). Sem env → endpoints
     respondem 503 acionável (mesmo padrão do `ai_not_configured`).
  3. **Backlog herdado**: os envs `CALENDAR_AI_*` da Fase 6 ainda NÃO estão em nenhum
     `.env*.example` — a wave 2 os adiciona junto com os novos (SPEC-B6 do SPECS2).
  4. **n8n**: importar `workflow-calendar-chat.json` + `workflow-calendar-transcribe.json`;
     credentials novas `calendar-ai-gemini` (Bearer, key do AI Studio) e OpenAI p/ whisper
     (reusa a credencial OpenAI existente do bot WhatsApp); Redis Chat Memory usa a
     credential "Redis account" existente.
  5. Realtime não muda infra (hub em memória, api single-instance); proxy da frente precisa
     permitir upgrade de WebSocket em `/v1/realtime/calendar` (mesma regra dos canais atuais).

## 6. Legado/mock — ✅ removido
- `useCalendarData.ts` (`CALENDAR_DATA_IS_MOCK`) **deletado**; responsáveis, feriados, config e
  membros vêm do back real. Notas persistem via API (`/v1/calendar/notes/{month}`).

## 7. Organização do código (skill de engenharia, < 450 linhas/arquivo)
- CSS: `assets/styles/calendar.css` virou **barrel** que importa
  `calendar/{shell,grid,notes-drawer,week-form}.css` (cada um < 450).
- Store: `stores/calendar.ts` (407) + `composables/useCalendarViewport.ts` (foco/scroll/rail/nav)
  + `domain/calendar/calendar-api.ts` (I/O). Back: módulo em arquivos < 450.
