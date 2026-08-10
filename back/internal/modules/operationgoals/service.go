package operationgoals

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/goalperiod"
)

type Service struct {
	repository  Repository
	storeFinder StoreFinder
	notifier    ContextPublisher
}

// ContextPublisher publica eventos de contexto via WebSocket para que clientes
// conectados sincronizem o cadastro de metas sem precisar recarregar a pagina.
type ContextPublisher interface {
	PublishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string, savedAt time.Time)
}

func NewService(repository Repository, storeFinder StoreFinder, notifier ContextPublisher) *Service {
	return &Service{repository: repository, storeFinder: storeFinder, notifier: notifier}
}

func (service *Service) List(ctx context.Context, principal auth.Principal, input ListInput) (GoalTargetListView, error) {
	if !canViewGoalTargets(principal) {
		return GoalTargetListView{}, ErrForbidden
	}

	targetMonth, err := normalizeTargetMonth(input.Month)
	if err != nil {
		return GoalTargetListView{}, ErrValidation
	}

	accessibleStores, err := service.listAccessibleStores(ctx, principal, input)
	if err != nil {
		return GoalTargetListView{}, err
	}

	response := GoalTargetListView{
		Month: targetMonth.Format(monthLayout),
		Summary: GoalTargetSummary{
			Month: targetMonth.Format(monthLayout),
		},
		Items: []GoalTargetView{},
	}

	if len(accessibleStores) == 0 {
		return response, nil
	}

	storeIDs := make([]string, 0, len(accessibleStores))
	for _, store := range accessibleStores {
		storeIDs = append(storeIDs, store.ID)
	}

	rows, err := service.repository.List(ctx, RepositoryListInput{
		Month:    targetMonth,
		StoreIDs: storeIDs,
	})
	if err != nil {
		return GoalTargetListView{}, err
	}

	response.Items = make([]GoalTargetView, 0, len(rows))
	response.Summary = buildSummary(targetMonth, rows)
	for _, row := range rows {
		response.Items = append(response.Items, row.View())
	}

	return response, nil
}

func (service *Service) Create(ctx context.Context, principal auth.Principal, input CreateInput) (GoalTargetView, error) {
	if !canEditGoalTargets(principal) {
		return GoalTargetView{}, ErrForbidden
	}

	goal, err := service.buildGoalForCreate(ctx, principal, input)
	if err != nil {
		return GoalTargetView{}, err
	}

	created, err := service.repository.Create(ctx, goal)
	if err != nil {
		return GoalTargetView{}, err
	}

	service.publishContextEvent(ctx, created.TenantID, "operationgoal", "created", created.ID)
	return created.View(), nil
}

func (service *Service) Update(ctx context.Context, principal auth.Principal, input UpdateInput) (GoalTargetView, error) {
	if !canEditGoalTargets(principal) {
		return GoalTargetView{}, ErrForbidden
	}

	goalID := strings.TrimSpace(input.ID)
	if goalID == "" {
		return GoalTargetView{}, ErrValidation
	}

	existing, err := service.repository.FindByID(ctx, goalID)
	if err != nil {
		return GoalTargetView{}, err
	}

	if _, err := service.storeFinder.FindAccessible(ctx, principal, existing.StoreID); err != nil {
		return GoalTargetView{}, mapStoreAccessError(err)
	}

	if input.MonthlyGoal != nil {
		existing.MonthlyGoal = maxFloat(*input.MonthlyGoal, 0)
	}

	if input.AvgTicketGoal != nil {
		existing.AvgTicketGoal = maxFloat(*input.AvgTicketGoal, 0)
	}

	if input.ConversionGoal != nil {
		existing.ConversionGoal = clampFloat(*input.ConversionGoal, 0, 100)
	}

	if input.PAGoal != nil {
		existing.PAGoal = maxFloat(*input.PAGoal, 0)
	}

	existing.UpdatedByUserID = strings.TrimSpace(principal.UserID)

	updated, err := service.repository.Update(ctx, existing)
	if err != nil {
		return GoalTargetView{}, err
	}

	service.publishContextEvent(ctx, updated.TenantID, "operationgoal", "updated", updated.ID)
	return updated.View(), nil
}

func (service *Service) Delete(ctx context.Context, principal auth.Principal, goalID string) error {
	if !canEditGoalTargets(principal) {
		return ErrForbidden
	}

	normalizedID := strings.TrimSpace(goalID)
	if normalizedID == "" {
		return ErrValidation
	}

	existing, err := service.repository.FindByID(ctx, normalizedID)
	if err != nil {
		return err
	}

	if _, err := service.storeFinder.FindAccessible(ctx, principal, existing.StoreID); err != nil {
		return mapStoreAccessError(err)
	}

	if err := service.repository.Delete(ctx, normalizedID); err != nil {
		return err
	}

	service.publishContextEvent(ctx, existing.TenantID, "operationgoal", "deleted", existing.ID)
	return nil
}

func (service *Service) publishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string) {
	if service.notifier == nil {
		return
	}

	service.notifier.PublishContextEvent(ctx, tenantID, resource, action, resourceID, time.Now().UTC())
}

