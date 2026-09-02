package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CustomerIntelligenceBridge is deliberately owned by the Omnichannel
// consumer. The composition root translates it to the independent
// customerintelligence.Runtime contract; neither module imports the other.
//
// The bridge proposes a decision. It cannot mutate the conversation, enqueue a
// channel message or call a provider. All effects remain in this package.
type CustomerIntelligenceBridge interface {
	ExecuteInteraction(context.Context, CustomerIntelligenceInteractionRequest) (CustomerIntelligenceDecision, error)
	RecordAcceptedOutcome(context.Context, CustomerIntelligenceAcceptedOutcome) error
}

type CustomerIntelligenceInteractionRequest struct {
	AccountID           string
	ClientAccountID     string
	ContactSourceID     string
	ContactExternalID   string
	ContactPhone        string
	ContactName         string
	ConversationID      string
	MessageID           string
	DispatchID          string
	Generation          int64
	Channel             string
	ProcessKey          string
	OperatorForced      bool
	Message             json.RawMessage
	OperationalState    json.RawMessage
	RoutingCatalog      json.RawMessage
	ChannelCapabilities json.RawMessage
	OccurredAt          time.Time
	// DerivedMemorySuppressed e calculado pelo Store no mesmo snapshot da
	// conversa/cutoff e nunca e controlado pelo payload HTTP.
	DerivedMemorySuppressed bool `json:"-"`
}

type CustomerIntelligenceDecision struct {
	DecisionID        string
	PipelineVersionID string
	RunID             string
	ProcessRuns       []CustomerIntelligenceProcessRunRef
	// OperationalEffectAllowed is derived by the trusted composition adapter
	// from persisted process-run execution modes. It is never model-controlled.
	// The Omnichannel still revalidates every run before applying an effect.
	OperationalEffectAllowed bool
	SubjectID                string
	RelationshipID           string
	Outcome                  string
	ReplyDraft               string
	NeedsHuman               bool
	HandoffReason            string
	HandoffSummary           string
	CloseRequested           bool
	CloseReason              string
	Confidence               float64
	HumanRequested           bool
	SensitiveTopic           bool
	ExtractedFields          map[string]any
	CandidateClaims          []CustomerIntelligenceAcceptedClaimRef
	ReasonCode               string
}

type CustomerIntelligenceProcessRunRef struct {
	ProcessKey              string `json:"processKey"`
	RunID                   string `json:"runId"`
	Status                  string `json:"status"`
	ExecutionMode           string `json:"executionMode"`
	ProcessDefinitionID     string `json:"processDefinitionId"`
	ProcessConfigVersionID  string `json:"processConfigVersionId"`
	PromptBindingID         string `json:"promptBindingId"`
	PlatformPromptVersionID string `json:"platformPromptVersionId"`
	AgencyPromptVersionID   string `json:"agencyPromptVersionId,omitempty"`
	ClientPromptVersionID   string `json:"clientPromptVersionId,omitempty"`
	ProcessPromptVersionID  string `json:"processPromptVersionId"`
	AgentVersionID          string `json:"agentVersionId"`
	ModelID                 string `json:"modelId"`
	ContextSnapshotID       string `json:"contextSnapshotId"`
	OutputSchemaVersion     string `json:"outputSchemaVersion"`
}

// CustomerIntelligenceAcceptedClaimRef never carries the extracted value.
// Customer Intelligence rehydrates it from RuntimeRunID after validating the
// complete ProcessRuns reference and tenant/client/relationship scope.
type CustomerIntelligenceAcceptedClaimRef struct {
	Ordinal                int      `json:"ordinal"`
	FactKey                string   `json:"factKey"`
	ValueType              string   `json:"valueType"`
	Confidence             float64  `json:"confidence"`
	EvidenceObservationIDs []string `json:"evidenceObservationIds"`
	ValidFrom              string   `json:"validFrom,omitempty"`
	ValidUntil             string   `json:"validUntil,omitempty"`
	ProcessKey             string   `json:"processKey"`
	RuntimeRunID           string   `json:"runtimeRunId"`
	PromptBindingID        string   `json:"promptBindingId"`
	OutputSchemaVersion    string   `json:"outputSchemaVersion"`
}

