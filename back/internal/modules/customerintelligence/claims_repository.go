package customerintelligence

import "context"

// candidateClaimRepository is intentionally separate from FoundationRepository
// so existing source/runtime fakes and external facades do not gain unrelated
// methods. The PostgreSQL repository implements both contracts.
type candidateClaimRepository interface {
	GetRuntimeClaimSource(
		ctx context.Context,
		scope Scope,
		subjectID, relationshipID, runID string,
	) (runtimeClaimSource, error)
	RecordOutcomeWithClaims(
		ctx context.Context,
		outcome AcceptedOutcome,
		claims []preparedCandidateClaim,
	) (bool, error)
	ListCandidateClaims(
		ctx context.Context,
		scope Scope,
		relationshipID, status string,
		limit int,
	) ([]CandidateClaim, error)
	GetCandidateClaim(
		ctx context.Context,
		scope Scope,
		claimID string,
	) (CandidateClaim, error)
	ReviewCandidateClaim(
		ctx context.Context,
		scope Scope,
		actorID, claimID string,
		input ClaimReviewInput,
	) (CandidateClaim, error)
}
