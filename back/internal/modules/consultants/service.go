package consultants

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository            Repository
	passwordHasher        auth.PasswordHasher
	accessEmailDomain     string
	defaultAccessPassword string
}

func NewService(repository Repository, passwordHasher auth.PasswordHasher, accessEmailDomain string, defaultAccessPassword string) *Service {
	return &Service{
		repository:            repository,
		passwordHasher:        passwordHasher,
		accessEmailDomain:     normalizeAccessEmailDomain(accessEmailDomain),
		defaultAccessPassword: resolveDefaultAccessPassword(defaultAccessPassword),
	}
}

func (service *Service) ListByStore(ctx context.Context, principal auth.Principal, storeID string) ([]ConsultantView, error) {
	resolvedStoreID, err := service.resolveStoreID(ctx, principal, storeID)
	if err != nil {
		return nil, err
	}

	consultants, err := service.repository.ListByStore(ctx, resolvedStoreID)
	if err != nil {
		return nil, err
	}

	views := make([]ConsultantView, 0, len(consultants))
	for _, consultant := range consultants {
		views = append(views, consultant.View())
	}

	return views, nil
}

func (service *Service) Create(ctx context.Context, principal auth.Principal, input CreateInput) (CreateResult, error) {
	if principal.Role != auth.RoleOwner && principal.Role != auth.RolePlatformAdmin {
		return CreateResult{}, ErrForbidden
	}

	resolvedStoreID, err := service.resolveStoreID(ctx, principal, input.StoreID)
	if err != nil {
		return CreateResult{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CreateResult{}, ErrValidation
	}

	storeContext, err := service.repository.ResolveStoreAccessContext(ctx, resolvedStoreID)
	if err != nil {
		return CreateResult{}, err
	}

	consultant, access, err := service.createConsultantWithAccess(ctx, Consultant{
		TenantID:       storeContext.TenantID,
		StoreID:        resolvedStoreID,
		EmployeeCode:   strings.TrimSpace(input.EmployeeCode),
		Name:           name,
		RoleLabel:      strings.TrimSpace(input.RoleLabel),
		Initials:       buildInitials(name),
		Color:          normalizeColor(input.Color),
		MonthlyGoal:    maxFloat(input.MonthlyGoal, 0),
		CommissionRate: maxFloat(input.CommissionRate, 0),
		ConversionGoal: clampFloat(input.ConversionGoal, 0, 100),
		AvgTicketGoal:  maxFloat(input.AvgTicketGoal, 0),
		PAGoal:         maxFloat(input.PAGoal, 0),
		Active:         true,
	}, storeContext.StoreCode)
	if err != nil {
		return CreateResult{}, err
	}

	return CreateResult{
		Consultant: consultant.View(),
		Access:     access,
	}, nil
}

func (service *Service) Update(ctx context.Context, principal auth.Principal, input UpdateInput) (ConsultantView, error) {
	if principal.Role != auth.RoleOwner && principal.Role != auth.RolePlatformAdmin {
		return ConsultantView{}, ErrForbidden
	}

	consultantID := strings.TrimSpace(input.ID)
	if consultantID == "" {
		return ConsultantView{}, ErrValidation
	}

	existing, err := service.repository.FindByID(ctx, consultantID)
	if err != nil {
		return ConsultantView{}, err
	}

	if err := service.ensureStoreAccess(ctx, principal, existing.StoreID); err != nil {
		return ConsultantView{}, err
	}

	if input.Name != nil {
		existing.Name = strings.TrimSpace(*input.Name)
		existing.Initials = buildInitials(existing.Name)
	}

	if input.StoreID != nil {
		resolvedStoreID, err := service.resolveStoreID(ctx, principal, *input.StoreID)
		if err != nil {
			return ConsultantView{}, err
		}

		storeContext, err := service.repository.ResolveStoreAccessContext(ctx, resolvedStoreID)
		if err != nil {
			return ConsultantView{}, err
		}

		existing.StoreID = resolvedStoreID
		existing.TenantID = storeContext.TenantID
	}

	if input.EmployeeCode != nil {
		existing.EmployeeCode = strings.TrimSpace(*input.EmployeeCode)
	}

	if input.RoleLabel != nil {
		existing.RoleLabel = strings.TrimSpace(*input.RoleLabel)
	}

	if input.Color != nil {
		existing.Color = normalizeColor(*input.Color)
	}

	if input.MonthlyGoal != nil {
		existing.MonthlyGoal = maxFloat(*input.MonthlyGoal, 0)
	}

	if input.CommissionRate != nil {
		existing.CommissionRate = maxFloat(*input.CommissionRate, 0)
	}

	if input.ConversionGoal != nil {
		existing.ConversionGoal = clampFloat(*input.ConversionGoal, 0, 100)
	}

	if input.AvgTicketGoal != nil {
		existing.AvgTicketGoal = maxFloat(*input.AvgTicketGoal, 0)
	}

	if input.PAGoal != nil {
		existing.PAGoal = maxFloat(*input.PAGoal, 0)
	}

	if input.Active != nil {
		existing.Active = *input.Active
	}

	if strings.TrimSpace(existing.Name) == "" {
		return ConsultantView{}, ErrValidation
	}

	updated, err := service.repository.Update(ctx, existing)
	if err != nil {
		return ConsultantView{}, err
	}

	if updated.UserID == "" {
		storeContext, err := service.repository.ResolveStoreAccessContext(ctx, updated.StoreID)
		if err != nil {
			return ConsultantView{}, err
		}

		reprovisioned, err := service.attachAccessToConsultant(ctx, updated, storeContext.StoreCode)
		if err != nil {
			return ConsultantView{}, err
		}

		updated = reprovisioned
	}

	return updated.View(), nil
}

func (service *Service) Archive(ctx context.Context, principal auth.Principal, consultantID string) error {
	if principal.Role != auth.RoleOwner && principal.Role != auth.RolePlatformAdmin {
		return ErrForbidden
	}

	trimmedID := strings.TrimSpace(consultantID)
	if trimmedID == "" {
		return ErrValidation
	}

	existing, err := service.repository.FindByID(ctx, trimmedID)
	if err != nil {
		return err
	}

	if err := service.ensureStoreAccess(ctx, principal, existing.StoreID); err != nil {
		return err
	}

	return service.repository.Archive(ctx, trimmedID)
}

func (service *Service) resolveStoreID(ctx context.Context, principal auth.Principal, storeID string) (string, error) {
	trimmedStoreID := strings.TrimSpace(storeID)
	if trimmedStoreID == "" {
		return "", ErrStoreRequired
	}

	if err := service.ensureStoreAccess(ctx, principal, trimmedStoreID); err != nil {
		return "", err
	}

	return trimmedStoreID, nil
}

func (service *Service) ensureStoreAccess(ctx context.Context, principal auth.Principal, storeID string) error {
	exists, err := service.repository.StoreExists(ctx, storeID)
	if err != nil {
		return err
	}

	if !exists {
		return ErrStoreNotFound
	}

	if principal.Role == auth.RolePlatformAdmin {
		return nil
	}

	for _, accessibleStoreID := range principal.StoreIDs {
		if accessibleStoreID == storeID {
			return nil
		}
	}

	return ErrForbidden
}

func buildInitials(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "CO"
	}

	first := []rune(parts[0])
	second := first
	if len(parts) > 1 {
		second = []rune(parts[1])
	}

	initials := string(first[0])
	if len(second) > 0 {
		initials += string(second[0])
	} else if len(first) > 1 {
		initials += string(first[1])
	} else {
		initials += "O"
	}

	return strings.ToUpper(initials)
}

