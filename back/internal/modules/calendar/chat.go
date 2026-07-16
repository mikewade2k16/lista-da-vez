package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// Contrato C7/C8: chat de IA do calendario (pergunta + transcricao de voz). O Go e
// so um proxy fino: monta o payload C7 (mesmo agregado de contexto do C9 SEM o
// campo account, via BuildAIContext) e repassa ao webhook do n8n. WAVE 4 (D4): o Go
// passa a PERSISTIR conversas/mensagens (calendar.chat_*, ver chat_store.go) e a IA le
// as ultimas N como memoria (history no payload) — a fonte da memoria e o banco, nao
// mais so o Redis do n8n. O escopo (client|all) e resolvido server-side pela permissao.

// Limites e timeouts do chat/transcricao.
const (
	// maxChatQuestion e o teto da pergunta (contrato C7): protege o payload e o
	// prompt da IA de textos absurdos. 12000 runas (~2000 palavras) cabe um briefing
	// FALADO longo (ditado continuo de perfil de cliente) sem estourar a janela do LLM.
	maxChatQuestion = 12000
	// chatAskTimeout e a janela do POST /chat/ask ao n8n. O AI Agent e sincrono
	// (LLM headless), entao a janela e larga; timeout vira 504 (DeadlineExceeded).
	// 120s (WAVE 12): resposta LONGA (ex.: listar tasks sem data com contexto de 100
	// tasks) + retries do n8n (4x2.5s) estouravam os 60s e viravam "IA fora do ar".
	chatAskTimeout = 120 * time.Second
	// chatStatusTimeout limita a verificacao leve do n8n feita ao abrir o chat e
	// antes de enviar. A rota /healthz nao chama modelo e nao consome tokens.
	chatStatusTimeout = 3 * time.Second
	// chatTranscribeTimeout e a janela do POST /chat/transcribe (Whisper demora
	// mais que uma resposta de texto).
	chatTranscribeTimeout = 120 * time.Second
	// chatMaxResponseBytes limita o corpo lido da resposta do n8n.
	chatMaxResponseBytes = 1 << 20 // 1 MiB
	// maxTranscribeBytes e o teto do audio (contrato C8): 15 MiB.
	maxTranscribeBytes = 15 << 20 // 15 MiB
	// chatLanguage e o idioma do payload do chat (produto pt-BR).
	chatLanguage = "pt-BR"
	// transcribeLanguage e o campo language repassado ao Whisper (codigo curto).
	transcribeLanguage = "pt"
	// chatHistoryLimit e o N da memoria (contrato D4): ultimas N mensagens JA existentes
	// da conversa viram history no payload. A pergunta atual vai no campo question (o n8n
	// concatena system+history+question, D5), entao ela NAO entra no history (sem duplicar).
	chatHistoryLimit = 12
	// Papeis das mensagens persistidas (calendar.chat_messages).
	chatRoleUser      = "user"
	chatRoleAssistant = "assistant"
	// maxChatTitleRunes limita o titulo derivado da 1a pergunta (contrato D4).
	maxChatTitleRunes = 80
	// maxChatProposals limita quantas propostas de criacao (evento/task) uma unica
	// resposta pode trazer (multi-tarefa, WAVE 5.1). Teto ~1 mes (uma por dia); pedidos
	// maiores a IA fatia e avisa. Protege o payload/persistencia e a UX de aprovacao.
	maxChatProposals = 31
)

// Sentinels do chat (mapeados em writeChatError, http_chat.go):
//   - ErrChatNotConfigured       -> 503 chat_not_configured (sem CALENDAR_CHAT_WEBHOOK_URL)
//   - ErrTranscribeNotConfigured -> 503 transcribe_not_configured (sem CALENDAR_TRANSCRIBE_WEBHOOK_URL)
//   - ErrInvalidQuestion         -> 400 invalid_question
//   - errChatUpstream            -> 502 upstream_error (rede/HTTP nao-2xx/JSON invalido)
//   - context.DeadlineExceeded   -> 504 upstream_timeout (sobe puro, nao embrulhado)
var (
	ErrChatNotConfigured       = errors.New("calendar: chat webhook nao configurado")
	ErrTranscribeNotConfigured = errors.New("calendar: transcribe webhook nao configurado")
	ErrInvalidQuestion         = errors.New("calendar: pergunta invalida")
	ErrInvalidProposalStatus   = errors.New("calendar: status de proposta invalido")
	errChatUpstream            = errors.New("calendar: chat upstream falhou")
)

var (
	chatISOYearMonthRe = regexp.MustCompile(`\b(20\d{2})-(0[1-9]|1[0-2])\b`)
	chatNumericDateRe  = regexp.MustCompile(`\b(?:0?[1-9]|[12]\d|3[01])[/-](0?[1-9]|1[0-2])(?:[/-](20\d{2}|\d{2}))?\b`)
	chatMonthNameRe    = regexp.MustCompile(`\b(janeiro|fevereiro|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)(?:\s+de)?\s*(20\d{2})?\b`)
	// chatDayNumberRe casa "dia 15" / "no dia 15" / "do dia 15" (numero do dia solto, ancorado no
	// mes em foco pela guarda de alvo). So depois de "dia" para nao pegar numeros quaisquer.
	chatDayNumberRe    = regexp.MustCompile(`\bdia\s+(\d{1,2})\b`)
	chatAnswerBulletRe = regexp.MustCompile(`^\s*(?:[-*]|\d+[.)])\s+`)
	chatAnswerDateRe   = regexp.MustCompile(`^\s*(?:[-*]\s*)?\d{1,2}[/-]\d{1,2}\b`)
	chatAnswerCountRe  = regexp.MustCompile(`(?i)\b\d+\s+eventos?\s+no\s+total\b`)
)

