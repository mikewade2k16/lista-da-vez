package stores

import (
	"context"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Store struct {
	ID                string
	TenantID          string
	Code              string
	Name              string
	City              string
	Active            bool
	DefaultTemplateID string
	MonthlyGoal       float64
	WeeklyGoal        float64
	AvgTicketGoal     float64
	ConversionGoal    float64
	PAGoal            float64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type StoreView struct {
	ID                string  `json:"id"`
	TenantID          string  `json:"tenantId"`
	Code              string  `json:"code"`
	Name              string  `json:"name"`
	City              string  `json:"city"`
	Active            bool    `json:"isActive"`
	DefaultTemplateID string  `json:"defaultTemplateId"`
	MonthlyGoal       float64 `json:"monthlyGoal"`
	WeeklyGoal        float64 `json:"weeklyGoal"`
	AvgTicketGoal     float64 `json:"avgTicketGoal"`
	ConversionGoal    float64 `json:"conversionGoal"`
	PAGoal            float64 `json:"paGoal"`
}

type ListInput struct {
	TenantID        string
	IncludeInactive bool
}

type CreateInput struct {
	TenantID          string
	Code              string
	Name              string
	City              string
	DefaultTemplateID string
	MonthlyGoal       float64
	WeeklyGoal        float64
	AvgTicketGoal     float64
	ConversionGoal    float64
	PAGoal            float64
	IsActive          *bool
}

type UpdateInput struct {
	ID                string
	Name              *string
	Code              *string
	City              *string
	DefaultTemplateID *string
	MonthlyGoal       *float64
	WeeklyGoal        *float64
	AvgTicketGoal     *float64
	ConversionGoal    *float64
	PAGoal            *float64
	IsActive          *bool
}

type DeleteDependency struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type DeleteResult struct {
	StoreID string    `json:"storeId"`
	Deleted bool      `json:"deleted"`
	SavedAt time.Time `json:"savedAt"`
}

type Repository interface {
	ListAccessible(ctx context.Context, principal auth.Principal, input ListInput) ([]Store, error)
	FindAccessibleByID(ctx context.Context, principal auth.Principal, storeID string) (Store, error)
	Create(ctx context.Context, store Store) (Store, error)
	Update(ctx context.Context, store Store) (Store, error)
	ListDeleteDependencies(ctx context.Context, storeID string) ([]DeleteDependency, error)
	Delete(ctx context.Context, storeID string) error
}

func (store Store) View() StoreView {
	return StoreView{
		ID:                store.ID,
		TenantID:          store.TenantID,
		Code:              store.Code,
		Name:              store.Name,
		City:              store.City,
		Active:            store.Active,
		DefaultTemplateID: store.DefaultTemplateID,
		MonthlyGoal:       store.MonthlyGoal,
		WeeklyGoal:        store.WeeklyGoal,
		AvgTicketGoal:     store.AvgTicketGoal,
		ConversionGoal:    store.ConversionGoal,
		PAGoal:            store.PAGoal,
	}
}
