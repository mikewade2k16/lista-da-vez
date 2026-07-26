package customerdata

import (
	"encoding/json"
	"time"
)

const (
	ModuleID   = "customer_data"
	SchemaName = "customer_data"
)

type CapabilityMode string

const (
	CapabilityOff    CapabilityMode = "off"
	CapabilityShadow CapabilityMode = "shadow"
	CapabilityOn     CapabilityMode = "on"
)

type WriterMode string

const (
	WriterLegacy WriterMode = "legacy"
	WriterShadow WriterMode = "shadow"
	WriterNew    WriterMode = "new"
)

type CapabilityState struct {
	CapabilityKey string         `json:"capabilityKey"`
	Mode          CapabilityMode `json:"mode"`
	Revision      int64          `json:"revision"`
	UpdatedAt     *time.Time     `json:"updatedAt,omitempty"`
}

type CapabilityStateInput struct {
	Mode             CapabilityMode `json:"mode"`
	ExpectedRevision int64          `json:"expectedRevision"`
	IdempotencyKey   string         `json:"idempotencyKey"`
	Reason           string         `json:"reason"`
}

type WriterState struct {
	EntityKey      string     `json:"entityKey"`
	Mode           WriterMode `json:"mode"`
	Watermark      *string    `json:"watermark,omitempty"`
	SourceChecksum *string    `json:"sourceChecksum,omitempty"`
	TargetChecksum *string    `json:"targetChecksum,omitempty"`
	ApprovedBy     *string    `json:"approvedBy,omitempty"`
	ApprovedAt     *time.Time `json:"approvedAt,omitempty"`
	Revision       int64      `json:"revision"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty"`
}

type WriterStateInput struct {
	Mode             WriterMode `json:"mode"`
	Watermark        *string    `json:"watermark,omitempty"`
	SourceChecksum   *string    `json:"sourceChecksum,omitempty"`
	TargetChecksum   *string    `json:"targetChecksum,omitempty"`
	ExpectedRevision int64      `json:"expectedRevision"`
	IdempotencyKey   string     `json:"idempotencyKey"`
	Reason           string     `json:"reason"`
}

type ControlStateView struct {
	ClientAccountID string            `json:"clientAccountId"`
	Capabilities    []CapabilityState `json:"capabilities"`
	Writers         []WriterState     `json:"writerStates"`
}

const (
	CapabilityCore          = "core"
	CapabilityIdentity      = "identity_resolution"
	CapabilityMatchingMerge = "matching_merge"
	CapabilityOffline       = "offline_interactions"
	CapabilitySegmentation  = "segmentation"
)

const (
	WriterRelationship = "relationship"
	WriterIdentity     = "identity"
	WriterNote         = "note"
	WriterConsent      = "consent"
	WriterMerge        = "merge"
	WriterSegment      = "segment_definition"
)

type Scope struct {
	AccountID       string
	ClientAccountID string
	ActorUserID     string
}

type PageInfo struct {
	NextCursor string `json:"nextCursor,omitempty"`
}

