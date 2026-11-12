package tools

import "context"

// Tipos de evento do canal de roteiros (scripts:account:{id}). O evento é só uma
// invalidação: leva o id e o updatedAt, nunca o conteúdo; a tela refaz o fetch pela API,
// que reaplica o escopo de leitura (scriptVisibleTo).
const (
	ScriptEventSaved     = "script.saved"
	ScriptEventDeleted   = "script.deleted"
	ScriptEventReordered = "script.reordered"
)

// ScriptEvent é o payload lean publicado após cada mutação persistida.
type ScriptEvent struct {
	Type      string
	AccountID string
	// ClientAccountIDs: contas-cliente que viam ou passam a ver o roteiro (cliente atual
	// visível e cliente anterior). Recebem a mesma invalidação, sem dado de outra conta.
	ClientAccountIDs []string
	ResourceID       string
	UpdatedAt        string
	// UserID limita "script.reordered" às abas do próprio usuário (a ordem é por usuário).
	UserID string
}

// ScriptPublisher entrega os eventos ao transporte realtime.
type ScriptPublisher interface {
	PublishScriptEvent(ctx context.Context, evt ScriptEvent)
}

type noopScriptPublisher struct{}

func (noopScriptPublisher) PublishScriptEvent(context.Context, ScriptEvent) {}

// Option configura o Module (ex.: tools.New(tools.WithScriptPublisher(realtimeService))).
type Option func(*Module)

// WithScriptPublisher liga o canal realtime dos roteiros. nil mantém o canal desligado.
func WithScriptPublisher(publisher ScriptPublisher) Option {
	return func(m *Module) {
		if publisher != nil {
			m.scriptPublisher = publisher
		}
	}
}
