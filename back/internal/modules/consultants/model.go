package consultants

import (
	"context"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Consultant struct {
	ID             string
	TenantID       string
	StoreID        string
	UserID         string
	AccessEmail    string
	AccessActive   bool
	EmployeeCode   string
	Name           string
	RoleLabel      string
	Initials       string
	Color          string
	MonthlyGoal    float64
	CommissionRate float64
	ConversionGoal float64
	AvgTicketGoal  float64
	PAGoal         float64
	Active         bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ConsultantAccessView struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Active bool   `json:"active"`
}

type ConsultantView struct {
	ID             string                `json:"id"`
	StoreID        string                `json:"storeId"`
	EmployeeCode   string                `json:"employeeCode,omitempty"`
	Name           string                `json:"name"`
	Role           string                `json:"role"`
	Initials       string                `json:"initials"`
	Color          string                `json:"color"`
	MonthlyGoal    float64               `json:"monthlyGoal"`
	CommissionRate float64               `json:"commissionRate"`
	ConversionGoal float64               `json:"conversionGoal"`
	AvgTicketGoal  float64               `json:"avgTicketGoal"`
	PAGoal         float64               `json:"paGoal"`
	Active         bool                  `json:"active"`
	Access         *ConsultantAccessView `json:"access,omitempty"`
}

type ProvisionedAccess struct {
	Email           string `json:"email"`
	InitialPassword string `json:"initialPassword"`
}

type CreateResult struct {
	Consultant ConsultantView     `json:"consultant"`
	Access     *ProvisionedAccess `json:"access,omitempty"`
}

type StoreAccessContext struct {
	TenantID  string
	StoreCode string
}

type ConsultantAccessSeed struct {
	Email        string
	PasswordHash string
}

type CreateInput struct {
	StoreID        string
	EmployeeCode   string
	Name           string
	RoleLabel      string
	Color          string
	MonthlyGoal    float64
	CommissionRate float64
	ConversionGoal float64
	AvgTicketGoal  float64
	PAGoal         float64
}

type UpdateInput struct {
	ID             string
	StoreID        *string
	Name           *string
	EmployeeCode   *string
	RoleLabel      *string
	Color          *string
	MonthlyGoal    *float64
	CommissionRate *float64
	ConversionGoal *float64
	AvgTicketGoal  *float64
	PAGoal         *float64
	Active         *bool
}

type LinkedAccessSyncInput struct {
	UserID       string
	DisplayName  string
	EmployeeCode string
	TenantID     string
	StoreID      string
	Role         auth.Role
	Active       bool
}

type Repository interface {
	StoreExists(ctx context.Context, storeID string) (bool, error)
	ResolveStoreAccessContext(ctx context.Context, storeID string) (StoreAccessContext, error)
	ListByStore(ctx context.Context, storeID string) ([]Consultant, error)
	FindByID(ctx context.Context, consultantID string) (Consultant, error)
	SyncLinkedIdentity(ctx context.Context, userID string, name string, initials string) error
	SyncLinkedAccess(ctx context.Context, input LinkedAccessSyncInput) error
	Create(ctx context.Context, consultant Consultant, access ConsultantAccessSeed) (Consultant, error)
	AttachAccess(ctx context.Context, consultant Consultant, access ConsultantAccessSeed) (Consultant, error)
	Update(ctx context.Context, consultant Consultant) (Consultant, error)
	Archive(ctx context.Context, consultantID string) error
}

func (consultant Consultant) View() ConsultantView {
	return ConsultantView{
		ID:             consultant.ID,
		StoreID:        consultant.StoreID,
		EmployeeCode:   consultant.EmployeeCode,
		Name:           consultant.Name,
		Role:           consultant.RoleLabel,
		Initials:       consultant.Initials,
		Color:          consultant.Color,
		MonthlyGoal:    consultant.MonthlyGoal,
		CommissionRate: consultant.CommissionRate,
		ConversionGoal: consultant.ConversionGoal,
		AvgTicketGoal:  consultant.AvgTicketGoal,
		PAGoal:         consultant.PAGoal,
		Active:         consultant.Active,
		Access:         buildAccessView(consultant),
	}
}

func buildAccessView(consultant Consultant) *ConsultantAccessView {
	if consultant.UserID == "" && consultant.AccessEmail == "" {
		return nil
	}

	return &ConsultantAccessView{
		UserID: consultant.UserID,
		Email:  consultant.AccessEmail,
		Active: consultant.AccessActive,
	}
}
