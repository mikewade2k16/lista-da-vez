package customerdata

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (s *Service) ListMatchCandidates(
	ctx context.Context,
	principal auth.Principal,
	clientAccountID string,
	filter MatchCandidateFilter,
) ([]MatchCandidate, string, error) {
	scope, err := s.authorizedScope(ctx, principal, clientAccountID, permMergeManage)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = boundedLimit(filter.Limit, 50, 100)
	if filter.Status != "" {
		switch filter.Status {
		case "pending", "accepted", "rejected", "expired":
		default:
			return nil, "", invalid("status", "unsupported")
		}
	}
	return s.repo.ListMatchCandidates(ctx, scope, filter)
}

func (s *Service) GetMatchCandidate(ctx context.Context, principal auth.Principal, candidateID string) (MatchCandidate, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceCandidate, candidateID, permMergeManage)
	if err != nil {
		return MatchCandidate{}, err
	}
	return s.repo.GetMatchCandidate(ctx, scope, candidateID)
}

func (s *Service) DecideMatchCandidate(
	ctx context.Context,
	principal auth.Principal,
	candidateID string,
	input MatchDecisionInput,
) (MatchCandidate, bool, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceCandidate, candidateID, permMergeManage)
	if err != nil {
		return MatchCandidate{}, false, err
	}
	if input.Decision != "accept" && input.Decision != "reject" {
		return MatchCandidate{}, false, invalid("decision", "unsupported")
	}
	if strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 1000 {
		return MatchCandidate{}, false, invalid("reason", "invalid_length")
	}
	if input.ExpectedRevision <= 0 || len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return MatchCandidate{}, false, invalid("revisionOrIdempotency", "invalid")
	}
	if err := s.requireCapability(ctx, scope, CapabilityMatchingMerge, false); err != nil {
		return MatchCandidate{}, false, err
	}
	if err := s.requireWriter(ctx, scope, WriterMerge); err != nil {
		return MatchCandidate{}, false, err
	}
	if input.CreateRelationship {
		if err := s.requireWriter(ctx, scope, WriterRelationship); err != nil {
			return MatchCandidate{}, false, err
		}
	}
	return s.repo.DecideMatchCandidate(ctx, scope, candidateID, input)
}

func (s *Service) MergeSubjects(
	ctx context.Context,
	principal auth.Principal,
	sourceSubjectID, clientAccountID string,
	input MergeInput,
) (MergeEvent, error) {
	scope, err := s.authorizedScope(ctx, principal, clientAccountID, permMergeManage)
	if err != nil {
		return MergeEvent{}, err
	}
	if strings.TrimSpace(sourceSubjectID) == "" || strings.TrimSpace(input.TargetSubjectID) == "" ||
		sourceSubjectID == input.TargetSubjectID {
		return MergeEvent{}, invalid("subjects", "invalid")
	}
	if input.ExpectedSourceRevision <= 0 || input.ExpectedTargetRevision <= 0 {
		return MergeEvent{}, invalid("expectedRevision", "must_be_positive")
	}
	if strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 1000 ||
		len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return MergeEvent{}, invalid("reasonOrIdempotency", "invalid")
	}
	if err := s.requireCapability(ctx, scope, CapabilityMatchingMerge, false); err != nil {
		return MergeEvent{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterMerge); err != nil {
		return MergeEvent{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterRelationship); err != nil {
		return MergeEvent{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterIdentity); err != nil {
		return MergeEvent{}, err
	}
	return s.repo.MergeSubjects(ctx, scope, sourceSubjectID, input)
}

func (s *Service) UndoMerge(
	ctx context.Context,
	principal auth.Principal,
	mergeEventID string,
	input UndoMergeInput,
) (MergeEvent, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceMerge, mergeEventID, permMergeManage)
	if err != nil {
		return MergeEvent{}, err
	}
	if strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 1000 ||
		len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return MergeEvent{}, invalid("reasonOrIdempotency", "invalid")
	}
	if err := s.requireCapability(ctx, scope, CapabilityMatchingMerge, false); err != nil {
		return MergeEvent{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterMerge); err != nil {
		return MergeEvent{}, err
	}
	return s.repo.UndoMerge(ctx, scope, mergeEventID, input)
}
