package customerintelligence

import (
	"context"
	"encoding/json"
	"time"
)

type FoundationRepository interface {
	GetCapability(ctx context.Context, scope Scope, key, scopeKey string) (Capability, error)
	ListCapabilities(ctx context.Context, scope Scope) ([]Capability, error)
	UpsertCapability(ctx context.Context, accountID, actorID string, input CapabilityInput) (Capability, error)

	ListSourceConfigs(ctx context.Context, scope Scope) ([]SourceConfig, error)
	GetSourceConfig(ctx context.Context, scope Scope, id string) (SourceConfig, error)
	UpsertSourceConfig(ctx context.Context, accountID, actorID string, input SourceConfigInput) (SourceConfig, error)
	ListRetentionPolicyVersions(ctx context.Context, accountID string) ([]RetentionPolicyVersion, error)
	CreateRetentionPolicyDraft(ctx context.Context, accountID, actorID, policyKey string, input RetentionPolicyDraftInput) (RetentionPolicyVersion, error)
	PublishRetentionPolicyVersion(ctx context.Context, accountID, actorID, id string, input PublishRetentionPolicyInput) (RetentionPolicyVersion, error)
	CreateSourceRun(ctx context.Context, request SourceSyncRequest) (SourceRun, bool, error)
	GetSourceRun(ctx context.Context, scope Scope, runID string) (SourceRun, error)
	ListSourceRuns(ctx context.Context, scope Scope, sourceConfigID string, limit int) ([]SourceRun, error)
	CompleteSourceRun(ctx context.Context, accountID, runID, status string, observed, accepted, rejected int, errorCode string) error
	InsertObservations(ctx context.Context, run SourceRun, observations []Observation) (int, error)

	InsertManualFact(ctx context.Context, accountID, actorID string, input ManualFactInput) (Fact, error)
	ListFacts(ctx context.Context, scope Scope, relationshipID string, limit int) ([]Fact, error)
	LatestSummary(ctx context.Context, scope Scope, relationshipID string) (summaryCiphertext string, summary Summary, err error)
	SaveContextSnapshot(ctx context.Context, envelope ContextEnvelope, ciphertext, hash string) (string, error)

	ListRecommendations(ctx context.Context, scope Scope, relationshipID string, limit int) ([]Recommendation, error)
	ReviewRecommendation(ctx context.Context, scope Scope, actorID, recommendationID string, input RecommendationFeedback) (Recommendation, error)
	ListSourceSuggestions(ctx context.Context, scope Scope, relationshipID string, limit int) ([]SourceSuggestion, error)
	ReviewSourceSuggestion(ctx context.Context, scope Scope, actorID, suggestionID string, input SourceSuggestionFeedback) (SourceSuggestion, error)
	RecordOutcome(ctx context.Context, outcome AcceptedOutcome) (bool, error)
	ListPortfolioOpportunities(ctx context.Context, accountID, targetClientAccountID string, limit int) ([]PortfolioOpportunity, error)
	CreatePortfolioOpportunity(ctx context.Context, accountID, actorID string, opportunity PortfolioOpportunity) (PortfolioOpportunity, error)
	ListAuditEvents(ctx context.Context, scope Scope, limit int) ([]AuditEvent, error)
}