// chatConfig guarda os webhooks do chat/transcricao (envs lidos no Build) e o
// http.Client de saida. URLs vazias => os handlers respondem 503. O client NAO tem
// Timeout global: o deadline vem do context por chamada (ask 60s / transcribe 120s),
// para mapear timeout em 504 (context.DeadlineExceeded).
type chatConfig struct {
	askURL        string
	transcribeURL string
	client        *http.Client
}

// ChatAskRequest e o body do POST /v1/calendar/chat/ask (contratos C7/D4). question
// obrigatoria; conversationId identifica a conversa PERSISTIDA (vazio = cria nova); month
// e o mes em foco (opcional). Escopo (WAVE 4/D4): scopeMode 'client'|'all' + scopeClientId
// (o cliente do escopo). clientId e o campo LEGADO C7 (compat): usado como fallback do
// scopeClientId quando este vem vazio. O escopo e SEMPRE normalizado server-side pela
// permissao (validateScope) — o body nunca decide sozinho o cliente/modo.
type ChatAskRequest struct {
	Question       string `json:"question"`
	ConversationID string `json:"conversationId"`
	ClientID       string `json:"clientId"`
	Month          string `json:"month"`
	ScopeMode      string `json:"scopeMode"`
	ScopeClientID  string `json:"scopeClientId"`
	// ViaVoice (WAVE 15): a mensagem veio de TRANSCRICAO DE AUDIO (Whisper/ditado). O prompt
	// trata erros foneticos como provaveis ("rios" ~ "reels") antes de dizer que nao entendeu.
	ViaVoice bool `json:"viaVoice"`
}

// chatWebhookPayload e o corpo Go -> n8n (webhook calendar-chat, contratos C7/D4/D5). O
// bloco ai vem da config da account + a KEY CRUA (chatPayloadAI). context = agregado do
// contexto SEM account: calendarChatContext (scope 'client', via BuildAIContext) OU
// AIContextAll (scope 'all', multi-cliente) — por isso `any`. history = as ultimas N
// mensagens JA existentes da conversa (memoria; contrato D5: o n8n monta
// system+context, depois ...history, depois a question).
type chatWebhookPayload struct {
	Question   string               `json:"question"`
	SessionKey string               `json:"sessionKey"`
	Language   string               `json:"language"`
	AI         chatPayloadAI        `json:"ai"`
	Context    any                  `json:"context"`
	History    []chatHistoryMessage `json:"history"`
	// ViaVoice (WAVE 15): repassa o sinal de transcricao de audio ao prompt (body.viaVoice).
	ViaVoice bool `json:"viaVoice"`
}

// chatHistoryMessage e a projecao {role,content} de uma mensagem persistida para o
// payload do n8n (contrato D5). So role/content: a memoria do LLM nao precisa de id nem
// timestamp, e o payload fica enxuto (token-bounded). Mapeada de []ChatMessage por toHistory.
type chatHistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatPayloadAI e o bloco ai do payload C7 (SPEC-B2): TODA a config de IA da conta
// (AIConfig achatado) + a KEY CRUA (apiKey) resolvida server-side pelo resolver de B1.
// O n8n le body.ai.apiKey e a usa como Authorization Bearer (SPEC-W1) — sem $env. A
// key so existe server-side: NUNCA e logada (nao logar o payload cru) nem devolvida
// ao front (o front recebe so o status mascarado via /ai-keys).
type chatPayloadAI struct {
	AIConfig
	APIKey string `json:"apiKey"`
}

// calendarChatContext e o bloco context do C7: o agregado C9 SEM o campo account.
// MESMAS chaves/shapes do AIContext (regra de unificacao C7/C9) — so omite account.
// People (WAVE 12) = pessoas da equipe (id+nome, mesmos dados do GET /responsibles):
// a IA resolve "responsavel vai ser a Iasmin" por NOME sem exigir ID do usuario.
type calendarChatContext struct {
	Client     *planClient      `json:"client"`
	Month      string           `json:"month"`
	Holidays   []Holiday        `json:"holidays"`
	MonthNotes string           `json:"monthNotes"`
	Events     []AIContextEvent `json:"events"`
	Tasks      []AIContextTask  `json:"tasks,omitempty"`
	People     []Member         `json:"people,omitempty"`
	Plans      []AIContextPlan  `json:"plans"`
}

// chatAnswer e a resposta do webhook calendar-chat. Proposal (WAVE 5, E7) e opcional: quando
// o usuario pede para CRIAR evento/task, a IA devolve a proposta AQUI (nao cria nada); o front
// mostra um cartao de confirmacao e cria pela API autenticada do usuario.
type chatAnswer struct {
	Answer   string        `json:"answer"`
	Proposal *ChatProposal `json:"proposal"`
	// Proposals (WAVE 5.1, multi-tarefa) = lista de propostas de criacao. O workflow novo
	// devolve isto; Proposal (singular) fica so p/ retrocompat de workflow antigo. O
	// sanitizeProposalList unifica os dois (lista tem prioridade; singular vira lista de 1).
	Proposals []ChatProposal `json:"proposals"`
	// EventIDs contem somente IDs do context.events que a resposta utilizou. O backend
	// cruza a lista com o contexto autoritativo antes de persistir/exibir qualquer card.
	EventIDs []string `json:"eventIds"`
	// AIError (WAVE 5) = o n8n nao conseguiu falar com o LLM (503/cota/chave/vazio). O
	// answer traz a mensagem amigavel; o front mostra isso como estado "IA off" (visual
	// distinto) e o back NAO persiste como mensagem (nao suja a memoria da conversa).
	AIError bool `json:"aiError"`
}

