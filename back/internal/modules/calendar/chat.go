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
	// prompt da IA de textos absurdos.
	maxChatQuestion = 4000
	// chatAskTimeout e a janela do POST /chat/ask ao n8n. O AI Agent e sincrono
	// (LLM headless), entao a janela e larga; timeout vira 504 (DeadlineExceeded).
	chatAskTimeout = 60 * time.Second
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
	errChatUpstream            = errors.New("calendar: chat upstream falhou")
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
type calendarChatContext struct {
	Client     *planClient      `json:"client"`
	Month      string           `json:"month"`
	Holidays   []Holiday        `json:"holidays"`
	MonthNotes string           `json:"monthNotes"`
	Events     []AIContextEvent `json:"events"`
	Plans      []AIContextPlan  `json:"plans"`
}

// chatAnswer e a resposta do webhook calendar-chat. Proposal (WAVE 5, E7) e opcional: quando
// o usuario pede para CRIAR evento/task, a IA devolve a proposta AQUI (nao cria nada); o front
// mostra um cartao de confirmacao e cria pela API autenticada do usuario.
type chatAnswer struct {
	Answer   string        `json:"answer"`
	Proposal *ChatProposal `json:"proposal"`
	// AIError (WAVE 5) = o n8n nao conseguiu falar com o LLM (503/cota/chave/vazio). O
	// answer traz a mensagem amigavel; o front mostra isso como estado "IA off" (visual
	// distinto) e o back NAO persiste como mensagem (nao suja a memoria da conversa).
	AIError bool `json:"aiError"`
}

// ChatProposal e a proposta de criacao devolvida pela IA (WAVE 5, E7). A IA NAO cria nada; e'
// passthrough validado (shape fechado) para o front confirmar. Kind = "event" | "task".
type ChatProposal struct {
	Kind   string             `json:"kind"`
	Fields ChatProposalFields `json:"fields"`
}

// ChatProposalFields sao os campos sugeridos (uniao dos de evento e task; o front usa os que
// fazem sentido para o kind). Nada aqui e autoritativo: a criacao real valida no endpoint.
type ChatProposalFields struct {
	Title    string `json:"title"`
	Date     string `json:"date,omitempty"`
	Time     string `json:"time,omitempty"`
	Type     string `json:"type,omitempty"`
	Status   string `json:"status,omitempty"`
	DueDate  string `json:"dueDate,omitempty"`
	ColumnID string `json:"columnId,omitempty"`
	ClientID string `json:"clientId,omitempty"`
}

// sanitizeProposal descarta propostas malformadas (kind fora de event|task ou sem titulo):
// nesse caso a resposta e tratada como texto normal (proposal nil).
func sanitizeProposal(p *ChatProposal) *ChatProposal {
	if p == nil {
		return nil
	}
	kind := strings.ToLower(strings.TrimSpace(p.Kind))
	if kind != "event" && kind != "task" {
		return nil
	}
	if strings.TrimSpace(p.Fields.Title) == "" {
		return nil
	}
	p.Kind = kind
	return p
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
	if _, err := s.store.AppendMessage(ctx, account, conv.ID, chatRoleUser, question); err != nil {
		return ChatAskResult{}, err
	}
	contextBlock, err := s.buildChatContext(ctx, account, access, target.mode, target.clientID, req.Month)
	if err != nil {
		return ChatAskResult{}, err
	}
	payload := chatWebhookPayload{
		Question:   question,
		SessionKey: calendarChatSessionKey(account, principal.UserID, conv.ID),
		Language:   chatLanguage,
		AI:         chatPayloadAI{AIConfig: effAI, APIKey: apiKey},
		Context:    contextBlock,
		History:    toHistory(prior),
	}
	answer, proposal, aiError, err := s.postChatAsk(ctx, payload)
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
	if !aiError {
		if _, err := s.store.AppendMessage(ctx, account, conv.ID, chatRoleAssistant, answer); err != nil {
			return ChatAskResult{}, err
		}
	}
	if err := s.store.TouchConversation(ctx, account, conv.ID, title); err != nil {
		return ChatAskResult{}, err
	}
	return ChatAskResult{Answer: answer, ConversationID: conv.ID, Title: title, Proposal: proposal, AIError: aiError}, nil
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
func (s *Service) postChatAsk(ctx context.Context, payload chatWebhookPayload) (string, *ChatProposal, bool, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, false, fmt.Errorf("%w: %v", errChatUpstream, err)
	}
	callCtx, cancel := context.WithTimeout(ctx, chatAskTimeout)
	defer cancel()
	raw, status, err := s.doChatRequest(callCtx, s.chat.askURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", nil, false, err
	}
	if status < 200 || status >= 300 {
		return "", nil, false, fmt.Errorf("%w: http %d", errChatUpstream, status)
	}
	var out chatAnswer
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", nil, false, fmt.Errorf("%w: resposta invalida: %v", errChatUpstream, err)
	}
	// aiError = o LLM falhou (n8n devolveu o aviso amigavel); sem proposta nesse caso.
	if out.AIError {
		return out.Answer, nil, true, nil
	}
	return out.Answer, sanitizeProposal(out.Proposal), false, nil
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
