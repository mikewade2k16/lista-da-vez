package customerintelligence

import (
	"context"
	"strings"
)

func (s *Service) RetentionPolicies(
	ctx context.Context,
	accountID string,
) ([]RetentionPolicyVersion, error) {
	if !validUUID(accountID) {
		return nil, ErrInvalidInput
	}
	return s.foundation.ListRetentionPolicyVersions(ctx, accountID)
}

func (s *Service) CreateRetentionPolicyDraft(
	ctx context.Context,
	accountID, actorID, policyKey string,
	input RetentionPolicyDraftInput,
) (RetentionPolicyVersion, error) {
	policyKey = strings.TrimSpace(policyKey)
	if !validUUID(accountID) ||
		!validUUID(actorID) ||
		!validRetentionPolicyDraft(policyKey, input) {
		return RetentionPolicyVersion{}, ErrInvalidInput
	}
	return s.foundation.CreateRetentionPolicyDraft(
		ctx,
		accountID,
		actorID,
		policyKey,
		input,
	)
}

func (s *Service) PublishRetentionPolicy(
	ctx context.Context,
	accountID, actorID, versionID string,
	input PublishRetentionPolicyInput,
) (RetentionPolicyVersion, error) {
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	input.ApprovalReference = strings.TrimSpace(input.ApprovalReference)
	if !validUUID(accountID) ||
		!validUUID(actorID) ||
		!validUUID(versionID) ||
		input.ExpectedRevision <= 0 ||
		len(input.ReasonCode) > 80 ||
		!safeKeyPattern.MatchString(input.ReasonCode) ||
		!requestKeyPattern.MatchString(input.ApprovalReference) {
		return RetentionPolicyVersion{}, ErrInvalidInput
	}
	return s.foundation.PublishRetentionPolicyVersion(
		ctx,
		accountID,
		actorID,
		versionID,
		input,
	)
}
