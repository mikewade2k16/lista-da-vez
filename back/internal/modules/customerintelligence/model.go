package customerintelligence

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type Scope struct {
	AccountID       string `json:"accountId"`
	ClientAccountID string `json:"clientAccountId"`
}

type Capability struct {
	ID              string          `json:"id"`
	AccountID       string          `json:"accountId"`
	ClientAccountID string          `json:"clientAccountId"`
	Key             string          `json:"key"`
	ScopeKey        string          `json:"scopeKey"`
	Mode            string          `json:"mode"`
	Config          json.RawMessage `json:"config"`
	Revision        int64           `json:"revision"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type CapabilityInput struct {
	ClientAccountID  string          `json:"clientAccountId"`
	Key              string          `json:"key"`
	ScopeKey         string          `json:"scopeKey"`
	Mode             string          `json:"mode"`
	Config           json.RawMessage `json:"config"`
	ExpectedRevision int64           `json:"expectedRevision"`
}

type AuditEvent struct {
	ID              string          `json:"id"`
	ClientAccountID string          `json:"clientAccountId,omitempty"`
	ActorUserID     string          `json:"actorUserId,omitempty"`
	EventType       string          `json:"eventType"`
	AggregateType   string          `json:"aggregateType"`
	AggregateID     string          `json:"aggregateId"`
	CorrelationID   string          `json:"correlationId,omitempty"`
	ReasonCode      string          `json:"reasonCode"`
	Metadata        json.RawMessage `json:"metadata"`
	OccurredAt      time.Time       `json:"occurredAt"`
}

type SourceConfig struct {
	ID                       string          `json:"id"`
	AccountID                string          `json:"accountId"`
	ClientAccountID          string          `json:"clientAccountId"`
	SourceKey                string          `json:"sourceKey"`
	ConnectionKey            string          `json:"connectionKey"`
	Status                   string          `json:"status"`
	Mode                     string          `json:"mode"`
	PurposeKey               string          `json:"purposeKey"`
	FieldAllowlist           []string        `json:"fieldAllowlist"`
	FreshnessSeconds         int             `json:"freshnessSeconds"`
	RetentionPolicyKey       string          `json:"retentionPolicyKey"`
	RetentionPolicyVersionID string          `json:"retentionPolicyVersionId"`
	RetentionPolicyVersion   int             `json:"retentionPolicyVersion"`
	SnapshotTTLSeconds       int             `json:"snapshotTtlSeconds"`
	OnExpiry                 string          `json:"onExpiry"`
	Config                   json.RawMessage `json:"config"`
	Revision                 int64           `json:"revision"`
	LastHealthStatus         string          `json:"lastHealthStatus"`
	UpdatedAt                time.Time       `json:"updatedAt"`
}

type SourceConfigInput struct {
	ClientAccountID    string          `json:"clientAccountId"`
	SourceKey          string          `json:"sourceKey"`
	ConnectionKey      string          `json:"connectionKey"`
	Status             string          `json:"status"`
	Mode               string          `json:"mode"`
	PurposeKey         string          `json:"purposeKey"`
	FieldAllowlist     []string        `json:"fieldAllowlist"`
	FreshnessSeconds   int             `json:"freshnessSeconds"`
	RetentionPolicyKey string          `json:"retentionPolicyKey"`
	SnapshotTTLSeconds int             `json:"snapshotTtlSeconds"`
	OnExpiry           string          `json:"onExpiry"`
	Config             json.RawMessage `json:"config"`
	ExpectedRevision   int64           `json:"expectedRevision"`
}

type SourceSyncRequest struct {
	AccountID       string `json:"accountId"`
	ClientAccountID string `json:"clientAccountId"`
	SourceConfigID  string `json:"sourceConfigId"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Trigger         string `json:"trigger"`
	RelationshipID  string `json:"relationshipId,omitempty"`
}

type SourceRun struct {
	ID                       string    `json:"id"`
	AccountID                string    `json:"accountId"`
	ClientAccountID          string    `json:"clientAccountId"`
	SourceConfigID           string    `json:"sourceConfigId"`
	SourceKey                string    `json:"sourceKey"`
	RetentionPolicyVersionID string    `json:"retentionPolicyVersionId"`
	Status                   string    `json:"status"`
	Trigger                  string    `json:"trigger"`
	ObservedCount            int       `json:"observedCount"`
	AcceptedCount            int       `json:"acceptedCount"`
	RejectedCount            int       `json:"rejectedCount"`
	ErrorCode                string    `json:"errorCode,omitempty"`
	CreatedAt                time.Time `json:"createdAt"`
}