// ChatProposal e a proposta de criacao devolvida pela IA (WAVE 5, E7). A IA NAO cria nada; e'
// passthrough validado (shape fechado) para o front confirmar. Kind = "event" | "task".
type ChatProposal struct {
	// Action (WAVE 5.1, preparado p/ CRUD futuro): create|update|delete. HOJE o front so
	// executa 'create'; update/delete ficam RESERVADOS (o schema ja carrega o campo para
	// nao exigir migration/quebra depois). Vazio/desconhecido => 'create'.
	Action string             `json:"action,omitempty"`
	Kind   string             `json:"kind"`
	Fields ChatProposalFields `json:"fields"`
}

// ChatProposalFields sao os campos sugeridos (uniao dos de evento e task; o front usa os que
// fazem sentido para o kind). Nada aqui e autoritativo: a criacao real valida no endpoint.
type ChatProposalFields struct {
	Title         string   `json:"title,omitempty"`
	Date          string   `json:"date,omitempty"`
	Time          string   `json:"time,omitempty"`
	Type          string   `json:"type,omitempty"`
	Status        string   `json:"status,omitempty"`
	Priority      string   `json:"priority,omitempty"`
	ResponsibleID string   `json:"responsibleId,omitempty"`
	InvolvedIDs   []string `json:"involvedIds,omitempty"`
	Description   string   `json:"description,omitempty"`
	ContentHTML   string   `json:"contentHtml,omitempty"`
	DueDate       string   `json:"dueDate,omitempty"`
	StartDate     string   `json:"startDate,omitempty"`
	DueEndDate    string   `json:"dueEndDate,omitempty"`
	ColumnID      string   `json:"columnId,omitempty"`
	ClientID      string   `json:"clientId,omitempty"`
	ClientName    string   `json:"clientName,omitempty"`
	Archived      *bool    `json:"archived,omitempty"`
	// TargetID (WAVE 5.1, preparado p/ CRUD): id do evento/task alvo de update/delete. Vazio
	// em create. Reservado — o front so usa em create hoje.
	TargetID string `json:"targetId,omitempty"`
	// Note (WAVE 7, kind=note): sub-objeto da proposta de anotacao do mes. Ver chat_proposals_crud.go.
	Note *ChatProposalNote `json:"note,omitempty"`
	// Profile (WAVE 7, kind=clientProfile): sub-objeto da proposta de perfil do cliente.
	Profile *ChatProposalProfile `json:"profile,omitempty"`
}

// sanitizeProposal descarta propostas malformadas e normaliza (action/kind/fields). Despacha a
// validacao por kind: event/task (WAVE 5.1) e note/clientProfile (WAVE 7, ver chat_proposals_crud.go).
// Proposta invalida => nil (a resposta vira texto normal).
func sanitizeProposal(p *ChatProposal) *ChatProposal {
	if p == nil {
		return nil
	}
	kind := canonicalProposalKind(p.Kind)
	if kind == "" {
		return nil
	}
	// Action (CRUD): create|update|delete; fora disso => create.
	action := strings.ToLower(strings.TrimSpace(p.Action))
	if action != "create" && action != "update" && action != "delete" {
		action = "create"
	}
	normalizeProposalFields(&p.Fields)
	ok := false
	switch kind {
	case "event", "task":
		ok = sanitizeContentProposal(action, p.Fields)
	case "note":
		ok = sanitizeNoteProposal(action, p.Fields.Note)
	case "clientProfile":
		ok = sanitizeProfileProposal(action, p.Fields.Profile)
	}
	if !ok {
		return nil
	}
	p.Action = action
	p.Kind = kind
	return p
}

