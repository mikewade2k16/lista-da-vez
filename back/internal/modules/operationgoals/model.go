package operationgoals

import (
	"context"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

const (
	ScopeStore      = "store"
	ScopeConsultant = "consultant"
	monthLayout     = "2006-01"
)

type GoalTarget struct {
	ID              string
	TenantID        string
	StoreID         string
	StoreCode       string
	StoreName       string
	ConsultantID    string
	ConsultantName  string
	TargetMonth     time.Time
	MonthlyGoal     float64
	AvgTicketGoal   float64
	ConversionGoal  float64
	PAGoal          float64
	CreatedByUserID string
	UpdatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type GoalTargetView struct {
	ID             string  `json:"id"`
	TenantID       string  `json:"tenantId"`
	Month          string  `json:"month"`
	Scope          string  `json:"scope"`
	StoreID        string  `json:"storeId"`
	StoreCode      string  `json:"storeCode"`
	StoreName      string  `json:"storeName"`
	ConsultantID   string  `json:"consultantId,omitempty"`
	ConsultantName string  `json:"consultantName,omitempty"`
	MonthlyGoal    float64 `json:"monthlyGoal"`
	AvgTicketGoal  float64 `json:"avgTicketGoal"`
	ConversionGoal float64 `json:"conversionGoal"`
	PAGoal         float64 `json:"paGoal"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

type GoalTargetSummary struct {
	Month              string  `json:"month"`
	TotalRows          int     `json:"totalRows"`
	StoreRows          int     `json:"storeRows"`
	ConsultantRows     int     `json:"consultantRows"`
	StoresCovered      int     `json:"storesCovered"`
	ConsultantsCovered int     `json:"consultantsCovered"`
	TotalMonthlyGoal   float64 `json:"totalMonthlyGoal"`
}

type GoalTargetListView struct {
	Month   string            `json:"month"`
	Summary GoalTargetSummary `json:"summary"`
	Items   []GoalTargetView  `json:"items"`
}

type ListInput struct {
	TenantID string
	StoreID  string
	Month    string
}

type CreateInput struct {
	StoreID        string
	ConsultantID   string
	Month          string
	MonthlyGoal    float64
	AvgTicketGoal  float64
	ConversionGoal float64
	PAGoal         float64
}

type UpdateInput struct {
	ID             string
	MonthlyGoal    *float64
	AvgTicketGoal  *float64
	ConversionGoal *float64
	PAGoal         *float64
}

type RepositoryListInput struct {
	Month    time.Time
	StoreIDs []string
}

type ConsultantReference struct {
	ID       string
	TenantID string
	StoreID  string
	Name     string
}

type Repository interface {
	List(ctx context.Context, input RepositoryListInput) ([]GoalTarget, error)
	FindByID(ctx context.Context, id string) (GoalTarget, error)
	Create(ctx context.Context, goal GoalTarget) (GoalTarget, error)
	Update(ctx context.Context, goal GoalTarget) (GoalTarget, error)
	Delete(ctx context.Context, id string) error
	FindConsultantByID(ctx context.Context, consultantID string) (ConsultantReference, error)
}

type StoreFinder interface {
	FindAccessible(ctx context.Context, principal auth.Principal, storeID string) (stores.StoreView, error)
	ListAccessible(ctx context.Context, principal auth.Principal, input stores.ListInput) ([]stores.StoreView, error)
}

func (goal GoalTarget) Scope() string {
	if strings.TrimSpace(goal.ConsultantID) != "" {
		return ScopeConsultant
	}

	return ScopeStore
}

func (goal GoalTarget) View() GoalTargetView {
	view := GoalTargetView{
		ID:             goal.ID,
		TenantID:       goal.TenantID,
		Month:          goal.TargetMonth.UTC().Format(monthLayout),
		Scope:          goal.Scope(),
		StoreID:        goal.StoreID,
		StoreCode:      goal.StoreCode,
		StoreName:      goal.StoreName,
		ConsultantID:   goal.ConsultantID,
		ConsultantName: goal.ConsultantName,
		MonthlyGoal:    goal.MonthlyGoal,
		AvgTicketGoal:  goal.AvgTicketGoal,
		ConversionGoal: goal.ConversionGoal,
		PAGoal:         goal.PAGoal,
		CreatedAt:      goal.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:      goal.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	if strings.TrimSpace(view.ConsultantID) == "" {
		view.ConsultantName = ""
	}

	return view
}
