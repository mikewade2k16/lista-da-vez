package planning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

const maxConfigurationBytes = 512 * 1024

var clockPattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

type Service struct {
	repository Repository
	stores     StoreFinder
	notifier   ContextPublisher
}

type ContextPublisher interface {
	PublishContextEvent(context.Context, string, string, string, string, time.Time)
}

func NewService(repository Repository, storeFinder StoreFinder, notifiers ...ContextPublisher) *Service {
	service := &Service{repository: repository, stores: storeFinder}
	if len(notifiers) > 0 {
		service.notifier = notifiers[0]
	}
	return service
}

func (service *Service) Get(ctx context.Context, principal auth.Principal, input GetInput) (Snapshot, error) {
	if !canView(principal) {
		return Snapshot{}, ErrForbidden
	}
	store, weekStart, err := service.resolveStoreAndWeek(ctx, principal, input.StoreID, input.WeekStart)
	if err != nil {
		return Snapshot{}, err
	}
	configuration, err := service.repository.FindConfiguration(ctx, store.TenantID, store.ID)
	if err != nil {
		return Snapshot{}, err
	}
	schedule, err := service.repository.FindSchedule(ctx, store.TenantID, store.ID, weekStart)
	if err != nil {
		return Snapshot{}, err
	}
	contracts, err := service.repository.ListContracts(ctx, store.TenantID, store.ID)
	if err != nil {
		return Snapshot{}, err
	}
	history := []ScheduleRevision{}
	if schedule != nil {
		history, err = service.repository.ListScheduleRevisions(ctx, store.TenantID, store.ID, schedule.ID)
		if err != nil {
			return Snapshot{}, err
		}
	}
	if schedule != nil && configuration != nil {
		engine, engineErr := buildEngineContext(weekStart, store.StoreType, configuration.Configuration, contracts)
		if engineErr == nil {
			schedule.Issues = validateEngineSchedule(engine, schedule.Shifts)
		}
	}
	return Snapshot{Configuration: configuration, Schedule: schedule, Contracts: contracts, History: history}, nil
}

func (service *Service) SaveConfiguration(ctx context.Context, principal auth.Principal, input SaveConfigurationInput) (StoreConfiguration, error) {
	if !canEdit(principal) {
		return StoreConfiguration{}, ErrForbidden
	}
	store, err := service.findStore(ctx, principal, input.StoreID)
	if err != nil {
		return StoreConfiguration{}, err
	}
	configuration := bytes.TrimSpace(input.Configuration)
	if len(configuration) == 0 || len(configuration) > maxConfigurationBytes || !json.Valid(configuration) {
		return StoreConfiguration{}, ErrValidation
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(configuration, &object) != nil || object == nil {
		return StoreConfiguration{}, ErrValidation
	}
	contracts, err := contractsFromConfiguration(configuration)
	if err != nil {
		return StoreConfiguration{}, err
	}
	if err = validateConfigurationRules(configuration); err != nil {
		return StoreConfiguration{}, err
	}
	contractIDs := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		contractIDs = append(contractIDs, contract.ConsultantID)
	}
	count, err := service.repository.CountStoreConsultants(ctx, store.TenantID, store.ID, contractIDs)
	if err != nil {
		return StoreConfiguration{}, err
	}
	if count != len(contractIDs) {
		return StoreConfiguration{}, ErrValidation
	}
	saved, err := service.repository.UpsertConfiguration(ctx, store.TenantID, store.ID, strings.TrimSpace(principal.UserID), configuration, contracts)
	if err == nil {
		service.publish(ctx, store.TenantID, "configuration.updated", store.ID)
	}
	return saved, err
}

