package omnichannel

import (
	"encoding/json"
	"time"
)

const (
	HandoffReasonRequested     = "requested"
	HandoffReasonLowConfidence = "low_confidence"
	HandoffReasonMaxTurns      = "max_turns"
	HandoffReasonToolFailed    = "tool_failed"
	HandoffReasonPolicy        = "policy"
	HandoffReasonError         = "error"
	HandoffReasonModel         = "model_handoff"
	HandoffReasonOperatorPause = "operator_paused"

	HandoffStatusRequested = "requested"
	HandoffStatusQueued    = "queued"
	HandoffStatusAccepted  = "accepted"
	HandoffStatusCancelled = "cancelled"
	HandoffStatusClosed    = "closed"
)

type HandoffView struct {
	ID                      string          `json:"id"`
	ConversationID          string          `json:"conversationId"`
	AIRunID                 *string         `json:"aiRunId"`
	RoutingDecisionID       *string         `json:"routingDecisionId"`
	PolicyID                *string         `json:"policyId"`
	PolicySnapshot          json.RawMessage `json:"policySnapshot"`
	ReasonCode              string          `json:"reasonCode"`
	Summary                 string          `json:"summary"`
	CollectedFields         json.RawMessage `json:"collectedFields"`
	SourceState             string          `json:"sourceState"`
	TargetQueueID           *string         `json:"targetQueueId"`
	Status                  string          `json:"status"`
	AcceptedByUserID        *string         `json:"acceptedByUserId"`
	RequestedAt             time.Time       `json:"requestedAt"`
	QueuedAt                *time.Time      `json:"queuedAt"`
	AcceptedAt              *time.Time      `json:"acceptedAt"`
	ClosedAt                *time.Time      `json:"closedAt"`
	CreatedAt               time.Time       `json:"createdAt"`
	UpdatedAt               time.Time       `json:"updatedAt"`
	CustomerNoticeMessageID *string         `json:"customerNoticeMessageId,omitempty"`
	customerNoticeMessage   *MessageView
	customerNoticeCreated   bool
}

// HandoffPolicyView é a configuração determinística que pode selecionar a fila
// de um novo handoff. Conditions é um objeto fechado e validado pelo backend;
// o modelo nunca cria ou altera policies.
type HandoffPolicyView struct {
	ID                     string         `json:"id"`
	Name                   string         `json:"name"`
	Priority               int            `json:"priority"`
	IsActive               bool           `json:"isActive"`
	Conditions             map[string]any `json:"conditions"`
	TargetQueueID          *string        `json:"targetQueueId"`
	FallbackQueueID        *string        `json:"fallbackQueueId"`
	CustomerNoticeTemplate string         `json:"customerNoticeTemplate"`
	CreatedAt              time.Time      `json:"createdAt"`
	UpdatedAt              time.Time      `json:"updatedAt"`
}

type HandoffPolicyInput struct {
	Name                   string         `json:"name"`
	Priority               int            `json:"priority"`
	IsActive               *bool          `json:"isActive"`
	Conditions             map[string]any `json:"conditions"`
	TargetQueueID          *string        `json:"targetQueueId"`
	FallbackQueueID        *string        `json:"fallbackQueueId"`
	CustomerNoticeTemplate string         `json:"customerNoticeTemplate"`
}

type HandoffPolicyPatch struct {
	Name                   *string         `json:"name"`
	Priority               *int            `json:"priority"`
	IsActive               *bool           `json:"isActive"`
	Conditions             *map[string]any `json:"conditions"`
	TargetQueueID          *string         `json:"targetQueueId"`
	FallbackQueueID        *string         `json:"fallbackQueueId"`
	CustomerNoticeTemplate *string         `json:"customerNoticeTemplate"`
}

type HandoffRequest struct {
	ReasonCode             string                               `json:"reasonCode"`
	Summary                string                               `json:"summary"`
	CollectedFields        json.RawMessage                      `json:"collectedFields"`
	TargetQueueID          *string                              `json:"targetQueueId"`
	IdempotencyKey         string                               `json:"idempotencyKey"`
	CustomerNotice         string                               `json:"-"`
	AIRunID                string                               `json:"-"`
	CapturedGeneration     int64                                `json:"-"`
	NoticeIdempotencyKey   string                               `json:"-"`
	IntelligenceAcceptance *CustomerIntelligenceAcceptedOutcome `json:"-"`
}

type TakeConversationRequest struct {
	IdempotencyKey string `json:"idempotencyKey"`
}

type SLAEventView struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"conversationId"`
	HandoffID      *string         `json:"handoffId"`
	EventType      string          `json:"eventType"`
	IdempotencyKey string          `json:"idempotencyKey"`
	OccurredAt     time.Time       `json:"occurredAt"`
	Metadata       json.RawMessage `json:"metadata"`
}

type QueueSLAPolicyView struct {
	ID                   string    `json:"id"`
	QueueID              string    `json:"queueId"`
	FirstResponseSeconds int       `json:"firstResponseSeconds"`
	ResolutionSeconds    int       `json:"resolutionSeconds"`
	BusinessHoursOnly    bool      `json:"businessHoursOnly"`
	IsActive             bool      `json:"isActive"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type QueueSLAPolicyInput struct {
	FirstResponseSeconds int   `json:"firstResponseSeconds"`
	ResolutionSeconds    int   `json:"resolutionSeconds"`
	BusinessHoursOnly    bool  `json:"businessHoursOnly"`
	IsActive             *bool `json:"isActive"`
}

func validHandoffReason(value string) bool {
	switch value {
	case HandoffReasonRequested, HandoffReasonLowConfidence, HandoffReasonMaxTurns,
		HandoffReasonToolFailed, HandoffReasonPolicy, HandoffReasonError,
		HandoffReasonModel, HandoffReasonOperatorPause:
		return true
	default:
		return false
	}
}
