package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type ActionKind string

const (
	ActionCreateCampaign       ActionKind = "create_campaign"
	ActionDuplicateCampaign    ActionKind = "duplicate_campaign"
	ActionUpdateCampaign       ActionKind = "update_campaign"
	ActionPauseCampaign        ActionKind = "pause_campaign"
	ActionResumeCampaign       ActionKind = "resume_campaign"
	ActionPromoteInstagramPost ActionKind = "promote_instagram_post"
)

type ActionStatus string

const (
	ActionPending   ActionStatus = "pending"
	ActionExecuting ActionStatus = "executing"
	ActionSucceeded ActionStatus = "succeeded"
	ActionFailed    ActionStatus = "failed"
	ActionUnknown   ActionStatus = "unknown"
	ActionCancelled ActionStatus = "cancelled"
	ActionExpired   ActionStatus = "expired"
)

type ActionProposalSource string

const (
	ActionSourceAssistant ActionProposalSource = "assistant"
	ActionSourceManual    ActionProposalSource = "manual"
)

var (
	ErrActionValidation          = errors.New("meta_ads: proposta de acao invalida")
	ErrActionPolicyRequired      = errors.New("meta_ads: politica de acoes nao configurada")
	ErrActionPolicyDenied        = errors.New("meta_ads: acao bloqueada pela politica")
	ErrActionBudgetCapExceeded   = errors.New("meta_ads: orcamento excede o teto configurado")
	ErrActionBudgetUnavailable   = errors.New("meta_ads: orcamento atual indisponivel para validacao")
	ErrActionReinforcedConfirm   = errors.New("meta_ads: acao financeira exige confirmacao reforcada")
	ErrActionSourceUnbound       = errors.New("meta_ads: proposta do assistente sem card vinculado")
	ErrActionProposalStale       = errors.New("meta_ads: proposta desatualizada")
	ErrActionCurrencyUnsupported = errors.New("meta_ads: moeda sem conversor de minor units suportado")
	ErrActionNotCancellable      = errors.New("meta_ads: proposta nao pode mais ser cancelada")
	ErrActionExpired             = errors.New("meta_ads: proposta expirada")
	ErrActionIdempotencyConflict = errors.New("meta_ads: chave idempotente reutilizada com outro payload")
	ErrMetaWritesDisabled        = errors.New("meta_ads: escritas Meta desabilitadas")
	ErrMetaActionUnavailable     = errors.New("meta_ads: executor nao suporta esta acao")
	ErrActionStepUncertain       = errors.New("meta_ads: etapa externa ja iniciada ou inconclusiva")
)