func normalizeProposalFields(f *ChatProposalFields) {
	if f == nil {
		return
	}
	f.Title = strings.TrimSpace(f.Title)
	f.Date = strings.TrimSpace(f.Date)
	f.Time = strings.TrimSpace(f.Time)
	f.Type = strings.TrimSpace(f.Type)
	f.Status = strings.TrimSpace(f.Status)
	f.Priority = strings.TrimSpace(f.Priority)
	f.ResponsibleID = strings.TrimSpace(f.ResponsibleID)
	f.Description = strings.TrimSpace(f.Description)
	f.ContentHTML = strings.TrimSpace(f.ContentHTML)
	f.DueDate = strings.TrimSpace(f.DueDate)
	f.StartDate = strings.TrimSpace(f.StartDate)
	f.DueEndDate = strings.TrimSpace(f.DueEndDate)
	f.ColumnID = strings.TrimSpace(f.ColumnID)
	f.ClientID = strings.TrimSpace(f.ClientID)
	f.ClientName = strings.TrimSpace(f.ClientName)
	f.TargetID = strings.TrimSpace(f.TargetID)
	// WAVE 7: normaliza os sub-objetos de anotacao/perfil (chat_proposals_crud.go).
	normalizeNoteField(f.Note)
	normalizeProfileField(f.Profile)
	if len(f.InvolvedIDs) == 0 {
		return
	}
	out := make([]string, 0, len(f.InvolvedIDs))
	seen := map[string]bool{}
	for _, raw := range f.InvolvedIDs {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	f.InvolvedIDs = out
}

func proposalHasEditableField(f ChatProposalFields) bool {
	return strings.TrimSpace(f.Title) != "" ||
		strings.TrimSpace(f.Date) != "" ||
		strings.TrimSpace(f.Time) != "" ||
		strings.TrimSpace(f.Type) != "" ||
		strings.TrimSpace(f.Status) != "" ||
		strings.TrimSpace(f.Priority) != "" ||
		strings.TrimSpace(f.ResponsibleID) != "" ||
		strings.TrimSpace(f.Description) != "" ||
		strings.TrimSpace(f.ContentHTML) != "" ||
		strings.TrimSpace(f.DueDate) != "" ||
		strings.TrimSpace(f.StartDate) != "" ||
		strings.TrimSpace(f.DueEndDate) != "" ||
		strings.TrimSpace(f.ColumnID) != "" ||
		strings.TrimSpace(f.ClientID) != "" ||
		strings.TrimSpace(f.ClientName) != "" ||
		len(f.InvolvedIDs) > 0 ||
		f.Archived != nil
}

// StoredProposal e uma proposta PERSISTIDA na mensagem (multi-tarefa, WAVE 5.1): a
// proposta + um id estavel (indice dentro da mensagem) + o status proprio. O front
// aprova/recusa cada uma pelo id; o status so muda por acao explicita do usuario.
type StoredProposal struct {
	ID     string             `json:"id"`
	Action string             `json:"action"`
	Kind   string             `json:"kind"`
	Fields ChatProposalFields `json:"fields"`
	Status string             `json:"status"`
}

// sanitizeProposalList unifica a saida do n8n (lista `proposals` do workflow novo OU o
// `proposal` singular do antigo), descarta malformadas (via sanitizeProposal) e aplica o
// teto maxChatProposals. Lista vazia = nenhuma proposta (resposta de texto normal).
func sanitizeProposalList(single *ChatProposal, list []ChatProposal) []ChatProposal {
	src := list
	if len(src) == 0 && single != nil {
		src = []ChatProposal{*single}
	}
	out := make([]ChatProposal, 0, len(src))
	seen := map[string]bool{}
	for i := range src {
		p := src[i]
		if clean := sanitizeProposal(&p); clean != nil {
			key := proposalDedupKey(*clean)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, *clean)
			if len(out) >= maxChatProposals {
				break
			}
		}
	}
	return out
}

func proposalDedupKey(p ChatProposal) string {
	raw, _ := json.Marshal(p.Fields)
	return p.Action + "|" + p.Kind + "|" + string(raw)
}

// storedProposalsFrom projeta as propostas sanitizadas em StoredProposal para persistir:
// id = indice (estavel e unico DENTRO da mensagem, suficiente para o update por proposta)
// e status inicial 'pending'.
func storedProposalsFrom(list []ChatProposal) []StoredProposal {
	out := make([]StoredProposal, 0, len(list))
	for i, p := range list {
		action := p.Action
		if action == "" {
			action = "create"
		}
		out = append(out, StoredProposal{
			ID: strconv.Itoa(i), Action: action, Kind: p.Kind, Fields: p.Fields, Status: "pending",
		})
	}
	return out
}

// transcribeText e a resposta do webhook calendar-transcribe.
type transcribeText struct {
	Text string `json:"text"`
}

// WithChat injeta os webhooks do chat/transcricao (envs lidos no Build do modulo).
// Devolve o proprio Service para encadear com WithAI.
func (s *Service) WithChat(askURL, transcribeURL string) *Service {
	s.chat = chatConfig{
		askURL:        strings.TrimSpace(askURL),
		transcribeURL: strings.TrimSpace(transcribeURL),
		client:        &http.Client{},
	}
	return s
}

// chatConfigured informa se o webhook do chat esta configurado.
func (s *Service) chatConfigured() bool { return strings.TrimSpace(s.chat.askURL) != "" }

// transcribeConfigured informa se o webhook da transcricao esta configurado.
func (s *Service) transcribeConfigured() bool { return strings.TrimSpace(s.chat.transcribeURL) != "" }

// chatClient devolve o http.Client de saida (fallback defensivo se WithChat nao
// rodou; o context ainda governa o timeout).
func (s *Service) chatClient() *http.Client {
	if s.chat.client == nil {
		return http.DefaultClient
	}
	return s.chat.client
}

// ChatStatus valida tudo que pode ser conferido sem executar prompt: webhook configurado,
// escopo permitido, kill switch, chave do provider efetivo e saude do n8n. Nenhuma conversa
// ou mensagem e criada e nenhum modelo e chamado, portanto a checagem nao consome tokens.
func (s *Service) ChatStatus(ctx context.Context, accountID string, principal auth.Principal, scopeMode, scopeClientID string) error {
	if !s.chatConfigured() {
		return ErrChatNotConfigured
	}
	account := strings.TrimSpace(accountID)
	access, err := s.resolveChatAccess(ctx, principal, account)
	if err != nil {
		return err
	}
	mode, clientID, err := access.validateScope(scopeMode, scopeClientID)
	if err != nil {
		return err
	}
	if mode != chatScopeClient {
		clientID = ""
	}
	effAI, err := s.EffectiveAIConfig(ctx, account, clientID)
	if err != nil {
		return err
	}
	if _, err := s.resolveDispatchKey(ctx, account, effAI.Enabled, effAI.Provider); err != nil {
		return err
	}
	return s.pingChatUpstream(ctx)
}

