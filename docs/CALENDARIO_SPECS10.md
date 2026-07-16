# Calendário — SPECS 10 (WAVE 15: inteligência de verdade no chat + painel = fonte da verdade)

Continuação de SPECS9 (WAVE 14). Branch `refactor/multitenant-complete`. Data: 2026-07-14.

## Visão (definida pelo dono, 2026-07-13)

O chat do calendário é uma **conversa com uma IA de marketing** (persona do painel), não um
formulário disfarçado. Ela precisa:

- Entender **typos e erros de transcrição de áudio** pelo contexto: "multi dia testi" →
  "Campanha multi-dia teste"; "tipo rios" → "reels" (não existe "rios" no mundo dela). Óbvio
  corrige e avisa; dúvida pergunta "você quis dizer X?".
- Ser **consultiva**: dicas, estratégias, ideias de campanhas/posts/reels/copys/carrosséis usando
  o PERFIL dos clientes do contexto.
- Continuar fazendo CRUD com confirmação no card — **o card É a confirmação**, nunca pedir
  "confirme se deseja..." por texto antes de propor.

A inteligência vem do **modelo + prompt** (gpt-4o + systemPrompt do painel + regras de domínio);
as guardas determinísticas do back são **rede de segurança**, não substituto.

## Princípio: PAINEL = FONTE DA VERDADE

Regra do dono: **toda configuração vem do painel/banco** (`calendar.config.ai`): apiKey, provider,
model, baseUrl, temperature, systemPrompt. O n8n é só a LIGAÇÃO dos nós (lógica de integração),
porque o workflow é exportado/importado via JSON — nada de chave/prompt/modelo hardcoded nele.

- O back manda a config no `body.ai` do webhook (`chatPayloadAI`); o nó "Montar contexto" usa
  `body.ai.*` sempre; `DEFAULT_MODEL`/`DEFAULT_BASE` são só fallback quando o painel está vazio.
- O systemPrompt do painel VENCE: o bloco de regras anexado pelo workflow é prefixado com "estas
  regras COMPLEMENTAM as instruções acima; em conflito de tom/persona, as instruções acima
  prevalecem".
- Auditoria automatizada no script de patch: apiKey/model/baseUrl/temperature/systemPrompt vêm de
  `body.ai`; nenhuma chave `sk-...` no jsCode. Passou 100%.
- Modelo da conta subiu para **gpt-4o** (decisão do dono; era gpt-4o-mini, que não sustentava o
  raciocínio de domínio). Trocado via config no banco de dev; em prod troca-se pela aba IA.

## O que foi feito

### n8n (`workflow-calendar-chat.json`, nó "Montar contexto") — patches via script Node

Regras novas no bloco sempre-anexado (persona do painel continua autoritativa):

- **INTELIGÊNCIA DE DOMÍNIO**: vive no mundo de marketing de conteúdo; palavra que não existe
  nesse mundo é provavelmente erro de transcrição/typo — corrige pelo termo mais próximo E AVISA
  ("rios" → "entendi reels"); ambíguo pergunta; NUNCA usa type fora da taxonomia.
- **TÍTULO COM ERRO**: compara título citado por SIMILARIDADE com `context.events/tasks`;
  1 candidato próximo → usa e segue (o card confirma); vários/distante → pergunta "você quis
  dizer {título}?"; nunca responde "não achei" sem tentar similaridade.
- **CONSULTORIA**: além do CRUD é estrategista; ideias/dicas/estratégias concretas usando o
  perfil do cliente do contexto; sugestão aceita pode virar proposta de criação.
- **MENSAGEM DE VOZ** (condicional `body.viaVoice === true`): "esta mensagem veio de transcrição
  de áudio: erros fonéticos são PROVÁVEIS — interprete pelo contexto antes de dizer que não
  entendeu/não achou".
- **O CARTÃO É A CONFIRMAÇÃO**: nunca pedir confirmação por texto antes de propor; intenção clara
  → proposta com o valor final já calculado. DATA RELATIVA ("adia 2 dias", "joga pra sexta"):
  calcula a partir da data ATUAL do item no contexto e propõe a data resultante. (Caso real do
  roteiro: "adia 2 dias" respondia "Confirme se deseja adiar para 27/07" sem card.)
