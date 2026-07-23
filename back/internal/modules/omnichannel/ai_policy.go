package omnichannel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// E2-BE-05: o resultado do modelo é input não confiável. A policy decide somente a intenção
// operacional; fila, estado e outbox continuam sendo aplicados pelos serviços Go.
type BrainDecision string

const (
	BrainContinueAI BrainDecision = "continue_ai"
	BrainHandoff    BrainDecision = "handoff"
	BrainNoReply    BrainDecision = "no_reply"
	BrainClose      BrainDecision = "close"
)

var ErrBrainSchemaInvalid = errors.New("omnichannel: brain.result.v2 schema invalid")

type BrainReplyV2 struct {
	Text               *string `json:"text"`
	InReplyToMessageID *string `json:"inReplyToMessageId"`
}

type BrainClassificationV2 struct {
	Intent     string  `json:"intent"`
	Confidence float64 `json:"confidence"`
	Sentiment  string  `json:"sentiment"`
}

type BrainRoutingSuggestionV2 struct {
	DepartmentSlug *string `json:"departmentSlug"`
	QueueSlug      *string `json:"queueSlug"`
}

type BrainHandoffV2 struct {
	Needed     bool    `json:"needed"`
	ReasonCode *string `json:"reasonCode"`
	Summary    *string `json:"summary"`
}

// BrainClosureV3 carries facts/proposals from the untrusted model. Requested
// never means authorized: the Go auto-close evaluator owns that decision.
type BrainClosureV3 struct {
	Requested      bool    `json:"requested"`
	HumanRequested bool    `json:"humanRequested"`
	SensitiveTopic bool    `json:"sensitiveTopic"`
	Reason         *string `json:"reason"`
}

type BrainUsageV2 struct {
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	Cost             float64 `json:"cost"`
}

type BrainToolCallV2 struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type BrainTraceV2 struct {
	ToolCalls []BrainToolCallV2 `json:"toolCalls"`
	Warnings  []string          `json:"warnings"`
}

// BrainResultV2 é a forma já desserializada do contrato. ExtractedFields é deliberadamente
// dinâmico, mas seus nomes/valores são filtrados contra collect_field_defs antes de persistir.
type BrainResultV2 struct {
	SchemaVersion    string                    `json:"schemaVersion"`
	DispatchID       string                    `json:"dispatchId"`
	Generation       int64                     `json:"generation"`
	Decision         BrainDecision             `json:"decision"`
	Reply            *BrainReplyV2             `json:"reply"`
	Classification   BrainClassificationV2     `json:"classification"`
	ExtractedFields  map[string]any            `json:"extractedFields"`
	ContactMemory    *ContactMemorySuggestion  `json:"contactMemory,omitempty"`
	SuggestedRouting *BrainRoutingSuggestionV2 `json:"suggestedRouting"`
	Handoff          BrainHandoffV2            `json:"handoff"`
	Closure          *BrainClosureV3           `json:"closure,omitempty"`
	Usage            BrainUsageV2              `json:"usage"`
	Trace            BrainTraceV2              `json:"trace"`
}

// BrainPolicyConfig vem da versão publicada do agente. MaxAITurns=0 desliga apenas o teto
// por conversa; confiança, falhas, quotas e os demais gates continuam ativos.
type BrainPolicyConfig struct {
	MinConfidence  float64
	MaxAITurns     int
	HandoffOnError bool
	HandoffOnLimit bool
}

func DefaultBrainPolicyConfig() BrainPolicyConfig {
	return BrainPolicyConfig{MinConfidence: 0.65, MaxAITurns: 0, HandoffOnError: true, HandoffOnLimit: true}
}

type BrainPolicyOutcome struct {
	Decision           BrainDecision
	ShouldSend         bool
	ShouldHandoff      bool
	ShouldClose        bool
	ReplyText          string
	InReplyToMessageID *string
	ReasonCode         string
	ExtractedFields    map[string]any
	SuggestedRouting   *BrainRoutingSuggestionV2
}