// chatHealthEndpoint aponta o webhook para o /healthz da mesma instancia n8n.
func chatHealthEndpoint(askURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(askURL))
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("%w: url do webhook invalida", errChatUpstream)
	}
	// Preserva um eventual subpath do n8n (ex.: /n8n/webhook/... -> /n8n/healthz).
	prefix := ""
	if idx := strings.Index(u.Path, "/webhook/"); idx >= 0 {
		prefix = strings.TrimRight(u.Path[:idx], "/")
	}
	u.Path = prefix + "/healthz"
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func (s *Service) pingChatUpstream(ctx context.Context) error {
	healthURL, err := chatHealthEndpoint(s.chat.askURL)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, chatStatusTimeout)
	defer cancel()
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(250 * time.Millisecond):
			case <-callCtx.Done():
				return callCtx.Err()
			}
		}
		lastErr = s.pingChatHealthOnce(callCtx, healthURL)
		if lastErr == nil {
			return nil
		}
		if callCtx.Err() != nil {
			return callCtx.Err()
		}
	}
	return lastErr
}

func (s *Service) pingChatHealthOnce(ctx context.Context, healthURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", errChatUpstream, err)
	}
	resp, err := s.chatClient().Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return context.DeadlineExceeded
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return context.Canceled
		}
		return fmt.Errorf("%w: %v", errChatUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: health http %d", errChatUpstream, resp.StatusCode)
	}
	return nil
}