type Subject struct {
	ID                  string    `json:"id"`
	AccountID           string    `json:"-"`
	SubjectType         string    `json:"subjectType"`
	Status              string    `json:"status"`
	MergedIntoSubjectID *string   `json:"mergedIntoSubjectId,omitempty"`
	Revision            int64     `json:"revision"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type PersonProfileInput struct {
	LegalName     *string `json:"legalName,omitempty"`
	PreferredName *string `json:"preferredName,omitempty"`
	Locale        *string `json:"locale,omitempty"`
	Timezone      *string `json:"timezone,omitempty"`
}

type Relationship struct {
	ID                       string          `json:"id"`
	ClientAccountID          string          `json:"clientAccountId"`
	SubjectID                string          `json:"subjectId"`
	DisplayName              string          `json:"displayName"`
	PreferredName            *string         `json:"preferredName"`
	LifecycleStatus          string          `json:"lifecycleStatus"`
	ClassificationSource     string          `json:"classificationSource"`
	ClassificationConfidence *float64        `json:"classificationConfidence,omitempty"`
	OwnerUserID              *string         `json:"ownerUserId"`
	Tags                     []string        `json:"tags"`
	CustomFields             json.RawMessage `json:"customFields"`
	FirstSeenAt              *time.Time      `json:"firstSeenAt"`
	LastSeenAt               *time.Time      `json:"lastSeenAt"`
	LastQualifiedAt          *time.Time      `json:"lastQualifiedAt"`
	ArchivedAt               *time.Time      `json:"archivedAt"`
	Revision                 int64           `json:"revision"`
	CreatedAt                time.Time       `json:"createdAt"`
	UpdatedAt                time.Time       `json:"updatedAt"`
}

type RelationshipInput struct {
	DisplayName     string          `json:"displayName"`
	PreferredName   *string         `json:"preferredName,omitempty"`
	LifecycleStatus string          `json:"lifecycleStatus"`
	OwnerUserID     *string         `json:"ownerUserId,omitempty"`
	Tags            []string        `json:"tags"`
	CustomFields    json.RawMessage `json:"customFields"`
}

type CreateSubjectInput struct {
	ClientAccountID string             `json:"clientAccountId,omitempty"`
	SubjectType     string             `json:"subjectType"`
	Profile         PersonProfileInput `json:"profile"`
	Relationship    RelationshipInput  `json:"relationship"`
	Identities      []IdentityInput    `json:"identities"`
	IdempotencyKey  string             `json:"idempotencyKey"`
}

type CreateSubjectResult struct {
	Subject      Subject        `json:"subject"`
	Relationship Relationship   `json:"relationship"`
	Identities   []IdentityView `json:"identities,omitempty"`
	Replayed     bool           `json:"replayed"`
}

type SubjectFilter struct {
	ClientAccountID string
	Query           string
	SubjectType     string
	LifecycleStatus string
	Tag             string
	OwnerUserID     string
	Archived        *bool
	UpdatedAfter    *time.Time
	Cursor          string
	Limit           int
}

type SubjectListItem struct {
	SubjectID         string         `json:"subjectId"`
	SubjectType       string         `json:"subjectType"`
	Relationship      Relationship   `json:"relationship"`
	PrimaryIdentities []IdentityView `json:"primaryIdentities,omitempty"`
}

type SubjectPage struct {
	Items []SubjectListItem `json:"items"`
	PageInfo
}

type SubjectPatch struct {
	PreferredName    *string `json:"preferredName,omitempty"`
	LegalName        *string `json:"legalName,omitempty"`
	Locale           *string `json:"locale,omitempty"`
	Timezone         *string `json:"timezone,omitempty"`
	ExpectedRevision int64   `json:"expectedRevision"`
}

type RelationshipPatch struct {
	DisplayName      *string          `json:"displayName,omitempty"`
	PreferredName    *string          `json:"preferredName,omitempty"`
	LifecycleStatus  *string          `json:"lifecycleStatus,omitempty"`
	OwnerUserID      *string          `json:"ownerUserId,omitempty"`
	Tags             *[]string        `json:"tags,omitempty"`
	CustomFields     *json.RawMessage `json:"customFields,omitempty"`
	Archive          *bool            `json:"archive,omitempty"`
	ExpectedRevision int64            `json:"expectedRevision"`
}

type IdentityInput struct {
	Kind               string          `json:"kind"`
	Issuer             string          `json:"issuer"`
	Value              string          `json:"value"`
	VerificationStatus string          `json:"verificationStatus,omitempty"`
	VerificationMethod string          `json:"verificationMethod,omitempty"`
	SourceRefType      string          `json:"sourceRefType,omitempty"`
	SourceRefID        string          `json:"sourceRefId,omitempty"`
	Metadata           json.RawMessage `json:"metadata,omitempty"`
	OccurredAt         *time.Time      `json:"occurredAt,omitempty"`
	IdempotencyKey     string          `json:"idempotencyKey,omitempty"`
}

type ProtectedIdentity struct {
	Kind               string
	Issuer             string
	Ciphertext         string
	Fingerprint        string
	KeyVersion         string
	MaskedValue        string
	VerificationStatus string
	VerificationMethod string
	SourceRefType      string
	SourceRefID        string
	Metadata           json.RawMessage
	OccurredAt         time.Time
	IdempotencyKey     string
}

type IdentityView struct {
	ID                 string     `json:"id"`
	Kind               string     `json:"kind"`
	Issuer             string     `json:"issuer"`
	MaskedValue        string     `json:"maskedValue"`
	VerificationStatus string     `json:"verificationStatus"`
	VerificationMethod string     `json:"verificationMethod,omitempty"`
	FirstSeenAt        time.Time  `json:"firstSeenAt"`
	LastSeenAt         time.Time  `json:"lastSeenAt"`
	VerifiedAt         *time.Time `json:"verifiedAt,omitempty"`
	RevokedAt          *time.Time `json:"revokedAt,omitempty"`
	Revision           int64      `json:"revision"`
}

type IdentityStateInput struct {
	VerificationMethod string `json:"verificationMethod,omitempty"`
	EvidenceRef        string `json:"evidenceRef,omitempty"`
	ExpectedRevision   int64  `json:"expectedRevision"`
	IdempotencyKey     string `json:"idempotencyKey"`
}

type SourceReference struct {
	SourceModule     string `json:"sourceModule"`
	SourceKey        string `json:"sourceKey"`
	SourceEntityType string `json:"sourceEntityType"`
	SourceEntityID   string `json:"sourceEntityId"`
	SourceVersion    string `json:"sourceVersion,omitempty"`
	SourceHash       string `json:"sourceHash,omitempty"`
}

// SourceEvidenceRequest is an owner-scoped, headless read contract used by
// trusted composition-root adapters. It never accepts scope from an end-user
// payload.
type SourceEvidenceRequest struct {
	AccountID       string
	ClientAccountID string
	RelationshipID  string
	Limit           int
}

type SourceEvidenceBundle struct {
	SubjectID      string               `json:"subjectId"`
	RelationshipID string               `json:"relationshipId"`
	SourceLinks    []SourceReference    `json:"sourceLinks"`
	Interactions   []OfflineInteraction `json:"offlineInteractions"`
}

type Note struct {
	ID             string     `json:"id"`
	RelationshipID string     `json:"relationshipId"`
	Content        string     `json:"content"`
	Revision       int64      `json:"revision"`
	ArchivedAt     *time.Time `json:"archivedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type NoteInput struct {
	Content        string           `json:"content"`
	ContextSource  *SourceReference `json:"contextSource,omitempty"`
	IdempotencyKey string           `json:"idempotencyKey,omitempty"`
}

