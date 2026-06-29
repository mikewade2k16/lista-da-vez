package core

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,60}[a-z0-9]$`)

// AdminService concentra as regras de negocio das rotas /v1/admin/accounts.
// Requer papel platform_admin — validado no handler HTTP, nao aqui.
type AdminService struct {
	repo  AdminRepository
	bus   events.Bus
	guard *httpapi.AccountModulesGuard
}

// NewAdminService cria o AdminService com suas dependencias.
func NewAdminService(repo AdminRepository, bus events.Bus, guard *httpapi.AccountModulesGuard) *AdminService {
	return &AdminService{repo: repo, bus: bus, guard: guard}
}

func (s *AdminService) ListAccounts(ctx context.Context, filter AdminListFilter) (AdminListAccountsResponse, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}

	accounts, total, err := s.repo.ListAccounts(ctx, filter)
	if err != nil {
		return AdminListAccountsResponse{}, err
	}

	return AdminListAccountsResponse{
		Accounts: accounts,
		Total:    total,
		Page:     filter.Page,
		PerPage:  filter.PerPage,
	}, nil
}

func (s *AdminService) GetAccount(ctx context.Context, accountID string) (AccountAdminView, error) {
	if strings.TrimSpace(accountID) == "" {
		return AccountAdminView{}, ErrAccountNotFound
	}
	return s.repo.FindAdminAccount(ctx, accountID)
}

func (s *AdminService) CreateAccount(ctx context.Context, input AdminCreateAccountInput) (AccountAdminView, error) {
	input.Slug = strings.TrimSpace(strings.ToLower(input.Slug))
	input.Name = strings.TrimSpace(input.Name)
	input.AdminEmail = strings.TrimSpace(input.AdminEmail)

	if input.Name == "" {
		return AccountAdminView{}, ErrValidationFailed("name e obrigatorio")
	}
	if !slugRegex.MatchString(input.Slug) {
		return AccountAdminView{}, ErrValidationFailed("slug deve conter apenas letras minusculas, numeros e hifens (min 2 chars)")
	}
	// adminEmail e OPCIONAL por design: clientes podem ser cadastrados so para
	// controle interno, sem usuario/acesso. Vazio -> conta sem dono (o dono pode
	// ser anexado depois via POST /v1/admin/users/{id}/memberships). Quando
	// preenchido, deve corresponder a um usuario ja existente em core.users.
	if input.PlanCode == "" {
		input.PlanCode = "standard"
	}

	return s.repo.CreateAccount(ctx, input)
}

func (s *AdminService) UpdateAccount(ctx context.Context, accountID string, input AdminUpdateAccountInput) (AccountAdminView, error) {
	if strings.TrimSpace(accountID) == "" {
		return AccountAdminView{}, ErrAccountNotFound
	}

	if input.Slug != nil {
		slug := strings.TrimSpace(strings.ToLower(*input.Slug))
		if !slugRegex.MatchString(slug) {
			return AccountAdminView{}, ErrValidationFailed("slug deve conter apenas letras minusculas, numeros e hifens")
		}
		input.Slug = &slug
	}

	if input.BillingMode != nil {
		if *input.BillingMode != "single" && *input.BillingMode != "per_store" {
			return AccountAdminView{}, ErrValidationFailed("billingMode deve ser 'single' ou 'per_store'")
		}
	}

	if input.MonthlyPaymentAmount != nil && *input.MonthlyPaymentAmount < 0 {
		return AccountAdminView{}, ErrValidationFailed("monthlyPaymentAmount nao pode ser negativo")
	}

	if input.PaymentDueDay != nil && (*input.PaymentDueDay < 1 || *input.PaymentDueDay > 31) {
		return AccountAdminView{}, ErrValidationFailed("paymentDueDay deve ser entre 1 e 31")
	}

	return s.repo.UpdateAccount(ctx, accountID, input)
}

func (s *AdminService) DeleteAccount(ctx context.Context, accountID string) error {
	if strings.TrimSpace(accountID) == "" {
		return ErrAccountNotFound
	}
	return s.repo.SoftDeleteAccount(ctx, accountID)
}

func (s *AdminService) GetModules(ctx context.Context, accountID string) (AdminModulesResponse, error) {
	modules, err := s.repo.GetAccountModules(ctx, accountID)
	if err != nil {
		return AdminModulesResponse{}, err
	}
	return AdminModulesResponse{Modules: modules}, nil
}

func (s *AdminService) SetModules(ctx context.Context, accountID string, input AdminSetModulesInput) (AdminModulesResponse, error) {
	for _, moduleID := range input.Enable {
		if err := s.repo.SetAccountModuleEnabled(ctx, accountID, moduleID, true); err != nil {
			return AdminModulesResponse{}, err
		}
	}
	for _, moduleID := range input.Disable {
		if err := s.repo.SetAccountModuleEnabled(ctx, accountID, moduleID, false); err != nil {
			return AdminModulesResponse{}, err
		}
	}

	if s.guard != nil {
		s.guard.Invalidate(accountID)
	}

	_ = s.bus.Publish(ctx, events.Event{
		AccountID: accountID,
		Topic:     "account.modules.changed",
		Payload:   map[string]any{"account_id": accountID},
	})

	return s.GetModules(ctx, accountID)
}

func (s *AdminService) GetStores(ctx context.Context, accountID string) (AdminStoresResponse, error) {
	stores, err := s.repo.GetAccountStores(ctx, accountID)
	if err != nil {
		return AdminStoresResponse{}, err
	}
	return AdminStoresResponse{Stores: stores}, nil
}

func (s *AdminService) SetStorePricing(ctx context.Context, accountID string, input AdminSetStorePricingInput) (AdminStoresResponse, error) {
	for _, entry := range input.Stores {
		if entry.Amount < 0 {
			return AdminStoresResponse{}, ErrValidationFailed("amount nao pode ser negativo")
		}
		if err := s.repo.SetStoreBillingAmount(ctx, entry.ID, entry.Amount); err != nil {
			return AdminStoresResponse{}, err
		}
	}
	return s.GetStores(ctx, accountID)
}

func (s *AdminService) RotateWebhook(ctx context.Context, accountID string) (AdminWebhookRotateResponse, error) {
	key, err := s.repo.RotateWebhookKey(ctx, accountID)
	if err != nil {
		return AdminWebhookRotateResponse{}, err
	}
	return AdminWebhookRotateResponse{WebhookKey: key}, nil
}

// ============================================================================
// Erros de validacao inline
// ============================================================================

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

// ErrValidationFailed cria um erro de validacao com mensagem legivel.
func ErrValidationFailed(msg string) error { return &validationError{msg: msg} }

// IsValidationError retorna true se o erro e de validacao (422 no handler).
func IsValidationError(err error) bool {
	var ve *validationError
	return errors.As(err, &ve)
}