// ChatAsk faz proxy ao webhook calendar-chat COM memoria e escopo persistidos (contrato
// D4). accountID/principal vem SEMPRE do middleware (nunca do body). Fluxo:
//  1. resolveChatAccess: acesso + clientes visiveis, 100% server-side.
//  2. resolveChatTarget: escopo normalizado (validateScope) e a conversa alvo — existente
//     (valida dono-ou-agencia + revalida o escopo salvo) ou nova (ainda NAO materializada).
//  3. AI EFETIVA + KEY CRUA ANTES de materializar/gravar: kill switch (ErrAIDisabled) e
//     key vazia (ErrAIKeyMissing) barram aqui, sem criar conversa orfa nem gravar nada.
//  4. Materializa a conversa nova (grava o escopo) so agora.
//  5. history = ultimas N JA existentes (ANTES de gravar a pergunta atual, p/ nao duplicar
//     no payload); grava a pergunta (role=user) — persistida ANTES da chamada ao n8n.
//  6. Contexto: 'client' -> BuildAIContext; 'all' -> BuildAIContextAll (multi-cliente).
//  7. Proxy ao n8n; grava a resposta (role=assistant); titula pela 1a pergunta (se vazio)
//     + updated_at. Responde {answer, conversationId, title}. O payload NUNCA vai para log
//     (contem ai.apiKey).
func (s *Service) ChatAsk(ctx context.Context, accountID string, principal auth.Principal, req ChatAskRequest) (ChatAskResult, error) {
	if !s.chatConfigured() {
		return ChatAskResult{}, ErrChatNotConfigured
	}
	question := strings.TrimSpace(req.Question)
	if question == "" || len([]rune(question)) > maxChatQuestion {
		return ChatAskResult{}, ErrInvalidQuestion
	}
	account := strings.TrimSpace(accountID)
	access, err := s.resolveChatAccess(ctx, principal, account)
	if err != nil {
		return ChatAskResult{}, err
	}
	target, err := s.resolveChatTarget(ctx, access, account, principal.UserID, req)
	if err != nil {
		return ChatAskResult{}, err
	}
	// Bloco ai EFETIVO + KEY CRUA (server-side) ANTES de materializar/gravar: em 'client'
	// considera override/kill por cliente (WAVE 3.1); em 'all' usa a config base da conta
	// (sem cliente unico). Barrar aqui evita conversa orfa quando a IA esta desligada.
	aiClientID := ""
	if target.mode == chatScopeClient {
		aiClientID = target.clientID
	}
	effAI, err := s.EffectiveAIConfig(ctx, account, aiClientID)
	if err != nil {
		return ChatAskResult{}, err
	}
	apiKey, err := s.resolveDispatchKey(ctx, account, effAI.Enabled, effAI.Provider)
	if err != nil {
		return ChatAskResult{}, err
	}
	conv := target.conv
	if !target.existing {
		conv, err = s.store.CreateConversation(ctx, account, principal.UserID, ChatConversationInput{
			ScopeMode:     target.mode,
			ScopeClientID: target.clientID,
		})
		if err != nil {
			return ChatAskResult{}, err
		}
	}
	// history = ultimas N JA existentes (ANTES de gravar a pergunta atual): a pergunta vai
	// no campo question, nao no history (o n8n concatena system+history+question, D5).
	prior, err := s.store.ListLastMessages(ctx, account, conv.ID, chatHistoryLimit)
	if err != nil {
		return ChatAskResult{}, err
	}
	if _, err := s.store.AppendMessage(ctx, account, conv.ID, ChatMessageInput{Role: chatRoleUser, Content: question}); err != nil {
		return ChatAskResult{}, err
	}
	contextMonth := inferChatMonth(question, req.Month, time.Now())
	contextBlock, err := s.buildChatContext(ctx, account, principal, access, target.mode, target.clientID, contextMonth)
	if err != nil {
		return ChatAskResult{}, err
	}
	// WAVE 14: mes alvo primeiro; titulo citado que NAO esta no mes em foco (tela em outro
	// mes/ano) e buscado em janela ampla e anexado ao contexto ANTES do LLM.
	contextBlock = s.appendWideTitleMatches(ctx, account, access, target.mode, target.clientID, contextMonth, question, contextBlock)
	// WAVE 16: no escopo 'all' os clientes vao enxutos; se a pergunta cita UM cliente, hidrata o
	// perfil COMPLETO dele (ctx.client) para a IA conseguir LER os dados do cliente citado.
	contextBlock = s.appendNamedClientProfile(ctx, account, question, contextBlock)
	payload := chatWebhookPayload{
		Question:   question,
		SessionKey: calendarChatSessionKey(account, principal.UserID, conv.ID),
		Language:   chatLanguage,
		AI:         chatPayloadAI{AIConfig: effAI, APIKey: apiKey},
		Context:    contextBlock,
		History:    toHistory(prior),
		ViaVoice:   req.ViaVoice,
	}
	answer, proposals, eventIDs, aiError, err := s.postChatAsk(ctx, payload)
	if err != nil {
		return ChatAskResult{}, err
	}
	title := conv.Title
	if strings.TrimSpace(title) == "" {
		title = deriveChatTitle(question)
	}
	// Falha da IA (aiError) NAO vira mensagem persistida: o front mostra o estado "IA off"
	// (visual distinto) e a memoria da conversa nao guarda "nao consegui falar com a IA".
	// A conversa ainda e titulada/bumpada (a pergunta do usuario ja foi gravada).
	var assistant ChatMessage
	if !aiError {
		items := selectContextEvents(contextEvents(contextBlock), eventIDs)
		answer = compactCalendarCardAnswer(answer, len(items))
		evs := contextEvents(contextBlock)
		tks := contextTasks(contextBlock)
		// WAVE 14: GUARDA DE ALVO (roda ANTES do resolve, pois pode REESCREVER o targetId). Quando o
		// usuario cita um dia/cliente, o BACK resolve o alvo (prioriza o calendario; 1 evento no dia =>
		// forca o alvo p/ ele; varios => barra e lista; so-em-tasks => avisa). resolvedTitle destaca o
		// alvo corrigido para o dono confirmar ("Vou alterar: X").
		// WAVE 14: RESOLVE responsavel/envolvidos e CLIENTE por NOME (o modelo manda ids lixo,
		// inventa ou esquece campos). Roda antes da guarda de alvo — nada depende do modelo.
		resolvePeopleInProposals(proposals, question, contextPeople(contextBlock))
		ctxClients := contextClients(contextBlock)
		resolveClientsInProposals(proposals, question, ctxClients)
		// WAVE 15: tipo fora da taxonomia (erro de voz "rios") vira o mais proximo ("reels");
		// irrecuperavel e limpo — rede de seguranca (o prompt ja manda o modelo corrigir e avisar).
		snapProposalTypes(proposals)
		// WAVE 15: a anotacao e SEMPRE do mes em foco DA TELA (req.Month) — NAO do
		// contextMonth: inferChatMonth muda o contexto quando a pergunta cita um mes
		// ("reescreve para: Planejamento agosto" => contexto de 2026-08), e ai o modelo
		// gravava a nota no mes errado "corretamente". Mes-alvo explicito e respeitado.
		noteMonth := strings.TrimSpace(req.Month)
		if !monthRe.MatchString(noteMonth) {
			noteMonth = contextMonth
		}
		snapNoteMonths(proposals, noteMonth, question)
		crit := extractTargetCriteria(question, contextMonth, ctxClients)
		kept, notice, resolvedTitle := guardProposalTargets(question, proposals, crit, ctxClients, evs, tks)
		proposals = kept
		// WAVE 15: update/delete que sobrou SEM targetId (modelo nao mandou e a guarda nao
		// resolveu pelo titulo/dia) sai aqui — um PATCH sem alvo nao aplica. Aviso deterministico
		// no lugar do answer do modelo (que costuma mentir "preparei a proposta").
		var droppedTargetless bool
		proposals, droppedTargetless = dropTargetlessEditable(proposals)
		if droppedTargetless && notice == "" {
			notice = "Nao consegui identificar qual item voce quer alterar. Me diga o titulo (ou o dia) do item do calendario."
		}
		if notice != "" {
			// Barrado (varios/so-tasks/sem-match): o aviso da guarda SUBSTITUI o texto da IA (que
			// pode estar errado/confuso). O card com a lista de escolha basta.
			answer = notice
		} else if resolvedTitle != "" {
			// Resolvido (1 alvo): frase curta e determinista SUBSTITUI o texto da IA (evita a
			// duplicacao "Vou alterar X" + "encontrei a tarefa X, vou atualizar").
			answer = "Vou alterar: " + resolvedTitle + ". Revise e confirme no cartao."
		}
		// WAVE 12: resolve os alvos (targetId ja corrigido pela guarda) e anexa o snapshot de cada
		// alvo aos calendarItems, para o card SEMPRE mostrar o titulo/"antes" do item que sera alterado.
		targets := resolveProposalTargets(proposals, evs, tks)
		items = mergeCalendarItems(items, targets)
		assistant, err = s.store.AppendMessage(ctx, account, conv.ID, ChatMessageInput{
			Role: chatRoleAssistant, Content: answer,
			Proposals: storedProposalsFrom(proposals), CalendarItems: items,
		})
		if err != nil {
			return ChatAskResult{}, err
		}
	}
	if err := s.store.TouchConversation(ctx, account, conv.ID, title); err != nil {
		return ChatAskResult{}, err
	}
	return ChatAskResult{Answer: answer, ConversationID: conv.ID, Title: title,
		Message: messageViewFrom(assistant), AIError: aiError}, nil
}

