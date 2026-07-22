package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

var (
	ErrAutomationNotReady          = errors.New("omnichannel: automation profile not ready")
	ErrAutomationNoUnansweredInput = errors.New("omnichannel: no unanswered inbound message")
)

const defaultAutoCloseMinConfidence = 0.90

// AutomationClientRef vem do mesmo catalogo permission-scoped usado pelo Calendario.
// O Omnichannel conhece somente este contrato pequeno, nunca o schema/API de tenants.
type AutomationClientRef struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type AutomationClientCatalog interface {
	ListAccessible(context.Context, auth.Principal) ([]AutomationClientRef, error)
}

// AutomationBusinessContext e a projecao minima do perfil estrategico compartilhado.
// A implementacao concreta e injetada pelo compositor; nao ha SQL cross-module aqui.
type AutomationBusinessContext struct {
	ClientID    string                  `json:"clientId"`
	Segment     string                  `json:"segment"`
	Positioning string                  `json:"positioning"`
	Description string                  `json:"description"`
	History     string                  `json:"history"`
	SiteURL     string                  `json:"siteUrl"`
	Instagram   string                  `json:"instagram"`
	Address     string                  `json:"address"`
	Objectives  string                  `json:"objectives"`
	BrandVoice  string                  `json:"brandVoice"`
	Extra       AutomationBusinessExtra `json:"extra"`
	UpdatedAt   time.Time               `json:"updatedAt"`
}

type AutomationBusinessExtra struct {
	Audience     string `json:"audience"`
	Offer        string `json:"offer"`
	Pillars      string `json:"pillars"`
	Cadence      string `json:"cadence"`
	Restrictions string `json:"restrictions"`
	Performance  string `json:"performance"`
	Assets       string `json:"assets"`
}

type AutomationBusinessContextProvider interface {
	Load(context.Context, string, string) (AutomationBusinessContext, bool, error)
}

type AutomationBusinessContextView struct {
	Source    string                    `json:"source"`
	Available bool                      `json:"available"`
	Filled    bool                      `json:"filled"`
	Profile   AutomationBusinessContext `json:"profile"`
}

type AutomationClosePolicyView struct {
	AutoCloseEnabled         bool    `json:"autoCloseEnabled"`
	MinimumConfidence        float64 `json:"minimumConfidence"`
	RequireAllRequiredFields bool    `json:"requireAllRequiredFields"`
	BlockOnHumanRequest      bool    `json:"blockOnHumanRequest"`
	BlockSensitiveTopics     bool    `json:"blockSensitiveTopics"`
	ValidGenerationRequired  bool    `json:"validGenerationRequired"`
}

type AutomationClosePolicyInput struct {
	AutoCloseEnabled         bool     `json:"autoCloseEnabled"`
	MinimumConfidence        *float64 `json:"minimumConfidence"`
	RequireAllRequiredFields *bool    `json:"requireAllRequiredFields"`
	BlockOnHumanRequest      *bool    `json:"blockOnHumanRequest"`
	BlockSensitiveTopics     *bool    `json:"blockSensitiveTopics"`
}

type AutomationProfileInput struct {
	WhatsAppInstanceID string                      `json:"whatsappInstanceId"`
	AIAgentID          string                      `json:"aiAgentId"`
	Enabled            bool                        `json:"enabled"`
	ClosePolicy        *AutomationClosePolicyInput `json:"closePolicy"`
}

type AutomationInstanceView struct {
	ID           string  `json:"id"`
	InstanceName string  `json:"instanceName"`
	Provider     string  `json:"provider"`
	DisplayName  *string `json:"displayName"`
	PhoneNumber  *string `json:"phoneNumber"`
	Active       bool    `json:"active"`
}

type AutomationAgentView struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Enabled         bool    `json:"enabled"`
	ActiveVersionID *string `json:"activeVersionId"`
}

type AutomationProfileView struct {
	ID               string                         `json:"id,omitempty"`
	Configured       bool                           `json:"configured"`
	Client           AutomationClientRef            `json:"client"`
	WhatsAppInstance *AutomationInstanceView        `json:"whatsappInstance"`
	AIAgent          *AutomationAgentView           `json:"aiAgent"`
	Enabled          bool                           `json:"enabled"`
	Ready            bool                           `json:"ready"`
	ReadinessIssues  []string                       `json:"readinessIssues"`
	ClosePolicy      AutomationClosePolicyView      `json:"closePolicy"`
	StrategicContext *AutomationBusinessContextView `json:"strategicContext,omitempty"`
	Revision         int64                          `json:"revision"`
	CreatedAt        *time.Time                     `json:"createdAt,omitempty"`
	UpdatedAt        *time.Time                     `json:"updatedAt,omitempty"`
}