type PromptRepository interface {
	ListProcesses(ctx context.Context) ([]ProcessDefinition, error)
	ListPromptVersions(ctx context.Context, accountID, clientAccountID, processKey string) ([]PromptVersion, error)
	ListPromptBindings(ctx context.Context, accountID, clientAccountID, processKey string) ([]PromptBinding, error)
	CreatePromptDraft(ctx context.Context, accountID, actorID string, input PromptDraftInput, variables []string) (PromptVersion, error)
	GetPromptVersion(ctx context.Context, accountID, id string) (PromptVersion, error)
	UpdatePromptDraft(ctx context.Context, accountID, actorID, id, content string, variables []string, expectedRevision int64) (PromptVersion, error)
	MarkPromptValidated(ctx context.Context, accountID, actorID, id string, variables []string) (PromptVersion, error)
	CreatePromptEvaluation(ctx context.Context, accountID, actorID, promptVersionID, status string, reasonCodes []string, scores json.RawMessage) (PromptEvaluation, error)
	ListPromptEvaluations(ctx context.Context, accountID, promptVersionID string, limit int) ([]PromptEvaluation, error)
	PublishPrompt(ctx context.Context, accountID, actorID, promptVersionID string, input PublishPromptInput) (PromptBinding, error)
	RollbackPrompt(ctx context.Context, accountID, actorID, bindingID string, input RollbackPromptInput) (PromptBinding, error)

	ListModels(ctx context.Context, accountID string) ([]AIModel, error)
	UpsertModel(ctx context.Context, accountID, actorID string, model AIModel) (AIModel, error)
	ListCredentials(ctx context.Context, accountID string) ([]credentialRecord, error)
	UpsertCredential(ctx context.Context, accountID, actorID string, input CredentialInput, ciphertext, last4 string) (credentialRecord, error)
	RevokeCredential(ctx context.Context, accountID, actorID, id string) error
	ListAgents(ctx context.Context, accountID, clientAccountID string) ([]AIAgent, error)
	AgentClientScope(ctx context.Context, accountID, agentID string) (string, error)
	AgentVersionClientScope(ctx context.Context, accountID, versionID string) (string, error)
	CreateAgent(ctx context.Context, accountID, actorID, clientAccountID, slug, name string) (AIAgent, error)
	UpdateAgent(ctx context.Context, accountID, actorID, id string, input AgentPatchInput) (AIAgent, error)
	CreateAgentVersion(ctx context.Context, accountID, actorID, agentID string, input AIAgentVersionInput) (AIAgentVersion, error)
	PublishAgentVersion(ctx context.Context, accountID, actorID, versionID string) (AIAgentVersion, error)
}

type RuntimeRepository interface {
	ResolvePipelineVersion(ctx context.Context, pipelineKey string) (string, error)
	ResolveExecutionPlan(ctx context.Context, scope Scope, processKey string) (ExecutionPlan, error)
	FindRuntimeResult(ctx context.Context, scope Scope, requestID, processKey string) (RuntimeResult, error)
	StartRuntimeRun(ctx context.Context, input RuntimeRunInput) (string, bool, error)
	CompleteRuntimeRun(ctx context.Context, input RuntimeRunCompletion) error
	ListRuntimeRuns(ctx context.Context, scope Scope, limit int) ([]RuntimeRunView, error)
}

type Repository interface {
	FoundationRepository
	PromptRepository
	RuntimeRepository
}

type ProcessDefinition struct {
	ID               string          `json:"id"`
	Key              string          `json:"key"`
	Label            string          `json:"label"`
	Description      string          `json:"description"`
	Status           string          `json:"status"`
	SchemaVersion    string          `json:"schemaVersion"`
	InputSchema      json.RawMessage `json:"inputSchema"`
	OutputSchema     json.RawMessage `json:"outputSchema"`
	AllowedVariables []string        `json:"allowedVariables"`
}

type credentialRecord struct {
	ID         string
	Provider   string
	Label      string
	Ciphertext string
	Last4      string
	Status     string
	UpdatedAt  time.Time
}

type ExecutionPlan struct {
	ProcessDefinitionID     string
	ProcessConfigVersionID  string
	ProcessKey              string
	SchemaVersion           string
	OutputSchema            json.RawMessage
	AllowedVariables        []string
	PromptBindingID         string
	PlatformPromptVersionID string
	AgencyPromptVersionID   string
	ClientPromptVersionID   string
	ProcessPromptVersionID  string
	PlatformPrompt          string
	AgencyPrompt            string
	ClientPrompt            string
	ProcessPrompt           string
	AgentVersionID          string
	ModelID                 string
	Provider                string
	Model                   string
	BaseURL                 string
	CredentialCiphertext    string
	Temperature             float64
	MaxOutputTokens         int
	TimeoutMS               int
	PromptOverride          string
	SourcePolicy            json.RawMessage
	ToolPolicy              json.RawMessage
	KnowledgePolicy         json.RawMessage
	RuntimePolicy           json.RawMessage
}

type RuntimeRunInput struct {
	Request           InteractionRequest
	PipelineVersionID string
	ProcessKey        string
	Plan              ExecutionPlan
	ContextID         string
	InputHash         string
	ExecutionMode     string
}

type RuntimeRunCompletion struct {
	AccountID        string
	RunID            string
	Status           string
	OutputCiphertext string
	OutputHash       string
	WarningCodes     []string
	ErrorCode        string
	Usage            Usage
}

type RuntimeResult struct {
	RunRef           ProcessRunRef
	Status           string
	OutputCiphertext string
	WarningCodes     []string
	Usage            Usage
}
