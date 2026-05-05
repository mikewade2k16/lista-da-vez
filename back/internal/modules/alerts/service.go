package alerts

import (
	"context"
	"slices"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
)

type Service struct {
	repository        Repository
	notifier          ContextPublisher
	operationsScanner OperationsScanner
}

type ContextPublisher interface {
	PublishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string, savedAt time.Time)
}

type noopRepository struct{}

func NewService(repository Repository) *Service {
	if repository == nil {
		repository = noopRepository{}
	}

	return &Service{repository: repository}
}

func (service *Service) SetContextPublisher(notifier ContextPublisher) {
	service.notifier = notifier
}

func (service *Service) List(ctx context.Context, principal auth.Principal, input ListInput) ([]AlertView, error) {
	if !canViewAlerts(principal) {
		return nil, ErrForbidden
	}

	tenantID, err := resolveTenantScope(principal, input.TenantID)
	if err != nil {
		return nil, err
	}

	storeID, storeIDs, err := resolveStoreScope(principal, input.StoreID)
	if err != nil {
		return nil, err
	}

	alerts, err := service.repository.List(ctx, ListInput{
		TenantID: tenantID,
		StoreID:  storeID,
		StoreIDs: storeIDs,
		Status:   strings.TrimSpace(input.Status),
		Type:     strings.TrimSpace(input.Type),
		Category: strings.TrimSpace(input.Category),
	})
	if err != nil {
		return nil, err
	}

	views := make([]AlertView, 0, len(alerts))
	for _, alert := range alerts {
		views = append(views, alert.View())
	}

	return views, nil
}

func (service *Service) Overview(ctx context.Context, principal auth.Principal, input OverviewInput) (Overview, error) {
	if !canViewAlerts(principal) {
		return Overview{}, ErrForbidden
	}

	tenantID, err := resolveTenantScope(principal, input.TenantID)
	if err != nil {
		return Overview{}, err
	}

	storeID, storeIDs, err := resolveStoreScope(principal, input.StoreID)
	if err != nil {
		return Overview{}, err
	}

	overview, err := service.repository.Overview(ctx, OverviewInput{
		TenantID: tenantID,
		StoreID:  storeID,
		StoreIDs: storeIDs,
	})
	if err != nil {
		return Overview{}, err
	}

	if overview.TenantID == "" {
		overview.TenantID = tenantID
	}
	if overview.StoreID == "" {
		overview.StoreID = storeID
	}

	return overview, nil
}

func (service *Service) Rules(ctx context.Context, principal auth.Principal, requestedTenantID string) (RulesView, error) {
	if !canViewAlerts(principal) {
		return RulesView{}, ErrForbidden
	}

	tenantID, err := resolveTenantScope(principal, requestedTenantID)
	if err != nil {
		return RulesView{}, err
	}

	return service.repository.LoadRules(ctx, tenantID)
}

func (service *Service) UpdateRules(ctx context.Context, principal auth.Principal, requestedTenantID string, input UpdateRulesInput) (RulesView, error) {
	if !canManageAlertRules(principal) {
		return RulesView{}, ErrForbidden
	}

	if err := validateRulesInput(input); err != nil {
		return RulesView{}, err
	}

	tenantID, err := resolveTenantScope(principal, requestedTenantID)
	if err != nil {
		return RulesView{}, err
	}

	rules, err := service.repository.UpsertRules(ctx, tenantID, principal.UserID, input)
	if err != nil {
		return RulesView{}, err
	}

	service.publishContextEvent(ctx, tenantID, "rules-updated", tenantID, derefTime(rules.UpdatedAt))
	return rules, nil
}

func (service *Service) FindByID(ctx context.Context, principal auth.Principal, alertID string) (*AlertView, error) {
	if !canViewAlerts(principal) {
		return nil, ErrForbidden
	}

	alert, err := service.repository.GetByID(ctx, strings.TrimSpace(alertID))
	if err != nil {
		return nil, err
	}
	if !canAccessAlert(principal, alert) {
		return nil, ErrForbidden
	}

	view := alert.View()
	return &view, nil
}

func (service *Service) Acknowledge(ctx context.Context, principal auth.Principal, alertID string, note string) (*AlertView, error) {
	if !canManageAlertActions(principal) {
		return nil, ErrForbidden
	}

	alert, err := service.repository.GetByID(ctx, strings.TrimSpace(alertID))
	if err != nil {
		return nil, err
	}
	if !canAccessAlert(principal, alert) {
		return nil, ErrForbidden
	}

	updated, err := service.repository.Acknowledge(ctx, alert.ID, Actor{
		UserID:      principal.UserID,
		DisplayName: principal.DisplayName,
	}, strings.TrimSpace(note))
	if err != nil {
		return nil, err
	}

	service.publishContextEvent(ctx, updated.TenantID, "acknowledged", updated.ID, updated.UpdatedAt)
	view := updated.View()
	return &view, nil
}

