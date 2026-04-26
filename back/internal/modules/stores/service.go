package stores

import (
	"context"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository Repository
	notifier   ContextPublisher
}

type ContextPublisher interface {
	PublishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string, savedAt time.Time)
}

func NewService(repository Repository, notifier ContextPublisher) *Service {
	return &Service{
		repository: repository,
		notifier:   notifier,
	}
}

func (service *Service) ListAccessible(ctx context.Context, principal auth.Principal, input ListInput) ([]StoreView, error) {
	tenantID, err := resolveTenantFilter(principal, input.TenantID)
	if err != nil {
		return nil, err
	}

	stores, err := service.repository.ListAccessible(ctx, principal, ListInput{
		TenantID:        tenantID,
		IncludeInactive: input.IncludeInactive,
	})
	if err != nil {
		return nil, err
	}

	views := make([]StoreView, 0, len(stores))
	for _, store := range stores {
		views = append(views, store.View())
	}

	return views, nil
}

func (service *Service) FindAccessible(ctx context.Context, principal auth.Principal, storeID string) (StoreView, error) {
	store, err := service.repository.FindAccessibleByID(ctx, principal, strings.TrimSpace(storeID))
	if err != nil {
		return StoreView{}, err
	}

	return store.View(), nil
}

func (service *Service) Create(ctx context.Context, principal auth.Principal, input CreateInput) (StoreView, error) {
	if !canEditStores(principal) {
		return StoreView{}, ErrForbidden
	}

	tenantID, err := resolveTenantForWrite(principal, input.TenantID)
	if err != nil {
		return StoreView{}, err
	}

	name := strings.TrimSpace(input.Name)
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	city := strings.TrimSpace(input.City)
	defaultTemplateID := strings.TrimSpace(input.DefaultTemplateID)
	active := true
	if input.IsActive != nil {
		active = *input.IsActive
	}

	if tenantID == "" {
		return StoreView{}, ErrTenantRequired
	}

	if name == "" || code == "" {
		return StoreView{}, ErrValidation
	}

	created, err := service.repository.Create(ctx, Store{
		TenantID:          tenantID,
		Code:              code,
		Name:              name,
		City:              city,
		DefaultTemplateID: defaultTemplateID,
		MonthlyGoal:       maxFloat(input.MonthlyGoal, 0),
		WeeklyGoal:        maxFloat(input.WeeklyGoal, 0),
		AvgTicketGoal:     maxFloat(input.AvgTicketGoal, 0),
		ConversionGoal:    maxFloat(input.ConversionGoal, 0),
		PAGoal:            maxFloat(input.PAGoal, 0),
		Active:            active,
	})
	if err != nil {
		return StoreView{}, err
	}

	service.publishContextEvent(ctx, created.TenantID, "store", "created", created.ID)
	return created.View(), nil
}

func (service *Service) Update(ctx context.Context, principal auth.Principal, input UpdateInput) (StoreView, error) {
	if !canEditStores(principal) {
		return StoreView{}, ErrForbidden
	}

	storeID := strings.TrimSpace(input.ID)
	if storeID == "" {
		return StoreView{}, ErrValidation
	}

	existing, err := service.repository.FindAccessibleByID(ctx, principal, storeID)
	if err != nil {
		return StoreView{}, err
	}

	if input.Name != nil {
		existing.Name = strings.TrimSpace(*input.Name)
	}

	if input.Code != nil {
		existing.Code = strings.ToUpper(strings.TrimSpace(*input.Code))
	}

	if input.City != nil {
		existing.City = strings.TrimSpace(*input.City)
	}

	if input.DefaultTemplateID != nil {
		existing.DefaultTemplateID = strings.TrimSpace(*input.DefaultTemplateID)
	}

	if input.MonthlyGoal != nil {
		existing.MonthlyGoal = maxFloat(*input.MonthlyGoal, 0)
	}

	if input.WeeklyGoal != nil {
		existing.WeeklyGoal = maxFloat(*input.WeeklyGoal, 0)
	}

	if input.AvgTicketGoal != nil {
		existing.AvgTicketGoal = maxFloat(*input.AvgTicketGoal, 0)
	}

	if input.ConversionGoal != nil {
		existing.ConversionGoal = maxFloat(*input.ConversionGoal, 0)
	}

	if input.PAGoal != nil {
		existing.PAGoal = maxFloat(*input.PAGoal, 0)
	}

	if input.IsActive != nil {
		existing.Active = *input.IsActive
	}

	if existing.Name == "" || existing.Code == "" {
		return StoreView{}, ErrValidation
	}

	updated, err := service.repository.Update(ctx, existing)
	if err != nil {
		return StoreView{}, err
	}

	service.publishContextEvent(ctx, updated.TenantID, "store", "updated", updated.ID)
	return updated.View(), nil
}