// ActionPolicy e a autorizacao financeira explicita da conta dona da ad account.
// Ausencia de linha fecha create/duplicate/resume e qualquer alteracao de budget.
type ActionPolicy struct {
	ID                string
	AccountID         string
	AdAccountID       string
	Currency          string
	MaxDailyBudget    *float64
	MaxLifetimeBudget *float64
	AllowCreate       bool
	AllowDuplicate    bool
	AllowResume       bool
	UpdatedByUserID   *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ActionPolicyInput struct {
	MaxDailyBudget    *float64 `json:"maxDailyBudget"`
	MaxLifetimeBudget *float64 `json:"maxLifetimeBudget"`
	AllowCreate       bool     `json:"allowCreate"`
	AllowDuplicate    bool     `json:"allowDuplicate"`
	AllowResume       bool     `json:"allowResume"`
}

type ActionPolicyView struct {
	Configured        bool     `json:"configured"`
	AdAccountID       string   `json:"adAccountId"`
	Currency          string   `json:"currency"`
	MaxDailyBudget    *float64 `json:"maxDailyBudget,omitempty"`
	MaxLifetimeBudget *float64 `json:"maxLifetimeBudget,omitempty"`
	AllowCreate       bool     `json:"allowCreate"`
	AllowDuplicate    bool     `json:"allowDuplicate"`
	AllowResume       bool     `json:"allowResume"`
	UpdatedAt         *string  `json:"updatedAt,omitempty"`
}

// ActionProposal e o comando canonico persistido antes de qualquer efeito na Graph.
// Payload ja passou pelo decoder fechado e nao contem token nem resposta bruta.
type ActionProposal struct {
	ID                           string
	AccountID                    string
	ResourceAccountID            string
	AdAccountID                  string
	MetaAdAccountID              string
	AdAccountName                string
	Currency                     string
	Action                       ActionKind
	Source                       ActionProposalSource
	SourceConversationID         *string
	SourceMessageID              *string
	SourceBound                  bool
	TargetCampaignID             *string
	TargetMetaCampaignID         string
	Payload                      json.RawMessage
	Summary                      string
	RequestHash                  string
	IdempotencyKey               string
	ConfirmationIdempotencyKey   *string
	CancellationIdempotencyKey   *string
	GuardSnapshotVersion         int
	GuardSnapshotHash            string
	ConnectionIDSnapshot         *string
	ConnectionRevisionSnapshot   *string
	AdAccountClientIDSnapshot    *string
	AdAccountUpdatedAtSnapshot   *time.Time
	AdAccountHashSnapshot        string
	PolicyConfiguredSnapshot     bool
	PolicyIDSnapshot             *string
	PolicyUpdatedAtSnapshot      *time.Time
	PolicyHashSnapshot           string
	PolicyCurrencySnapshot       string
	PolicyMaxDailySnapshot       *float64
	PolicyMaxLifetimeSnapshot    *float64
	PolicyAllowCreateSnapshot    bool
	PolicyAllowDuplicateSnapshot bool
	PolicyAllowResumeSnapshot    bool
	CampaignSyncedAtSnapshot     *time.Time
	CampaignHashSnapshot         string
	CampaignNameSnapshot         string
	CampaignStatusSnapshot       string
	CampaignDailySnapshot        *float64
	CampaignLifetimeSnapshot     *float64
	ClaimedConnectionID          *string
	ClaimedConnectionRevision    *string
	Status                       ActionStatus
	AttemptCount                 int
	ExternalEntityID             string
	ResultSnapshot               json.RawMessage
	ErrorCode                    string
	ErrorMessage                 string
	CreatedByUserID              *string
	ConfirmedByUserID            *string
	ConfirmedAt                  *time.Time
	ExecutionStartedAt           *time.Time
	CompletedAt                  *time.Time
	ReconciledAt                 *time.Time
	CreatedAt                    time.Time
	ExpiresAt                    time.Time
	UpdatedAt                    time.Time
}

type ActionProposalView struct {
	ID                           string               `json:"id"`
	Action                       ActionKind           `json:"action"`
	Source                       ActionProposalSource `json:"source"`
	AdAccountID                  string               `json:"adAccountId"`
	MetaAdAccountID              string               `json:"metaAdAccountId"`
	AdAccountName                string               `json:"adAccountName"`
	Currency                     string               `json:"currency"`
	TargetCampaignID             *string              `json:"targetCampaignId,omitempty"`
	TargetMetaCampaignID         string               `json:"targetMetaCampaignId,omitempty"`
	Payload                      json.RawMessage      `json:"payload"`
	Summary                      string               `json:"summary"`
	Status                       ActionStatus         `json:"status"`
	IdempotencyKey               string               `json:"idempotencyKey"`
	ConfirmationIdempotencyKey   *string              `json:"confirmationIdempotencyKey,omitempty"`
	CancellationIdempotencyKey   *string              `json:"cancellationIdempotencyKey,omitempty"`
	ExecutionAvailable           bool                 `json:"executionAvailable"`
	CanConfirm                   bool                 `json:"canConfirm"`
	RequiresSpendAcknowledgement bool                 `json:"requiresSpendAcknowledgement"`
	ExternalEntityID             string               `json:"externalEntityId,omitempty"`
	Result                       json.RawMessage      `json:"result"`
	ErrorCode                    string               `json:"errorCode,omitempty"`
	ErrorMessage                 string               `json:"errorMessage,omitempty"`
	ConfirmedAt                  *time.Time           `json:"confirmedAt,omitempty"`
	ExecutionStartedAt           *time.Time           `json:"executionStartedAt,omitempty"`
	CompletedAt                  *time.Time           `json:"completedAt,omitempty"`
	ReconciledAt                 *time.Time           `json:"reconciledAt,omitempty"`
	CreatedAt                    time.Time            `json:"createdAt"`
	ExpiresAt                    time.Time            `json:"expiresAt"`
	UpdatedAt                    time.Time            `json:"updatedAt"`
}

// ActionProposalInput e o shape fechado compartilhado por criacao manual e pelo
// adaptador interno do Assistente 360. Account/client nunca entram neste DTO.
type ActionProposalInput struct {
	Action      ActionKind      `json:"action"`
	AdAccountID string          `json:"adAccountId"`
	Payload     json.RawMessage `json:"payload"`
}

// ActionConfirmationInput torna explicito o acknowledgement adicional exigido
// por acoes que retomam ou alteram gasto. A chave idempotente continua no header HTTP.
type ActionConfirmationInput struct {
	AcknowledgeSpend bool `json:"acknowledgeSpend"`
}

// AssistantActionProposalInput nao e um body HTTP. Calendar chama este contrato
// depois de validar a conversa/mensagem e entrega a allowlist de ad accounts do
// contexto autoritativo; a chave idempotente nasce do messageID+indice no Go.
type AssistantActionProposalInput struct {
	ConversationID      string
	MessageID           string
	ProposalIndex       int
	AllowedAdAccountIDs []string
	Action              ActionKind
	AdAccountID         string
	Payload             json.RawMessage
}

type ActionExecutionOutcome struct {
	Status           ActionStatus
	ExternalEntityID string
	Result           json.RawMessage
	ErrorCode        string
	ErrorMessage     string
}

// ActionExecutorError classifica o efeito externo. Ambiguous=true significa
// que a Graph pode ter aplicado a mutacao apesar do erro de transporte; o
// service persiste `unknown` e nunca repete automaticamente.
type ActionExecutorError struct {
	Code             string
	Message          string
	Ambiguous        bool
	ExternalEntityID string
	Result           json.RawMessage
}

func (e *ActionExecutorError) Error() string {
	if e == nil || e.Code == "" {
		return "meta_ads: falha no executor"
	}
	return "meta_ads: falha no executor: " + e.Code
}

// ActionExecutor e deliberadamente separado do runner/MCP. Implementacoes so
// recebem uma proposta canonica ja confirmada e devem ser at-most-once.
type ActionExecutor interface {
	Supports(action ActionKind) bool
	Execute(ctx context.Context, proposal ActionProposal) (ActionExecutionOutcome, error)
	Reconcile(ctx context.Context, proposal ActionProposal) (ActionExecutionOutcome, error)
}