func (service *Service) Resolve(ctx context.Context, principal auth.Principal, alertID string, note string) (*AlertView, error) {
	if !canManageAlertActions(principal) {
		return nil, ErrForbidden
	}

	alert, err := service.repository.GetByID(ctx, strings.TrimSpace(alertID))
	if err != nil {
		return nil, err
	}
	if !canAccessAlert(principal, alert) {
		return nil, ErrForbidden
	}

	updated, err := service.repository.Resolve(ctx, alert.ID, Actor{
		UserID:      principal.UserID,
		DisplayName: principal.DisplayName,
	}, strings.TrimSpace(note))
	if err != nil {
		return nil, err
	}

	service.publishContextEvent(ctx, updated.TenantID, "resolved", updated.ID, updated.UpdatedAt)
	view := updated.View()
	return &view, nil
}

func (service *Service) RespondToAlert(ctx context.Context, principal auth.Principal, alertID string, response string) (*AlertRespondResult, error) {
	if !canRespondToAlert(principal) {
		return nil, ErrForbidden
	}

	alert, err := service.repository.GetByID(ctx, strings.TrimSpace(alertID))
	if err != nil {
		return nil, err
	}
	if !canAccessAlert(principal, alert) {
		return nil, ErrForbidden
	}

	updated, err := service.repository.RespondToAlert(ctx, AlertRespondInput{
		AlertID:  alert.ID,
		TenantID: alert.TenantID,
		Response: strings.TrimSpace(response),
	}, Actor{
		UserID:      principal.UserID,
		DisplayName: principal.DisplayName,
	})
	if err != nil {
		return nil, err
	}

	service.publishContextEvent(ctx, updated.TenantID, "responded", updated.ID, updated.UpdatedAt)

	if updated.ExternalNotifiedAt == nil {
		rules, rulesErr := service.repository.LoadOperationalRules(ctx, updated.StoreID)
		if rulesErr == nil && rules.NotifyExternal {
			_ = service.repository.MarkExternalNotified(ctx, updated.ID)
		}
	}

	openFinishModal := strings.TrimSpace(response) == InteractionResponseForgotten
	return &AlertRespondResult{
		Alert:           *updated,
		OpenFinishModal: openFinishModal,
		ServiceID:       updated.ServiceID,
	}, nil
}

func (service *Service) LoadOperationalRules(ctx context.Context, storeID string) (operations.OperationalAlertRules, error) {
	rules, err := service.repository.LoadOperationalRules(ctx, strings.TrimSpace(storeID))
	if err != nil {
		return operations.OperationalAlertRules{}, err
	}

	minutes := rules.LongOpenServiceMinutes
	loadedDynamicDefinitions := false
	if strings.TrimSpace(rules.TenantID) != "" {
		if definitions, definitionsErr := service.repository.LoadActiveRulesForTrigger(ctx, rules.TenantID, TriggerLongOpenService); definitionsErr == nil && len(definitions) > 0 {
			loadedDynamicDefinitions = true
			minutes = 0
			for _, definition := range definitions {
				if definition.ThresholdMinutes < 1 {
					continue
				}
				if minutes < 1 || definition.ThresholdMinutes < minutes {
					minutes = definition.ThresholdMinutes
				}
			}
		} else if definitionsErr == nil {
			loadedDynamicDefinitions = true
			minutes = 0
		}
	}
	if minutes < 1 {
		if loadedDynamicDefinitions {
			return operations.OperationalAlertRules{
				LongOpenServiceMinutes: 0,
				NotifyDashboard:        rules.NotifyDashboard,
				NotifyOperationContext: rules.NotifyOperationContext,
			}, nil
		}
		minutes = defaultLongOpenMinutes
	}

	return operations.OperationalAlertRules{
		LongOpenServiceMinutes: minutes,
		NotifyDashboard:        rules.NotifyDashboard,
		NotifyOperationContext: rules.NotifyOperationContext,
	}, nil
}