func buildSummary(targetMonth time.Time, rows []GoalTarget) GoalTargetSummary {
	storesCovered := map[string]struct{}{}
	consultantsCovered := map[string]struct{}{}
	summary := GoalTargetSummary{
		Month: targetMonth.Format(monthLayout),
	}

	for _, row := range rows {
		summary.TotalRows += 1
		summary.TotalMonthlyGoal += row.MonthlyGoal
		storesCovered[row.StoreID] = struct{}{}
		if row.Scope() == ScopeConsultant {
			summary.ConsultantRows += 1
			consultantsCovered[row.ConsultantID] = struct{}{}
		} else {
			summary.StoreRows += 1
		}
	}

	summary.StoresCovered = len(storesCovered)
	summary.ConsultantsCovered = len(consultantsCovered)
	return summary
}

func (service *Service) buildGoalForCreate(ctx context.Context, principal auth.Principal, input CreateInput) (GoalTarget, error) {
	storeID := strings.TrimSpace(input.StoreID)
	if storeID == "" {
		return GoalTarget{}, ErrStoreRequired
	}

	targetMonth, err := normalizeTargetMonth(input.Month)
	if err != nil {
		return GoalTarget{}, ErrValidation
	}

	storeView, err := service.storeFinder.FindAccessible(ctx, principal, storeID)
	if err != nil {
		return GoalTarget{}, mapStoreAccessError(err)
	}

	goal := GoalTarget{
		TenantID:        storeView.TenantID,
		StoreID:         storeView.ID,
		StoreCode:       strings.TrimSpace(storeView.Code),
		StoreName:       strings.TrimSpace(storeView.Name),
		TargetMonth:     targetMonth,
		Week:            normalizeWeek(targetMonth, input.Week),
		MonthlyGoal:     maxFloat(input.MonthlyGoal, 0),
		AvgTicketGoal:   maxFloat(input.AvgTicketGoal, 0),
		ConversionGoal:  clampFloat(input.ConversionGoal, 0, 100),
		PAGoal:          maxFloat(input.PAGoal, 0),
		CreatedByUserID: strings.TrimSpace(principal.UserID),
		UpdatedByUserID: strings.TrimSpace(principal.UserID),
	}

	consultantID := strings.TrimSpace(input.ConsultantID)
	if consultantID == "" {
		return goal, nil
	}

	consultant, err := service.repository.FindConsultantByID(ctx, consultantID)
	if err != nil {
		return GoalTarget{}, err
	}

	if consultant.StoreID != storeView.ID || consultant.TenantID != storeView.TenantID {
		return GoalTarget{}, ErrValidation
	}

	goal.ConsultantID = consultant.ID
	goal.ConsultantName = consultant.Name
	return goal, nil
}

func (service *Service) listAccessibleStores(ctx context.Context, principal auth.Principal, input ListInput) ([]stores.StoreView, error) {
	requestedStoreID := strings.TrimSpace(input.StoreID)
	if requestedStoreID != "" {
		storeView, err := service.storeFinder.FindAccessible(ctx, principal, requestedStoreID)
		if err != nil {
			return nil, mapStoreAccessError(err)
		}

		return []stores.StoreView{storeView}, nil
	}

	tenantID := strings.TrimSpace(input.TenantID)
	if principal.Role == auth.RolePlatformAdmin && tenantID == "" {
		return nil, ErrTenantRequired
	}

	rows, err := service.storeFinder.ListAccessible(ctx, principal, stores.ListInput{TenantID: tenantID})
	if err != nil {
		return nil, mapStoreAccessError(err)
	}

	sort.SliceStable(rows, func(left int, right int) bool {
		return rows[left].Name < rows[right].Name
	})

	return rows, nil
}

func normalizeTargetMonth(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}

	parsed, err := time.Parse(monthLayout, trimmed)
	if err == nil {
		return time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}

	parsed, err = time.Parse("2006-01-02", trimmed)
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}

func canViewGoalTargets(principal auth.Principal) bool {
	if principal.PermissionsResolved && accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionMultiStoreView) {
		return true
	}

	switch principal.Role {
	case auth.RolePlatformAdmin, auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing, auth.RoleManager:
		return true
	default:
		return false
	}
}

func canEditGoalTargets(principal auth.Principal) bool {
	if principal.PermissionsResolved && accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionMultiStoreEdit) {
		return true
	}

	switch principal.Role {
	case auth.RolePlatformAdmin, auth.RoleOwner, auth.RoleDirector, auth.RoleManager:
		return true
	default:
		return false
	}
}

func mapStoreAccessError(err error) error {
	switch {
	case errors.Is(err, stores.ErrStoreNotFound):
		return ErrStoreNotFound
	case errors.Is(err, stores.ErrForbidden), errors.Is(err, stores.ErrTenantForbidden):
		return ErrForbidden
	default:
		return err
	}
}

// normalizeWeek limita a semana aos periodos existentes no mes.
func normalizeWeek(month time.Time, week int) int {
	if week < 0 {
		return 0
	}
	if week > goalperiod.Count(month) {
		return goalperiod.Count(month)
	}

	return week
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