// DecodeBrainResultV2 rejeita campos desconhecidos, JSON concatenado, enums abertos e limites
// excessivos. As mensagens de erro citam apenas caminho/classe, nunca o conteúdo do cliente.
func DecodeBrainResultV2(raw []byte) (BrainResultV2, error) {
	var out BrainResultV2
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return BrainResultV2{}, fmt.Errorf("%w: payload", ErrBrainSchemaInvalid)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return BrainResultV2{}, fmt.Errorf("%w: multiple json values", ErrBrainSchemaInvalid)
	}
	if err := validateBrainResultV2(out); err != nil {
		return BrainResultV2{}, err
	}
	if out.ExtractedFields == nil {
		out.ExtractedFields = map[string]any{}
	}
	return out, nil
}

func validateBrainResultV2(out BrainResultV2) error {
	invalid := func(reason string) error { return fmt.Errorf("%w: %s", ErrBrainSchemaInvalid, reason) }
	if out.SchemaVersion != "brain.result.v2" && out.SchemaVersion != "brain.result.v3" {
		return invalid("schemaVersion")
	}
	if strings.TrimSpace(out.DispatchID) == "" || len(out.DispatchID) > 64 || out.Generation < 0 {
		return invalid("identity")
	}
	switch out.Decision {
	case BrainContinueAI, BrainHandoff, BrainNoReply:
	case BrainClose:
		if out.SchemaVersion != "brain.result.v3" {
			return invalid("decision")
		}
	default:
		return invalid("decision")
	}
	if strings.TrimSpace(out.Classification.Intent) == "" || len(out.Classification.Intent) > 120 ||
		out.Classification.Confidence < 0 || out.Classification.Confidence > 1 {
		return invalid("classification")
	}
	switch out.Classification.Sentiment {
	case "positive", "neutral", "negative", "unknown":
	default:
		return invalid("sentiment")
	}
	if len(out.ExtractedFields) > 100 {
		return invalid("extractedFields")
	}
	if out.ContactMemory != nil {
		normalized := normalizeContactMemory(*out.ContactMemory)
		if len(normalized.Facts) != len(out.ContactMemory.Facts) ||
			len(normalized.Preferences) != len(out.ContactMemory.Preferences) {
			return invalid("contactMemory")
		}
		if out.ContactMemory.Summary != nil && len([]rune(*out.ContactMemory.Summary)) > 1000 {
			return invalid("contactMemory.summary")
		}
	}
	if out.Reply != nil {
		if out.Reply.Text != nil && len([]rune(*out.Reply.Text)) > 4000 {
			return invalid("reply.text")
		}
		if out.Reply.InReplyToMessageID != nil && len(*out.Reply.InReplyToMessageID) > 64 {
			return invalid("reply.inReplyToMessageId")
		}
	}
	if out.SuggestedRouting != nil {
		if out.SuggestedRouting.DepartmentSlug != nil && len(*out.SuggestedRouting.DepartmentSlug) > 120 {
			return invalid("suggestedRouting.departmentSlug")
		}
		if out.SuggestedRouting.QueueSlug != nil && len(*out.SuggestedRouting.QueueSlug) > 120 {
			return invalid("suggestedRouting.queueSlug")
		}
	}
	if out.Handoff.ReasonCode != nil && len(*out.Handoff.ReasonCode) > 120 {
		return invalid("handoff.reasonCode")
	}
	if out.Handoff.Summary != nil && len([]rune(*out.Handoff.Summary)) > 4000 {
		return invalid("handoff.summary")
	}
	if out.SchemaVersion == "brain.result.v2" && out.Closure != nil {
		return invalid("closure")
	}
	if out.SchemaVersion == "brain.result.v3" {
		if out.Closure == nil {
			return invalid("closure")
		}
		if out.Closure.Reason != nil && len([]rune(*out.Closure.Reason)) > 1000 {
			return invalid("closure.reason")
		}
		if out.Decision == BrainClose && !out.Closure.Requested {
			return invalid("closure.requested")
		}
		if out.Decision == BrainClose &&
			(out.Reply == nil || out.Reply.Text == nil || strings.TrimSpace(*out.Reply.Text) == "") {
			return invalid("close reply")
		}
	}
	if out.Usage.Provider == "" || out.Usage.Model == "" || out.Usage.PromptTokens < 0 ||
		out.Usage.CompletionTokens < 0 || out.Usage.PromptTokens > 10000000 ||
		out.Usage.CompletionTokens > 10000000 || out.Usage.Cost < 0 || out.Usage.Cost > 1000000 {
		return invalid("usage")
	}
	if len(out.Trace.ToolCalls) > 50 || len(out.Trace.Warnings) > 50 {
		return invalid("trace")
	}
	for _, call := range out.Trace.ToolCalls {
		if call.Name == "" || len(call.Name) > 120 {
			return invalid("trace.toolCalls.name")
		}
		switch call.Status {
		case "ok", "error", "denied", "timeout":
		default:
			return invalid("trace.toolCalls.status")
		}
	}
	for _, warning := range out.Trace.Warnings {
		if warning == "" || len(warning) > 500 {
			return invalid("trace.warnings")
		}
	}
	return nil
}