func (service *Service) SaveSchedule(ctx context.Context, principal auth.Principal, input SaveScheduleInput) (Schedule, error) {
	if !canEdit(principal) {
		return Schedule{}, ErrForbidden
	}
	store, weekStart, err := service.resolveStoreAndWeek(ctx, principal, input.StoreID, input.WeekStart)
	if err != nil {
		return Schedule{}, err
	}
	status := strings.TrimSpace(input.Status)
	if status != "saved" && status != "published" {
		return Schedule{}, ErrValidation
	}
	consultantSet := map[string]struct{}{}
	shiftKeys := map[string]struct{}{}
	for _, shift := range input.Shifts {
		shiftDate, dateErr := time.Parse("2006-01-02", shift.ISODate)
		if dateErr != nil || shiftDate.Before(weekStart) || shiftDate.After(weekStart.AddDate(0, 0, 6)) ||
			strings.TrimSpace(shift.StaffID) == "" || !clockPattern.MatchString(shift.StartsAt) ||
			!clockPattern.MatchString(shift.EndsAt) || shift.EndsAt <= shift.StartsAt ||
			shift.BreakMinutes < 0 || shift.BreakMinutes > 240 {
			return Schedule{}, ErrValidation
		}
		if shift.TemplateID != "opening" && shift.TemplateID != "middle" && shift.TemplateID != "closing" {
			return Schedule{}, ErrValidation
		}
		shiftKey := strings.TrimSpace(shift.StaffID) + ":" + shift.ISODate
		if _, duplicated := shiftKeys[shiftKey]; duplicated {
			return Schedule{}, ErrValidation
		}
		shiftKeys[shiftKey] = struct{}{}
		consultantSet[strings.TrimSpace(shift.StaffID)] = struct{}{}
	}
	consultantIDs := make([]string, 0, len(consultantSet))
	for consultantID := range consultantSet {
		consultantIDs = append(consultantIDs, consultantID)
	}
	sort.Strings(consultantIDs)
	count, err := service.repository.CountStoreConsultants(ctx, store.TenantID, store.ID, consultantIDs)
	if err != nil {
		return Schedule{}, err
	}
	if count != len(consultantIDs) {
		return Schedule{}, ErrValidation
	}
	targetMonth, goalWeek, err := normalizeGoalPeriod(weekStart, input.TargetMonth, input.GoalWeek)
	if err != nil {
		return Schedule{}, err
	}
	contracts, err := service.repository.ListContracts(ctx, store.TenantID, store.ID)
	if err != nil {
		return Schedule{}, err
	}
	configuration, err := service.repository.FindConfiguration(ctx, store.TenantID, store.ID)
	if err != nil {
		return Schedule{}, err
	}
	if configuration == nil {
		return Schedule{}, ErrValidation
	}
	engine, err := buildEngineContext(weekStart, store.StoreType, configuration.Configuration, contracts)
	if err != nil {
		return Schedule{}, err
	}
	issues := validateEngineSchedule(engine, input.Shifts)
	if status == "published" && hasHardEngineIssue(issues) {
		return Schedule{}, ErrScheduleRestrictions
	}
	target, err := service.repository.FindStoreWeeklyGoal(ctx, store.TenantID, store.ID, targetMonth, goalWeek)
	if err != nil {
		return Schedule{}, err
	}
	allocations := calculateGoalAllocations(target, contracts, input.Shifts)
	saved, err := service.repository.SaveSchedule(ctx, RepositoryScheduleInput{
		TenantID: store.TenantID, StoreID: store.ID, UserID: strings.TrimSpace(principal.UserID),
		WeekStart: weekStart, TargetMonth: targetMonth, GoalWeek: goalWeek, Status: status,
		Shifts: input.Shifts, Allocations: allocations, ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return Schedule{}, err
	}
	saved.Issues = issues
	service.publish(ctx, store.TenantID, "schedule.updated", store.ID)
	service.publishGoalUpdate(ctx, store.TenantID, store.ID)
	return saved, nil
}

func (service *Service) GenerateSchedule(ctx context.Context, principal auth.Principal, input GenerateScheduleInput) (Schedule, error) {
	if !canEdit(principal) {
		return Schedule{}, ErrForbidden
	}
	store, weekStart, err := service.resolveStoreAndWeek(ctx, principal, input.StoreID, input.WeekStart)
	if err != nil {
		return Schedule{}, err
	}
	targetMonth, goalWeek, err := normalizeGoalPeriod(weekStart, input.TargetMonth, input.GoalWeek)
	if err != nil {
		return Schedule{}, err
	}
	configuration, err := service.repository.FindConfiguration(ctx, store.TenantID, store.ID)
	if err != nil {
		return Schedule{}, err
	}
	if configuration == nil {
		return Schedule{}, ErrValidation
	}
	contracts, err := service.repository.ListContracts(ctx, store.TenantID, store.ID)
	if err != nil {
		return Schedule{}, err
	}
	engine, err := buildEngineContext(weekStart, store.StoreType, configuration.Configuration, contracts)
	if err != nil {
		return Schedule{}, err
	}
	shifts := generateEngineSchedule(engine)
	issues := validateEngineSchedule(engine, shifts)
	target, err := service.repository.FindStoreWeeklyGoal(ctx, store.TenantID, store.ID, targetMonth, goalWeek)
	if err != nil {
		return Schedule{}, err
	}
	saved, err := service.repository.SaveSchedule(ctx, RepositoryScheduleInput{
		TenantID: store.TenantID, StoreID: store.ID, UserID: strings.TrimSpace(principal.UserID),
		WeekStart: weekStart, TargetMonth: targetMonth, GoalWeek: goalWeek, Status: "saved",
		Shifts: shifts, Allocations: calculateGoalAllocations(target, contracts, shifts), ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return Schedule{}, err
	}
	saved.Issues = issues
	service.publish(ctx, store.TenantID, "schedule.generated", store.ID)
	service.publishGoalUpdate(ctx, store.TenantID, store.ID)
	return saved, nil
}

func hasHardEngineIssue(issues []PlanningIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "hard" {
			return true
		}
	}
	return false
}

func (service *Service) ReopenSchedule(ctx context.Context, principal auth.Principal, storeID, rawWeek string, expectedVersion int64) (Schedule, error) {
	if !canEdit(principal) {
		return Schedule{}, ErrForbidden
	}
	store, weekStart, err := service.resolveStoreAndWeek(ctx, principal, storeID, rawWeek)
	if err != nil {
		return Schedule{}, err
	}
	if expectedVersion < 1 {
		return Schedule{}, ErrValidation
	}
	saved, err := service.repository.ReopenSchedule(ctx, store.TenantID, store.ID, strings.TrimSpace(principal.UserID), weekStart, expectedVersion)
	if err == nil {
		service.publish(ctx, store.TenantID, "schedule.reopened", store.ID)
	}
	return saved, err
}

func (service *Service) publish(ctx context.Context, tenantID, action, storeID string) {
	if service.notifier != nil {
		service.notifier.PublishContextEvent(ctx, tenantID, "planning", action, storeID, time.Now().UTC())
	}
}

func (service *Service) publishGoalUpdate(ctx context.Context, tenantID, storeID string) {
	if service.notifier != nil {
		service.notifier.PublishContextEvent(ctx, tenantID, "operationgoal", "generated", storeID, time.Now().UTC())
	}
}

func (service *Service) resolveStoreAndWeek(ctx context.Context, principal auth.Principal, storeID, rawWeek string) (stores.StoreView, time.Time, error) {
	store, err := service.findStore(ctx, principal, storeID)
	if err != nil {
		return stores.StoreView{}, time.Time{}, err
	}
	weekStart, err := time.Parse("2006-01-02", strings.TrimSpace(rawWeek))
	if err != nil || weekStart.Weekday() != time.Monday {
		return stores.StoreView{}, time.Time{}, ErrValidation
	}
	return store, weekStart, nil
}

func (service *Service) findStore(ctx context.Context, principal auth.Principal, storeID string) (stores.StoreView, error) {
	if strings.TrimSpace(storeID) == "" {
		return stores.StoreView{}, ErrValidation
	}
	store, err := service.stores.FindAccessible(ctx, principal, strings.TrimSpace(storeID))
	if err == nil {
		return store, nil
	}
	if errors.Is(err, stores.ErrStoreNotFound) {
		return stores.StoreView{}, ErrStoreNotFound
	}
	if errors.Is(err, stores.ErrForbidden) || errors.Is(err, stores.ErrTenantForbidden) {
		return stores.StoreView{}, ErrStoreNotFound
	}
	return stores.StoreView{}, err
}

func canView(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionPlanningView) ||
			accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionPlanningEdit)
	}
	switch principal.Role {
	case auth.RolePlatformAdmin, auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing, auth.RoleManager:
		return true
	default:
		return false
	}
}

func canEdit(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionPlanningEdit)
	}
	switch principal.Role {
	case auth.RolePlatformAdmin, auth.RoleOwner, auth.RoleDirector, auth.RoleManager:
		return true
	default:
		return false
	}
}
