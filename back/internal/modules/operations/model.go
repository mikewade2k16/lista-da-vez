package operations

import (
	"context"
	"time"
)

type ConsultantProfile struct {
	ID             string
	StoreID        string
	Name           string
	Role           string
	Initials       string
	Color          string
	MonthlyGoal    float64
	CommissionRate float64
	ConversionGoal float64
	AvgTicketGoal  float64
	PAGoal         float64
}

type QueueEntry struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Role           string  `json:"role"`
	Initials       string  `json:"initials"`
	Color          string  `json:"color"`
	MonthlyGoal    float64 `json:"monthlyGoal,omitempty"`
	CommissionRate float64 `json:"commissionRate,omitempty"`
	QueueJoinedAt  int64   `json:"queueJoinedAt"`
}

type SkippedPerson struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ActiveService struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Role                 string          `json:"role"`
	Initials             string          `json:"initials"`
	Color                string          `json:"color"`
	MonthlyGoal          float64         `json:"monthlyGoal,omitempty"`
	CommissionRate       float64         `json:"commissionRate,omitempty"`
	ServiceID            string          `json:"serviceId"`
	ServiceStartedAt     int64           `json:"serviceStartedAt"`
	QueueJoinedAt        int64           `json:"queueJoinedAt"`
	QueueWaitMs          int64           `json:"queueWaitMs"`
	QueuePositionAtStart int             `json:"queuePositionAtStart"`
	StartMode            string          `json:"startMode"`
	SkippedPeople        []SkippedPerson `json:"skippedPeople"`
}

type PausedEmployee struct {
	PersonID  string `json:"personId"`
	Reason    string `json:"reason"`
	Kind      string `json:"kind,omitempty"`
	StartedAt int64  `json:"startedAt"`
}

type ConsultantSession struct {
	PersonID   string `json:"personId"`
	Status     string `json:"status"`
	StartedAt  int64  `json:"startedAt"`
	EndedAt    int64  `json:"endedAt"`
	DurationMs int64  `json:"durationMs"`
}

type ConsultantStatus struct {
	Status    string `json:"status"`
	StartedAt int64  `json:"startedAt"`
}

type ProductEntry struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	Price    float64 `json:"price"`
	IsCustom bool    `json:"isCustom,omitempty"`
}

type ServiceHistoryEntry struct {
	ServiceID                  string            `json:"serviceId"`
	StoreID                    string            `json:"storeId"`
	StoreName                  string            `json:"storeName"`
	PersonID                   string            `json:"personId"`
	PersonName                 string            `json:"personName"`
	StartedAt                  int64             `json:"startedAt"`
	FinishedAt                 int64             `json:"finishedAt"`
	DurationMs                 int64             `json:"durationMs"`
	FinishOutcome              string            `json:"finishOutcome"`
	StartMode                  string            `json:"startMode"`
	QueuePositionAtStart       int               `json:"queuePositionAtStart"`
	QueueWaitMs                int64             `json:"queueWaitMs"`
	SkippedPeople              []SkippedPerson   `json:"skippedPeople"`
	SkippedCount               int               `json:"skippedCount"`
	IsWindowService            bool              `json:"isWindowService"`
	IsGift                     bool              `json:"isGift"`
	ProductSeen                string            `json:"productSeen"`
	ProductClosed              string            `json:"productClosed"`
	ProductDetails             string            `json:"productDetails"`
	ProductsSeen               []ProductEntry    `json:"productsSeen"`
	ProductsClosed             []ProductEntry    `json:"productsClosed"`
	ProductsSeenNone           bool              `json:"productsSeenNone"`
	VisitReasonsNotInformed    bool              `json:"visitReasonsNotInformed"`
	CustomerSourcesNotInformed bool              `json:"customerSourcesNotInformed"`
	CustomerName               string            `json:"customerName"`
	CustomerPhone              string            `json:"customerPhone"`
	CustomerEmail              string            `json:"customerEmail"`
	IsExistingCustomer         bool              `json:"isExistingCustomer"`
	VisitReasons               []string          `json:"visitReasons"`
	VisitReasonDetails         map[string]string `json:"visitReasonDetails"`
	CustomerSources            []string          `json:"customerSources"`
	CustomerSourceDetails      map[string]string `json:"customerSourceDetails"`
	LossReasons                []string          `json:"lossReasons"`
	LossReasonDetails          map[string]string `json:"lossReasonDetails"`
	LossReasonID               string            `json:"lossReasonId"`
	LossReason                 string            `json:"lossReason"`
	SaleAmount                 float64           `json:"saleAmount"`
	CustomerProfession         string            `json:"customerProfession"`
	QueueJumpReason            string            `json:"queueJumpReason"`
	Notes                      string            `json:"notes"`
	CampaignMatches            []CampaignMatch   `json:"campaignMatches"`
	CampaignBonusTotal         float64           `json:"campaignBonusTotal"`
}

