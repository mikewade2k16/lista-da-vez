package alerts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
)

const (
	CategoryOperational = "operational"
	CategoryPositive    = "positive"

	SeverityInfo      = "info"
	SeverityAttention = "attention"
	SeverityCritical  = "critical"

	StatusActive        = "active"
	StatusAcknowledged  = "acknowledged"
	StatusResolved      = "resolved"
	StatusClosedByAdmin = "closed_by_admin"

	ActionTriggered    = "triggered"
	ActionAcknowledged = "acknowledged"
	ActionResolved     = "resolved"
	ActionAutoResolved = "auto_resolved"
	ActionRulesUpdated = "rules_updated"

	SignalLongOpenServiceTriggered = "long_open_service.triggered"
	SignalLongOpenServiceResolved  = "long_open_service.resolved"

	SourceModuleOperations = "operations"
	RulesSourceDefaults    = "module-defaults"
	RulesSourceDatabase    = "tenant-operational-alert-rules"

	TypeLongOpenService             = "long_open_service"
	TypeIdleStoreDuringBusiness     = "idle_store_during_business_hours"
	TypeServiceOutsideBusiness      = "service_outside_business_hours"
	TypeStoreOpenWithoutUsage       = "store_open_without_operation_usage"
	defaultLongOpenMinutes          = 25
	defaultIdleStoreMinutes         = 20
	defaultAfterClosingGraceMinutes = 15

	InteractionKindNone             = "none"
	InteractionKindReminder         = "reminder"
	InteractionKindResponseRequired = "response_required"

	InteractionResponseStillHappening = "still_happening"
	InteractionResponseForgotten      = "forgotten"

	// Trigger types for rule definitions
	TriggerLongOpenService      = "long_open_service"
	TriggerLongQueueWait        = "long_queue_wait"
	TriggerLongPause            = "long_pause"
	TriggerIdleStore            = "idle_store"
	TriggerOutsideBusinessHours = "outside_business_hours"

	// Display kinds for alerts
	DisplayKindCardBadge   = "card_badge"
	DisplayKindBanner      = "banner"
	DisplayKindToast       = "toast"
	DisplayKindCornerPopup = "corner_popup"
	DisplayKindCenterModal = "center_modal"
	DisplayKindFullscreen  = "fullscreen"

	// Color themes for alerts
	ColorThemeAmber  = "amber"
	ColorThemeRed    = "red"
	ColorThemeBlue   = "blue"
	ColorThemeGreen  = "green"
	ColorThemePurple = "purple"
	ColorThemeSlate  = "slate"

	// Interaction kinds for rules
	InteractionKindDismiss       = "dismiss"
	InteractionKindConfirmChoice = "confirm_choice"
	InteractionKindSelectOption  = "select_option"

	// External notification channels
	ExternalChannelNone     = "none"
	ExternalChannelWhatsapp = "whatsapp"
	ExternalChannelEmail    = "email"
)

type ResponseOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Alert struct {
	ID                  string
	TenantID            string
	StoreID             string
	ServiceID           string
	ConsultantID        string
	Type                string
	Category            string
	Severity            string
	Status              string
	SourceModule        string
	DedupeKey           string
	Headline            string
	Body                string
	OpenedAt            time.Time
	LastTriggeredAt     time.Time
	AcknowledgedAt      *time.Time
	ResolvedAt          *time.Time
	Metadata            map[string]any
	InteractionKind     string
	InteractionResponse string
	RespondedAt         *time.Time
	ExternalNotifiedAt  *time.Time
	RuleDefinitionID    string
	DisplayKind         string
	ColorTheme          string
	ResponseOptions     []ResponseOption
	IsMandatory         bool
	ConsultantName      string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type AlertView struct {
	ID                  string           `json:"id"`
	TenantID            string           `json:"tenantId"`
	StoreID             string           `json:"storeId,omitempty"`
	ServiceID           string           `json:"serviceId,omitempty"`
	ConsultantID        string           `json:"consultantId,omitempty"`
	Type                string           `json:"type"`
	Category            string           `json:"category"`
	Severity            string           `json:"severity"`
	Status              string           `json:"status"`
	SourceModule        string           `json:"sourceModule"`
	Headline            string           `json:"headline"`
	Body                string           `json:"body"`
	OpenedAt            time.Time        `json:"openedAt"`
	LastTriggeredAt     time.Time        `json:"lastTriggeredAt"`
	AcknowledgedAt      *time.Time       `json:"acknowledgedAt,omitempty"`
	ResolvedAt          *time.Time       `json:"resolvedAt,omitempty"`
	Metadata            map[string]any   `json:"metadata,omitempty"`
	InteractionKind     string           `json:"interactionKind"`
	InteractionResponse string           `json:"interactionResponse,omitempty"`
	RespondedAt         *time.Time       `json:"respondedAt,omitempty"`
	ExternalNotifiedAt  *time.Time       `json:"externalNotifiedAt,omitempty"`
	RuleDefinitionID    string           `json:"ruleDefinitionId,omitempty"`
	DisplayKind         string           `json:"displayKind"`
	ColorTheme          string           `json:"colorTheme"`
	ResponseOptions     []ResponseOption `json:"responseOptions,omitempty"`
	IsMandatory         bool             `json:"isMandatory"`
	ConsultantName      string           `json:"consultantName,omitempty"`
	CreatedAt           time.Time        `json:"createdAt"`
	UpdatedAt           time.Time        `json:"updatedAt"`
}

func (alert Alert) View() AlertView {
	return AlertView{
		ID:                  alert.ID,
		TenantID:            alert.TenantID,
		StoreID:             alert.StoreID,
		ServiceID:           alert.ServiceID,
		ConsultantID:        alert.ConsultantID,
		Type:                alert.Type,
		Category:            alert.Category,
		Severity:            alert.Severity,
		Status:              alert.Status,
		SourceModule:        alert.SourceModule,
		Headline:            alert.Headline,
		Body:                alert.Body,
		OpenedAt:            alert.OpenedAt,
		LastTriggeredAt:     alert.LastTriggeredAt,
		AcknowledgedAt:      alert.AcknowledgedAt,
		ResolvedAt:          alert.ResolvedAt,
		Metadata:            alert.Metadata,
		InteractionKind:     alert.InteractionKind,
		InteractionResponse: alert.InteractionResponse,
		RespondedAt:         alert.RespondedAt,
		ExternalNotifiedAt:  alert.ExternalNotifiedAt,
		RuleDefinitionID:    alert.RuleDefinitionID,
		DisplayKind:         alert.DisplayKind,
		ColorTheme:          alert.ColorTheme,
		ResponseOptions:     alert.ResponseOptions,
		IsMandatory:         alert.IsMandatory,
		ConsultantName:      alert.ConsultantName,
		CreatedAt:           alert.CreatedAt,
		UpdatedAt:           alert.UpdatedAt,
	}
}

type ListInput struct {
	TenantID string
	StoreID  string
	StoreIDs []string
	Status   string
	Type     string
	Category string
}

type OverviewInput struct {
	TenantID string
	StoreID  string
	StoreIDs []string
}

type Overview struct {
	TenantID       string `json:"tenantId"`
	StoreID        string `json:"storeId,omitempty"`
	TotalActive    int    `json:"totalActive"`
	CriticalActive int    `json:"criticalActive"`
	Acknowledged   int    `json:"acknowledged"`
	ResolvedToday  int    `json:"resolvedToday"`
}

type RulesView struct {
	TenantID                 string     `json:"tenantId"`
	LongOpenServiceMinutes   int        `json:"longOpenServiceMinutes"`
	IdleStoreMinutes         int        `json:"idleStoreMinutes"`
	AfterClosingGraceMinutes int        `json:"afterClosingGraceMinutes"`
	NotifyDashboard          bool       `json:"notifyDashboard"`
	NotifyOperationContext   bool       `json:"notifyOperationContext"`
	NotifyExternal           bool       `json:"notifyExternal"`
	Source                   string     `json:"source"`
	UpdatedAt                *time.Time `json:"updatedAt,omitempty"`
}

type UpdateRulesInput struct {
	LongOpenServiceMinutes   *int  `json:"longOpenServiceMinutes,omitempty"`
	IdleStoreMinutes         *int  `json:"idleStoreMinutes,omitempty"`
	AfterClosingGraceMinutes *int  `json:"afterClosingGraceMinutes,omitempty"`
	NotifyDashboard          *bool `json:"notifyDashboard,omitempty"`
	NotifyOperationContext   *bool `json:"notifyOperationContext,omitempty"`
	NotifyExternal           *bool `json:"notifyExternal,omitempty"`
}

type Actor struct {
	UserID      string
	DisplayName string
}

type OperationalRules struct {
	TenantID               string
	LongOpenServiceMinutes int
	NotifyDashboard        bool
	NotifyOperationContext bool
	NotifyExternal         bool
}

type AlertRespondInput struct {
	AlertID  string
	TenantID string
	Response string
}

type AlertRespondResult struct {
	Alert           Alert
	OpenFinishModal bool
	ServiceID       string
}

type OperationalSignalInput struct {
	TenantID       string
	StoreID        string
	ServiceID      string
	ConsultantID   string
	SignalType     string
	TriggeredAt    time.Time
	Metadata       map[string]any
	ConsultantName string
	ElapsedMinutes int
	TriggerType    string
}

type SignalMutation struct {
	TenantID string
	AlertID  string
	Action   string
	SavedAt  time.Time
}

type RuleDefinition struct {
	ID                     string
	TenantID               string
	Name                   string
	Description            string
	IsActive               bool
	TriggerType            string
	ThresholdMinutes       int
	Severity               string
	DisplayKind            string
	ColorTheme             string
	TitleTemplate          string
	BodyTemplate           string
	InteractionKind        string
	ResponseOptions        []ResponseOption
	IsMandatory            bool
	NotifyDashboard        bool
	NotifyOperationContext bool
	NotifyExternal         bool
	ExternalChannel        string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type RuleDefinitionView struct {
	ID                     string           `json:"id"`
	TenantID               string           `json:"tenantId"`
	Name                   string           `json:"name"`
	Description            string           `json:"description"`
	IsActive               bool             `json:"isActive"`
	TriggerType            string           `json:"triggerType"`
	ThresholdMinutes       int              `json:"thresholdMinutes"`
	Severity               string           `json:"severity"`
	DisplayKind            string           `json:"displayKind"`
	ColorTheme             string           `json:"colorTheme"`
	TitleTemplate          string           `json:"titleTemplate"`
	BodyTemplate           string           `json:"bodyTemplate"`
	InteractionKind        string           `json:"interactionKind"`
	ResponseOptions        []ResponseOption `json:"responseOptions"`
	IsMandatory            bool             `json:"isMandatory"`
	NotifyDashboard        bool             `json:"notifyDashboard"`
	NotifyOperationContext bool             `json:"notifyOperationContext"`
	NotifyExternal         bool             `json:"notifyExternal"`
	ExternalChannel        string           `json:"externalChannel"`
	CreatedAt              time.Time        `json:"createdAt"`
	UpdatedAt              time.Time        `json:"updatedAt"`
}

func (rule RuleDefinition) View() RuleDefinitionView {
	return RuleDefinitionView{
		ID:                     rule.ID,
		TenantID:               rule.TenantID,
		Name:                   rule.Name,
		Description:            rule.Description,
		IsActive:               rule.IsActive,
		TriggerType:            rule.TriggerType,
		ThresholdMinutes:       rule.ThresholdMinutes,
		Severity:               rule.Severity,
		DisplayKind:            rule.DisplayKind,
		ColorTheme:             rule.ColorTheme,
		TitleTemplate:          rule.TitleTemplate,
		BodyTemplate:           rule.BodyTemplate,
		InteractionKind:        rule.InteractionKind,
		ResponseOptions:        rule.ResponseOptions,
		IsMandatory:            rule.IsMandatory,
		NotifyDashboard:        rule.NotifyDashboard,
		NotifyOperationContext: rule.NotifyOperationContext,
		NotifyExternal:         rule.NotifyExternal,
		ExternalChannel:        rule.ExternalChannel,
		CreatedAt:              rule.CreatedAt,
		UpdatedAt:              rule.UpdatedAt,
	}
}

type CreateRuleInput struct {
	TenantID               string
	Name                   string
	Description            string
	IsActive               bool
	TriggerType            string
	ThresholdMinutes       int
	Severity               string
	DisplayKind            string
	ColorTheme             string
	TitleTemplate          string
	BodyTemplate           string
	InteractionKind        string
	ResponseOptions        []ResponseOption
	IsMandatory            bool
	NotifyDashboard        bool
	NotifyOperationContext bool
	NotifyExternal         bool
	ExternalChannel        string
}

type UpdateRuleInput struct {
	Name                   *string
	Description            *string
	IsActive               *bool
	TriggerType            *string
	ThresholdMinutes       *int
	Severity               *string
	DisplayKind            *string
	ColorTheme             *string
	TitleTemplate          *string
	BodyTemplate           *string
	InteractionKind        *string
	ResponseOptions        []ResponseOption
	IsMandatory            *bool
	NotifyDashboard        *bool
	NotifyOperationContext *bool
	NotifyExternal         *bool
	ExternalChannel        *string
}

type ListRulesInput struct {
	TenantID    string
	TriggerType string
	OnlyActive  bool
}

type Repository interface {
	List(ctx context.Context, input ListInput) ([]Alert, error)
	Overview(ctx context.Context, input OverviewInput) (Overview, error)
	GetByID(ctx context.Context, alertID string) (*Alert, error)
	LoadRules(ctx context.Context, tenantID string) (RulesView, error)
	UpsertRules(ctx context.Context, tenantID string, updatedByUserID string, input UpdateRulesInput) (RulesView, error)
	Acknowledge(ctx context.Context, alertID string, actor Actor, note string) (*Alert, error)
	Resolve(ctx context.Context, alertID string, actor Actor, note string) (*Alert, error)
	RespondToAlert(ctx context.Context, input AlertRespondInput, actor Actor) (*Alert, error)
	MarkExternalNotified(ctx context.Context, alertID string) error
	LoadOperationalRules(ctx context.Context, storeID string) (OperationalRules, error)
	ProcessOperationalSignals(ctx context.Context, signals []OperationalSignalInput) ([]SignalMutation, error)
	ListRules(ctx context.Context, input ListRulesInput) ([]RuleDefinition, error)
	GetRule(ctx context.Context, ruleID string) (*RuleDefinition, error)
	CreateRule(ctx context.Context, input CreateRuleInput, actor Actor) (*RuleDefinition, error)
	UpdateRule(ctx context.Context, ruleID string, input UpdateRuleInput, actor Actor) (*RuleDefinition, error)
	DeleteRule(ctx context.Context, ruleID string) error
	LoadActiveRulesForTrigger(ctx context.Context, tenantID string, triggerType string) ([]RuleDefinition, error)
}

type OperationsScanner interface {
	ScanForRule(ctx context.Context, ruleID string, triggerType string, tenantID string, thresholdMinutes int) ([]operations.OperationalAlertSignal, error)
}

// RenderTemplate substitui variáveis no template (ex: {elapsed}, {consultant}, {store}, {threshold})
func RenderTemplate(tmpl string, vars map[string]string) string {
	out := tmpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

// FormatElapsed formata uma duração em minutos para formato legível (ex: "1h17min" ou "23 min")
func FormatElapsed(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}
	hours := minutes / 60
	rem := minutes % 60
	if rem == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dmin", hours, rem)
}

// ElapsedMinutesSince calcula minutos decorridos desde um tempo
func ElapsedMinutesSince(since time.Time, now time.Time) int {
	return int(now.Sub(since).Minutes())
}

func NormalizeColorTheme(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	switch strings.ToLower(trimmed) {
	case ColorThemeAmber:
		return "#F59E0B"
	case ColorThemeRed:
		return "#EF4444"
	case ColorThemeBlue:
		return "#3B82F6"
	case ColorThemeGreen:
		return "#10B981"
	case ColorThemePurple:
		return "#A855F7"
	case ColorThemeSlate:
		return "#64748B"
	}

	if len(trimmed) == 6 && isHexColor(trimmed) {
		return "#" + strings.ToUpper(trimmed)
	}

	if len(trimmed) == 7 && strings.HasPrefix(trimmed, "#") && isHexColor(trimmed[1:]) {
		return "#" + strings.ToUpper(trimmed[1:])
	}

	return ""
}

func isHexColor(value string) bool {
	if value == "" {
		return false
	}

	for _, char := range value {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
			continue
		}
		return false
	}

	return true
}
