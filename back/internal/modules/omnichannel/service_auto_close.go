package omnichannel

import (
	"context"
	"encoding/json"
	"strings"
)

// SystemTryAutoClose evaluates an untrusted model proposal under the current
// lock and persists both accepted and rejected decisions for pilot tuning.
func (s *Service) SystemTryAutoClose(ctx context.Context, accountID, conversationID string, in AutoCloseRequest) (AutoCloseDecisionView, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(conversationID) == "" {
		return AutoCloseDecisionView{}, ErrForbidden
	}
	if in.Proposal.Confidence < 0 || in.Proposal.Confidence > 1 {
		return AutoCloseDecisionView{}, ErrInvalidBody
	}
	in.FinalReply = strings.TrimSpace(in.FinalReply)
	if len([]rune(in.FinalReply)) > maxContentRunes || (in.FinalReply != "" && strings.TrimSpace(in.ReplyIdempotencyKey) == "") {
		return AutoCloseDecisionView{}, ErrInvalidBody
	}
	return s.store.ApplyAIAutoClose(ctx, accountID, conversationID, in, func(locked autoCloseLockedContext) (autoClosePersistence, error) {
		evaluation := EvaluateAutoClose(locked.Policy, in.Proposal, locked.Collected,
			in.CapturedGeneration, locked.Snapshot.AIGeneration)
		policyJSON, _ := json.Marshal(map[string]any{
			"profileFound":             locked.Policy.Found,
			"profileEnabled":           locked.Policy.ProfileEnabled,
			"autoCloseEnabled":         locked.Policy.AutoCloseEnabled,
			"minimumConfidence":        locked.Policy.MinimumConfidence,
			"requireAllRequiredFields": locked.Policy.RequireAllRequiredFields,
			"blockOnHumanRequest":      locked.Policy.BlockOnHumanRequest,
			"blockSensitiveTopics":     locked.Policy.BlockSensitiveTopics,
			"validGenerationRequired":  true,
		})
		out := autoClosePersistence{Evaluation: evaluation, PolicyJSON: policyJSON}
		if !evaluation.Accepted {
			return out, nil
		}
		update, _, err := s.decideTransition(ctx, accountID, EventConvClose, TransitionPayload{}, locked.Snapshot)
		if err != nil {
			return autoClosePersistence{}, err
		}
		update.PreserveAIMessageID = strings.TrimSpace(in.PreserveAIMessageID)
		out.Update = update
		return out, nil
	})
}
