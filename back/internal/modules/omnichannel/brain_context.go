package omnichannel

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

func buildBrainRequestV2(p triageParams, fields []CollectFieldView) BrainRequestV2 {
	channel := strings.ToUpper(strings.TrimSpace(p.Channel))
	if channel != "INSTAGRAM" {
		channel = "WHATSAPP"
	}
	state := strings.TrimSpace(p.ConversationState)
	if state == "" {
		state = string(StateAIActive)
	}
	contactID := strings.TrimSpace(p.ContactID)
	if contactID == "" {
		contactID = "unknown"
	}
	relationship := stringValue(p.ContactContext, "relationshipStatus", "relationship_status")
	switch relationship {
	case "new_lead", "known_lead", "customer", "inactive", "unknown":
	default:
		relationship = "unknown"
	}
	tags := stringSliceValue(p.ContactContext, "tags")
	var summary *string
	if value := stringValue(p.ContactContext, "summary"); value != "" {
		summary = &value
	}
	requestSchema := "brain.request.v2"
	if strings.EqualFold(strings.TrimSpace(p.Version.WorkflowContract), "brain.v3") {
		requestSchema = "brain.request.v3"
	}
	request := BrainRequestV2{
		SchemaVersion: requestSchema, DispatchID: p.DispatchID, Generation: p.AIGeneration,
		Tenant:       BrainTenantV2{AccountID: p.AccountID, Timezone: "America/Sao_Paulo"},
		Conversation: BrainConversationV2{ID: derefStringPtr(p.ConversationID), State: state, Channel: channel},
		Contact: BrainContactV2{ID: contactID, RelationshipStatus: relationship, Tags: tags,
			Origin: brainOriginFromContext(p.ContactContext), Summary: summary},
		CollectedFields: p.ContactContext,
		RequiredFields:  requiredFieldKeys(fields),
		PendingFields:   pendingFieldKeys(fields, p.ContactContext),
		LocalTime:       BrainLocalTimeV2{Now: time.Now().Format(time.RFC3339), InsideBusinessHours: true},
		Agent:           BrainAgentV2{ID: p.Agent.ID, VersionID: p.Version.ID, Model: p.Version.Model, Layers: layerMap(p.Version.Layers)},
		Capabilities:    BrainCapabilitiesV2{Tools: []string{}, Multimodal: false},
	}
	if requestSchema == "brain.request.v3" && p.BusinessContext != nil {
		request.Client = &BrainClientV3{ID: p.BusinessContext.ClientID}
		request.BusinessContext = p.BusinessContext
	}
	request.Messages = make([]BrainMessageV2, 0, len(p.History))
	for index, message := range p.History {
		role := "assistant"
		if message.Role == "contact" {
			role = "contact"
		}
		id := strings.TrimSpace(message.ID)
		if id == "" {
			id = "history-" + strconv.Itoa(index)
		}
		text := message.Text
		request.Messages = append(request.Messages, BrainMessageV2{ID: id, Role: role, Type: "TEXT", Text: &text, Media: nil})
	}
	return request
}

func triageOutputFromBrainResult(result BrainResultV2) TriageOutput {
	out := TriageOutput{Intent: result.Classification.Intent, Confidence: result.Classification.Confidence,
		ExtractedFields: result.ExtractedFields, NeedsHuman: result.Decision == BrainHandoff || result.Handoff.Needed}
	if result.Handoff.ReasonCode != nil {
		out.HandoffReason = *result.Handoff.ReasonCode
	}
	if result.Handoff.Summary != nil {
		out.HandoffSummary = *result.Handoff.Summary
	}
	if result.Closure != nil {
		out.CloseRequested = result.Decision == BrainClose || result.Closure.Requested
		out.HumanRequested = result.Closure.HumanRequested
		out.SensitiveTopic = result.Closure.SensitiveTopic
		if result.Closure.Reason != nil {
			out.CloseReason = *result.Closure.Reason
		}
	}
	if result.Reply != nil && result.Reply.Text != nil {
		out.ReplyDraft = *result.Reply.Text
	}
	if result.SuggestedRouting != nil {
		if result.SuggestedRouting.DepartmentSlug != nil {
			out.SuggestedDepartment = *result.SuggestedRouting.DepartmentSlug
		}
		if result.SuggestedRouting.QueueSlug != nil {
			out.SuggestedQueue = *result.SuggestedRouting.QueueSlug
		}
	}
	if result.Decision == BrainNoReply {
		out.ReplyDraft = ""
	}
	if out.ExtractedFields == nil {
		out.ExtractedFields = map[string]any{}
	}
	return out
}

func layerMap(raw json.RawMessage) map[string]any {
	var out map[string]any
	if len(raw) > 0 && json.Unmarshal(raw, &out) == nil && out != nil {
		return out
	}
	return map[string]any{}
}

func requiredFieldKeys(fields []CollectFieldView) []string {
	out := make([]string, 0)
	for _, field := range fields {
		if field.Required && strings.TrimSpace(field.Key) != "" {
			out = append(out, field.Key)
		}
	}
	return out
}

func pendingFieldKeys(fields []CollectFieldView, context map[string]any) []string {
	out := make([]string, 0)
	for _, field := range fields {
		if !field.Required || strings.TrimSpace(field.Key) == "" {
			continue
		}
		value, ok := context[field.Key]
		if !ok || value == nil || strings.TrimSpace(toStringValue(value)) == "" {
			out = append(out, field.Key)
		}
	}
	return out
}

func brainOriginFromContext(context map[string]any) BrainOriginV2 {
	origin, _ := context["origin"].(map[string]any)
	return BrainOriginV2{
		Channel: optionalString(origin, "channel"), Source: optionalString(origin, "source"),
		LandingPage: optionalString(origin, "landingPageSlug"), Campaign: optionalString(origin, "campaign"),
		Referrer: optionalString(origin, "referrer"), FirstTouchAt: optionalString(origin, "firstTouchAt"),
		LastTouchAt: optionalString(origin, "lastTouchAt"),
	}
}

func optionalString(values map[string]any, key string) *string {
	if values == nil {
		return nil
	}
	value := strings.TrimSpace(toStringValue(values[key]))
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(toStringValue(values[key])); value != "" {
			return value
		}
	}
	return ""
}

func stringSliceValue(values map[string]any, key string) []string {
	items, _ := values[key].([]any)
	if len(items) == 0 {
		if typed, ok := values[key].([]string); ok {
			return typed
		}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value := strings.TrimSpace(toStringValue(item)); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func toStringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func derefStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