func normalizeColor(color string) string {
	trimmed := strings.TrimSpace(color)
	if trimmed == "" {
		return "#168aad"
	}

	return trimmed
}

func maxFloat(value float64, minimum float64) float64 {
	if value < minimum {
		return minimum
	}

	return value
}

func clampFloat(value float64, minimum float64, maximum float64) float64 {
	if value < minimum {
		return minimum
	}

	if value > maximum {
		return maximum
	}

	return value
}

func (service *Service) createConsultantWithAccess(ctx context.Context, consultant Consultant, storeCode string) (Consultant, *ProvisionedAccess, error) {
	if service.passwordHasher == nil {
		return Consultant{}, nil, ErrAccessProvisioning
	}

	passwordHash, err := service.passwordHasher.Hash(service.defaultAccessPassword)
	if err != nil {
		return Consultant{}, nil, err
	}

	for attempt := 0; attempt < 20; attempt++ {
		email := buildConsultantAccessEmail(consultant.Name, storeCode, service.accessEmailDomain, attempt)
		created, err := service.repository.Create(ctx, consultant, ConsultantAccessSeed{
			Email:        email,
			PasswordHash: passwordHash,
		})
		if err == nil {
			return created, &ProvisionedAccess{
				Email:           email,
				InitialPassword: service.defaultAccessPassword,
			}, nil
		}

		if errors.Is(err, ErrAccessConflict) {
			continue
		}

		return Consultant{}, nil, err
	}

	return Consultant{}, nil, ErrAccessProvisioning
}

func (service *Service) attachAccessToConsultant(ctx context.Context, consultant Consultant, storeCode string) (Consultant, error) {
	if service.passwordHasher == nil {
		return Consultant{}, ErrAccessProvisioning
	}

	passwordHash, err := service.passwordHasher.Hash(service.defaultAccessPassword)
	if err != nil {
		return Consultant{}, err
	}

	for attempt := 0; attempt < 20; attempt++ {
		email := buildConsultantAccessEmail(consultant.Name, storeCode, service.accessEmailDomain, attempt)
		updated, err := service.repository.AttachAccess(ctx, consultant, ConsultantAccessSeed{
			Email:        email,
			PasswordHash: passwordHash,
		})
		if err == nil {
			return updated, nil
		}

		if errors.Is(err, ErrAccessConflict) {
			continue
		}

		return Consultant{}, err
	}

	return Consultant{}, ErrAccessProvisioning
}

var accessEmailSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

func buildConsultantAccessEmail(name string, storeCode string, domain string, attempt int) string {
	baseName := strings.Trim(accessEmailSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "."), ".")
	if baseName == "" {
		baseName = "consultor"
	}

	baseStoreCode := strings.Trim(accessEmailSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(storeCode)), ""), ".")
	if baseStoreCode == "" {
		baseStoreCode = "loja"
	}

	localPart := fmt.Sprintf("%s.%s", baseName, baseStoreCode)
	if attempt > 0 {
		localPart = fmt.Sprintf("%s.%d", localPart, attempt+1)
	}

	return fmt.Sprintf("%s@%s", localPart, domain)
}

func normalizeAccessEmailDomain(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "acesso.omni.local"
	}

	return strings.TrimPrefix(trimmed, "@")
}

func resolveDefaultAccessPassword(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "Omni@123"
	}

	return trimmed
}
