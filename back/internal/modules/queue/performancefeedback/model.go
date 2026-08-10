package performancefeedback

import (
	"context"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

const (
	StatusDraft        = "draft"
	StatusShared       = "shared"
	StatusAcknowledged = "acknowledged"
	CadenceMonthly     = "monthly"
	CadenceWeekly      = "weekly"
)

type Period struct {
	Month    string `json:"month"`
	Week     int    `json:"week"`
	DateFrom string `json:"dateFrom"`
	DateTo   string `json:"dateTo"`
	Label    string `json:"label"`
}

type Consultant struct {
	ID       string `json:"id"`
	UserID   string `json:"-"`
	Name     string `json:"name"`
	Initials string `json:"initials"`
	Color    string `json:"color"`
}

type Metrics struct {
	SoldValue            float64  `json:"soldValue"`
	Attendances          int      `json:"attendances"`
	Conversions          int      `json:"conversions"`
	NonConversions       int      `json:"nonConversions"`
	ConversionRate       float64  `json:"conversionRate"`
	TicketAverage        float64  `json:"ticketAverage"`
	PAScore              float64  `json:"paScore"`
	QualityScore         float64  `json:"qualityScore"`
	AvgDurationMs        float64  `json:"avgDurationMs"`
	NonClientConversions int      `json:"nonClientConversions"`
	QueueJumpRate        float64  `json:"queueJumpRate"`
	QueueJumpServices    int      `json:"queueJumpServices"`
	CancellationRate     float64  `json:"cancellationRate"`
	ERPOrders            int      `json:"erpOrders"`
	SoldValueSource      string   `json:"soldValueSource,omitempty"`
	TicketAverageSource  string   `json:"ticketAverageSource,omitempty"`
	PAScoreSource        string   `json:"paScoreSource,omitempty"`
	SalesGoal            float64  `json:"salesGoal"`
	TicketGoal           float64  `json:"ticketGoal"`
	ConversionGoal       float64  `json:"conversionGoal"`
	PAGoal               float64  `json:"paGoal"`
	TranscriptionScore   *float64 `json:"transcriptionScore,omitempty"`
	TranscriptionSamples int      `json:"transcriptionSamples"`
}

type FeedbackSection struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ContentHTML string `json:"contentHtml"`
}

type Review struct {
	ID                  string            `json:"id"`
	TenantID            string            `json:"tenantId"`
	StoreID             string            `json:"storeId"`
	StoreName           string            `json:"storeName"`
	ConsultantID        string            `json:"consultantId"`
	ConsultantUserID    string            `json:"-"`
	ConsultantName      string            `json:"consultantName"`
	Period              Period            `json:"period"`
	Status              string            `json:"status"`
	FeedbackSections    []FeedbackSection `json:"feedbackSections"`
	ConsultantNotesHTML string            `json:"consultantNotesHtml"`
	Metrics             Metrics           `json:"metrics"`
	CreatedByUserID     string            `json:"createdByUserId"`
	UpdatedByUserID     string            `json:"updatedByUserId"`
	SharedAt            *time.Time        `json:"sharedAt,omitempty"`
	AcknowledgedAt      *time.Time        `json:"acknowledgedAt,omitempty"`
	CreatedAt           time.Time         `json:"createdAt"`
	UpdatedAt           time.Time         `json:"updatedAt"`
	Version             int               `json:"version"`
}

type HistoryItem struct {
	ID        string    `json:"id"`
	Period    Period    `json:"period"`
	Status    string    `json:"status"`
	Metrics   Metrics   `json:"metrics"`
	UpdatedAt time.Time `json:"updatedAt"`
	Version   int       `json:"version"`
}

type Settings struct {
	TenantID        string            `json:"tenantId"`
	Cadence         string            `json:"cadence"`
	DefaultSections []FeedbackSection `json:"defaultSections"`
	Configured      bool              `json:"configured"`
	UpdatedByUserID string            `json:"updatedByUserId,omitempty"`
	CreatedAt       time.Time         `json:"createdAt,omitempty"`
	UpdatedAt       time.Time         `json:"updatedAt,omitempty"`
	Version         int               `json:"version"`
}

type ContextView struct {
	Store       stores.StoreView `json:"store"`
	Consultants []Consultant     `json:"consultants"`
	Selected    *Consultant      `json:"selectedConsultant,omitempty"`
	Period      Period           `json:"period"`
	Metrics     *Metrics         `json:"metrics,omitempty"`
	Review      *Review          `json:"review,omitempty"`
	History     []HistoryItem    `json:"history"`
	CanManage   bool             `json:"canManage"`
	CanRespond  bool             `json:"canRespond"`
	Settings    Settings         `json:"settings"`
}

type ContextInput struct {
	StoreID      string
	ConsultantID string
	Month        string
	Week         int
}

type ManagerInput struct {
	StoreID          string
	ConsultantID     string
	Month            string
	Week             int
	FeedbackSections []FeedbackSection
	Status           string
	ExpectedVersion  int
	MetricsSnapshot  *Metrics
}

type SettingsInput struct {
	StoreID         string
	Cadence         string
	DefaultSections []FeedbackSection
	ExpectedVersion int
}

type ConsultantInput struct {
	ReviewID            string
	ConsultantNotesHTML string
	ExpectedVersion     int
}

type GoalSnapshot struct {
	SalesGoal      float64
	TicketGoal     float64
	ConversionGoal float64
	PAGoal         float64
}

type Repository interface {
	ListConsultants(ctx context.Context, tenantID string, storeID string, userID string) ([]Consultant, error)
	FindConsultant(ctx context.Context, tenantID string, storeID string, consultantID string) (Consultant, error)
	LoadGoal(ctx context.Context, tenantID string, storeID string, consultantID string, period Period) (GoalSnapshot, error)
	LoadTranscriptionScore(ctx context.Context, tenantID string, storeID string, consultantID string, period Period) (*float64, int, error)
	FindByPeriod(ctx context.Context, tenantID string, storeID string, consultantID string, period Period) (Review, error)
	FindByID(ctx context.Context, tenantID string, reviewID string) (Review, error)
	ListHistory(ctx context.Context, tenantID string, storeID string, consultantID string, limit int) ([]HistoryItem, error)
	FindSettings(ctx context.Context, tenantID string) (Settings, error)
	UpsertSettings(ctx context.Context, settings Settings, expectedVersion int) (Settings, error)
	UpsertManager(ctx context.Context, review Review, expectedVersion int) (Review, error)
	UpdateConsultant(ctx context.Context, review Review, expectedVersion int) (Review, error)
}

type StoreFinder interface {
	FindAccessible(ctx context.Context, principal auth.Principal, storeID string) (stores.StoreView, error)
	ListAccessible(ctx context.Context, principal auth.Principal, input stores.ListInput) ([]stores.StoreView, error)
}

type MetricsProvider interface {
	LoadConsultantMetrics(ctx context.Context, principal auth.Principal, storeID string, consultantID string, dateFrom string, dateTo string) (Metrics, error)
}