- **UPDATE MÍNIMO**: proposta de update leva APENAS os campos pedidos (além de
  targetId/targetKind); reenviar campo que não mudou altera o item sem o usuário pedir. (Caso
  real: editar o título carregava priority junto e trocava alta→média.)

### Back — sinal de voz + redes de segurança (`chat.go`, `chat_target_guard.go`, `chat_proposals_crud.go`)

- **ViaVoice**: `ChatAskRequest.ViaVoice` → `chatWebhookPayload.ViaVoice` (`body.viaVoice` no nó).
- **Fuzzy de título como fallback** em `titleMatchedEvents`: sem hit exato, tier fuzzy por
  Levenshtein normalizada (`fuzzyTitleMatch`, janela deslizante de tokens k±1 sobre
  `foldChatLabel`; limiar 0.25, margem 0.10 sobre o 2º, título ≥ 6 runas). Conservador: 1
  candidato inequívoco ou nada. `levenshtein()` local, sem dependência.
- **Snap de type**: `snapProposalTypes` — type fora da taxonomia (`eventTypeSet`) vira o mais
  próximo com ≤ 2 edições ("rels"→"reels"); irrecuperável ("rios", distância 3 = fonético, é
  trabalho do MODELO) é LIMPO — tipo inventado nunca chega ao card.
- **Resolver cliente por nome**: `resolveClientsInProposals` — clientId lixo → tenta clientName →
  cliente citado na pergunta → senão LIMPA; id válido preenche o ClientName (label do card).
