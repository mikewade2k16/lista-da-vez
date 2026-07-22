package omnichannel

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// ListAIToolRuns returns only masked, tenant-scoped evidence. audit.view is
// intentionally separate from agents.manage so an auditor cannot change a
// binding or approve a side effect.
func (s *AIService) ListAIToolRuns(ctx context.Context, accountID string, p auth.Principal, agentID, status, beforeID string, limit int) ([]AIToolRunView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.audit.view"); err != nil {
		return nil, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) {
		return nil, ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return nil, err
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && !validAIToolRunStatus(status) {
		return nil, ErrValidation
	}
	return s.store.ListAIToolRuns(ctx, accountID, agentID, status, strings.TrimSpace(beforeID), normalizeRunLimit(limit))
}

// ListAIToolApprovals exposes the human approval queue without exposing
// ciphertext or unmasked arguments.
//
// DecideAIToolApproval is the only human approval surface. It never executes a
// provider call; it atomically records the decision, and a retry of the same
// signed n8n call is what may continue an approved proposal through Go's
// registry and policy checks.
func (s *AIService) ListAIToolApprovals(ctx context.Context, accountID string, p auth.Principal, agentID, beforeID string, limit int) ([]AIToolApprovalView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.audit.view"); err != nil {
		return nil, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) {
		return nil, ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return nil, err
	}
	return s.store.ListAIToolApprovals(ctx, accountID, agentID, normalizeRunLimit(limit), strings.TrimSpace(beforeID))
}

func (s *AIService) DecideAIToolApproval(ctx context.Context, accountID string, p auth.Principal, agentID, approvalID string, approved bool, reason string) (AIToolApprovalView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIToolApprovalView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) ||
		!omnichannelUUIDPattern.MatchString(strings.TrimSpace(approvalID)) {
		return AIToolApprovalView{}, ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return AIToolApprovalView{}, err
	}
	if len([]rune(strings.TrimSpace(reason))) > 500 {
		return AIToolApprovalView{}, ErrValidation
	}
	if err := s.store.DecideAIToolApproval(ctx, accountID, agentID, approvalID, p.UserID, approved, reason); err != nil {
		return AIToolApprovalView{}, err
	}
	return s.store.ApprovalView(ctx, accountID, agentID, approvalID)
}
