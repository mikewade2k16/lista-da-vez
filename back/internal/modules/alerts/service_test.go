package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
)

type fakeRepository struct {
	listInput         ListInput
	overviewInput     OverviewInput
	alerts            []Alert
	overview          Overview
	rules             RulesView
	updatedRules      RulesView
	updateRulesInput  UpdateRulesInput
	updateRulesTenant string
	operationalRules  OperationalRules
	processedSignals  []OperationalSignalInput
	signalMutations   []SignalMutation
	ruleDefinitions   []RuleDefinition
	createdRules      []CreateRuleInput
	updatedRuleInput  UpdateRuleInput
	deletedRuleID     string
}

func (repository *fakeRepository) List(_ context.Context, input ListInput) ([]Alert, error) {
	repository.listInput = input
	return append([]Alert{}, repository.alerts...), nil
}

func (repository *fakeRepository) Overview(_ context.Context, input OverviewInput) (Overview, error) {
	repository.overviewInput = input
	return repository.overview, nil
}

func (repository *fakeRepository) GetByID(_ context.Context, alertID string) (*Alert, error) {
	for _, alert := range repository.alerts {
		if alert.ID == alertID {
			copy := alert
			return &copy, nil
		}
	}
	return nil, ErrNotFound
}

func (repository *fakeRepository) LoadRules(_ context.Context, _ string) (RulesView, error) {
	if repository.rules.Source == "" {
		return defaultRules("tenant-1"), nil
	}
	return repository.rules, nil
}

func (repository *fakeRepository) UpsertRules(_ context.Context, tenantID string, _ string, input UpdateRulesInput) (RulesView, error) {
	repository.updateRulesTenant = tenantID
	repository.updateRulesInput = input
	if repository.updatedRules.Source == "" {
		now := time.Now().UTC()
		repository.updatedRules = RulesView{
			TenantID:                 tenantID,
			LongOpenServiceMinutes:   *input.LongOpenServiceMinutes,
			IdleStoreMinutes:         defaultIdleStoreMinutes,
			AfterClosingGraceMinutes: defaultAfterClosingGraceMinutes,
			NotifyDashboard:          true,
			NotifyOperationContext:   true,
			NotifyExternal:           false,
			Source:                   RulesSourceDatabase,
			UpdatedAt:                &now,
		}
	}
	return repository.updatedRules, nil
}