// CustomerIntelligenceFailure is the consumer-owned, safe failure contract.
// It deliberately carries no prompt, provider response, message body or
// credential. Technical failures must never masquerade as a no_reply decision.
type CustomerIntelligenceFailure struct {
	Kind      string
	Code      string
	Retryable bool
	cause     error
}

func (e *CustomerIntelligenceFailure) Error() string {
	if e == nil {
		return "omnichannel: customer intelligence failure"
	}
	return fmt.Sprintf(
		"omnichannel: customer intelligence %s (%s)",
		strings.TrimSpace(e.Kind),
		strings.TrimSpace(e.Code),
	)
}

func (e *CustomerIntelligenceFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func NewCustomerIntelligenceFailure(
	kind, code string,
	retryable bool,
	cause error,
) error {
	return &CustomerIntelligenceFailure{
		Kind: strings.TrimSpace(kind), Code: strings.TrimSpace(code),
		Retryable: retryable, cause: cause,
	}
}

func CustomerIntelligenceFailureDetails(
	err error,
) (kind, code string, retryable bool, ok bool) {
	var failure *CustomerIntelligenceFailure
	if !errors.As(err, &failure) {
		return "", "", false, false
	}
	return failure.Kind, failure.Code, failure.Retryable, true
}

type CustomerIntelligenceAcceptedOutcome struct {
	EventID           string                                 `json:"eventId"`
	AccountID         string                                 `json:"accountId"`
	ClientAccountID   string                                 `json:"clientAccountId"`
	ContactSourceID   string                                 `json:"contactSourceId"`
	ConversationID    string                                 `json:"conversationId"`
	MessageID         string                                 `json:"messageId,omitempty"`
	DispatchID        string                                 `json:"dispatchId"`
	DecisionID        string                                 `json:"decisionId"`
	PipelineVersionID string                                 `json:"pipelineVersionId"`
	RunID             string                                 `json:"runId,omitempty"`
	ProcessRuns       []CustomerIntelligenceProcessRunRef    `json:"processRuns,omitempty"`
	Claims            []CustomerIntelligenceAcceptedClaimRef `json:"claims,omitempty"`
	SubjectID         string                                 `json:"subjectId"`
	RelationshipID    string                                 `json:"relationshipId"`
	Generation        int64                                  `json:"generation"`
	Outcome           string                                 `json:"outcome"`
}

type CustomerIntelligenceExecutionPolicy struct {
	Mode          string
	FailurePolicy string
}

func (s *Store) CustomerIntelligencePolicy(
	ctx context.Context,
	accountID string,
) (CustomerIntelligenceExecutionPolicy, error) {
	var policy CustomerIntelligenceExecutionPolicy
	err := s.pool.QueryRow(ctx, `
		select
			coalesce((
				select customer_intelligence_mode
				from messaging.account_config
				where account_id = $1::uuid
			), 'off'),
			coalesce((
				select customer_intelligence_failure_policy
			from messaging.account_config
			where account_id = $1::uuid
			), 'retry_then_handoff')`,
		accountID,
	).Scan(&policy.Mode, &policy.FailurePolicy)
	if err != nil {
		return CustomerIntelligenceExecutionPolicy{}, err
	}
	switch strings.ToLower(strings.TrimSpace(policy.Mode)) {
	case "shadow", "on":
		policy.Mode = strings.ToLower(strings.TrimSpace(policy.Mode))
	default:
		policy.Mode = "off"
	}
	switch strings.ToLower(strings.TrimSpace(policy.FailurePolicy)) {
	case "legacy_fallback", "immediate_handoff":
		policy.FailurePolicy = strings.ToLower(strings.TrimSpace(policy.FailurePolicy))
	default:
		policy.FailurePolicy = "retry_then_handoff"
	}
	return policy, nil
}

// CustomerIntelligenceMode remains as a compatibility facade for callers that
// only need the rollout mode.
func (s *Store) CustomerIntelligenceMode(ctx context.Context, accountID string) (string, error) {
	policy, err := s.CustomerIntelligencePolicy(ctx, accountID)
	return policy.Mode, err
}
