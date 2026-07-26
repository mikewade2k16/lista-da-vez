package customerintelligence

import "time"

const (
	defaultRetentionPolicyKey    = "customer_profile.default"
	defaultSnapshotTTLSeconds    = 90 * 24 * 60 * 60
	minSnapshotTTLSeconds        = 24 * 60 * 60
	maxSnapshotTTLSeconds        = 10 * 365 * 24 * 60 * 60
	retentionActionTombstone     = "tombstone"
	retentionActionCryptoShred   = "crypto_shred"
	retentionStateActive         = "active"
	retentionStateTombstoned     = "tombstoned"
	retentionStateCryptoShredded = "crypto_shredded"
)

type RetentionPolicyVersion struct {
	ID                    string     `json:"id"`
	AccountID             string     `json:"accountId"`
	PolicyKey             string     `json:"policyKey"`
	Version               int        `json:"version"`
	Status                string     `json:"status"`
	SnapshotTTLSeconds    int        `json:"snapshotTtlSeconds"`
	OnExpiry              string     `json:"onExpiry"`
	LegalHoldBehavior     string     `json:"legalHoldBehavior"`
	BlockReingestion      bool       `json:"blockReingestion"`
	Revision              int64      `json:"revision"`
	CreatedByUserID       string     `json:"createdByUserId,omitempty"`
	PublishedByUserID     string     `json:"publishedByUserId,omitempty"`
	PublicationReasonCode string     `json:"publicationReasonCode,omitempty"`
	ApprovalReference     string     `json:"approvalReference,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	PublishedAt           *time.Time `json:"publishedAt,omitempty"`
}

type RetentionPolicyDraftInput struct {
	SnapshotTTLSeconds int    `json:"snapshotTtlSeconds"`
	OnExpiry           string `json:"onExpiry"`
}

type PublishRetentionPolicyInput struct {
	ExpectedRevision  int64  `json:"expectedRevision"`
	ReasonCode        string `json:"reasonCode"`
	ApprovalReference string `json:"approvalReference"`
}

func validRetentionPolicyInput(input SourceConfigInput) bool {
	if input.RetentionPolicyKey != "" &&
		(len(input.RetentionPolicyKey) > 160 ||
			!safeKeyPattern.MatchString(input.RetentionPolicyKey)) {
		return false
	}
	if input.SnapshotTTLSeconds != 0 &&
		(input.SnapshotTTLSeconds < minSnapshotTTLSeconds ||
			input.SnapshotTTLSeconds > maxSnapshotTTLSeconds) {
		return false
	}
	return input.OnExpiry == "" ||
		input.OnExpiry == retentionActionTombstone ||
		input.OnExpiry == retentionActionCryptoShred
}

func validRetentionPolicyDraft(
	policyKey string,
	input RetentionPolicyDraftInput,
) bool {
	return len(policyKey) <= 160 &&
		safeKeyPattern.MatchString(policyKey) &&
		input.SnapshotTTLSeconds >= minSnapshotTTLSeconds &&
		input.SnapshotTTLSeconds <= maxSnapshotTTLSeconds &&
		(input.OnExpiry == retentionActionTombstone ||
			input.OnExpiry == retentionActionCryptoShred)
}

func effectiveObservationExpiry(
	observedAt time.Time,
	adapterExpiry *time.Time,
	ttlSeconds int,
) time.Time {
	policyExpiry := observedAt.Add(time.Duration(ttlSeconds) * time.Second)
	if adapterExpiry != nil && adapterExpiry.Before(policyExpiry) {
		return adapterExpiry.UTC()
	}
	return policyExpiry.UTC()
}