type CampaignMatch struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	BonusAmount float64 `json:"bonusAmount"`
}

type Snapshot struct {
	StoreID                    string                      `json:"storeId"`
	WaitingList                []QueueEntry                `json:"waitingList"`
	ActiveServices             []ActiveService             `json:"activeServices"`
	PausedEmployees            []PausedEmployee            `json:"pausedEmployees"`
	ConsultantActivitySessions []ConsultantSession         `json:"consultantActivitySessions"`
	ConsultantCurrentStatus    map[string]ConsultantStatus `json:"consultantCurrentStatus"`
	ServiceHistory             []ServiceHistoryEntry       `json:"serviceHistory"`
}

type SnapshotState struct {
	StoreID                    string
	WaitingList                []QueueStateItem
	ActiveServices             []ActiveServiceState
	PausedEmployees            []PausedStateItem
	ConsultantActivitySessions []ConsultantSession
	ConsultantCurrentStatus    map[string]ConsultantStatus
	ServiceHistory             []ServiceHistoryEntry
}

type QueueStateItem struct {
	ConsultantID  string
	QueueJoinedAt int64
}

type ActiveServiceState struct {
	ConsultantID         string
	ServiceID            string
	ServiceStartedAt     int64
	QueueJoinedAt        int64
	QueueWaitMs          int64
	QueuePositionAtStart int
	StartMode            string
	SkippedPeople        []SkippedPerson
}

type PausedStateItem struct {
	ConsultantID string
	Reason       string
	Kind         string
	StartedAt    int64
}

type OperationOverviewStore struct {
	StoreID        string `json:"storeId"`
	StoreName      string `json:"storeName"`
	StoreCode      string `json:"storeCode,omitempty"`
	City           string `json:"city,omitempty"`
	WaitingCount   int    `json:"waitingCount"`
	ActiveCount    int    `json:"activeCount"`
	PausedCount    int    `json:"pausedCount"`
	AvailableCount int    `json:"availableCount"`
}

type OperationOverviewPerson struct {
	StoreID          string  `json:"storeId"`
	StoreName        string  `json:"storeName"`
	StoreCode        string  `json:"storeCode,omitempty"`
	PersonID         string  `json:"personId"`
	Name             string  `json:"name"`
	Role             string  `json:"role"`
	Initials         string  `json:"initials"`
	Color            string  `json:"color"`
	MonthlyGoal      float64 `json:"monthlyGoal,omitempty"`
	CommissionRate   float64 `json:"commissionRate,omitempty"`
	Status           string  `json:"status"`
	StatusStartedAt  int64   `json:"statusStartedAt"`
	QueueJoinedAt    int64   `json:"queueJoinedAt,omitempty"`
	QueuePosition    int     `json:"queuePosition,omitempty"`
	ServiceID        string  `json:"serviceId,omitempty"`
	ServiceStartedAt int64   `json:"serviceStartedAt,omitempty"`
	QueueWaitMs      int64   `json:"queueWaitMs,omitempty"`
	StartMode        string  `json:"startMode,omitempty"`
	PauseReason      string  `json:"pauseReason,omitempty"`
	PauseKind        string  `json:"pauseKind,omitempty"`
}

