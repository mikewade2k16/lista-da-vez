package omnichannel

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// E2-BE-03: dispatch durável do cérebro. O status persistido no PostgreSQL é a única
// autoridade; este tipo não cria um timer ou estado paralelo em memória.
type AIDispatchStatus string

const (
	AIDispatchBuffering  AIDispatchStatus = "buffering"
	AIDispatchQueued     AIDispatchStatus = "queued"
	AIDispatchProcessing AIDispatchStatus = "processing"
	AIDispatchCompleted  AIDispatchStatus = "completed"
	AIDispatchCancelled  AIDispatchStatus = "cancelled"
	AIDispatchFailed     AIDispatchStatus = "failed"
)

var (
	// ErrAIDispatchInvalidInput é seguro para log/HTTP: não carrega conteúdo do cliente.
	ErrAIDispatchInvalidInput = errors.New("omnichannel: invalid ai dispatch input")
)

// AIDispatchRecord é a projeção controlada de messaging.ai_dispatches. Nunca contém prompt,
// chave de API ou payload de provider.
type AIDispatchRecord struct {
	ID             string
	AccountID      string
	ConversationID string
	AgentVersionID string
	Generation     int64
	Status         AIDispatchStatus
	MessageIDs     []string
	RunAfter       time.Time
	LockedAt       *time.Time
	CompletedAt    *time.Time
	IdempotencyKey string
	ResultRunID    *string
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// aiDispatchIdempotencyKey é determinística, tenant-scoped pelo índice da tabela e não contém
// PII. A generation muda quando uma nova mensagem é anexada ao agrupamento.
func aiDispatchIdempotencyKey(conversationID string, generation int64) string {
	return fmt.Sprintf("brain:%s:%d", strings.TrimSpace(conversationID), generation)
}

func validAIDispatchStatus(status AIDispatchStatus) bool {
	switch status {
	case AIDispatchBuffering, AIDispatchQueued, AIDispatchProcessing,
		AIDispatchCompleted, AIDispatchCancelled, AIDispatchFailed:
		return true
	default:
		return false
	}
}
