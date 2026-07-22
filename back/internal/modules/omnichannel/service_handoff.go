package omnichannel

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (s *Service) ListHandoffs(ctx context.Context, accountID string, p auth.Principal, conversationID string) ([]HandoffView, error) {
	if err := s.requirePermission(ctx, accountID, p, "omnichannel.conversations.view"); err != nil {
		return nil, err
	}
	if _, err := NewActionsService(s.store, s, nil, nil, nil, nil).resolveConversation(ctx, accountID, p, conversationID); err != nil {
		return nil, err
	}
	return s.store.ListHandoffs(ctx, accountID, conversationID)
}

func (s *Service) GetQueueSLAPolicy(ctx context.Context, accountID string, p auth.Principal, queueID string) (QueueSLAPolicyView, error) {
	if err := s.requirePermission(ctx, accountID, p, "omnichannel.settings.manage"); err != nil {
		return QueueSLAPolicyView{}, err
	}
	if err := s.assertActiveQueue(ctx, accountID, queueID); err != nil {
		return QueueSLAPolicyView{}, translate(err)
	}
	return s.store.GetQueueSLAPolicy(ctx, accountID, queueID)
}

func (s *Service) UpsertQueueSLAPolicy(ctx context.Context, accountID string, p auth.Principal, queueID string, in QueueSLAPolicyInput) (QueueSLAPolicyView, error) {
	if err := s.requirePermission(ctx, accountID, p, "omnichannel.settings.manage"); err != nil {
		return QueueSLAPolicyView{}, err
	}
	if err := s.assertActiveQueue(ctx, accountID, queueID); err != nil {
		return QueueSLAPolicyView{}, translate(err)
	}
	if in.FirstResponseSeconds < 1 || in.FirstResponseSeconds > 2592000 || in.ResolutionSeconds < 1 || in.ResolutionSeconds > 7776000 {
		return QueueSLAPolicyView{}, ErrInvalidBody
	}
	return s.store.UpsertQueueSLAPolicy(ctx, accountID, queueID, in)
}

func (s *Service) ListSLAEvents(ctx context.Context, accountID string, p auth.Principal, conversationID string) ([]SLAEventView, error) {
	if err := s.requirePermission(ctx, accountID, p, "omnichannel.conversations.view"); err != nil {
		return nil, err
	}
	if _, err := NewActionsService(s.store, s, nil, nil, nil, nil).resolveConversation(ctx, accountID, p, conversationID); err != nil {
		return nil, err
	}
	return s.store.ListSLAEvents(ctx, accountID, conversationID)
}

func normalizeHandoffRequest(in *HandoffRequest) error {
	in.ReasonCode = strings.ToLower(strings.TrimSpace(in.ReasonCode))
	if in.ReasonCode == "" {
		in.ReasonCode = HandoffReasonRequested
	}
	if !validHandoffReason(in.ReasonCode) || len([]rune(in.Summary)) > 12000 {
		return ErrInvalidBody
	}
	in.Summary = strings.TrimSpace(in.Summary)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.IdempotencyKey == "" || len([]rune(in.IdempotencyKey)) > 128 {
		return ErrInvalidBody
	}
	if in.TargetQueueID != nil {
		queueID := strings.TrimSpace(*in.TargetQueueID)
		if !omnichannelUUIDPattern.MatchString(queueID) {
			return ErrInvalidBody
		}
		in.TargetQueueID = &queueID
	}
	if len(in.CollectedFields) == 0 || string(in.CollectedFields) == "null" {
		in.CollectedFields = json.RawMessage(`{}`)
	}
	var fields map[string]any
	if json.Unmarshal(in.CollectedFields, &fields) != nil || fields == nil || len(in.CollectedFields) > 64000 {
		return ErrInvalidBody
	}
	for key := range fields {
		key = strings.ToLower(strings.TrimSpace(key))
		for _, marker := range []string{"secret", "token", "password", "credential", "apikey", "api_key"} {
			if strings.Contains(key, marker) {
				return ErrInvalidBody
			}
		}
	}
	return nil
}

// SystemRequestHandoff is the internal, authenticated runtime path used by the
// AI dispatcher. It deliberately skips user RBAC, but keeps the same body
// validation and tenant-scoped repository transaction as the HTTP action.
func (s *Service) SystemRequestHandoff(ctx context.Context, accountID, conversationID string, in HandoffRequest) (HandoffView, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(conversationID) == "" {
		return HandoffView{}, ErrForbidden
	}
	if err := normalizeHandoffRequest(&in); err != nil {
		return HandoffView{}, err
	}
	if in.TargetQueueID != nil {
		if err := s.assertActiveQueue(ctx, accountID, *in.TargetQueueID); err != nil {
			return HandoffView{}, translate(err)
		}
	}
	return s.store.CreateHandoff(ctx, accountID, conversationID, "", in)
}

// RequestAutomationHandoff is the operator-authenticated automation path. RBAC
// and client visibility are enforced by AutomationService before this call;
// this method preserves the actor in the canonical handoff audit transaction.
func (s *Service) RequestAutomationHandoff(ctx context.Context, accountID, conversationID, actorID string, in HandoffRequest) (HandoffView, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(conversationID) == "" || strings.TrimSpace(actorID) == "" {
		return HandoffView{}, ErrForbidden
	}
	if err := normalizeHandoffRequest(&in); err != nil {
		return HandoffView{}, err
	}
	if in.TargetQueueID != nil {
		if err := s.assertActiveQueue(ctx, accountID, *in.TargetQueueID); err != nil {
			return HandoffView{}, translate(err)
		}
	}
	return s.store.CreateHandoff(ctx, accountID, conversationID, actorID, in)
}

func normalizeAIHandoffReason(value string, humanRequested bool) string {
	if humanRequested {
		return HandoffReasonRequested
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "requested", "human_requested", "operator_requested", "attendant_requested":
		return HandoffReasonRequested
	case "low_confidence", "confidence":
		return HandoffReasonLowConfidence
	case "max_turns", "max_ai_turns", "turn_limit":
		return HandoffReasonMaxTurns
	case "tool_failed", "tool_error", "tool_timeout":
		return HandoffReasonToolFailed
	case "error", "provider_error", "schema_invalid":
		return HandoffReasonError
	case "model_handoff":
		return HandoffReasonModel
	default:
		return HandoffReasonPolicy
	}
}