const (
	ObservationScopeSubject  = "subject"
	ObservationScopeBusiness = "business"

	ObservationClassificationRelationship    = "customer_relationship"
	ObservationClassificationBusinessContext = "client_business_context"
)

type Observation struct {
	IdempotencyKey     string          `json:"idempotencyKey"`
	EntityType         string          `json:"entityType"`
	EntityID           string          `json:"entityId"`
	Version            string          `json:"version"`
	ScopeType          string          `json:"-"`
	Classification     string          `json:"-"`
	SubjectID          string          `json:"subjectId,omitempty"`
	RelationshipID     string          `json:"relationshipId,omitempty"`
	OccurredAt         *time.Time      `json:"occurredAt,omitempty"`
	Snapshot           json.RawMessage `json:"snapshot"`
	SnapshotCiphertext string          `json:"-"`
	Sensitivity        string          `json:"sensitivity"`
	PurposeKey         string          `json:"purposeKey"`
	ExpiresAt          *time.Time      `json:"expiresAt,omitempty"`
}

type SourceAdapter interface {
	Fetch(ctx context.Context, config SourceConfig, relationshipID string) ([]Observation, error)
}

type Fact struct {
	ID                string          `json:"id"`
	SubjectID         string          `json:"subjectId"`
	RelationshipID    string          `json:"relationshipId"`
	Key               string          `json:"key"`
	Version           int             `json:"version"`
	Value             json.RawMessage `json:"value"`
	ValueCiphertext   string          `json:"-"`
	ValueType         string          `json:"valueType"`
	Confidence        float64         `json:"confidence"`
	VerificationState string          `json:"verificationState"`
	Sensitivity       string          `json:"sensitivity"`
	ValidFrom         *time.Time      `json:"validFrom,omitempty"`
	ValidUntil        *time.Time      `json:"validUntil,omitempty"`
	UpdatedAt         time.Time       `json:"updatedAt"`
	Evidence          []EvidenceRef   `json:"evidence"`
}

type EvidenceRef struct {
	ObservationID string `json:"observationId"`
	SourceKey     string `json:"sourceKey"`
	Locator       string `json:"locator"`
}

type ManualFactInput struct {
	ClientAccountID    string          `json:"clientAccountId"`
	SubjectID          string          `json:"subjectId"`
	RelationshipID     string          `json:"relationshipId"`
	FactKey            string          `json:"factKey"`
	Value              json.RawMessage `json:"value"`
	ValueCiphertext    string          `json:"-"`
	ValueType          string          `json:"valueType"`
	Sensitivity        string          `json:"sensitivity"`
	ValidFrom          *time.Time      `json:"validFrom,omitempty"`
	ValidUntil         *time.Time      `json:"validUntil,omitempty"`
	EvidenceNote       string          `json:"evidenceNote"`
	IdempotencyKey     string          `json:"idempotencyKey"`
	EvidenceCiphertext string          `json:"-"`
}

type Summary struct {
	ID             string          `json:"id"`
	RelationshipID string          `json:"relationshipId"`
	SummaryType    string          `json:"summaryType"`
	Body           json.RawMessage `json:"body"`
	GeneratedAt    time.Time       `json:"generatedAt"`
}

// RelationshipProfileView is the safe UI projection. The private LLM context
// envelope is deliberately not part of this HTTP contract: source observations
// are inspected only through the masked and audited observation endpoints.
type RelationshipProfileView struct {
	ClientAccountID string   `json:"clientAccountId"`
	RelationshipID  string   `json:"relationshipId"`
	Facts           []Fact   `json:"facts"`
	Summary         *Summary `json:"summary,omitempty"`
	Warnings        []string `json:"warnings"`
}

type ContextRequest struct {
	AccountID       string   `json:"accountId"`
	ClientAccountID string   `json:"clientAccountId"`
	SubjectID       string   `json:"subjectId,omitempty"`
	RelationshipID  string   `json:"relationshipId"`
	ProcessKeys     []string `json:"processKeys"`
	Purpose         string   `json:"purpose"`
	SourceKeys      []string `json:"sourceKeys,omitempty"`
	MaxItems        int      `json:"maxItems,omitempty"`
	MaxTokens       int      `json:"maxTokens,omitempty"`
}

