package omnichannel

import (
	"context"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const channelBindingManagePermission = "omnichannel.instances.manage"

type ChannelClientResourceView struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ChannelClientBindingView struct {
	ID                string                    `json:"id"`
	ClientAccountID   string                    `json:"clientAccountId"`
	Channel           string                    `json:"channel"`
	ChannelResource   ChannelClientResourceView `json:"channelResource"`
	EffectiveFrom     time.Time                 `json:"effectiveFrom"`
	EffectiveTo       *time.Time                `json:"effectiveTo"`
	Source            string                    `json:"source"`
	Reason            string                    `json:"reason"`
	Revision          int64                     `json:"revision"`
	CreatedAt         time.Time                 `json:"createdAt"`
	UpdatedAt         time.Time                 `json:"updatedAt"`
	ClientAccountName string                    `json:"clientAccountName,omitempty"`
}

type ChannelClientBindingPage struct {
	Items      []ChannelClientBindingView `json:"items"`
	HasMore    bool                       `json:"hasMore"`
	NextCursor string                     `json:"nextCursor,omitempty"`
}

type ChannelClientBindingFilter struct {
	ClientAccountID string
	Channel         string
	State           string
	Cursor          string
	Limit           int
}

type CreateChannelClientBindingInput struct {
	ClientAccountID   string     `json:"clientAccountId"`
	Channel           string     `json:"channel"`
	ChannelResourceID string     `json:"channelResourceId"`
	EffectiveFrom     *time.Time `json:"effectiveFrom"`
	Reason            string     `json:"reason"`
	IdempotencyKey    string     `json:"idempotencyKey"`
}

type ReassignChannelClientBindingInput struct {
	TargetClientAccountID string    `json:"targetClientAccountId"`
	EffectiveAt           time.Time `json:"effectiveAt"`
	Reason                string    `json:"reason"`
	ExpectedRevision      int64     `json:"expectedRevision"`
	IdempotencyKey        string    `json:"idempotencyKey"`
}

type EndChannelClientBindingInput struct {
	EffectiveAt      time.Time `json:"effectiveAt"`
	Reason           string    `json:"reason"`
	ExpectedRevision int64     `json:"expectedRevision"`
	IdempotencyKey   string    `json:"idempotencyKey"`
}

type ChannelClientBindingExceptionView struct {
	Channel            string `json:"channel"`
	ChannelResourceID  string `json:"channelResourceId,omitempty"`
	BindingState       string `json:"bindingState"`
	ReasonCode         string `json:"reasonCode"`
	ConversationCount  int64  `json:"conversationCount"`
	TouchpointCount    int64  `json:"touchpointCount"`
	LatestConversation string `json:"latestConversationAt,omitempty"`
}

type ChannelClientBindingExceptionsResponse struct {
	Items []ChannelClientBindingExceptionView `json:"items"`
}

type ChannelClientBindingPolicyView struct {
	ChannelBindingMode                string    `json:"channelBindingMode"`
	CustomerIntelligenceMode          string    `json:"customerIntelligenceMode"`
	CustomerIntelligenceFailurePolicy string    `json:"customerIntelligenceFailurePolicy"`
	Revision                          int64     `json:"revision"`
	UpdatedAt                         time.Time `json:"updatedAt"`
}

type ChannelClientBindingPolicyInput struct {
	ChannelBindingMode                string `json:"channelBindingMode"`
	CustomerIntelligenceMode          string `json:"customerIntelligenceMode"`
	CustomerIntelligenceFailurePolicy string `json:"customerIntelligenceFailurePolicy"`
	ExpectedRevision                  int64  `json:"expectedRevision"`
}

type ResolveChannelClientBindingExceptionInput struct {
	ClientAccountID   string    `json:"clientAccountId"`
	Channel           string    `json:"channel"`
	ChannelResourceID string    `json:"channelResourceId"`
	EffectiveAt       time.Time `json:"effectiveAt"`
	Reason            string    `json:"reason"`
	IdempotencyKey    string    `json:"idempotencyKey"`
}

type ChannelClientBindingRepairPreviewInput struct {
	BindingID                string    `json:"bindingId"`
	Watermark                time.Time `json:"watermark"`
	Reason                   string    `json:"reason"`
	IdempotencyKey           string    `json:"idempotencyKey"`
	IncludeClosed            bool      `json:"includeClosed"`
	ConfirmNoRetroactiveMove bool      `json:"confirmNoRetroactiveMove"`
}

type ChannelClientBindingRepairApplyInput struct {
	PreviewID       string `json:"previewId"`
	PreviewChecksum string `json:"previewChecksum"`
	Reason          string `json:"reason"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Confirm         bool   `json:"confirm"`
}

type ChannelClientBindingRepairJobView struct {
	ID                string     `json:"id"`
	Channel           string     `json:"channel"`
	ChannelResourceID string     `json:"channelResourceId"`
	ClientAccountID   string     `json:"clientAccountId"`
	BindingID         string     `json:"bindingId"`
	Mode              string     `json:"mode"`
	Status            string     `json:"status"`
	Watermark         time.Time  `json:"watermark"`
	PreviewJobID      *string    `json:"previewJobId,omitempty"`
	PreviewChecksum   string     `json:"previewChecksum"`
	ScannedCount      int64      `json:"scannedCount"`
	EligibleCount     int64      `json:"eligibleCount"`
	RepairedCount     int64      `json:"repairedCount"`
	QuarantinedCount  int64      `json:"quarantinedCount"`
	SkippedCount      int64      `json:"skippedCount"`
	LastErrorCode     string     `json:"lastErrorCode,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	StartedAt         *time.Time `json:"startedAt,omitempty"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type channelClientBindingWrite struct {
	ClientAccountID string
	Channel         string
	ResourceID      string
	EffectiveFrom   time.Time
	Reason          string
	IdempotencyKey  string
	RequestHash     string
	ActorUserID     string
	Source          string
}

type channelClientBindingRepository interface {
	ListChannelClientBindings(context.Context, string, ChannelClientBindingFilter) ([]ChannelClientBindingView, error)
	GetChannelClientBinding(context.Context, string, string) (ChannelClientBindingView, error)
	ChannelBindingClientEligible(context.Context, string, string) (bool, error)
	ChannelBindingResourceExists(context.Context, string, string, string) (exists bool, active bool, err error)
	CreateChannelClientBinding(context.Context, string, channelClientBindingWrite) (string, error)
	ReassignChannelClientBinding(context.Context, string, string, string, ReassignChannelClientBindingInput, string) (string, error)
	EndChannelClientBinding(context.Context, string, string, string, EndChannelClientBindingInput, string) (string, error)
	ListChannelClientBindingExceptions(context.Context, string) ([]ChannelClientBindingExceptionView, error)
	GetChannelClientBindingPolicy(context.Context, string) (ChannelClientBindingPolicyView, error)
	UpdateChannelClientBindingPolicy(context.Context, string, ChannelClientBindingPolicyInput) (ChannelClientBindingPolicyView, error)
	CreateChannelClientBindingRepairPreview(context.Context, string, auth.Principal, ChannelClientBindingRepairPreviewInput, string) (ChannelClientBindingRepairJobView, error)
	ApplyChannelClientBindingRepair(context.Context, string, auth.Principal, ChannelClientBindingRepairApplyInput, string) (ChannelClientBindingRepairJobView, error)
	GetChannelClientBindingRepairJob(context.Context, string, string) (ChannelClientBindingRepairJobView, error)
}

type channelBindingSnapshot struct {
	ClientAccountID string
	BindingID       string
	State           string
	BoundAt         *time.Time
}