// inferChatMonth reconhece formatos comuns em pt-BR para que uma pergunta sobre outro mês
// consulte o período citado, não apenas o mês que estava aberto na tela. Sem mês explícito,
// preserva o fallback enviado pelo calendário.
func inferChatMonth(question, fallback string, now time.Time) string {
	normalized := strings.ToLower(question)
	normalized = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e",
		"ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c").Replace(normalized)
	if match := chatISOYearMonthRe.FindStringSubmatch(normalized); len(match) == 3 {
		return match[1] + "-" + match[2]
	}
	baseYear := now.Year()
	if monthRe.MatchString(strings.TrimSpace(fallback)) {
		if year, err := strconv.Atoi(fallback[:4]); err == nil {
			baseYear = year
		}
	}
	if match := chatNumericDateRe.FindStringSubmatch(normalized); len(match) == 3 {
		year := baseYear
		if match[2] != "" {
			parsed, _ := strconv.Atoi(match[2])
			if parsed < 100 {
				parsed += 2000
			}
			year = parsed
		}
		month, _ := strconv.Atoi(match[1])
		return fmt.Sprintf("%04d-%02d", year, month)
	}
	if match := chatMonthNameRe.FindStringSubmatch(normalized); len(match) == 3 {
		months := map[string]int{"janeiro": 1, "fevereiro": 2, "marco": 3, "abril": 4,
			"maio": 5, "junho": 6, "julho": 7, "agosto": 8, "setembro": 9,
			"outubro": 10, "novembro": 11, "dezembro": 12}
		year := baseYear
		if match[2] != "" {
			year, _ = strconv.Atoi(match[2])
		}
		return fmt.Sprintf("%04d-%02d", year, months[match[1]])
	}
	return strings.TrimSpace(fallback)
}

func contextEvents(block any) []AIContextEvent {
	switch value := block.(type) {
	case calendarChatContext:
		return value.Events
	case AIContextAll:
		return value.Events
	default:
		return nil
	}
}

// selectContextEvents descarta IDs inventados pelo modelo, remove duplicados e preserva
// a ordem pedida pelo LLM. Os dados retornados sao sempre o snapshot real do backend.
func selectContextEvents(events []AIContextEvent, ids []string) []AIContextEvent {
	byID := make(map[string]AIContextEvent, len(events))
	for _, event := range events {
		byID[event.ID] = event
	}
	out := make([]AIContextEvent, 0, len(ids))
	seen := map[string]bool{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if event, ok := byID[id]; ok && !seen[id] {
			seen[id] = true
			out = append(out, event)
		}
	}
	return out
}

func compactCalendarCardAnswer(answer string, itemCount int) string {
	trimmed := strings.TrimSpace(answer)
	if itemCount <= 0 || trimmed == "" {
		return trimmed
	}
	lines := strings.Split(trimmed, "\n")
	listLines := 0
	kept := make([]string, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if isCalendarAnswerListLine(line) {
			listLines++
			continue
		}
		if chatAnswerCountRe.MatchString(line) {
			continue
		}
		kept = append(kept, line)
	}
	threshold := itemCount
	if threshold > 4 {
		threshold = 4
	}
	if listLines < threshold {
		return trimmed
	}
	noun := "itens"
	if itemCount == 1 {
		noun = "item"
	}
	parts := []string{fmt.Sprintf("Encontrei %d %s no calendario.", itemCount, noun)}
	parts = append(parts, kept...)
	if !strings.Contains(strings.ToLower(strings.Join(kept, " ")), "card") {
		parts = append(parts, "A lista completa esta nos cards abaixo.")
	}
	return strings.Join(parts, " ")
}

func isCalendarAnswerListLine(line string) bool {
	clean := strings.TrimSpace(line)
	return strings.HasPrefix(clean, "\u2022") ||
		chatAnswerBulletRe.MatchString(clean) ||
		chatAnswerDateRe.MatchString(clean)
}

// toHistory projeta as mensagens persistidas em {role,content} para o payload do n8n
// (contrato D5), preservando a ordem cronologica devolvida pelo store.
func toHistory(msgs []ChatMessage) []chatHistoryMessage {
	out := make([]chatHistoryMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, chatHistoryMessage{Role: m.Role, Content: m.Content})
	}
	return out
}

// chatContextFrom converte o agregado C9 (AIContext) no bloco context do C7,
// apenas OMITINDO o campo account (regra de unificacao C7/C9). Nao ha remontagem:
// os campos vem prontos do BuildAIContext.
func chatContextFrom(c AIContext) calendarChatContext {
	return calendarChatContext{
		Client:     c.Client,
		Month:      c.Month,
		Holidays:   c.Holidays,
		MonthNotes: c.MonthNotes,
		Events:     c.Events,
		Plans:      c.Plans,
	}
}

// calendarChatSessionKey monta a chave de memoria da conversa para o n8n (espelho
// de omniChatSessionKey do automation). Escopa por account + user (isola memoria
// entre operadores) + conversationId do front. Sem conversationId, cai numa chave
// estavel por usuario.
func calendarChatSessionKey(accountID, userID, conversationID string) string {
	base := accountID + "|" + userID
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return base
	}
	return base + "|" + conversationID
}

