package core

import (
	"context"
	"time"
)

// AccountAdminView e o DTO completo retornado pelas rotas /v1/admin/accounts.
// Inclui campos de billing/contact adicionados na migration 0123 e agregados
// computados (userCount, userNicks, projectCount, projectSegments, modules, stores).
type AccountAdminView struct {
	ID                      string              `json:"id"`
	OrganizationID          string              `json:"organizationId,omitempty"`
	Slug                    string              `json:"slug"`
	Name                    string              `json:"name"`
	PlanCode                string              `json:"planCode"`
	Active                  bool                `json:"active"`
	IsAgency                bool                `json:"isAgency"`
	BillingMode             string              `json:"billingMode"`
	MonthlyPaymentAmount    float64             `json:"monthlyPaymentAmount"`
	PaymentDueDay           *int                `json:"paymentDueDay,omitempty"`
	WebhookEnabled          bool                `json:"webhookEnabled"`
	ContactPhone            string              `json:"contactPhone,omitempty"`
	ContactSite             string              `json:"contactSite,omitempty"`
	ContactAddress          string              `json:"contactAddress,omitempty"`
	LogoPath                string              `json:"logoPath,omitempty"`
	RequireUserStoreLink    bool                `json:"requireUserStoreLink"`
	RequireUserRegistration bool                `json:"requireUserRegistration"`
	UserCount               int                 `json:"userCount"`
	UserNicks               string              `json:"userNicks"`
	ProjectCount            int                 `json:"projectCount"`
	ProjectSegments         string              `json:"projectSegments"`
	Modules                 []AccountModuleView `json:"modules"`
	Stores                  []StoreAdminView    `json:"stores"`
	CreatedAt               time.Time           `json:"createdAt"`
	UpdatedAt               time.Time           `json:"updatedAt"`
}

// AdminListFilter parametriza GET /v1/admin/accounts.
type AdminListFilter struct {
	Q              string
	Status         string // "active" | "inactive" | "" (todos)
	OrganizationID string
	Page           int
	PerPage        int
}

// AdminListAccountsResponse e o body de GET /v1/admin/accounts.
type AdminListAccountsResponse struct {
	Accounts []AccountAdminView `json:"accounts"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PerPage  int                `json:"perPage"`
}

// AdminCreateAccountInput e o body de POST /v1/admin/accounts.
// AdminEmail deve pertencer a um usuario ja existente em core.users.
type AdminCreateAccountInput struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	PlanCode   string `json:"planCode"`
	AdminEmail string `json:"adminEmail"`
}

// AdminUpdateAccountInput e o body de PATCH /v1/admin/accounts/:id.
// Campos nil nao sao atualizados (semantica de patch).
// `OrganizationID` aceita string vazia para desvincular (=> NULL no banco).
type AdminUpdateAccountInput struct {
	Active                  *bool    `json:"active"`
	Name                    *string  `json:"name"`
	Slug                    *string  `json:"slug"`
	PlanCode                *string  `json:"planCode"`
	OrganizationID          *string  `json:"organizationId"`
	BillingMode             *string  `json:"billingMode"`
	MonthlyPaymentAmount    *float64 `json:"monthlyPaymentAmount"`
	PaymentDueDay           *int     `json:"paymentDueDay"`
	WebhookEnabled          *bool    `json:"webhookEnabled"`
	ContactPhone            *string  `json:"contactPhone"`
	ContactSite             *string  `json:"contactSite"`
	ContactAddress          *string  `json:"contactAddress"`
	LogoPath                *string  `json:"logoPath"`
	RequireUserStoreLink    *bool    `json:"requireUserStoreLink"`
	RequireUserRegistration *bool    `json:"requireUserRegistration"`
}

// AccountModuleView representa um modulo com seu estado para uma account.
type AccountModuleView struct {
	ModuleID string `json:"moduleId"`
	Label    string `json:"label"`
	IsCore   bool   `json:"isCore"`
	Enabled  bool   `json:"enabled"`
}

// AdminModulesResponse e o body de GET /v1/admin/accounts/:id/modules.
type AdminModulesResponse struct {
	Modules []AccountModuleView `json:"modules"`
}

// AdminSetModulesInput e o body de PUT /v1/admin/accounts/:id/modules.
type AdminSetModulesInput struct {
	Enable  []string `json:"enable"`
	Disable []string `json:"disable"`
}

// StoreAdminView representa uma loja com billing por loja (modo per_store).
type StoreAdminView struct {
	ID            string  `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	City          string  `json:"city"`
	Active        bool    `json:"active"`
	BillingAmount float64 `json:"billingAmount"`
}

// AdminStoresResponse e o body de GET /v1/admin/accounts/:id/stores.
type AdminStoresResponse struct {
	Stores []StoreAdminView `json:"stores"`
}

// AdminSetStorePricingInput e o body de PUT /v1/admin/accounts/:id/stores.
type AdminSetStorePricingInput struct {
	Stores []StorePricingEntry `json:"stores"`
}

// StorePricingEntry especifica o valor de billing de uma loja.
type StorePricingEntry struct {
	ID     string  `json:"id"`
	Amount float64 `json:"amount"`
}

// AdminWebhookRotateResponse e o body de POST /v1/admin/accounts/:id/webhook/rotate.
type AdminWebhookRotateResponse struct {
	WebhookKey string `json:"webhookKey"`
}

// AdminRepository abstrai o acesso admin ao schema core e public (legado).
type AdminRepository interface {
	ListAccounts(ctx context.Context, filter AdminListFilter) ([]AccountAdminView, int, error)
	FindAdminAccount(ctx context.Context, accountID string) (AccountAdminView, error)
	CreateAccount(ctx context.Context, input AdminCreateAccountInput) (AccountAdminView, error)
	UpdateAccount(ctx context.Context, accountID string, input AdminUpdateAccountInput) (AccountAdminView, error)
	SoftDeleteAccount(ctx context.Context, accountID string) error
	GetAccountModules(ctx context.Context, accountID string) ([]AccountModuleView, error)
	SetAccountModuleEnabled(ctx context.Context, accountID, moduleID string, enabled bool) error
	GetAccountStores(ctx context.Context, accountID string) ([]StoreAdminView, error)
	SetStoreBillingAmount(ctx context.Context, storeID string, amount float64) error
	RotateWebhookKey(ctx context.Context, accountID string) (string, error)
}