func (service *Service) ReceiveOperationalSignals(ctx context.Context, signals []operations.OperationalAlertSignal) error {
	if len(signals) == 0 {
		return nil
	}

	inputs := service.operationalSignalsToInputs(signals)

	mutations, err := service.repository.ProcessOperationalSignals(ctx, inputs)
	if err != nil {
		return err
	}

	for _, mutation := range mutations {
		service.publishContextEvent(ctx, mutation.TenantID, mutation.Action, mutation.AlertID, mutation.SavedAt)
	}

	return nil
}

func (service *Service) ListRules(ctx context.Context, principal auth.Principal, input ListRulesInput) ([]RuleDefinitionView, error) {
	if !canManageAlertRules(principal) {
		return nil, ErrForbidden
	}

	tenantID, err := resolveTenantScope(principal, input.TenantID)
	if err != nil {
		return nil, err
	}

	rules, err := service.repository.ListRules(ctx, ListRulesInput{
		TenantID:    tenantID,
		TriggerType: strings.TrimSpace(input.TriggerType),
		OnlyActive:  input.OnlyActive,
	})
	if err != nil {
		return nil, err
	}

	views := make([]RuleDefinitionView, 0, len(rules))
	for _, rule := range rules {
		views = append(views, rule.View())
	}

	return views, nil
}

func (service *Service) GetRule(ctx context.Context, principal auth.Principal, ruleID string) (*RuleDefinitionView, error) {
	if !canManageAlertRules(principal) {
		return nil, ErrForbidden
	}

	rule, err := service.repository.GetRule(ctx, strings.TrimSpace(ruleID))
	if err != nil {
		return nil, err
	}

	tenantID, err := resolveTenantScope(principal, rule.TenantID)
	if err != nil {
		return nil, err
	}
	if tenantID != rule.TenantID {
		return nil, ErrForbidden
	}

	view := rule.View()
	return &view, nil
}

func (service *Service) CreateRule(ctx context.Context, principal auth.Principal, input CreateRuleInput) (*RuleDefinitionView, error) {
	if !canManageAlertRules(principal) {
		return nil, ErrForbidden
	}

	if err := validateRuleInput(input); err != nil {
		return nil, err
	}

	tenantID, err := resolveTenantScope(principal, input.TenantID)
	if err != nil {
		return nil, err
	}

	rule, err := service.repository.CreateRule(ctx, CreateRuleInput{
		TenantID:               tenantID,
		Name:                   strings.TrimSpace(input.Name),
		Description:            strings.TrimSpace(input.Description),
		IsActive:               input.IsActive,
		TriggerType:            strings.TrimSpace(input.TriggerType),
		ThresholdMinutes:       input.ThresholdMinutes,
		Severity:               strings.TrimSpace(input.Severity),
		DisplayKind:            strings.TrimSpace(input.DisplayKind),
		ColorTheme:             NormalizeColorTheme(input.ColorTheme),
		TitleTemplate:          strings.TrimSpace(input.TitleTemplate),
		BodyTemplate:           strings.TrimSpace(input.BodyTemplate),
		InteractionKind:        strings.TrimSpace(input.InteractionKind),
		ResponseOptions:        input.ResponseOptions,
		IsMandatory:            input.IsMandatory,
		NotifyDashboard:        input.NotifyDashboard,
		NotifyOperationContext: input.NotifyOperationContext,
		NotifyExternal:         input.NotifyExternal,
		ExternalChannel:        strings.TrimSpace(input.ExternalChannel),
	}, Actor{
		UserID:      principal.UserID,
		DisplayName: principal.DisplayName,
	})
	if err != nil {
		return nil, err
	}

	service.publishContextEvent(ctx, tenantID, "rule-created", rule.ID, rule.UpdatedAt)
	view := rule.View()
	return &view, nil
}

func (service *Service) UpdateRule(ctx context.Context, principal auth.Principal, ruleID string, input UpdateRuleInput) (*RuleDefinitionView, error) {
	if !canManageAlertRules(principal) {
		return nil, ErrForbidden
	}

	rule, err := service.repository.GetRule(ctx, strings.TrimSpace(ruleID))
	if err != nil {
		return nil, err
	}

	tenantID, err := resolveTenantScope(principal, rule.TenantID)
	if err != nil {
		return nil, err
	}
	if tenantID != rule.TenantID {
		return nil, ErrForbidden
	}
	if input.ColorTheme != nil {
		normalizedColorTheme := NormalizeColorTheme(*input.ColorTheme)
		if normalizedColorTheme == "" {
			return nil, ErrValidation
		}
		input.ColorTheme = &normalizedColorTheme
	}

	updated, err := service.repository.UpdateRule(ctx, rule.ID, input, Actor{
		UserID:      principal.UserID,
		DisplayName: principal.DisplayName,
	})
	if err != nil {
		return nil, err
	}

	service.publishContextEvent(ctx, tenantID, "rule-updated", updated.ID, updated.UpdatedAt)
	view := updated.View()
	return &view, nil
}

