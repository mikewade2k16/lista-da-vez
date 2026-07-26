package omnichannel

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// AutoCloseProposal is the model's untrusted request to end a conversation.
// It never changes state by itself; EvaluateAutoClose is the Go policy gate.
type AutoCloseProposal struct {
	Requested      bool
	Confidence     float64
	HumanRequested bool
	SensitiveTopic bool
	Reason         string
}

// AutoCloseRuntimePolicy is the materialized, tenant-scoped configuration used
// for one decision. Generation validity is intentionally not configurable.
type AutoCloseRuntimePolicy struct {
	Found                    bool
	ProfileID                *string
	ProfileEnabled           bool
	AutoCloseEnabled         bool
	MinimumConfidence        float64
	RequireAllRequiredFields bool
	BlockOnHumanRequest      bool
	BlockSensitiveTopics     bool
	RequiredFields           []string
}

type AutoCloseEvaluation struct {
	Accepted           bool
	ReasonCodes        []string
	MissingFields      []string
	CapturedGeneration int64
	CurrentGeneration  int64
}

type AutoCloseDecisionView struct {
	ID                  string    `json:"id"`
	ConversationID      string    `json:"conversationId"`
	Requested           bool      `json:"requested"`
	Accepted            bool      `json:"accepted"`
	ReasonCodes         []string  `json:"reasonCodes"`
	Confidence          float64   `json:"confidence"`
	MinimumConfidence   float64   `json:"minimumConfidence"`
	MissingFields       []string  `json:"missingFields"`
	CapturedGeneration  int64     `json:"capturedGeneration"`
	CurrentGeneration   int64     `json:"currentGeneration"`
	CreatedAt           time.Time `json:"createdAt"`
	FinalMessageID      *string   `json:"finalMessageId,omitempty"`
	finalMessage        *MessageView
	finalMessageCreated bool
}

type AutoCloseRequest struct {
	Proposal               AutoCloseProposal
	AIRunID                string
	IdempotencyKey         string
	CapturedGeneration     int64
	PreserveAIMessageID    string
	FinalReply             string
	ReplyIdempotencyKey    string
	IntelligenceAcceptance *CustomerIntelligenceAcceptedOutcome
}

type autoCloseLockedContext struct {
	Snapshot  convSnapshot
	Policy    AutoCloseRuntimePolicy
	Collected map[string]any
}

type autoClosePersistence struct {
	Evaluation AutoCloseEvaluation
	Update     stateUpdate
	PolicyJSON json.RawMessage
}

const (
	autoCloseReasonNotRequested       = "close_not_requested"
	autoCloseReasonProfileMissing     = "automation_profile_missing"
	autoCloseReasonAutomationDisabled = "automation_disabled"
	autoCloseReasonDisabled           = "auto_close_disabled"
	autoCloseReasonLowConfidence      = "confidence_below_minimum"
	autoCloseReasonRequiredFields     = "required_fields_missing"
	autoCloseReasonHumanRequested     = "human_requested"
	autoCloseReasonSensitiveTopic     = "sensitive_topic"
	autoCloseReasonStaleGeneration    = "stale_generation"
)

// EvaluateAutoClose applies every safety gate and returns all blockers, rather
// than only the first one, so the pilot can be calibrated from auditable data.
func EvaluateAutoClose(policy AutoCloseRuntimePolicy, proposal AutoCloseProposal, collected map[string]any, capturedGeneration, currentGeneration int64) AutoCloseEvaluation {
	out := AutoCloseEvaluation{CapturedGeneration: capturedGeneration, CurrentGeneration: currentGeneration}
	add := func(reason string) { out.ReasonCodes = append(out.ReasonCodes, reason) }

	if !proposal.Requested {
		add(autoCloseReasonNotRequested)
	}
	if !policy.Found {
		add(autoCloseReasonProfileMissing)
	} else {
		if !policy.ProfileEnabled {
			add(autoCloseReasonAutomationDisabled)
		}
		if !policy.AutoCloseEnabled {
			add(autoCloseReasonDisabled)
		}
		if proposal.Confidence < policy.MinimumConfidence {
			add(autoCloseReasonLowConfidence)
		}
		if policy.RequireAllRequiredFields {
			out.MissingFields = missingRequiredFields(policy.RequiredFields, collected)
			if len(out.MissingFields) > 0 {
				add(autoCloseReasonRequiredFields)
			}
		}
		if policy.BlockOnHumanRequest && proposal.HumanRequested {
			add(autoCloseReasonHumanRequested)
		}
		if policy.BlockSensitiveTopics && proposal.SensitiveTopic {
			add(autoCloseReasonSensitiveTopic)
		}
	}
	// A stale generation is always rejected, even when every configurable gate
	// is disabled. This is the concurrency lease, not a business preference.
	if capturedGeneration != currentGeneration {
		add(autoCloseReasonStaleGeneration)
	}
	out.Accepted = len(out.ReasonCodes) == 0
	return out
}

func missingRequiredFields(required []string, collected map[string]any) []string {
	missing := make([]string, 0)
	seen := make(map[string]struct{}, len(required))
	for _, raw := range required {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		value, ok := collected[key]
		if !ok || emptyCollectedValue(value) {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

func emptyCollectedValue(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	// false and zero are legitimate answers and must not be treated as absent.
	return false
}