type ContextEnvelope struct {
	SchemaVersion   string               `json:"schemaVersion"`
	SnapshotID      string               `json:"snapshotId,omitempty"`
	AccountID       string               `json:"accountId"`
	ClientAccountID string               `json:"clientAccountId"`
	SubjectID       string               `json:"subjectId,omitempty"`
	RelationshipID  string               `json:"relationshipId"`
	ProcessKeys     []string             `json:"processKeys"`
	Purpose         string               `json:"purpose"`
	AsOf            time.Time            `json:"asOf"`
	ExpiresAt       time.Time            `json:"expiresAt"`
	Facts           []Fact               `json:"facts"`
	Observations    []ContextObservation `json:"observations"`
	Summary         *Summary             `json:"summary,omitempty"`
	Provenance      []EvidenceRef        `json:"provenance"`
	Warnings        []string             `json:"warnings"`
	Budget          ContextBudget        `json:"budget"`
	Metadata        json.RawMessage      `json:"metadata"`
}

type ContextBudget struct {
	MaxItems        int `json:"maxItems"`
	IncludedItems   int `json:"includedItems"`
	MaxTokens       int `json:"maxTokens"`
	EstimatedTokens int `json:"estimatedTokens"`
}

type InteractionRequest struct {
	SchemaVersion       string          `json:"schemaVersion"`
	RequestID           string          `json:"requestId"`
	InteractionID       string          `json:"interactionId,omitempty"`
	AccountID           string          `json:"accountId"`
	ClientAccountID     string          `json:"clientAccountId"`
	SubjectID           string          `json:"subjectId,omitempty"`
	RelationshipID      string          `json:"relationshipId"`
	ConversationID      string          `json:"conversationId,omitempty"`
	PipelineKey         string          `json:"pipelineKey,omitempty"`
	AIGeneration        int64           `json:"aiGeneration"`
	Message             json.RawMessage `json:"message"`
	OperationalState    json.RawMessage `json:"operationalState,omitempty"`
	RoutingCatalog      json.RawMessage `json:"routingCatalog,omitempty"`
	ChannelCapabilities json.RawMessage `json:"channelCapabilities,omitempty"`
	Purpose             string          `json:"purpose"`
	Locale              string          `json:"locale,omitempty"`
	Channel             string          `json:"channel,omitempty"`
	AsOf                time.Time       `json:"asOf,omitempty"`
	DeadlineAt          time.Time       `json:"deadlineAt,omitempty"`
	SourceKeys          []string        `json:"sourceKeys,omitempty"`
	MaxItems            int             `json:"maxItems,omitempty"`
	MaxTokens           int             `json:"maxTokens,omitempty"`
	CorrelationID       string          `json:"correlationId,omitempty"`
}

type ProcessRunRef struct {
	ProcessKey              string `json:"processKey"`
	RunID                   string `json:"runId"`
	Status                  string `json:"status"`
	ExecutionMode           string `json:"executionMode"`
	ProcessDefinitionID     string `json:"processDefinitionId"`
	ProcessConfigVersionID  string `json:"processConfigVersionId"`
	PromptBindingID         string `json:"promptBindingId"`
	PlatformPromptVersionID string `json:"platformPromptVersionId"`
	AgencyPromptVersionID   string `json:"agencyPromptVersionId,omitempty"`
	ClientPromptVersionID   string `json:"clientPromptVersionId,omitempty"`
	ProcessPromptVersionID  string `json:"processPromptVersionId"`
	AgentVersionID          string `json:"agentVersionId"`
	ModelID                 string `json:"modelId"`
	ContextSnapshotID       string `json:"contextSnapshotId"`
	OutputSchemaVersion     string `json:"outputSchemaVersion"`
}

type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
	LatencyMs        int `json:"latencyMs"`
}

type RuntimeRunView struct {
	ID                  string     `json:"id"`
	RequestID           string     `json:"requestId"`
	ClientAccountID     string     `json:"clientAccountId"`
	ProcessKey          string     `json:"processKey"`
	PromptBindingID     string     `json:"promptBindingId"`
	AgentVersionID      string     `json:"agentVersionId"`
	ModelID             string     `json:"modelId"`
	OutputSchemaVersion string     `json:"outputSchemaVersion"`
	Status              string     `json:"status"`
	ErrorCode           string     `json:"errorCode,omitempty"`
	Usage               Usage      `json:"usage"`
	CreatedAt           time.Time  `json:"createdAt"`
	CompletedAt         *time.Time `json:"completedAt,omitempty"`
}

