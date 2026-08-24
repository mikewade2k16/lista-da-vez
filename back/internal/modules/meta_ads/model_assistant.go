package metaads

import (
	"encoding/json"
	"time"
)

// ============================================================================
// Modelos de compatibilidade do chat/runner legado (meta_ads.assistant_messages).
// O produto usa o contrato compartilhado /v1/assistant/chat/*.
// ============================================================================

// Papeis das mensagens do chat (check constraint no banco).
const (
	assistantRoleUser      = "user"
	assistantRoleAssistant = "assistant"
)

// AssistantMessage e uma mensagem persistida do chat do assistente. Actions
// guarda o jsonb cru ([]AssistantAction serializado); nil = sem acoes.
type AssistantMessage struct {
	ID        string
	AccountID string
	Role      string
	Content   string
	Actions   []byte
	CreatedAt time.Time
}

// ============================================================================
// Views (contrato JSON CONGELADO — o front codifica contra este shape)
// ============================================================================

// AssistantAction registra a observacao devolvida pelo runner legado read-only.
// Status: "ok" | "error".
type AssistantAction struct {
	Tool    string `json:"tool"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

// AssistantMessageView e uma mensagem do chat para o painel. Actions NUNCA e
// null no JSON — sem acoes vira slice vazio.
type AssistantMessageView struct {
	ID        string            `json:"id"`
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	Actions   []AssistantAction `json:"actions"`
	CreatedAt string            `json:"createdAt"`
}

// AssistantSendResult preserva o DTO do servico legado, hoje sem rota publica
// montada. O contrato de produto fica em /v1/assistant/chat/*.
type AssistantSendResult struct {
	Messages      []AssistantMessageView `json:"messages"`
	SyncTriggered bool                   `json:"syncTriggered"`
}

// AssistantHealthView e o status interno do runner legado/read-only.
type AssistantHealthView struct {
	OK         bool   `json:"ok"`
	ClaudeAuth bool   `json:"claudeAuth"`
	Detail     string `json:"detail"`
	MetaAuth   string `json:"metaAuth"`
}

// ============================================================================
// Mapeadores dominio -> view
// ============================================================================

// toAssistantMessageView converte a linha persistida na view do painel.
// Actions ausentes/invalidas viram slice vazio (nunca null no JSON).
func toAssistantMessageView(m AssistantMessage) AssistantMessageView {
	actions := []AssistantAction{}
	if len(m.Actions) > 0 {
		if err := json.Unmarshal(m.Actions, &actions); err != nil || actions == nil {
			actions = []AssistantAction{}
		}
	}
	return AssistantMessageView{
		ID:        m.ID,
		Role:      m.Role,
		Content:   m.Content,
		Actions:   actions,
		CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339),
	}
}