func (service *Service) DeleteRule(ctx context.Context, principal auth.Principal, ruleID string) error {
	if !canManageAlertRules(principal) {
		return ErrForbidden
	}

	rule, err := service.repository.GetRule(ctx, strings.TrimSpace(ruleID))
	if err != nil {
		return err
	}

	tenantID, err := resolveTenantScope(principal, rule.TenantID)
	if err != nil {
		return err
	}
	if tenantID != rule.TenantID {
		return ErrForbidden
	}

	err = service.repository.DeleteRule(ctx, rule.ID)
	if err != nil {
		return err
	}

	service.publishContextEvent(ctx, tenantID, "rule-deleted", rule.ID, time.Now().UTC())
	return nil
}

func (service *Service) SetOperationsScanner(scanner OperationsScanner) {
	service.operationsScanner = scanner
}

func (service *Service) ApplyRuleNow(ctx context.Context, principal auth.Principal, ruleID string) (int, error) {
	if !canManageAlertRules(principal) {
		return 0, ErrForbidden
	}

	rule, err := service.repository.GetRule(ctx, strings.TrimSpace(ruleID))
	if err != nil {
		return 0, err
	}

	tenantID, err := resolveTenantScope(principal, rule.TenantID)
	if err != nil {
		return 0, err
	}
	if tenantID != rule.TenantID {
		return 0, ErrForbidden
	}
	if !rule.IsActive {
		return 0, nil
	}

	if service.operationsScanner == nil {
		return 0, nil
	}

	result, err := service.operationsScanner.ScanForRule(ctx, rule.ID, rule.TriggerType, rule.TenantID, rule.ThresholdMinutes)
	if err != nil {
		return 0, err
	}

	signals := service.operationalSignalsToInputs(result)

	mutations, err := service.repository.ProcessOperationalSignals(ctx, signals)
	if err != nil {
		return 0, err
	}

	for _, mutation := range mutations {
		service.publishContextEvent(ctx, mutation.TenantID, mutation.Action, mutation.AlertID, mutation.SavedAt)
	}

	return len(mutations), nil
}

func (service *Service) operationalSignalsToInputs(signals []operations.OperationalAlertSignal) []OperationalSignalInput {
	inputs := make([]OperationalSignalInput, 0, len(signals))
	for _, signal := range signals {
		storeID := strings.TrimSpace(signal.StoreID)
		serviceID := strings.TrimSpace(signal.ServiceID)
		signalType := strings.TrimSpace(signal.SignalType)
		if storeID == "" || serviceID == "" || signalType == "" {
			continue
		}

		triggeredAt := signal.TriggeredAt.UTC()
		if triggeredAt.IsZero() {
			triggeredAt = time.Now().UTC()
		}

		inputs = append(inputs, OperationalSignalInput{
			TenantID:       strings.TrimSpace(signal.TenantID),
			StoreID:        storeID,
			ServiceID:      serviceID,
			ConsultantID:   strings.TrimSpace(signal.ConsultantID),
			SignalType:     signalType,
			TriggeredAt:    triggeredAt,
			Metadata:       cloneMetadata(signal.Metadata),
			ConsultantName: strings.TrimSpace(signal.ConsultantName),
			ElapsedMinutes: signal.ElapsedMinutes,
			TriggerType:    strings.TrimSpace(signal.TriggerType),
		})
	}

	return inputs
}

func canViewAlerts(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionAlertsView) ||
			accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionAlertsEdit)
	}

	switch principal.Role {
	case auth.RolePlatformAdmin, auth.RoleOwner, auth.RoleManager, auth.RoleStoreTerminal:
		return true
	default:
		return false
	}
}

func canManageAlertRules(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionAlertsRulesManage) ||
			accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionAlertsEdit)
	}

	return principal.Role == auth.RolePlatformAdmin || principal.Role == auth.RoleOwner
}

func canRespondToAlert(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionAlertsActionsManage) ||
			accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionAlertsEdit)
	}

	switch principal.Role {
	case auth.RolePlatformAdmin, auth.RoleOwner, auth.RoleManager, auth.RoleStoreTerminal, auth.RoleConsultant:
		return true
	default:
		return false
	}
}