// ApplyBrainPolicy converte a decisão do modelo em uma intenção operacional. Nunca grava fila
// nem muda state; o service posterior valida routing e cria outbox sob lock.
func ApplyBrainPolicy(result BrainResultV2, cfg BrainPolicyConfig, aiTurns int) (BrainPolicyOutcome, error) {
	if err := validateBrainResultV2(result); err != nil {
		return BrainPolicyOutcome{}, err
	}
	if cfg.MinConfidence < 0 || cfg.MinConfidence > 1 || cfg.MaxAITurns < 0 || aiTurns < 0 {
		return BrainPolicyOutcome{}, fmt.Errorf("%w: policy config", ErrBrainSchemaInvalid)
	}
	out := BrainPolicyOutcome{
		Decision:         result.Decision,
		ExtractedFields:  result.ExtractedFields,
		SuggestedRouting: result.SuggestedRouting,
	}
	switch result.Decision {
	case BrainClose:
		out.ShouldClose = true
		if result.Reply != nil && result.Reply.Text != nil {
			out.ShouldSend = strings.TrimSpace(*result.Reply.Text) != ""
			out.ReplyText = *result.Reply.Text
			out.InReplyToMessageID = result.Reply.InReplyToMessageID
		}
		out.ReasonCode = "model_close"
		return out, nil
	case BrainHandoff:
		out.ShouldHandoff = true
		out.ReasonCode = derefBrainString(result.Handoff.ReasonCode, "model_handoff")
		return out, nil
	case BrainNoReply:
		out.ReasonCode = "model_no_reply"
		return out, nil
	case BrainContinueAI:
		if result.Handoff.Needed {
			out.Decision = BrainHandoff
			out.ShouldHandoff = true
			out.ReasonCode = derefBrainString(result.Handoff.ReasonCode, "model_handoff")
			return out, nil
		}
		if result.Classification.Confidence < cfg.MinConfidence {
			return limitOutcome(out, cfg.HandoffOnLimit, "low_confidence"), nil
		}
		if cfg.MaxAITurns > 0 && aiTurns >= cfg.MaxAITurns {
			return limitOutcome(out, cfg.HandoffOnLimit, "max_ai_turns"), nil
		}
		if result.Reply == nil || result.Reply.Text == nil || strings.TrimSpace(*result.Reply.Text) == "" {
			return BrainPolicyOutcome{}, fmt.Errorf("%w: continue_ai reply", ErrBrainSchemaInvalid)
		}
		out.ShouldSend = true
		out.ReplyText = *result.Reply.Text
		out.InReplyToMessageID = result.Reply.InReplyToMessageID
		return out, nil
	default:
		return BrainPolicyOutcome{}, fmt.Errorf("%w: decision", ErrBrainSchemaInvalid)
	}
}

func BrainErrorPolicy(cfg BrainPolicyConfig, reasonCode string) BrainPolicyOutcome {
	if cfg.HandoffOnError {
		return BrainPolicyOutcome{Decision: BrainHandoff, ShouldHandoff: true, ReasonCode: derefBrainString(&reasonCode, "brain_error")}
	}
	return BrainPolicyOutcome{Decision: BrainNoReply, ReasonCode: derefBrainString(&reasonCode, "brain_error")}
}

func limitOutcome(out BrainPolicyOutcome, handoff bool, reason string) BrainPolicyOutcome {
	out.ReasonCode = reason
	if handoff {
		out.Decision = BrainHandoff
		out.ShouldHandoff = true
	} else {
		out.Decision = BrainNoReply
	}
	return out
}

func derefBrainString(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}
