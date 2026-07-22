package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

var (
	ErrAIToolNotRegistered = errors.New("omnichannel: ai tool not registered")
	ErrAIToolDenied        = errors.New("omnichannel: ai tool denied")
	ErrAIToolApproval      = errors.New("omnichannel: ai tool approval required")
	ErrAIToolInProgress    = errors.New("omnichannel: ai tool call in progress")
	ErrAIToolArguments     = errors.New("omnichannel: ai tool arguments invalid")
)

// AIToolInvocation é o único contrato que um handler de ferramenta recebe. Ele não
// contém credenciais nem o Principal do operador; a autorização já ocorreu no gateway.
type AIToolInvocation struct {
	AccountID      string
	ConversationID string
	DispatchID     string
	Generation     int64
	AgentID        string
	ToolID         string
	Operation      string
	Arguments      json.RawMessage
}

// AIToolHandler implementa uma tool previamente registrada pelo compositor da aplicação.
// O registry não descobre endpoints, executa SQL ou interpreta URLs vindas do modelo.
type AIToolHandler func(context.Context, AIToolInvocation) (json.RawMessage, error)

// AIToolRegistry é explícito e imutável durante uma execução. O módulo começa sem handlers
// quando nenhum adaptador corporativo foi injetado: nesse caso a chamada falha fechado e é
// auditada, em vez de simular uma resposta.
type AIToolRegistry struct {
	mu       sync.RWMutex
	handlers map[string]AIToolHandler
}

func NewAIToolRegistry() *AIToolRegistry {
	return &AIToolRegistry{handlers: make(map[string]AIToolHandler)}
}

func (r *AIToolRegistry) Register(toolID string, handler AIToolHandler) error {
	if r == nil || handler == nil {
		return ErrValidation
	}
	toolID = strings.TrimSpace(toolID)
	if toolID == "" || len([]rune(toolID)) > 160 || strings.ContainsAny(toolID, "\r\n") {
		return ErrValidation
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[toolID]; exists {
		return ErrConflict
	}
	r.handlers[toolID] = handler
	return nil
}

func (r *AIToolRegistry) resolve(toolID string) (AIToolHandler, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[strings.TrimSpace(toolID)]
	return handler, ok
}