// postChatAsk envia o payload C7 ao webhook calendar-chat e devolve o answer. Erros:
// errChatUpstream (rede/HTTP nao-2xx/JSON invalido), context.DeadlineExceeded
// (timeout, puro para o handler mapear em 504).
func (s *Service) postChatAsk(ctx context.Context, payload chatWebhookPayload) (string, []ChatProposal, []string, bool, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, nil, false, fmt.Errorf("%w: %v", errChatUpstream, err)
	}
	callCtx, cancel := context.WithTimeout(ctx, chatAskTimeout)
	defer cancel()
	raw, status, err := s.doChatRequest(callCtx, s.chat.askURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", nil, nil, false, err
	}
	if status < 200 || status >= 300 {
		return "", nil, nil, false, fmt.Errorf("%w: http %d", errChatUpstream, status)
	}
	var out chatAnswer
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", nil, nil, false, fmt.Errorf("%w: resposta invalida: %v", errChatUpstream, err)
	}
	// aiError = o LLM falhou (n8n devolveu o aviso amigavel); sem proposta nesse caso.
	if out.AIError {
		return out.Answer, nil, nil, true, nil
	}
	return out.Answer, sanitizeProposalList(out.Proposal, out.Proposals), out.EventIDs, false, nil
}

// ChatTranscribe repassa o audio ao webhook calendar-transcribe (multipart file +
// provider/apiKey/model/language) e devolve o texto. O provider/modelo vem da config
// v4 (transcribeProvider/Model) e a KEY CRUA do resolver (openai usa a key "openai",
// gemini usa "gemini"). Kill switch e sem-key idem chat/plano. O conteudo ja vem em
// memoria (lido do request, limitado a 15 MiB); NADA e gravado em disco.
func (s *Service) ChatTranscribe(ctx context.Context, accountID, fileName, contentType string, content []byte) (string, error) {
	if !s.transcribeConfigured() {
		return "", ErrTranscribeNotConfigured
	}
	account := strings.TrimSpace(accountID)
	cfg, err := s.store.GetConfig(ctx, account)
	if err != nil {
		return "", err
	}
	// Provider de transcricao (CFG v4): openai|gemini. Kill switch + KEY CRUA do
	// provider (server-side); enabled=false => ErrAIDisabled, key vazia => ErrAIKeyMissing.
	// A transcricao usa o config GERAL da conta (sem override por-cliente — nao ha cliente
	// no contexto do audio).
	provider := cfg.AI.TranscribeProvider
	var apiKey string
	if provider == "local" {
		// Whisper self-hosted (http://whisper:8000): NAO precisa de key; so o kill switch
		// se aplica (o n8n usa a URL interna do container, sem Authorization).
		if !cfg.AI.Enabled {
			return "", ErrAIDisabled
		}
	} else {
		apiKey, err = s.resolveDispatchKey(ctx, account, cfg.AI.Enabled, provider)
		if err != nil {
			return "", err
		}
	}
	body, boundary, err := buildTranscribeBody(provider, apiKey, cfg.AI.TranscribeModel, fileName, contentType, content)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errChatUpstream, err)
	}
	callCtx, cancel := context.WithTimeout(ctx, chatTranscribeTimeout)
	defer cancel()
	raw, status, err := s.doChatRequest(callCtx, s.chat.transcribeURL, boundary, body)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("%w: http %d", errChatUpstream, status)
	}
	var out transcribeText
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("%w: resposta invalida: %v", errChatUpstream, err)
	}
	return out.Text, nil
}

// buildTranscribeBody monta o corpo multipart (campos provider/apiKey/model/language
// + o file com o audio) e devolve o buffer + o Content-Type (com boundary). Os campos
// sao lidos pelo n8n (SPEC-W2): provider roteia o Switch (openai|gemini), apiKey vira
// Authorization Bearer, model escolhe o modelo (vazio = default do provider), language
// segue para o Whisper (ignorado no ramo gemini). Tudo em memoria; a key NAO e logada.
func buildTranscribeBody(provider, apiKey, model, fileName, contentType string, content []byte) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fields := [][2]string{
		{"provider", provider},
		{"apiKey", apiKey},
		{"model", model},
		{"language", transcribeLanguage},
	}
	for _, f := range fields {
		if err := mw.WriteField(f[0], f[1]); err != nil {
			return nil, "", err
		}
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, sanitizeAudioName(fileName)))
	if strings.TrimSpace(contentType) != "" {
		header.Set("Content-Type", contentType)
	}
	part, err := mw.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(content); err != nil {
		return nil, "", err
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return &buf, mw.FormDataContentType(), nil
}

// sanitizeAudioName evita quebrar o header Content-Disposition com aspas/quebras de
// linha vindas do nome original; nome vazio vira um default.
func sanitizeAudioName(name string) string {
	name = strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(strings.TrimSpace(name))
	if name == "" {
		return "audio"
	}
	return name
}

// doChatRequest executa o POST ao webhook e devolve corpo (limitado) + status.
// Timeout/cancelamento sobe puro (context.DeadlineExceeded/Canceled) para o handler
// mapear em 504; demais falhas de transporte viram errChatUpstream.
func (s *Service) doChatRequest(ctx context.Context, url, contentType string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", errChatUpstream, err)
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := s.chatClient().Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, 0, err
		}
		return nil, 0, fmt.Errorf("%w: %v", errChatUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, chatMaxResponseBytes))
	if err != nil {
		// Deadline/cancelamento durante a leitura tambem sobe puro (504).
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, 0, err
		}
		return nil, 0, fmt.Errorf("%w: %v", errChatUpstream, err)
	}
	return raw, resp.StatusCode, nil
}