func (repository *fakeRepository) Acknowledge(_ context.Context, alertID string, actor Actor, note string) (*Alert, error) {
	alert, err := repository.GetByID(context.Background(), alertID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	alert.Status = StatusAcknowledged
	alert.AcknowledgedAt = &now
	alert.UpdatedAt = now
	alert.Metadata = map[string]any{"note": note, "actor": actor.DisplayName}
	return alert, nil
}

func (repository *fakeRepository) Resolve(_ context.Context, alertID string, actor Actor, note string) (*Alert, error) {
	alert, err := repository.GetByID(context.Background(), alertID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	alert.Status = StatusResolved
	alert.ResolvedAt = &now
	alert.UpdatedAt = now
	alert.Metadata = map[string]any{"note": note, "actor": actor.DisplayName}
	return alert, nil
}

func (repository *fakeRepository) LoadOperationalRules(_ context.Context, _ string) (OperationalRules, error) {
	if repository.operationalRules.LongOpenServiceMinutes < 1 {
		repository.operationalRules.LongOpenServiceMinutes = defaultLongOpenMinutes
		repository.operationalRules.NotifyDashboard = true
		repository.operationalRules.NotifyOperationContext = true
	}
	return repository.operationalRules, nil
}

func (repository *fakeRepository) ProcessOperationalSignals(_ context.Context, signals []OperationalSignalInput) ([]SignalMutation, error) {
	repository.processedSignals = append(repository.processedSignals, signals...)
	return append([]SignalMutation{}, repository.signalMutations...), nil
}

func (repository *fakeRepository) RespondToAlert(_ context.Context, input AlertRespondInput, _ Actor) (*Alert, error) {
	for i, alert := range repository.alerts {
		if alert.ID == input.AlertID {
			repository.alerts[i].InteractionResponse = input.Response
			repository.alerts[i].Status = StatusAcknowledged
			return &repository.alerts[i], nil
		}
	}
	return nil, ErrNotFound
}

func (repository *fakeRepository) MarkExternalNotified(_ context.Context, _ string) error {
	return nil
}

func (repository *fakeRepository) ListRules(_ context.Context, input ListRulesInput) ([]RuleDefinition, error) {
	rules := make([]RuleDefinition, 0, len(repository.ruleDefinitions))
	for _, rule := range repository.ruleDefinitions {
		if input.TenantID != "" && rule.TenantID != input.TenantID {
			continue
		}
		if input.TriggerType != "" && rule.TriggerType != input.TriggerType {
			continue
		}
		if input.OnlyActive && !rule.IsActive {
			continue
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (repository *fakeRepository) GetRule(_ context.Context, ruleID string) (*RuleDefinition, error) {
	for _, rule := range repository.ruleDefinitions {
		if rule.ID == ruleID {
			copy := rule
			return &copy, nil
		}
	}
	return nil, ErrNotFound
}

func (repository *fakeRepository) CreateRule(_ context.Context, input CreateRuleInput, _ Actor) (*RuleDefinition, error) {
	repository.createdRules = append(repository.createdRules, input)
	now := time.Now().UTC()
	rule := RuleDefinition{
		ID:                     "rule-created",
		TenantID:               input.TenantID,
		Name:                   input.Name,
		Description:            input.Description,
		IsActive:               input.IsActive,
		TriggerType:            input.TriggerType,
		ThresholdMinutes:       input.ThresholdMinutes,
		Severity:               input.Severity,
		DisplayKind:            input.DisplayKind,
		ColorTheme:             input.ColorTheme,
		TitleTemplate:          input.TitleTemplate,
		BodyTemplate:           input.BodyTemplate,
		InteractionKind:        input.InteractionKind,
		ResponseOptions:        append([]ResponseOption{}, input.ResponseOptions...),
		IsMandatory:            input.IsMandatory,
		NotifyDashboard:        input.NotifyDashboard,
		NotifyOperationContext: input.NotifyOperationContext,
		NotifyExternal:         input.NotifyExternal,
		ExternalChannel:        input.ExternalChannel,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	repository.ruleDefinitions = append(repository.ruleDefinitions, rule)
	return &rule, nil
}

func (repository *fakeRepository) UpdateRule(_ context.Context, ruleID string, input UpdateRuleInput, _ Actor) (*RuleDefinition, error) {
	repository.updatedRuleInput = input
	for i, rule := range repository.ruleDefinitions {
		if rule.ID != ruleID {
			continue
		}
		if input.IsActive != nil {
			rule.IsActive = *input.IsActive
		}
		if input.TriggerType != nil {
			rule.TriggerType = *input.TriggerType
		}
		rule.UpdatedAt = time.Now().UTC()
		repository.ruleDefinitions[i] = rule
		return &rule, nil
	}
	return nil, ErrNotFound
}

func (repository *fakeRepository) DeleteRule(_ context.Context, ruleID string) error {
	repository.deletedRuleID = ruleID
	return nil
}

func (repository *fakeRepository) LoadActiveRulesForTrigger(ctx context.Context, tenantID string, triggerType string) ([]RuleDefinition, error) {
	return repository.ListRules(ctx, ListRulesInput{TenantID: tenantID, TriggerType: triggerType, OnlyActive: true})
}

type fakeContextPublisher struct {
	resources []string
}

func (publisher *fakeContextPublisher) PublishContextEvent(_ context.Context, tenantID string, resource string, action string, resourceID string, _ time.Time) {
	publisher.resources = append(publisher.resources, tenantID+":"+resource+":"+action+":"+resourceID)
}

func TestListResolvesTenantAndStoreScope(t *testing.T) {
	repository := &fakeRepository{
		alerts: []Alert{{
			ID:              "alert-1",
			TenantID:        "tenant-1",
			StoreID:         "store-1",
			Type:            TypeLongOpenService,
			Category:        CategoryOperational,
			Severity:        SeverityCritical,
			Status:          StatusActive,
			Headline:        "Atendimento em aberto ha muito tempo",
			Body:            "O atendimento ultrapassou o limite configurado.",
			OpenedAt:        time.Now().UTC(),
			LastTriggeredAt: time.Now().UTC(),
		}},
	}
	service := NewService(repository)

	alerts, err := service.List(context.Background(), auth.Principal{
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
		StoreIDs: []string{"store-1", "store-2"},
	}, ListInput{
		StoreID:  "store-1",
		Status:   StatusActive,
		Type:     TypeLongOpenService,
		Category: CategoryOperational,
	})
	if err != nil {
		t.Fatalf("expected List to succeed, got %v", err)
	}
	if repository.listInput.TenantID != "tenant-1" {
		t.Fatalf("expected tenant-1, got %q", repository.listInput.TenantID)
	}
	if repository.listInput.StoreID != "store-1" {
		t.Fatalf("expected store-1, got %q", repository.listInput.StoreID)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected one alert, got %d", len(alerts))
	}
	if alerts[0].ID != "alert-1" {
		t.Fatalf("expected alert-1, got %q", alerts[0].ID)
	}
}

func TestListRejectsStoreOutsideScope(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, err := service.List(context.Background(), auth.Principal{
		Role:     auth.RoleManager,
		TenantID: "tenant-1",
		StoreIDs: []string{"store-1"},
	}, ListInput{StoreID: "store-2"})
	if err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestRulesRequireTenantForPlatformAdmin(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, err := service.Rules(context.Background(), auth.Principal{Role: auth.RolePlatformAdmin}, "")
	if err != ErrTenantRequired {
		t.Fatalf("expected ErrTenantRequired, got %v", err)
	}
}

func TestRulesReturnModuleDefaults(t *testing.T) {
	service := NewService(&fakeRepository{})

	rules, err := service.Rules(context.Background(), auth.Principal{
		Role:     auth.RoleStoreTerminal,
		TenantID: "tenant-1",
	}, "")
	if err != nil {
		t.Fatalf("expected Rules to succeed, got %v", err)
	}
	if rules.TenantID != "tenant-1" {
		t.Fatalf("expected tenant-1, got %q", rules.TenantID)
	}
	if rules.LongOpenServiceMinutes != defaultLongOpenMinutes {
		t.Fatalf("expected long open minutes %d, got %d", defaultLongOpenMinutes, rules.LongOpenServiceMinutes)
	}
	if rules.AfterClosingGraceMinutes != defaultAfterClosingGraceMinutes {
		t.Fatalf("expected after closing grace minutes %d, got %d", defaultAfterClosingGraceMinutes, rules.AfterClosingGraceMinutes)
	}
	if !rules.NotifyDashboard || !rules.NotifyOperationContext {
		t.Fatalf("expected dashboard and operation context notifications to default to true")
	}
	if rules.NotifyExternal {
		t.Fatalf("expected external delivery to default to false")
	}
	if rules.Source != "module-defaults" {
		t.Fatalf("expected source module-defaults, got %q", rules.Source)
	}
}

func TestListUsesAccessibleStoresWhenFilterIsEmpty(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository)

	_, err := service.List(context.Background(), auth.Principal{
		Role:     auth.RoleManager,
		TenantID: "tenant-1",
		StoreIDs: []string{"store-1", "store-2"},
	}, ListInput{})
	if err != nil {
		t.Fatalf("expected List to succeed, got %v", err)
	}
	if len(repository.listInput.StoreIDs) != 2 {
		t.Fatalf("expected two accessible stores, got %d", len(repository.listInput.StoreIDs))
	}
}

func TestUpdateRulesPublishesContextEvent(t *testing.T) {
	now := time.Now().UTC()
	repository := &fakeRepository{
		updatedRules: RulesView{
			TenantID:                 "tenant-1",
			LongOpenServiceMinutes:   35,
			IdleStoreMinutes:         defaultIdleStoreMinutes,
			AfterClosingGraceMinutes: defaultAfterClosingGraceMinutes,
			NotifyDashboard:          true,
			NotifyOperationContext:   true,
			NotifyExternal:           false,
			Source:                   RulesSourceDatabase,
			UpdatedAt:                &now,
		},
	}
	publisher := &fakeContextPublisher{}
	service := NewService(repository)
	service.SetContextPublisher(publisher)

	minutes := 35
	_, err := service.UpdateRules(context.Background(), auth.Principal{
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
		UserID:   "user-1",
	}, "", UpdateRulesInput{LongOpenServiceMinutes: &minutes})
	if err != nil {
		t.Fatalf("expected UpdateRules to succeed, got %v", err)
	}
	if repository.updateRulesTenant != "tenant-1" {
		t.Fatalf("expected tenant-1, got %q", repository.updateRulesTenant)
	}
	if len(publisher.resources) != 1 {
		t.Fatalf("expected one context event, got %d", len(publisher.resources))
	}
}

func TestReceiveOperationalSignalsPublishesContextEvents(t *testing.T) {
	repository := &fakeRepository{
		signalMutations: []SignalMutation{{
			TenantID: "tenant-1",
			AlertID:  "alert-1",
			Action:   "opened",
			SavedAt:  time.Now().UTC(),
		}},
	}
	publisher := &fakeContextPublisher{}
	service := NewService(repository)
	service.SetContextPublisher(publisher)

	err := service.ReceiveOperationalSignals(context.Background(), []operations.OperationalAlertSignal{{
		StoreID:      "store-1",
		ServiceID:    "service-1",
		ConsultantID: "consultant-1",
		SignalType:   operations.SignalLongOpenServiceTriggered,
		TriggeredAt:  time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("expected ReceiveOperationalSignals to succeed, got %v", err)
	}
	if len(repository.processedSignals) != 1 {
		t.Fatalf("expected one processed signal, got %d", len(repository.processedSignals))
	}
	if len(publisher.resources) != 1 {
		t.Fatalf("expected one context event, got %d", len(publisher.resources))
	}
}

func TestCreateRuleValidatesInteractionKind(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository)

	_, err := service.CreateRule(context.Background(), auth.Principal{
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
		UserID:   "user-1",
	}, CreateRuleInput{
		TenantID:         "tenant-1",
		Name:             "Test Rule",
		TriggerType:      TriggerLongOpenService,
		ThresholdMinutes: 5,
		Severity:         SeverityCritical,
		DisplayKind:      DisplayKindBanner,
		ColorTheme:       ColorThemeAmber,
		TitleTemplate:    "Test",
		BodyTemplate:     "Test body",
		InteractionKind:  InteractionKindConfirmChoice,
		ResponseOptions:  []ResponseOption{}, // Too few options
		IsMandatory:      false,
	})
	if err != ErrValidation {
		t.Fatalf("expected ErrValidation for confirm_choice with 0 options, got %v", err)
	}
}

func TestCreateRuleValidatesMandatoryInteraction(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository)

	_, err := service.CreateRule(context.Background(), auth.Principal{
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
		UserID:   "user-1",
	}, CreateRuleInput{
		TenantID:         "tenant-1",
		Name:             "Test Rule",
		TriggerType:      TriggerLongOpenService,
		ThresholdMinutes: 5,
		Severity:         SeverityCritical,
		DisplayKind:      DisplayKindBanner,
		ColorTheme:       ColorThemeAmber,
		TitleTemplate:    "Test",
		BodyTemplate:     "Test body",
		InteractionKind:  InteractionKindNone,
		ResponseOptions:  []ResponseOption{},
		IsMandatory:      true, // Invalid: mandatory with none interaction
	})
	if err != ErrValidation {
		t.Fatalf("expected ErrValidation for mandatory with none interaction, got %v", err)
	}
}