type InteractionDecision struct {
	SchemaVersion     string          `json:"schemaVersion"`
	RequestID         string          `json:"requestId"`
	DecisionID        string          `json:"decisionId,omitempty"`
	InteractionID     string          `json:"interactionId"`
	AccountID         string          `json:"accountId"`
	ClientAccountID   string          `json:"clientAccountId"`
	SubjectID         string          `json:"subjectId"`
	RelationshipID    string          `json:"relationshipId"`
	ConversationID    string          `json:"conversationId,omitempty"`
	PipelineKey       string          `json:"pipelineKey"`
	PipelineVersionID string          `json:"pipelineVersionId"`
	ProcessRuns       []ProcessRunRef `json:"processRunRefs"`
	AIGeneration      int64           `json:"aiGeneration"`
	Outcome           string          `json:"outcome"`
	ReplyDraft        *string         `json:"replyDraft,omitempty"`
	NeedsHuman        bool            `json:"needsHuman"`
	ReasonCode        string          `json:"reasonCode"`
	DepartmentID      *string         `json:"departmentId,omitempty"`
	QueueID           *string         `json:"queueId,omitempty"`
	Intent            string          `json:"intent,omitempty"`
	Categories        []string        `json:"categories"`
	LeadStage         string          `json:"leadStage,omitempty"`
	Confidence        float64         `json:"confidence"`
	ExtractedClaims   json.RawMessage `json:"extractedClaims,omitempty"`
	ToolResults       json.RawMessage `json:"toolResults,omitempty"`
	Closure           json.RawMessage `json:"closure,omitempty"`
	Usage             Usage           `json:"usage"`
	Warnings          []string        `json:"warnings"`
}

type AcceptedOutcome struct {
	AccountID       string             `json:"accountId"`
	ClientAccountID string             `json:"clientAccountId"`
	EventID         string             `json:"eventId"`
	InteractionID   string             `json:"interactionId,omitempty"`
	DecisionID      string             `json:"decisionId,omitempty"`
	SubjectID       string             `json:"subjectId,omitempty"`
	RelationshipID  string             `json:"relationshipId,omitempty"`
	ConversationID  string             `json:"conversationId,omitempty"`
	OutcomeType     string             `json:"outcomeType"`
	Accepted        bool               `json:"accepted"`
	ActorType       string             `json:"actorType"`
	ActorID         string             `json:"actorId,omitempty"`
	ProcessRuns     []ProcessRunRef    `json:"processRunRefs,omitempty"`
	Claims          []AcceptedClaimRef `json:"claims,omitempty"`
	Payload         json.RawMessage    `json:"payload"`
	OccurredAt      time.Time          `json:"occurredAt"`
}

type Recommendation struct {
	ID                string          `json:"id"`
	ClientAccountID   string          `json:"clientAccountId"`
	RelationshipID    string          `json:"relationshipId"`
	Type              string          `json:"type"`
	Status            string          `json:"status"`
	Payload           json.RawMessage `json:"payload"`
	PayloadCiphertext string          `json:"-"`
	Confidence        float64         `json:"confidence"`
	ReasonCodes       []string        `json:"reasonCodes"`
	ValidUntil        *time.Time      `json:"validUntil,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
}

type RecommendationFeedback struct {
	Status   string          `json:"status"`
	Reason   string          `json:"reason"`
	Metadata json.RawMessage `json:"metadata"`
}

type SourceSuggestion struct {
	ID                  string     `json:"id"`
	ClientAccountID     string     `json:"clientAccountId"`
	RelationshipID      string     `json:"relationshipId"`
	SourceKey           string     `json:"sourceKey"`
	GapCodes            []string   `json:"gapCodes"`
	RationaleCode       string     `json:"rationaleCode"`
	Rationale           string     `json:"rationale"`
	Confidence          float64    `json:"confidence"`
	Status              string     `json:"status"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	RationaleCiphertext string     `json:"-"`
}

type SourceSuggestionFeedback struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type PortfolioOpportunity struct {
	ID                    string          `json:"id"`
	TargetClientAccountID string          `json:"targetClientAccountId"`
	OrganizationID        string          `json:"organizationId,omitempty"`
	SegmentKey            string          `json:"segmentKey"`
	CohortClass           string          `json:"cohortClass"`
	OpportunityType       string          `json:"opportunityType"`
	RationaleCode         string          `json:"rationaleCode"`
	CohortSize            int             `json:"-"`
	SuppressionThreshold  int             `json:"-"`
	Aggregate             json.RawMessage `json:"-"`
	SuppressionPolicy     json.RawMessage `json:"suppressionPolicy"`
	Confidence            float64         `json:"confidence"`
	Status                string          `json:"status"`
	ExpiresAt             *time.Time      `json:"expiresAt,omitempty"`
	CreatedAt             time.Time       `json:"createdAt"`
}