- **Update/delete SEM targetId não morre mais na sanitização** (`sanitizeContentProposal`):
  a guarda de alvo resolve o targetId pelo título/dia citados DEPOIS; quem descarta o que sobrar
  sem alvo é `dropTargetlessEditable` (pós-guarda), com aviso determinístico ("Não consegui
  identificar qual item..."). Caso real do roteiro: "o cliente da X na verdade é a Bari" veio sem
  targetId e a proposta era descartada ANTES da guarda — o answer mentia "preparei a proposta"
  com card nenhum.

### Front — sinal de voz (`useCalendarChat.ts`, `CalendarChatPanel.vue`)

- `draftFromVoice` (useState compartilhado): marcado no ditado ao vivo (watch em
  `live.transcript`) e no Whisper (`stopAndTranscribe`); limpo quando o usuário digita
  (`onInput` com `!isCapturing`). `ask()` envia `viaVoice` no body e zera o flag.

## Rodada Gemini free (2026-07-14, cota OpenAI esgotada) — 3 bugs de INTEGRAÇÃO achados

A cota da OpenAI esgotou no meio da validação; a pedido do dono a validação seguiu com
`gemini-2.5-flash` (chave global já cadastrada; só troca de config — painel = fonte da verdade).
A rodada expôs 3 bugs REAIS de integração (independentes de modelo — o Gemini só variou o shape
o bastante para revelá-los):

1. **Extrator do n8n descartava proposta que o back saberia resolver**: o nó "Extrair resposta"
   tinha `update/delete sem targetId => descarta` (a MESMA validação antiga removida do back) e
   `hasEditable` que ignorava clientId/clientName — "troca o cliente pra Bari" (só clientId+
   clientName) morria NO WORKFLOW e o back nem via. Fix: extrator passa tudo adiante; quem decide
   é o back (guarda resolve pelo título; `dropTargetlessEditable` descarta com aviso no fim).
   REGRA: validação de entrada mora numa camada só — a que tem contexto para resolver.
2. **Front roteava task espelhada pelo caminho de EVENTO (full-replace)**: a guarda reescreve o
   targetId para o id do EVENTO espelho; o apply achava o evento e ia pro full-replace de evento
   — `priority || base.priority || 'media'` gravava "media" no evento (o espelho não tem
   priority) e o mirror levava pra task ("prioridade mudou sozinha", ocorreu nas DUAS rodadas);
   delete apagava SÓ o espelho (task ficava no board). Fix: kind:'task' com evento espelho
   (`existing?.taskId`) roteia para `applyTaskTarget` (patch PARCIAL na task; o mirror sincroniza).
3. **dueDate como ISO completo quebrava o PATCH**: Gemini manda `"2026-07-27T16:30:00Z"` num
   campo que o fluxo esperava como data pura + time separado — o PATCH de evento falhava e o
   card ficava "pending" sem aviso claro. Fix: `splitProposalDate` normaliza ISO → {date, time}
   nos 3 caminhos (update task, create task, full-replace de evento).
4. **Anotação no mês errado — causa raiz é o `inferChatMonth`, não o modelo**: "reescreve a
   anotação do mês para: Planejamento agosto..." fazia `inferChatMonth` (chat.go) detectar
   "agosto" NA PERGUNTA e montar o CONTEXTO de 2026-08 — o modelo então gravava no mês errado
   "corretamente" (recebeu context.month=2026-08). Regra de prompt sozinha não resolve (o
   contexto inteiro aponta pro mês errado). Fix determinístico: `snapNoteMonths` (back) força
   `note.month` = **mês da TELA (`req.Month`)** — não o contextMonth — salvo mês-alvo explícito
   ("anota no mês de agosto: ..."). Regra registrada: heurística que retargeta o CONTEXTO pela
   pergunta não pode decidir o ALVO DE ESCRITA; escrita ancora na tela.

5. **Delete de task espelhada apagava só o espelho (2ª camada)**: além do roteamento por
   `existing?.taskId`, o payload da janela às vezes chega sem `taskId` no evento → fallback:
   kind:'task' com evento achado SEMPRE tenta o caminho de task, casando a task do board pelo
   TÍTULO do evento espelho (espelho é 1:1); só avisa se não existir task — nunca mexe só no
   espelho. Validado ao vivo: `task.deleted` no audit via card do chat.
6. **Status inventado derrubava o CREATE**: gpt-4o-mini emitiu `status: "Raw"` e o POST inteiro
   falhava (400). `snapProposalTypes` agora normaliza TAMBÉM o Status contra `eventStatusSet`
   (≤2 edições casa o válido; lixo é limpo e o back usa o default "planejado"). 34 testes Go.
7. **Modelos baratos pedem confirmação/campos opcionais em vez de propor** (mini pergunta
   "qual a descrição?" num create que já tem título+data): limitação de aderência do modelo —
   as regras "O CARTÃO É A CONFIRMAÇÃO"/"Seja PROATIVO" seguram o gpt-4o e o gemini-2.5-flash,
   o mini escorrega às vezes. Um follow-up curto resolve; com créditos, gpt-4o é o indicado.

## Armadilhas descobertas (registrar)

- **`npm run n8n:import` via Bash come as barras**: `powershell -File scripts\dev\n8n-import.ps1`
  dentro de um comando bash vira `scriptsdevn8n-import.ps1` (ENOENT silencioso no meio de um `&&`
  longo). Sempre importar via `npm run n8n:import:chat` no PowerShell e **verificar no SQLite do
  n8n** que o marcador novo chegou (`workflow_entity.nodes LIKE '%REGRA NOVA%'`) — copiar TAMBÉM o
  `database.sqlite-wal` senão a leitura vê a versão velha.
- **429 da OpenAI em turnos rápidos**: roteiro E2E com contexto grande estoura TPM; o workflow
  devolve `aiError` e o back NÃO persiste resposta (por design). No teste: pacing entre turnos +
  clicar o "Repetir" do estado "IA fora do ar".

## Validação — ROTEIRO CRUD COMPLETO pelo chat (Playwright + DB)

Login mikewade2k16@gmail.com. Cada passo aplica o card e confere `tasks.tasks` /
`calendar.events` / `calendar.notes` via psql.

**Status 2026-07-14**: rodada 1 com gpt-4o validou 10/13 passos até a cota da OpenAI esgotar
(429 persistente; diagnóstico diferencial com conversa nova + gpt-4o-mini). A pedido do dono a
validação seguiu com **gemini-2.5-flash free**: rodada 3 expôs os bugs de integração da seção
acima; rodada 4 (pós-fixes) validou **create completo + TODOS os edits** — título, data+hora
(16:30 local→19:30Z), UPDATE MÍNIMO (prioridade não vaza mais), data RELATIVA ("adia 2 dias" →
27/07 mantendo a hora), tipo "rios"→reels, prioridade, responsável "Mikee"→Mike Wade (coluna
UUID + label), cliente Pérola→Bari (0ffd62c2 no banco). Rodada 4b (focada) cobriu o resto:
descrição, typo no título alvo, anotações (com o fix do mês), delete board+calendário e
consultoria. Instabilidade do free tier (503/429 intermitente) é contornada no teste com
pacing + botão Repetir do estado "IA fora do ar". gpt-4o volta quando o dono recarregar créditos
(troca pela aba IA; zero código).

1. CREATE task completa (título, data+hora, tipo, prioridade, responsável POR NOME, cliente,
   descrição) — labels legíveis no banco.
2. EDIT campo a campo em turnos separados: título; data+hora; data RELATIVA ("adia 2 dias");
   tipo com erro de voz ("rios" → reels); prioridade; responsável com NOME ERRADO ("Mikee" →
   Mike Wade); cliente (Pérola → Bari); descrição. UPDATE MÍNIMO conferido (campos não pedidos
   não mudam).
3. Typo no título alvo ("campanha multi dia testi") → resolve/pergunta o título certo.
4. Anotações do mês: acrescentar / reescrever / limpar → `calendar.notes`.
5. DELETE citando título com pequeno erro → some do board e do calendário.
6. Consultoria ("3 ideias de reels para a Pérola usando o perfil dela") → estratégica, SEM
   propostas de CRUD.
7. Regressão: suite Go do módulo (31 testes) verde; eslint 0 erros.

## WAVE 16 — Ler perfil do cliente citado + ditado sem limite (2026-07-15)

Dois problemas que o dono levantou:

### 1. "A IA não lê os itens do perfil do cliente"

Causa (não é bug, é o desenho do contexto): no escopo **um cliente selecionado**, o back manda
`ctx.client.profile` COMPLETO — ela lê tudo. No escopo **GERAL**, cada cliente vai ENXUTO (nome,
segmento, um trecho do tom de voz e a lista de campos vazios) para não estourar o contexto com
muitos clientes. Aí "traz os dados do cliente X" respondia "não temos" — o dado existe no banco,
só não viajava.

Fix (decisão do dono: **hidratar o cliente citado sob demanda**): `appendNamedClientProfile`
(chat_targets.go) roda no `ChatAsk`; quando a pergunta nomeia UM cliente visível
(`singleNamedClient`, inequívoco), busca o perfil COMPLETO dele (`GetClientProfile`, scoped por
account) e o anexa como `AIContextAll.Client` (`*planClient`, json:"client"). O workflow **já**
renderiza "Cliente em foco" com todos os campos — **zero mudança no workflow**. Mantém leve no
geral, lê sob demanda. Validado: no geral, "traz os dados da Pérola" agora retorna
Posicionamento "Luxo" e a descrição (campos que só existem no perfil completo, não no enxuto).

Efeito colateral corrigido: `foldChatLabel` ficou tolerante a pontuação (`?`, `,`, `.`, `:`
grudados no nome viram espaço; `/` e `:` só sobrevivem ENTRE DÍGITOS, p/ data/hora "15/07"/"14:30")
— senão "da Perola:" não casava "Perola". Melhora TODO o casamento por nome/título do chat.

### 2. Gravação ao vivo "tem limite de tempo" + limite de texto

- **Tempo**: o ditado ao vivo usa a Web Speech API do navegador, que **encerra a sessão sozinha**
  (silêncio ou limite interno do Chrome, ~60s). O `onend` só marcava `idle` — por isso "cortava".
  Fix (`useLiveDictation.ts`): flag `wantListening` = intenção do usuário; enquanto true, o `onend`
  **reinicia a sessão** mantendo o texto já transcrito (`finalText` acumula entre sessões). Ditado
  contínuo, sem limite de tempo, até a pessoa parar. Erros fatais (mic negado) desligam; transitórios
  (no-speech/network) reiniciam.
- **Texto**: a pergunta era capada em 4000 runas no back (`maxChatQuestion`). Um briefing FALADO
  longo batia nisso. Subiu para **12000** (~2000 palavras).

## Notas de Deploy

- **Rebuild api** (`docker compose build --no-cache api` — armadilha do cache de embed) — guarda
  nova + ViaVoice + hidratação de perfil (WAVE 16) + `maxChatQuestion` 12000. SEM migration. SEM env.
- **Rebuild web** — ditado contínuo (WAVE 16). SEM env.
- **Reimportar `calendar-chat` no n8n** (`npm run n8n:import:chat`) e CONFERIR no banco do n8n
  que as regras novas chegaram (marcador `O CARTAO E A CONFIRMACAO`). WAVE 16 NÃO mexe no workflow.
- **Modelo**: contas de prod trocam para `gpt-4o` pela aba IA do painel (config no banco; nada
  de código).
