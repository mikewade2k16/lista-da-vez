package tools

import "context"

// Tipos de evento do canal de roteiros (scripts:account:{id}).
//   - script.cell: UMA célula mudou; leva campo, linha e o conteúdo novo, aplicado direto
//     pela tela (sem refetch). Só chega a contas com leitura (dona + cliente visível).
//   - script.saved: estrutura/metadados (linhas, cliente) ou criação; a tela refaz o fetch.
//   - script.deleted / script.reordered: invalidação.
const (
	ScriptEventCell      = "script.cell"
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
	// UserID: autor da mudança; em "script.reordered" limita às abas do próprio usuário.
	UserID string
	// Só em script.cell: campo ("title", "description", "audio", "video"), linha e conteúdo.
	Field string
	RowID string
	Value string
}

// ScriptLocks consulta as travas de edição por célula (presença do realtime). Retorna o
// nome de quem trava, ou "" se livre/do próprio usuário. Implementado pelo realtime.Service.
type ScriptLocks interface {
	ScriptFieldLocker(scriptID, fieldKey, userID string) string
	ScriptAnyLocker(scriptID, userID string) string
}

type noopScriptLocks struct{}

func (noopScriptLocks) ScriptFieldLocker(string, string, string) string { return "" }
func (noopScriptLocks) ScriptAnyLocker(string, string) string           { return "" }

// ScriptCellKey é a chave de trava de uma célula, igual à usada pelo front na presença.
func ScriptCellKey(field, rowID string) string {
	if field == "title" || field == "description" {
		return field
	}
	return "cell:" + rowID + ":" + field
}

// WithScriptLocks liga a checagem de travas no servidor (PATCH/remoção de linha/exclusão).
func WithScriptLocks(locks ScriptLocks) Option {
	return func(m *Module) {
		if locks != nil {
			m.scriptLocks = locks
		}
	}
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

// ValidScriptID valida o id de um roteiro (uuid) para quem consome o módulo (ex.: realtime).
func ValidScriptID(id string) bool { return isUUID(id) }
