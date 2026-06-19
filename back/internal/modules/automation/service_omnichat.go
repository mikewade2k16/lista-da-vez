package automation

import (
	"context"
	"strings"
)

// OmniChatResultView e a resposta do Omni Chat para o painel de Operacao. products
// vem da tool de catalogo (via n8n) para o front renderizar cards com imagem; vazio
// em pergunta nao-produto.
type OmniChatResultView struct {
	Answer   string       `json:"answer"`
	Topic    string       `json:"topic,omitempty"`
	Products []ProductHit `json:"products,omitempty"`
}

// OmniChatAsk responde uma pergunta do chat interno (painel de Operacao) via
// n8n. Reusa a automacao default da account para montar o systemMessage do Tony
// (persona + knowledge docs + guardrails), sem sessao WAHA. O escopo (scope) vem
// do principal autenticado, nunca do body — defesa multi-tenant. O scope e'
// assinado num context token HMAC (Fase 2) injetado no body do webhook; o n8n o
// devolve nas tools de dados (catalogo etc.) e o Go le o escopo SO do token.
//
// Erros propagados (mapeados em http_omnichat.go):
//   - ErrN8NNotConfigured -> 503 omnichat_not_configured
//   - errN8NFailed         -> 502 omnichat_error
//   - context.DeadlineExceeded -> 504 omnichat_timeout
//
// Janela de memoria do Omni Chat (interacoes pergunta+resposta que o n8n mantem no
// contexto). default quando a conta nunca salvou; clamp defensivo nos extremos.
const (
	defaultHistoryWindow = 5
	minHistoryWindow     = 1
	maxHistoryWindow     = 20
)

func (s *Service) OmniChatAsk(ctx context.Context, scope ContextScope, question, topic, conversationID string) (OmniChatResultView, error) {
	// Persona DEDICADA do Omni Chat (Perola Joias — copiloto de vendas/conhecimento)
	// + janela de memoria, ambos do banco (settings jsonb da automacao default; se a
	// conta nunca salvou, persona cai no embed omniChatPersona e janela no default).
	// NAO reusa o Tony (WhatsApp). Catalogo DESCONECTADO por ora.
	systemMessage, _, historyWindow, err := s.OmniChatConfig(ctx, scope.AccountID)
	if err != nil {
		return OmniChatResultView{}, err
	}

	// Context token (Fase 2). Mantido (inofensivo) para reconectar as tools de dados
	// (catalogo etc.) depois sem retrabalho; hoje o workflow nao chama nenhuma tool.
	// Se o secret nao estiver configurado, Issue falha e seguimos sem token.
	contextToken, err := s.ctxMgr.Issue(scope)
	if err != nil {
		contextToken = ""
	}

	topic = strings.TrimSpace(topic)
	result, err := s.n8n.Ask(ctx, OmniChatRunRequest{
		Question:      question,
		Topic:         topic,
		SystemMessage: systemMessage,
		SessionRef:    "omni-chat-" + scope.AccountID,
		ContextToken:  contextToken,
		SessionKey:    omniChatSessionKey(scope, conversationID),
		HistoryWindow: historyWindow,
	})
	if err != nil {
		return OmniChatResultView{}, err
	}

	return OmniChatResultView{Answer: result.Answer, Topic: topic, Products: result.Products}, nil
}

// omniChatSessionKey monta a chave de memoria da conversa para o n8n. Escopa por
// account + user (isola memoria entre operadores — nunca por accountID puro) + o
// conversationId que o front gera por conversa. Sem conversationId, cai numa chave
// estavel por usuario (a memoria ainda segue a conversa daquele operador).
func omniChatSessionKey(scope ContextScope, conversationID string) string {
	base := scope.AccountID + "|" + scope.UserID
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return base
	}
	return base + "|" + conversationID
}

// clampHistoryWindow normaliza a janela: 0/ausente -> default; fora da faixa ->
// limita aos extremos.
func clampHistoryWindow(n int) int {
	switch {
	case n <= 0:
		return defaultHistoryWindow
	case n < minHistoryWindow:
		return minHistoryWindow
	case n > maxHistoryWindow:
		return maxHistoryWindow
	default:
		return n
	}
}

// OmniChatConfig retorna a config EFETIVA do Omni Chat da account: systemPrompt
// (custom salvo OU embed default), isDefault, e a janela de memoria (default quando
// nao salva). Uma unica resolucao de automacao default (GetOrCreateDefault) p/ as
// duas leituras de settings.
func (s *Service) OmniChatConfig(ctx context.Context, accountID string) (systemPrompt string, isDefault bool, historyWindow int, err error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return "", false, 0, err
	}
	custom, err := s.store.GetOmniChatPersonaSetting(ctx, a.ID)
	if err != nil {
		return "", false, 0, err
	}
	win, err := s.store.GetOmniChatHistoryWindow(ctx, a.ID)
	if err != nil {
		return "", false, 0, err
	}
	historyWindow = clampHistoryWindow(win)
	if strings.TrimSpace(custom) == "" {
		return omniChatPersona, true, historyWindow, nil
	}
	return custom, false, historyWindow, nil
}

// SetOmniChatConfig grava persona (systemPrompt) e janela de memoria no settings
// jsonb da automacao default da account. Devolve os valores salvos (prompt com trim,
// janela com clamp). A partir daqui o prompt efetivo deixa de ser o default embutido.
func (s *Service) SetOmniChatConfig(ctx context.Context, accountID, prompt string, historyWindow int) (savedPrompt string, savedWindow int, err error) {
	prompt = strings.TrimSpace(prompt)
	historyWindow = clampHistoryWindow(historyWindow)
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return "", 0, err
	}
	if err := s.store.SetOmniChatPersonaSetting(ctx, a.ID, prompt); err != nil {
		return "", 0, err
	}
	if err := s.store.SetOmniChatHistoryWindow(ctx, a.ID, historyWindow); err != nil {
		return "", 0, err
	}
	return prompt, historyWindow, nil
}