func (service *Service) Archive(ctx context.Context, principal auth.Principal, storeID string) (StoreView, error) {
	active := false
	return service.Update(ctx, principal, UpdateInput{
		ID:       strings.TrimSpace(storeID),
		IsActive: &active,
	})
}

func (service *Service) Restore(ctx context.Context, principal auth.Principal, storeID string) (StoreView, error) {
	active := true
	return service.Update(ctx, principal, UpdateInput{
		ID:       strings.TrimSpace(storeID),
		IsActive: &active,
	})
}

func (service *Service) Delete(ctx context.Context, principal auth.Principal, storeID string) (DeleteResult, error) {
	if !canEditStores(principal) {
		return DeleteResult{}, ErrForbidden
	}

	normalizedStoreID := strings.TrimSpace(storeID)
	if normalizedStoreID == "" {
		return DeleteResult{}, ErrValidation
	}

	store, err := service.repository.FindAccessibleByID(ctx, principal, normalizedStoreID)
	if err != nil {
		return DeleteResult{}, err
	}

	dependencies, err := service.repository.ListDeleteDependencies(ctx, store.ID)
	if err != nil {
		return DeleteResult{}, err
	}

	if len(dependencies) > 0 {
		return DeleteResult{}, &DeleteBlockedError{
			StoreID:      store.ID,
			Dependencies: dependencies,
		}
	}

	if err := service.repository.Delete(ctx, store.ID); err != nil {
		return DeleteResult{}, err
	}

	result := DeleteResult{
		StoreID: store.ID,
		Deleted: true,
		SavedAt: time.Now().UTC(),
	}
	service.publishContextEvent(ctx, store.TenantID, "store", "deleted", store.ID)
	return result, nil
}

func (service *Service) publishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string) {
	if service.notifier == nil {
		return
	}

	service.notifier.PublishContextEvent(ctx, tenantID, resource, action, resourceID, time.Now().UTC())
}

func ResolveDefaultActiveStoreID(principal auth.Principal, stores []StoreView) string {
	for _, candidate := range principal.StoreIDs {
		for _, store := range stores {
			if store.ID == candidate {
				return store.ID
			}
		}
	}

	if len(stores) == 0 {
		return ""
	}

	return stores[0].ID
}

func resolveTenantFilter(principal auth.Principal, requestedTenantID string) (string, error) {
	tenantID := strings.TrimSpace(requestedTenantID)

	if principal.Role == auth.RolePlatformAdmin {
		return tenantID, nil
	}

	if principal.TenantID == "" {
		return "", ErrTenantForbidden
	}

	if tenantID == "" {
		return principal.TenantID, nil
	}

	if tenantID != principal.TenantID {
		return "", ErrForbidden
	}

	return tenantID, nil
}

func resolveTenantForWrite(principal auth.Principal, requestedTenantID string) (string, error) {
	tenantID := strings.TrimSpace(requestedTenantID)

	switch principal.Role {
	case auth.RolePlatformAdmin:
		if tenantID == "" {
			return "", ErrTenantRequired
		}

		return tenantID, nil
	case auth.RoleOwner:
		if principal.TenantID == "" {
			return "", ErrTenantForbidden
		}

		if tenantID == "" || tenantID == principal.TenantID {
			return principal.TenantID, nil
		}

		return "", ErrForbidden
	default:
		return "", ErrForbidden
	}
}

func canEditStores(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionMultiStoreEdit)
	}

	return principal.Role == auth.RoleOwner || principal.Role == auth.RolePlatformAdmin
}

func maxFloat(value float64, minimum float64) float64 {
	if value < minimum {
		return minimum
	}

	return value
}
