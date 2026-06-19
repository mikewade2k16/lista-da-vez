package automation

import (
	"context"
	"strings"
)

// OmniChatResultView e a resposta do Omni Chat para o painel de Operacao.
type OmniChatResultView struct {
	Answer string `json:"answer"`
	Topic  string `json:"topic,omitempty"`
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
func (s *Service) OmniChatAsk(ctx context.Context, scope ContextScope, question, topic string) (OmniChatResultView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, scope.AccountID)
	if err != nil {
		return OmniChatResultView{}, err
	}
	persona, err := s.ensurePersona(ctx, a)
	if err != nil {
		return OmniChatResultView{}, err
	}
	docs, err := s.store.ListKnowledgeDocs(ctx, a.ID)
	if err != nil {
		return OmniChatResultView{}, err
	}
	systemMessage := buildSystemMessage(persona.SystemPrompt, docs)

	// Context token (Fase 2). Se o secret nao estiver configurado, Issue falha;
	// nesse caso seguimos sem token (chat sem tools de dados) em vez de quebrar o
	// chat inteiro — as tools simplesmente recusam (401) sem um token valido.
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
	})
	if err != nil {
		return OmniChatResultView{}, err
	}

	return OmniChatResultView{Answer: result.Answer, Topic: topic}, nil
}