type AutomationInterventionView struct {
	ID                 string              `json:"id"`
	Client             AutomationClientRef `json:"client"`
	ConversationID     string              `json:"conversationId"`
	ContactName        string              `json:"contactName"`
	ContactPhone       string              `json:"contactPhone"`
	WhatsAppInstanceID string              `json:"whatsappInstanceId"`
	InstanceName       string              `json:"instanceName"`
	ReasonCode         string              `json:"reasonCode"`
	Summary            string              `json:"summary"`
	CollectedFieldKeys []string            `json:"collectedFieldKeys"`
	Status             string              `json:"status"`
	ConversationState  string              `json:"conversationState"`
	TargetQueueID      *string             `json:"targetQueueId"`
	WaitingSince       time.Time           `json:"waitingSince"`
}

type AutomationAttendanceView struct {
	ID                    string              `json:"id"`
	Mode                  string              `json:"mode"`
	Client                AutomationClientRef `json:"client"`
	ConversationID        string              `json:"conversationId"`
	ContactName           string              `json:"contactName"`
	ContactPhone          string              `json:"contactPhone"`
	WhatsAppInstanceID    string              `json:"whatsappInstanceId"`
	InstanceName          string              `json:"instanceName"`
	ConversationState     string              `json:"conversationState"`
	DispatchStatus        string              `json:"dispatchStatus"`
	HandoffID             *string             `json:"handoffId"`
	ReasonCode            string              `json:"reasonCode"`
	Summary               string              `json:"summary"`
	AIConfidence          *float64            `json:"aiConfidence"`
	MinimumConfidence     *float64            `json:"minimumConfidence"`
	MaxAITurns            *int                `json:"maxAiTurns"`
	UnansweredCount       int64               `json:"unansweredCount"`
	PendingMessagePreview string              `json:"pendingMessagePreview"`
	PendingSince          *time.Time          `json:"pendingSince"`
	ActivitySince         time.Time           `json:"activitySince"`
}

type AutomationActionInput struct {
	IdempotencyKey string `json:"idempotencyKey"`
}

type AutomationActionResult struct {
	ConversationID string `json:"conversationId"`
	State          string `json:"state"`
	DispatchID     string `json:"dispatchId,omitempty"`
}

type automationAttendanceRow struct {
	ConversationID        string
	ClientAccountID       string
	ContactName           string
	ContactPhone          string
	WhatsAppInstanceID    string
	InstanceName          string
	ConversationState     string
	DispatchStatus        string
	HandoffID             *string
	ReasonCode            string
	Summary               string
	AIRunStatus           string
	AIRunError            string
	AIConfidence          *float64
	MinimumConfidence     *float64
	MaxAITurns            *int
	UnansweredCount       int64
	PendingMessagePreview string
	PendingSince          *time.Time
	ActivitySince         time.Time
}

type automationConversationScope struct {
	ClientAccountID   string
	ConversationState string
}

type automationInterventionRow struct {
	ID                 string
	ClientAccountID    string
	ConversationID     string
	ContactName        string
	ContactPhone       string
	WhatsAppInstanceID string
	InstanceName       string
	ReasonCode         string
	Summary            string
	CollectedFields    json.RawMessage
	Status             string
	ConversationState  string
	TargetQueueID      *string
	WaitingSince       time.Time
}

type automationProfileRow struct {
	ID                         string
	ClientAccountID            string
	WhatsAppInstanceID         string
	AIAgentID                  string
	Enabled                    bool
	AutoCloseEnabled           bool
	AutoCloseMinConfidence     float64
	AutoCloseRequireAllFields  bool
	AutoCloseBlockHumanRequest bool
	AutoCloseBlockSensitive    bool
	Revision                   int64
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
	InstanceName               string
	InstanceProvider           string
	InstanceDisplayName        *string
	InstancePhoneNumber        *string
	InstanceActive             bool
	AgentName                  string
	AgentEnabled               bool
	AgentActiveVersionID       *string
	AgentReady                 bool
}

type automationBindingReadiness struct {
	InstanceFound bool
	AgentFound    bool
	InstanceReady bool
	AgentReady    bool
}

type automationProfileWrite struct {
	WhatsAppInstanceID         string
	AIAgentID                  string
	Enabled                    bool
	AutoCloseEnabled           bool
	AutoCloseMinConfidence     float64
	AutoCloseRequireAllFields  bool
	AutoCloseBlockHumanRequest bool
	AutoCloseBlockSensitive    bool
}

type automationProfileRepository interface {
	ListAutomationProfiles(context.Context, string) ([]automationProfileRow, error)
	GetAutomationProfile(context.Context, string, string) (automationProfileRow, error)
	UpsertAutomationProfile(context.Context, string, string, string, automationProfileWrite) (automationProfileRow, error)
	AutomationBindingReadiness(context.Context, string, string, string) (automationBindingReadiness, error)
	ListAutomationInterventions(context.Context, string, string, int) ([]automationInterventionRow, error)
	ListAutomationAttendances(context.Context, string, string, int) ([]automationAttendanceRow, error)
	AutomationConversationScope(context.Context, string, string) (automationConversationScope, error)
	ReplayAutomationWithAI(context.Context, string, string, string, string) (AIDispatchRecord, error)
}