func canManageAlertActions(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionAlertsActionsManage) ||
			accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionAlertsEdit)
	}

	switch principal.Role {
	case auth.RolePlatformAdmin, auth.RoleOwner, auth.RoleManager, auth.RoleStoreTerminal:
		return true
	default:
		return false
	}
}

func resolveTenantScope(principal auth.Principal, requestedTenantID string) (string, error) {
	normalizedTenantID := strings.TrimSpace(requestedTenantID)

	if principal.Role == auth.RolePlatformAdmin {
		if normalizedTenantID == "" {
			return "", ErrTenantRequired
		}
		return normalizedTenantID, nil
	}

	principalTenantID := strings.TrimSpace(principal.TenantID)
	if principalTenantID == "" {
		return "", ErrTenantRequired
	}
	if normalizedTenantID != "" && normalizedTenantID != principalTenantID {
		return "", ErrForbidden
	}

	return principalTenantID, nil
}

func resolveStoreScope(principal auth.Principal, requestedStoreID string) (string, []string, error) {
	normalizedStoreID := strings.TrimSpace(requestedStoreID)
	if normalizedStoreID == "" {
		if principal.Role == auth.RolePlatformAdmin || len(principal.StoreIDs) == 0 {
			return "", nil, nil
		}

		return "", normalizeStoreIDs(principal.StoreIDs), nil
	}
	if principal.Role == auth.RolePlatformAdmin {
		return normalizedStoreID, nil, nil
	}
	if len(principal.StoreIDs) == 0 {
		return normalizedStoreID, nil, nil
	}
	if !slices.Contains(principal.StoreIDs, normalizedStoreID) {
		return "", nil, ErrForbidden
	}

	return normalizedStoreID, nil, nil
}

func canAccessAlert(principal auth.Principal, alert *Alert) bool {
	if alert == nil {
		return false
	}

	if principal.Role == auth.RolePlatformAdmin {
		return true
	}
	if strings.TrimSpace(principal.TenantID) == "" || strings.TrimSpace(alert.TenantID) != strings.TrimSpace(principal.TenantID) {
		return false
	}
	if len(principal.StoreIDs) == 0 {
		return true
	}

	return slices.Contains(principal.StoreIDs, strings.TrimSpace(alert.StoreID))
}

func validateRulesInput(input UpdateRulesInput) error {
	if input.LongOpenServiceMinutes != nil && *input.LongOpenServiceMinutes < 1 {
		return ErrValidation
	}
	if input.IdleStoreMinutes != nil && *input.IdleStoreMinutes < 1 {
		return ErrValidation
	}
	if input.AfterClosingGraceMinutes != nil && *input.AfterClosingGraceMinutes < 0 {
		return ErrValidation
	}

	return nil
}

func validateRuleInput(input CreateRuleInput) error {
	if strings.TrimSpace(input.TenantID) == "" {
		return ErrValidation
	}
	if strings.TrimSpace(input.Name) == "" {
		return ErrValidation
	}
	if strings.TrimSpace(input.TriggerType) == "" {
		return ErrValidation
	}
	if input.ThresholdMinutes < 1 {
		return ErrValidation
	}
	if strings.TrimSpace(input.Severity) == "" {
		return ErrValidation
	}
	if strings.TrimSpace(input.DisplayKind) == "" {
		return ErrValidation
	}
	if NormalizeColorTheme(input.ColorTheme) == "" {
		return ErrValidation
	}
	if strings.TrimSpace(input.TitleTemplate) == "" {
		return ErrValidation
	}
	if strings.TrimSpace(input.InteractionKind) == "" {
		return ErrValidation
	}

	// Se interaction_kind requer resposta, responseOptions deve ter ≥ 2 itens
	if strings.TrimSpace(input.InteractionKind) == InteractionKindConfirmChoice ||
		strings.TrimSpace(input.InteractionKind) == InteractionKindSelectOption {
		if len(input.ResponseOptions) < 2 {
			return ErrValidation
		}
	}

	// Se isMandatory, interactionKind não pode ser "none"
	if input.IsMandatory && strings.TrimSpace(input.InteractionKind) == InteractionKindNone {
		return ErrValidation
	}

	return nil
}

func normalizeStoreIDs(storeIDs []string) []string {
	normalized := make([]string, 0, len(storeIDs))
	seen := make(map[string]struct{}, len(storeIDs))
	for _, storeID := range storeIDs {
		trimmed := strings.TrimSpace(storeID)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	return normalized
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		cloned[trimmedKey] = value
	}

	return cloned
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Now().UTC()
	}

	return value.UTC()
}