type PromptVersion struct {
	ID              string          `json:"id"`
	AccountID       string          `json:"accountId"`
	ClientAccountID string          `json:"clientAccountId,omitempty"`
	ProcessKey      string          `json:"processKey"`
	Layer           string          `json:"layer"`
	Version         int             `json:"version"`
	Status          string          `json:"status"`
	Content         string          `json:"content,omitempty"`
	Variables       []string        `json:"variables"`
	OutputSchema    json.RawMessage `json:"outputSchema"`
	Revision        int64           `json:"revision"`
	CreatedAt       time.Time       `json:"createdAt"`
	ValidatedAt     *time.Time      `json:"validatedAt,omitempty"`
	PublishedAt     *time.Time      `json:"publishedAt,omitempty"`
}

type PromptDraftInput struct {
	ClientAccountID  string          `json:"clientAccountId,omitempty"`
	ProcessKey       string          `json:"processKey"`
	Layer            string          `json:"layer"`
	Content          string          `json:"content"`
	OutputSchema     json.RawMessage `json:"outputSchema"`
	BasedOnVersionID string          `json:"basedOnVersionId,omitempty"`
}

type PromptValidation struct {
	Valid       bool     `json:"valid"`
	Variables   []string `json:"variables"`
	ReasonCodes []string `json:"reasonCodes"`
}

type PromptEvaluation struct {
	ID          string          `json:"id"`
	Status      string          `json:"status"`
	Scores      json.RawMessage `json:"scores"`
	ReasonCodes []string        `json:"reasonCodes"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type PublishPromptInput struct {
	ClientAccountID string          `json:"clientAccountId,omitempty"`
	AgentVersionID  string          `json:"agentVersionId"`
	SourcePolicy    json.RawMessage `json:"sourcePolicy"`
	ToolPolicy      json.RawMessage `json:"toolPolicy"`
	KnowledgePolicy json.RawMessage `json:"knowledgePolicy"`
	RuntimePolicy   json.RawMessage `json:"runtimePolicy"`
}

type RollbackPromptInput struct {
	TargetPromptVersionID string `json:"targetPromptVersionId"`
	ReasonCode            string `json:"reasonCode"`
}

type PromptBinding struct {
	ID                     string    `json:"id"`
	AccountID              string    `json:"accountId"`
	ClientAccountID        string    `json:"clientAccountId,omitempty"`
	ProcessKey             string    `json:"processKey"`
	ProcessPromptVersionID string    `json:"processPromptVersionId"`
	AgentVersionID         string    `json:"agentVersionId"`
	Status                 string    `json:"status"`
	Revision               int64     `json:"revision"`
	PublishedAt            time.Time `json:"publishedAt"`
}

type AIModel struct {
	ID       string          `json:"id"`
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
	BaseURL  string          `json:"baseUrl"`
	Status   string          `json:"status"`
	Config   json.RawMessage `json:"config"`
	Revision int64           `json:"revision"`
}

type AICredential struct {
	ID        string           `json:"id"`
	Provider  string           `json:"provider"`
	Label     string           `json:"label"`
	Status    secretbox.Status `json:"secret"`
	UpdatedAt time.Time        `json:"updatedAt"`
}

type CredentialInput struct {
	Provider string `json:"provider"`
	Label    string `json:"label"`
	APIKey   string `json:"apiKey"`
}

type AIAgent struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Purpose         string    `json:"purpose"`
	Status          string    `json:"status"`
	ActiveVersionID string    `json:"activeVersionId,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Revision        int64     `json:"revision"`
}

type AgentPatchInput struct {
	Name             string `json:"name"`
	Enabled          *bool  `json:"enabled"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type AIAgentVersion struct {
	ID              string          `json:"id"`
	AgentID         string          `json:"agentId"`
	Version         int             `json:"version"`
	Status          string          `json:"status"`
	ModelID         string          `json:"modelId"`
	CredentialID    string          `json:"credentialId"`
	Temperature     float64         `json:"temperature"`
	MaxOutputTokens int             `json:"maxOutputTokens"`
	TimeoutMS       int             `json:"timeoutMs"`
	PromptOverride  string          `json:"promptOverride,omitempty"`
	Config          json.RawMessage `json:"config"`
}

type AIAgentVersionInput struct {
	ModelID         string          `json:"modelId"`
	CredentialID    string          `json:"credentialId"`
	Temperature     float64         `json:"temperature"`
	MaxOutputTokens int             `json:"maxOutputTokens"`
	TimeoutMS       int             `json:"timeoutMs"`
	PromptOverride  string          `json:"promptOverride"`
	Config          json.RawMessage `json:"config"`
}

type PromptPatchInput struct {
	Content          string          `json:"content"`
	OutputSchema     json.RawMessage `json:"outputSchema,omitempty"`
	ExpectedRevision int64           `json:"expectedRevision"`
}
