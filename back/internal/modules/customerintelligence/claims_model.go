package customerintelligence

import (
	"encoding/json"
	"time"
)

// AcceptedClaimRef is safe to cross the Omnichannel integration outbox. It
// identifies a claim inside an encrypted runtime result but deliberately does
// not repeat the extracted value in messaging.*.
type AcceptedClaimRef struct {
	Ordinal                int      `json:"ordinal"`
	FactKey                string   `json:"factKey"`
	ValueType              string   `json:"valueType"`
	Confidence             float64  `json:"confidence"`
	EvidenceObservationIDs []string `json:"evidenceObservationIds"`
	ValidFrom              string   `json:"validFrom,omitempty"`
	ValidUntil             string   `json:"validUntil,omitempty"`
	ProcessKey             string   `json:"processKey"`
	RuntimeRunID           string   `json:"runtimeRunId"`
	PromptBindingID        string   `json:"promptBindingId"`
	OutputSchemaVersion    string   `json:"outputSchemaVersion"`
}

type CandidateClaim struct {
	ID                      string          `json:"id"`
	AccountID               string          `json:"accountId"`
	ClientAccountID         string          `json:"clientAccountId"`
	SubjectID               string          `json:"subjectId"`
	RelationshipID          string          `json:"relationshipId"`
	FactDefinitionID        string          `json:"factDefinitionId"`
	FactDefinitionVersionID string          `json:"factDefinitionVersionId"`
	FactKey                 string          `json:"factKey"`
	ValueType               string          `json:"valueType"`
	Value                   json.RawMessage `json:"value"`
	valueCiphertext         string
	ExtractionMethod        string        `json:"extractionMethod"`
	ExtractorKey            string        `json:"extractorKey"`
	ExtractorVersion        string        `json:"extractorVersion"`
	PromptBindingID         string        `json:"promptBindingId,omitempty"`
	RuntimeRunID            string        `json:"runtimeRunId,omitempty"`
	Confidence              float64       `json:"confidence"`
	VerificationState       string        `json:"verificationState"`
	ValidFrom               *time.Time    `json:"validFrom,omitempty"`
	ValidUntil              *time.Time    `json:"validUntil,omitempty"`
	Sensitivity             string        `json:"sensitivity"`
	Status                  string        `json:"status"`
	SourceOutcomeEventID    string        `json:"sourceOutcomeEventId,omitempty"`
	SourceClaimOrdinal      *int          `json:"sourceClaimOrdinal,omitempty"`
	Revision                int64         `json:"revision"`
	ReviewedByUserID        string        `json:"reviewedByUserId,omitempty"`
	ReviewedAt              *time.Time    `json:"reviewedAt,omitempty"`
	ReviewReasonCode        string        `json:"reviewReasonCode,omitempty"`
	Evidence                []EvidenceRef `json:"evidence"`
	CreatedAt               time.Time     `json:"createdAt"`
	UpdatedAt               time.Time     `json:"updatedAt"`
}

type ClaimReviewInput struct {
	Status           string `json:"status"`
	ReasonCode       string `json:"reasonCode"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type runtimeClaimSource struct {
	RunRef           ProcessRunRef
	SubjectID        string
	RelationshipID   string
	OutputCiphertext string
}

type preparedCandidateClaim struct {
	Reference                  AcceptedClaimRef
	ValueCiphertext            string
	ValueCiphertextFingerprint string
	ValidFrom                  *time.Time
	ValidUntil                 *time.Time
}