func (service *Service) publishContextEvent(ctx context.Context, tenantID string, action string, resourceID string, savedAt time.Time) {
	if service.notifier == nil || strings.TrimSpace(tenantID) == "" {
		return
	}

	if savedAt.IsZero() {
		savedAt = time.Now().UTC()
	}

	service.notifier.PublishContextEvent(ctx, tenantID, "alerts", strings.TrimSpace(action), strings.TrimSpace(resourceID), savedAt.UTC())
}

func (noopRepository) List(context.Context, ListInput) ([]Alert, error) {
	return []Alert{}, nil
}

func (noopRepository) Overview(_ context.Context, input OverviewInput) (Overview, error) {
	return Overview{
		TenantID: strings.TrimSpace(input.TenantID),
		StoreID:  strings.TrimSpace(input.StoreID),
	}, nil
}

func (noopRepository) GetByID(context.Context, string) (*Alert, error) {
	return nil, ErrNotFound
}

func (noopRepository) LoadRules(_ context.Context, tenantID string) (RulesView, error) {
	return RulesView{
		TenantID:                 strings.TrimSpace(tenantID),
		LongOpenServiceMinutes:   defaultLongOpenMinutes,
		IdleStoreMinutes:         defaultIdleStoreMinutes,
		AfterClosingGraceMinutes: defaultAfterClosingGraceMinutes,
		NotifyDashboard:          true,
		NotifyOperationContext:   true,
		NotifyExternal:           false,
		Source:                   RulesSourceDefaults,
	}, nil
}

func (noopRepository) UpsertRules(ctx context.Context, tenantID string, _ string, input UpdateRulesInput) (RulesView, error) {
	current, err := noopRepository{}.LoadRules(ctx, tenantID)
	if err != nil {
		return RulesView{}, err
	}
	if input.LongOpenServiceMinutes != nil {
		current.LongOpenServiceMinutes = *input.LongOpenServiceMinutes
	}
	if input.IdleStoreMinutes != nil {
		current.IdleStoreMinutes = *input.IdleStoreMinutes
	}
	if input.AfterClosingGraceMinutes != nil {
		current.AfterClosingGraceMinutes = *input.AfterClosingGraceMinutes
	}
	if input.NotifyDashboard != nil {
		current.NotifyDashboard = *input.NotifyDashboard
	}
	if input.NotifyOperationContext != nil {
		current.NotifyOperationContext = *input.NotifyOperationContext
	}
	if input.NotifyExternal != nil {
		current.NotifyExternal = *input.NotifyExternal
	}
	now := time.Now().UTC()
	current.Source = RulesSourceDatabase
	current.UpdatedAt = &now
	return current, nil
}

func (noopRepository) Acknowledge(context.Context, string, Actor, string) (*Alert, error) {
	return nil, ErrNotFound
}

func (noopRepository) Resolve(context.Context, string, Actor, string) (*Alert, error) {
	return nil, ErrNotFound
}

func (noopRepository) RespondToAlert(context.Context, AlertRespondInput, Actor) (*Alert, error) {
	return nil, ErrNotFound
}

func (noopRepository) MarkExternalNotified(context.Context, string) error {
	return nil
}

func (noopRepository) LoadOperationalRules(context.Context, string) (OperationalRules, error) {
	return OperationalRules{
		LongOpenServiceMinutes: defaultLongOpenMinutes,
		NotifyDashboard:        true,
		NotifyOperationContext: true,
	}, nil
}

func (noopRepository) ProcessOperationalSignals(context.Context, []OperationalSignalInput) ([]SignalMutation, error) {
	return nil, nil
}

func (noopRepository) ListRules(context.Context, ListRulesInput) ([]RuleDefinition, error) {
	return nil, nil
}

func (noopRepository) GetRule(context.Context, string) (*RuleDefinition, error) {
	return nil, nil
}

func (noopRepository) CreateRule(context.Context, CreateRuleInput, Actor) (*RuleDefinition, error) {
	return nil, nil
}

func (noopRepository) UpdateRule(context.Context, string, UpdateRuleInput, Actor) (*RuleDefinition, error) {
	return nil, nil
}

func (noopRepository) DeleteRule(context.Context, string) error {
	return nil
}

func (noopRepository) LoadActiveRulesForTrigger(context.Context, string, string) ([]RuleDefinition, error) {
	return nil, nil
}