type OperationOverview struct {
	Scope                string                    `json:"scope"`
	Stores               []OperationOverviewStore  `json:"stores"`
	WaitingList          []OperationOverviewPerson `json:"waitingList"`
	ActiveServices       []OperationOverviewPerson `json:"activeServices"`
	PausedEmployees      []OperationOverviewPerson `json:"pausedEmployees"`
	AvailableConsultants []OperationOverviewPerson `json:"availableConsultants"`
}

type PersistInput struct {
	StoreID          string
	WaitingList      []QueueStateItem
	ActiveServices   []ActiveServiceState
	PausedEmployees  []PausedStateItem
	CurrentStatus    map[string]ConsultantStatus
	AppendedSessions []ConsultantSession
	AppendedHistory  []ServiceHistoryEntry
}

type QueueCommandInput struct {
	StoreID  string `json:"storeId"`
	PersonID string `json:"personId"`
}

type PauseCommandInput struct {
	StoreID  string `json:"storeId"`
	PersonID string `json:"personId"`
	Reason   string `json:"reason"`
}

type AssignTaskCommandInput struct {
	StoreID  string `json:"storeId"`
	PersonID string `json:"personId"`
	Reason   string `json:"reason"`
}

type StartCommandInput struct {
	StoreID  string `json:"storeId"`
	PersonID string `json:"personId"`
}

type FinishCommandInput struct {
	StoreID                    string            `json:"storeId"`
	PersonID                   string            `json:"personId"`
	Outcome                    string            `json:"outcome"`
	IsWindowService            bool              `json:"isWindowService"`
	IsGift                     bool              `json:"isGift"`
	ProductSeen                string            `json:"productSeen"`
	ProductClosed              string            `json:"productClosed"`
	ProductDetails             string            `json:"productDetails"`
	ProductsSeen               []ProductEntry    `json:"productsSeen"`
	ProductsClosed             []ProductEntry    `json:"productsClosed"`
	ProductsSeenNone           bool              `json:"productsSeenNone"`
	VisitReasonsNotInformed    bool              `json:"visitReasonsNotInformed"`
	CustomerSourcesNotInformed bool              `json:"customerSourcesNotInformed"`
	CustomerName               string            `json:"customerName"`
	CustomerPhone              string            `json:"customerPhone"`
	CustomerEmail              string            `json:"customerEmail"`
	IsExistingCustomer         bool              `json:"isExistingCustomer"`
	VisitReasons               []string          `json:"visitReasons"`
	VisitReasonDetails         map[string]string `json:"visitReasonDetails"`
	CustomerSources            []string          `json:"customerSources"`
	CustomerSourceDetails      map[string]string `json:"customerSourceDetails"`
	LossReasons                []string          `json:"lossReasons"`
	LossReasonDetails          map[string]string `json:"lossReasonDetails"`
	LossReasonID               string            `json:"lossReasonId"`
	LossReason                 string            `json:"lossReason"`
	SaleAmount                 float64           `json:"saleAmount"`
	CustomerProfession         string            `json:"customerProfession"`
	QueueJumpReason            string            `json:"queueJumpReason"`
	Notes                      string            `json:"notes"`
	CampaignMatches            []CampaignMatch   `json:"campaignMatches"`
	CampaignBonusTotal         float64           `json:"campaignBonusTotal"`
}

type Repository interface {
	StoreExists(ctx context.Context, storeID string) (bool, error)
	GetStoreName(ctx context.Context, storeID string) (string, error)
	GetMaxConcurrentServices(ctx context.Context, storeID string) (int, error)
	ListRoster(ctx context.Context, storeID string) ([]ConsultantProfile, error)
	LoadSnapshot(ctx context.Context, storeID string) (SnapshotState, error)
	Persist(ctx context.Context, input PersistInput) error
}

type PublishedEvent struct {
	StoreID  string
	Action   string
	PersonID string
	SavedAt  time.Time
}

type EventPublisher interface {
	PublishOperationEvent(ctx context.Context, event PublishedEvent)
}

type MutationAck struct {
	OK       bool      `json:"ok"`
	StoreID  string    `json:"storeId"`
	SavedAt  time.Time `json:"savedAt"`
	Action   string    `json:"action,omitempty"`
	PersonID string    `json:"personId,omitempty"`
}