type NotePatch struct {
	Content          string `json:"content"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type OfflineInteraction struct {
	ID                string    `json:"id"`
	RelationshipID    string    `json:"relationshipId"`
	InteractionType   string    `json:"interactionType"`
	OccurredAt        time.Time `json:"occurredAt"`
	Timezone          string    `json:"timezone"`
	DurationSeconds   *int      `json:"durationSeconds,omitempty"`
	Title             string    `json:"title"`
	Content           *string   `json:"content,omitempty"`
	Sensitivity       string    `json:"sensitivity"`
	PurposeKey        string    `json:"purposeKey"`
	SourceExternalRef *string   `json:"sourceExternalRef,omitempty"`
	Status            string    `json:"status"`
	Revision          int64     `json:"revision"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type OfflineInteractionInput struct {
	AccountID         string    `json:"-"`
	ClientAccountID   string    `json:"clientAccountId,omitempty"`
	RelationshipID    string    `json:"relationshipId,omitempty"`
	InteractionType   string    `json:"interactionType"`
	OccurredAt        time.Time `json:"occurredAt"`
	Timezone          string    `json:"timezone"`
	DurationSeconds   *int      `json:"durationSeconds,omitempty"`
	Title             string    `json:"title"`
	Content           string    `json:"content"`
	Sensitivity       string    `json:"sensitivity"`
	PurposeKey        string    `json:"purposeKey"`
	SourceExternalRef *string   `json:"sourceExternalRef,omitempty"`
	IdempotencyKey    string    `json:"idempotencyKey"`
}

type OfflineInteractionPatch struct {
	InteractionType  *string    `json:"interactionType,omitempty"`
	OccurredAt       *time.Time `json:"occurredAt,omitempty"`
	Timezone         *string    `json:"timezone,omitempty"`
	DurationSeconds  *int       `json:"durationSeconds,omitempty"`
	Title            *string    `json:"title,omitempty"`
	Content          *string    `json:"content,omitempty"`
	Sensitivity      *string    `json:"sensitivity,omitempty"`
	PurposeKey       *string    `json:"purposeKey,omitempty"`
	ExpectedRevision int64      `json:"expectedRevision"`
}

type Consent struct {
	ID             string     `json:"id"`
	RelationshipID string     `json:"relationshipId"`
	Purpose        string     `json:"purpose"`
	Channel        string     `json:"channel"`
	Status         string     `json:"status"`
	SourceModule   string     `json:"sourceModule"`
	SourceRef      *string    `json:"sourceRef,omitempty"`
	EvidenceHash   *string    `json:"evidenceHash,omitempty"`
	EffectiveAt    time.Time  `json:"effectiveAt"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type ConsentInput struct {
	Purpose        string     `json:"purpose"`
	Channel        string     `json:"channel"`
	Status         string     `json:"status"`
	SourceModule   string     `json:"sourceModule"`
	SourceRef      *string    `json:"sourceRef,omitempty"`
	EvidenceHash   *string    `json:"evidenceHash,omitempty"`
	EffectiveAt    time.Time  `json:"effectiveAt"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	IdempotencyKey string     `json:"idempotencyKey"`
}

type MatchCandidate struct {
	ID                      string          `json:"id"`
	ClientAccountID         string          `json:"clientAccountId"`
	IncomingSourceKey       string          `json:"incomingSourceKey"`
	IncomingSourceType      string          `json:"incomingSourceType"`
	IncomingSourceID        string          `json:"incomingSourceId"`
	CandidateSubjectID      *string         `json:"candidateSubjectId,omitempty"`
	CandidateRelationshipID *string         `json:"candidateRelationshipId,omitempty"`
	MatchMethod             string          `json:"matchMethod"`
	MatchConfidence         float64         `json:"matchConfidence"`
	EvidenceRefs            json.RawMessage `json:"evidenceRefs"`
	RiskFlags               json.RawMessage `json:"riskFlags"`
	Status                  string          `json:"status"`
	DecisionReason          *string         `json:"decisionReason,omitempty"`
	Revision                int64           `json:"revision"`
	CreatedAt               time.Time       `json:"createdAt"`
	UpdatedAt               time.Time       `json:"updatedAt"`
}

type MatchCandidateFilter struct {
	Status string
	Cursor string
	Limit  int
}

type MatchDecisionInput struct {
	Decision           string  `json:"decision"`
	TargetSubjectID    *string `json:"targetSubjectId,omitempty"`
	CreateRelationship bool    `json:"createRelationship"`
	Reason             string  `json:"reason"`
	ExpectedRevision   int64   `json:"expectedRevision"`
	IdempotencyKey     string  `json:"idempotencyKey"`
}

type MergeInput struct {
	TargetSubjectID        string `json:"targetSubjectId"`
	Reason                 string `json:"reason"`
	ExpectedSourceRevision int64  `json:"expectedSourceRevision"`
	ExpectedTargetRevision int64  `json:"expectedTargetRevision"`
	IdempotencyKey         string `json:"idempotencyKey"`
}

type UndoMergeInput struct {
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type MergeEvent struct {
	ID                      string          `json:"id"`
	ClientAccountID         string          `json:"clientAccountId"`
	SourceSubjectID         string          `json:"sourceSubjectId"`
	TargetSubjectID         string          `json:"targetSubjectId"`
	AffectedRelationshipIDs []string        `json:"affectedRelationshipIds"`
	Reason                  string          `json:"reason"`
	EventKind               string          `json:"eventKind"`
	ReversesEventID         *string         `json:"reversesEventId,omitempty"`
	Snapshot                json.RawMessage `json:"-"`
	CreatedAt               time.Time       `json:"createdAt"`
	Replayed                bool            `json:"replayed"`
}

type DeterministicProfile struct {
	Subject      Subject              `json:"subject"`
	Relationship Relationship         `json:"relationship"`
	Identities   []IdentityView       `json:"identities,omitempty"`
	Notes        []Note               `json:"notes,omitempty"`
	Interactions []OfflineInteraction `json:"offlineInteractions,omitempty"`
	Consents     []Consent            `json:"consents,omitempty"`
}

type ResolveSubjectRequest struct {
	AccountID       string
	ClientAccountID string
	RequestID       string
	Source          SourceReference
	Identities      []IdentityInput
	DisplayName     string
	OccurredAt      time.Time
	Purpose         string
	AllowCreate     bool
}

type ResolveSubjectResult struct {
	Status          string   `json:"status"`
	SubjectID       string   `json:"subjectId,omitempty"`
	RelationshipID  string   `json:"relationshipId,omitempty"`
	CandidateID     string   `json:"candidateId,omitempty"`
	MatchMethod     string   `json:"matchMethod,omitempty"`
	MatchConfidence float64  `json:"matchConfidence,omitempty"`
	ReasonCodes     []string `json:"reasonCodes"`
	Replayed        bool     `json:"replayed"`
}

type ResolveRelationshipRequest = ResolveSubjectRequest
type ResolveRelationshipResult = ResolveSubjectResult

type DeterministicProfileRequest struct {
	AccountID       string
	ClientAccountID string
	RelationshipID  string
}

type SegmentContextRequest struct {
	AccountID       string
	ClientAccountID string
	RelationshipID  string
	AsOf            time.Time
}

type SegmentContext struct {
	RelationshipID string               `json:"relationshipId"`
	AsOf           time.Time            `json:"asOf"`
	Segments       []SegmentContextItem `json:"segments"`
}

type SegmentContextItem struct {
	SegmentID      string    `json:"segmentId"`
	SegmentKey     string    `json:"segmentKey"`
	VersionID      string    `json:"versionId"`
	MaterializedAt time.Time `json:"materializedAt"`
}

type Segment struct {
	ID                       string     `json:"id"`
	ClientAccountID          string     `json:"clientAccountId"`
	SegmentKey               string     `json:"segmentKey"`
	Name                     string     `json:"name"`
	Description              *string    `json:"description,omitempty"`
	Status                   string     `json:"status"`
	ActiveVersionID          *string    `json:"activeVersionId,omitempty"`
	CurrentMaterializationID *string    `json:"currentMaterializationId,omitempty"`
	Revision                 int64      `json:"revision"`
	ArchivedAt               *time.Time `json:"archivedAt,omitempty"`
	CreatedAt                time.Time  `json:"createdAt"`
	UpdatedAt                time.Time  `json:"updatedAt"`
}

type SegmentVersion struct {
	ID                    string          `json:"id"`
	SegmentID             string          `json:"segmentId"`
	VersionNumber         int             `json:"versionNumber"`
	Status                string          `json:"status"`
	FilterSchemaVersion   string          `json:"filterSchemaVersion"`
	FieldCatalogVersion   string          `json:"fieldCatalogVersion"`
	FilterAST             json.RawMessage `json:"filterAst"`
	EvaluationPolicy      json.RawMessage `json:"evaluationPolicy"`
	DefinitionHash        string          `json:"definitionHash"`
	ValidationHash        *string         `json:"validationHash,omitempty"`
	ValidationReasonCodes []string        `json:"validationReasonCodes"`
	Revision              int64           `json:"revision"`
	CreatedAt             time.Time       `json:"createdAt"`
	UpdatedAt             time.Time       `json:"updatedAt"`
	ValidatedAt           *time.Time      `json:"validatedAt,omitempty"`
	PublishedAt           *time.Time      `json:"publishedAt,omitempty"`
}

type SegmentDraftInput struct {
	FilterSchemaVersion string          `json:"filterSchemaVersion"`
	FieldCatalogVersion string          `json:"fieldCatalogVersion"`
	FilterAST           json.RawMessage `json:"filterAst"`
	EvaluationPolicy    json.RawMessage `json:"evaluationPolicy"`
}

type CreateSegmentInput struct {
	ClientAccountID string            `json:"clientAccountId,omitempty"`
	SegmentKey      string            `json:"segmentKey"`
	Name            string            `json:"name"`
	Description     *string           `json:"description,omitempty"`
	Draft           SegmentDraftInput `json:"draft"`
	IdempotencyKey  string            `json:"idempotencyKey"`
}

type CreateSegmentResult struct {
	Segment  Segment        `json:"segment"`
	Version  SegmentVersion `json:"version"`
	Replayed bool           `json:"replayed"`
}

type SegmentPatch struct {
	Name             *string `json:"name,omitempty"`
	Description      *string `json:"description,omitempty"`
	ExpectedRevision int64   `json:"expectedRevision"`
}

type SegmentVersionPatch struct {
	FilterAST        json.RawMessage `json:"filterAst"`
	EvaluationPolicy json.RawMessage `json:"evaluationPolicy"`
	ChangeSummary    string          `json:"changeSummary,omitempty"`
	ExpectedRevision int64           `json:"expectedRevision"`
}

type CreateSegmentVersionInput struct {
	BaseVersionID  *string            `json:"baseVersionId,omitempty"`
	Draft          *SegmentDraftInput `json:"draft,omitempty"`
	ChangeSummary  string             `json:"changeSummary,omitempty"`
	IdempotencyKey string             `json:"idempotencyKey"`
}

type SegmentValidationResult struct {
	VersionID      string   `json:"versionId"`
	Status         string   `json:"status"`
	ValidationHash string   `json:"validationHash"`
	ReasonCodes    []string `json:"reasonCodes"`
	EstimatedCost  int      `json:"estimatedCost"`
	Revision       int64    `json:"revision"`
}

type PublishSegmentVersionInput struct {
	ExpectedRevision int64  `json:"expectedRevision"`
	ValidationHash   string `json:"validationHash"`
	Reason           string `json:"reason"`
	IdempotencyKey   string `json:"idempotencyKey"`
}

type RollbackSegmentInput struct {
	TargetVersionID         string `json:"targetVersionId"`
	ExpectedSegmentRevision int64  `json:"expectedSegmentRevision"`
	Reason                  string `json:"reason"`
	IdempotencyKey          string `json:"idempotencyKey"`
}

type SegmentEvaluationRequest struct {
	VersionID      string     `json:"versionId,omitempty"`
	Mode           string     `json:"mode"`
	AsOf           *time.Time `json:"asOf,omitempty"`
	IdempotencyKey string     `json:"idempotencyKey"`
}

type SegmentEvaluationRun struct {
	ID                  string          `json:"id"`
	SegmentID           string          `json:"segmentId"`
	VersionID           string          `json:"versionId"`
	Mode                string          `json:"mode"`
	Status              string          `json:"status"`
	AsOf                time.Time       `json:"asOf"`
	DefinitionHash      string          `json:"definitionHash"`
	FieldCatalogVersion string          `json:"fieldCatalogVersion"`
	MatchedCount        *int64          `json:"matchedCount,omitempty"`
	ExcludedCount       *int64          `json:"excludedCount,omitempty"`
	ErrorCount          *int64          `json:"errorCount,omitempty"`
	ReasonCodes         json.RawMessage `json:"reasonCodes"`
	RequestedAt         time.Time       `json:"requestedAt"`
	StartedAt           *time.Time      `json:"startedAt,omitempty"`
	FinishedAt          *time.Time      `json:"finishedAt,omitempty"`
	Replayed            bool            `json:"replayed"`
}

type SegmentMaterialization struct {
	ID              string     `json:"id"`
	SegmentID       string     `json:"segmentId"`
	VersionID       string     `json:"versionId"`
	EvaluationRunID string     `json:"evaluationRunId"`
	AsOf            time.Time  `json:"asOf"`
	Status          string     `json:"status"`
	MemberCount     int64      `json:"memberCount"`
	FreshUntil      *time.Time `json:"freshUntil,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
}

type SegmentMember struct {
	RelationshipID string    `json:"relationshipId"`
	SubjectID      string    `json:"subjectId"`
	MatchedAt      time.Time `json:"matchedAt"`
}

type SegmentFilter struct {
	SchemaVersion string     `json:"schemaVersion"`
	Root          FilterNode `json:"root"`
}

type FilterNode struct {
	Type     string          `json:"type"`
	Operator string          `json:"operator,omitempty"`
	Children []FilterNode    `json:"children,omitempty"`
	FieldKey string          `json:"fieldKey,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
}

type SegmentFieldDefinition struct {
	FieldKey       string   `json:"fieldKey"`
	DataType       string   `json:"dataType"`
	Operators      []string `json:"operators"`
	Classification string   `json:"classification"`
	Local          bool     `json:"local"`
}

type SegmentFieldCatalog struct {
	Version string                   `json:"version"`
	Fields  []SegmentFieldDefinition `json:"fields"`
}

type CompiledFilter struct {
	Where string
	Args  []any
	Cost  int
}
