package settings

import (
	"context"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type fakeRepository struct {
	defaultTenantID     string
	resolveErr          error
	accessible          map[string]bool
	records             map[string]Record
	operationSections   map[string]OperationSectionRecord
	modalSections       map[string]ModalSectionRecord
	optionGroups        map[string]map[string][]OptionItem
	productCatalogs     map[string][]ProductItem
	savedAt             time.Time
	upsertOptionCalls   int
	replaceOptionCalls  int
	upsertProductCalls  int
	replaceProductCalls int
	applyTemplateCalls  int

	lastSavedRecord           Record
	lastSavedOperationSection OperationSectionRecord
	lastSavedModalSection     ModalSectionRecord
	lastAppliedTemplate       OperationTemplateApplyRecord
	lastReplacedOptionTenant  string
	lastReplacedOptionKind    string
	lastReplacedOptionItems   []OptionItem
	lastUpsertedOptionTenant  string
	lastUpsertedOptionKind    string
	lastUpsertedOption        OptionItem
	lastDeletedOptionTenant   string
	lastDeletedOptionKind     string
	lastDeletedOptionID       string
	lastReplacedProductTenant string
	lastReplacedProducts      []ProductItem
	lastUpsertedProductTenant string
	lastUpsertedProduct       ProductItem
	lastDeletedProductTenant  string
	lastDeletedProductID      string
}

func (repository *fakeRepository) mutationTime() time.Time {
	if repository.savedAt.IsZero() {
		return time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	}

	return repository.savedAt
}

func (repository *fakeRepository) TenantExists(context.Context, string) (bool, error) {
	return true, nil
}

func (repository *fakeRepository) CanAccessTenant(_ context.Context, _ auth.Principal, tenantID string) (bool, error) {
	if repository.accessible == nil {
		return true, nil
	}

	return repository.accessible[tenantID], nil
}

func (repository *fakeRepository) ResolveDefaultTenantID(context.Context, auth.Principal) (string, error) {
	if repository.resolveErr != nil {
		return "", repository.resolveErr
	}

	if repository.defaultTenantID == "" {
		return "", ErrTenantRequired
	}

	return repository.defaultTenantID, nil
}

func (repository *fakeRepository) GetByTenant(_ context.Context, tenantID string) (Record, bool, error) {
	record, ok := repository.records[tenantID]
	return record, ok, nil
}

func (repository *fakeRepository) GetOperationSection(_ context.Context, tenantID string) (OperationSectionRecord, bool, error) {
	if repository.operationSections != nil {
		if section, ok := repository.operationSections[tenantID]; ok {
			return section, true, nil
		}
	}

	record, ok := repository.records[tenantID]
	if !ok {
		return OperationSectionRecord{}, false, nil
	}

	return recordToOperationSection(record), true, nil
}

func (repository *fakeRepository) GetModalSection(_ context.Context, tenantID string) (ModalSectionRecord, bool, error) {
	if repository.modalSections != nil {
		if section, ok := repository.modalSections[tenantID]; ok {
			return section, true, nil
		}
	}

	record, ok := repository.records[tenantID]
	if !ok {
		return ModalSectionRecord{}, false, nil
	}

	return recordToModalSection(record), true, nil
}

func (repository *fakeRepository) GetOptionGroup(_ context.Context, tenantID string, kind string) ([]OptionItem, error) {
	if repository.optionGroups == nil {
		return nil, nil
	}

	return cloneOptions(repository.optionGroups[tenantID][kind]), nil
}

func (repository *fakeRepository) GetProductCatalog(_ context.Context, tenantID string) ([]ProductItem, error) {
	if repository.productCatalogs == nil {
		return nil, nil
	}

	return cloneProducts(repository.productCatalogs[tenantID]), nil
}

func (repository *fakeRepository) Upsert(_ context.Context, record Record) (Record, error) {
	record.UpdatedAt = repository.mutationTime()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = record.UpdatedAt
	}

	repository.lastSavedRecord = record
	if repository.records == nil {
		repository.records = make(map[string]Record)
	}
	repository.records[record.TenantID] = record

	return record, nil
}

func (repository *fakeRepository) UpsertOperationSection(_ context.Context, section OperationSectionRecord) (OperationSectionRecord, error) {
	section.UpdatedAt = repository.mutationTime()
	if section.CreatedAt.IsZero() {
		section.CreatedAt = section.UpdatedAt
	}

	repository.lastSavedOperationSection = section
	if repository.operationSections == nil {
		repository.operationSections = make(map[string]OperationSectionRecord)
	}
	repository.operationSections[section.TenantID] = section

	return section, nil
}

func (repository *fakeRepository) UpsertModalSection(_ context.Context, section ModalSectionRecord) (ModalSectionRecord, error) {
	section.UpdatedAt = repository.mutationTime()
	if section.CreatedAt.IsZero() {
		section.CreatedAt = section.UpdatedAt
	}

	repository.lastSavedModalSection = section
	if repository.modalSections == nil {
		repository.modalSections = make(map[string]ModalSectionRecord)
	}
	repository.modalSections[section.TenantID] = section

	return section, nil
}

func (repository *fakeRepository) ApplyOperationTemplate(_ context.Context, record OperationTemplateApplyRecord) (time.Time, error) {
	repository.applyTemplateCalls++
	repository.lastAppliedTemplate = record

	if repository.operationSections == nil {
		repository.operationSections = make(map[string]OperationSectionRecord)
	}
	if repository.modalSections == nil {
		repository.modalSections = make(map[string]ModalSectionRecord)
	}
	if repository.optionGroups == nil {
		repository.optionGroups = make(map[string]map[string][]OptionItem)
	}
	if repository.optionGroups[record.TenantID] == nil {
		repository.optionGroups[record.TenantID] = make(map[string][]OptionItem)
	}

	record.OperationSection.UpdatedAt = repository.mutationTime()
	record.ModalSection.UpdatedAt = repository.mutationTime()
	repository.operationSections[record.TenantID] = record.OperationSection
	repository.modalSections[record.TenantID] = record.ModalSection
	repository.optionGroups[record.TenantID][optionKindVisitReason] = cloneOptions(record.VisitReasonOptions)
	repository.optionGroups[record.TenantID][optionKindCustomerSource] = cloneOptions(record.CustomerSourceOptions)

	return repository.mutationTime(), nil
}

func (repository *fakeRepository) ReplaceOptionGroup(_ context.Context, tenantID string, kind string, options []OptionItem) (time.Time, error) {
	repository.replaceOptionCalls++
	repository.lastReplacedOptionTenant = tenantID
	repository.lastReplacedOptionKind = kind
	repository.lastReplacedOptionItems = cloneOptions(options)

	if repository.optionGroups == nil {
		repository.optionGroups = make(map[string]map[string][]OptionItem)
	}
	if repository.optionGroups[tenantID] == nil {
		repository.optionGroups[tenantID] = make(map[string][]OptionItem)
	}
	repository.optionGroups[tenantID][kind] = cloneOptions(options)

	return repository.mutationTime(), nil
}

func (repository *fakeRepository) UpsertOption(_ context.Context, tenantID string, kind string, option OptionItem) (time.Time, error) {
	repository.upsertOptionCalls++
	repository.lastUpsertedOptionTenant = tenantID
	repository.lastUpsertedOptionKind = kind
	repository.lastUpsertedOption = option

	if repository.optionGroups == nil {
		repository.optionGroups = make(map[string]map[string][]OptionItem)
	}
	if repository.optionGroups[tenantID] == nil {
		repository.optionGroups[tenantID] = make(map[string][]OptionItem)
	}

	nextItems, _ := upsertOptionGroupItem(repository.optionGroups[tenantID][kind], option)
	repository.optionGroups[tenantID][kind] = cloneOptions(nextItems)

	return repository.mutationTime(), nil
}

func (repository *fakeRepository) DeleteOption(_ context.Context, tenantID string, kind string, optionID string) (time.Time, error) {
	repository.lastDeletedOptionTenant = tenantID
	repository.lastDeletedOptionKind = kind
	repository.lastDeletedOptionID = optionID

	if repository.optionGroups != nil && repository.optionGroups[tenantID] != nil {
		repository.optionGroups[tenantID][kind] = removeOptionGroupItem(repository.optionGroups[tenantID][kind], optionID)
	}

	return repository.mutationTime(), nil
}

func (repository *fakeRepository) ReplaceProducts(_ context.Context, tenantID string, products []ProductItem) (time.Time, error) {
	repository.replaceProductCalls++
	repository.lastReplacedProductTenant = tenantID
	repository.lastReplacedProducts = cloneProducts(products)

	if repository.productCatalogs == nil {
		repository.productCatalogs = make(map[string][]ProductItem)
	}
	repository.productCatalogs[tenantID] = cloneProducts(products)

	return repository.mutationTime(), nil
}

func (repository *fakeRepository) UpsertProduct(_ context.Context, tenantID string, product ProductItem) (time.Time, error) {
	repository.upsertProductCalls++
	repository.lastUpsertedProductTenant = tenantID
	repository.lastUpsertedProduct = product

	if repository.productCatalogs == nil {
		repository.productCatalogs = make(map[string][]ProductItem)
	}

	nextProducts, _ := upsertProductCatalogItem(repository.productCatalogs[tenantID], product)
	repository.productCatalogs[tenantID] = cloneProducts(nextProducts)

	return repository.mutationTime(), nil
}

func (repository *fakeRepository) DeleteProduct(_ context.Context, tenantID string, productID string) (time.Time, error) {
	repository.lastDeletedProductTenant = tenantID
	repository.lastDeletedProductID = productID

	if repository.productCatalogs != nil {
		repository.productCatalogs[tenantID] = removeProductCatalogItem(repository.productCatalogs[tenantID], productID)
	}

	return repository.mutationTime(), nil
}

func TestGetBundleResolvesDefaultTenantForGlobalPrincipal(t *testing.T) {
	service := NewService(&fakeRepository{
		defaultTenantID: "tenant-1",
		records: map[string]Record{
			"tenant-1": {
				TenantID:                    "tenant-1",
				SelectedOperationTemplateID: defaultTemplateID,
				Settings:                    DefaultBundle("tenant-1", defaultTemplateID).Settings,
				ModalConfig:                 DefaultBundle("tenant-1", defaultTemplateID).ModalConfig,
			},
		},
	}, nil)

	bundle, err := service.GetBundle(context.Background(), auth.Principal{
		UserID: "user-1",
		Role:   auth.RolePlatformAdmin,
	}, "")
	if err != nil {
		t.Fatalf("GetBundle returned error: %v", err)
	}

	if bundle.TenantID != "tenant-1" {
		t.Fatalf("expected tenant-1, got %q", bundle.TenantID)
	}
}

func TestGetBundleRejectsAmbiguousGlobalPrincipal(t *testing.T) {
	service := NewService(&fakeRepository{
		resolveErr: ErrTenantRequired,
		records:    map[string]Record{},
	}, nil)

	if _, err := service.GetBundle(context.Background(), auth.Principal{
		UserID: "user-1",
		Role:   auth.RolePlatformAdmin,
	}, ""); err != ErrTenantRequired {
		t.Fatalf("expected ErrTenantRequired, got %v", err)
	}
}

func TestGetBundleUsesRequestedTenantForGlobalPrincipal(t *testing.T) {
	service := NewService(&fakeRepository{
		resolveErr: ErrTenantRequired,
		accessible: map[string]bool{
			"tenant-2": true,
		},
		records: map[string]Record{
			"tenant-2": {
				TenantID:                    "tenant-2",
				SelectedOperationTemplateID: defaultTemplateID,
				Settings:                    DefaultBundle("tenant-2", defaultTemplateID).Settings,
				ModalConfig:                 DefaultBundle("tenant-2", defaultTemplateID).ModalConfig,
			},
		},
	}, nil)

	bundle, err := service.GetBundle(context.Background(), auth.Principal{
		UserID: "user-1",
		Role:   auth.RolePlatformAdmin,
	}, "tenant-2")
	if err != nil {
		t.Fatalf("GetBundle returned error: %v", err)
	}

	if bundle.TenantID != "tenant-2" {
		t.Fatalf("expected tenant-2, got %q", bundle.TenantID)
	}
}

func TestSaveOperationSectionUsesSectionDefaultsAndClampsPerConsultant(t *testing.T) {
	repository := &fakeRepository{
		savedAt: time.Date(2026, 4, 29, 15, 4, 5, 0, time.UTC),
	}
	service := NewService(repository, nil)

	selectedTemplateID := "joalheria-relacionamento"
	maxConcurrentServices := 2
	maxConcurrentServicesPerConsultant := 7

	ack, err := service.SaveOperationSection(context.Background(), auth.Principal{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     auth.RoleOwner,
	}, OperationSectionInput{
		SelectedOperationTemplateID: &selectedTemplateID,
		Settings: &AppSettingsPatch{
			MaxConcurrentServices:              &maxConcurrentServices,
			MaxConcurrentServicesPerConsultant: &maxConcurrentServicesPerConsultant,
		},
	})
	if err != nil {
		t.Fatalf("SaveOperationSection returned error: %v", err)
	}

	if ack.TenantID != "tenant-1" {
		t.Fatalf("expected tenant-1 in ack, got %q", ack.TenantID)
	}
	if !ack.SavedAt.Equal(repository.savedAt) {
		t.Fatalf("expected savedAt %v, got %v", repository.savedAt, ack.SavedAt)
	}
	if repository.lastSavedOperationSection.SelectedOperationTemplateID != selectedTemplateID {
		t.Fatalf("expected selected template %q, got %q", selectedTemplateID, repository.lastSavedOperationSection.SelectedOperationTemplateID)
	}
	if repository.lastSavedOperationSection.CoreSettings.MaxConcurrentServices != 2 {
		t.Fatalf("expected maxConcurrentServices=2, got %d", repository.lastSavedOperationSection.CoreSettings.MaxConcurrentServices)
	}
	if repository.lastSavedOperationSection.CoreSettings.MaxConcurrentServicesPerConsultant != 2 {
		t.Fatalf("expected perConsultant to be clamped to 2, got %d", repository.lastSavedOperationSection.CoreSettings.MaxConcurrentServicesPerConsultant)
	}
	if repository.lastSavedOperationSection.AlertSettings.AlertMinConversionRate != 0 {
		t.Fatalf("expected alert defaults to stay at 0, got %v", repository.lastSavedOperationSection.AlertSettings.AlertMinConversionRate)
	}
}

func TestSaveModalSectionUsesOperationTemplateDefaultsWhenModalSectionMissing(t *testing.T) {
	repository := &fakeRepository{
		savedAt: time.Date(2026, 4, 29, 16, 0, 0, 0, time.UTC),
		operationSections: map[string]OperationSectionRecord{
			"tenant-1": defaultOperationSectionRecord("tenant-1", "joalheria-fluxo-rapido"),
		},
	}
	service := NewService(repository, nil)

	title := "Fechamento rapido"
	requireVisitReasonJustification := true
	visitReasonJustificationMinChars := 32

	_, err := service.SaveModalSection(context.Background(), auth.Principal{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     auth.RoleOwner,
	}, ModalSectionInput{
		ModalConfig: &ModalConfigPatch{
			Title:                            &title,
			RequireVisitReasonJustification:  &requireVisitReasonJustification,
			VisitReasonJustificationMinChars: &visitReasonJustificationMinChars,
		},
	})
	if err != nil {
		t.Fatalf("SaveModalSection returned error: %v", err)
	}

	saved := repository.lastSavedModalSection
	if saved.SelectedOperationTemplateID != "joalheria-fluxo-rapido" {
		t.Fatalf("expected template joalheria-fluxo-rapido, got %q", saved.SelectedOperationTemplateID)
	}
	if saved.ModalConfig.Title != title {
		t.Fatalf("expected title %q, got %q", title, saved.ModalConfig.Title)
	}
	if saved.ModalConfig.ShowEmailField {
		t.Fatalf("expected fluxo rapido modal to keep showEmailField=false")
	}
	if saved.ModalConfig.RequireCustomerSource {
		t.Fatalf("expected fluxo rapido modal to keep requireCustomerSource=false")
	}
	if !saved.ModalConfig.RequireVisitReasonJustification {
		t.Fatalf("expected visit reason justification to be enabled")
	}
	if saved.ModalConfig.VisitReasonJustificationMinChars != 32 {
		t.Fatalf("expected visit reason justification min chars 32, got %d", saved.ModalConfig.VisitReasonJustificationMinChars)
	}
}

func TestApplyOperationTemplatePersistsTemplateAsSingleMutation(t *testing.T) {
	repository := &fakeRepository{
		savedAt: time.Date(2026, 4, 30, 9, 30, 0, 0, time.UTC),
		operationSections: map[string]OperationSectionRecord{
			"tenant-1": {
				TenantID:                    "tenant-1",
				SelectedOperationTemplateID: defaultTemplateID,
				CoreSettings: OperationCoreSettings{
					MaxConcurrentServices:              10,
					MaxConcurrentServicesPerConsultant: 3,
					TimingFastCloseMinutes:             5,
					TimingLongServiceMinutes:           25,
					TimingLowSaleAmount:                1200,
					ServiceCancelWindowSeconds:         30,
					TestModeEnabled:                    true,
					AutoFillFinishModal:                true,
				},
				AlertSettings: AlertSettings{
					AlertMinConversionRate: 42,
					AlertMaxQueueJumpRate:  9,
					AlertMinPaScore:        1.7,
					AlertMinTicketAverage:  3000,
				},
			},
		},
		modalSections: map[string]ModalSectionRecord{
			"tenant-1": {
				TenantID:                    "tenant-1",
				SelectedOperationTemplateID: defaultTemplateID,
				ModalConfig: func() ModalConfig {
					config := DefaultBundle("tenant-1", defaultTemplateID).ModalConfig
					config.Title = "Titulo customizado"
					return config
				}(),
			},
		},
	}
	service := NewService(repository, nil)

	ack, err := service.ApplyOperationTemplate(context.Background(), auth.Principal{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     auth.RoleOwner,
	}, OperationTemplateApplyInput{
		TemplateID: "joalheria-fluxo-rapido",
	})
	if err != nil {
		t.Fatalf("ApplyOperationTemplate returned error: %v", err)
	}

	if repository.applyTemplateCalls != 1 {
		t.Fatalf("expected one template apply repository call, got %d", repository.applyTemplateCalls)
	}
	if repository.replaceOptionCalls != 0 || repository.upsertOptionCalls != 0 {
		t.Fatalf("expected no option calls outside template transaction, got replace=%d upsert=%d", repository.replaceOptionCalls, repository.upsertOptionCalls)
	}
	if ack.TenantID != "tenant-1" || !ack.SavedAt.Equal(repository.savedAt) {
		t.Fatalf("unexpected ack: %+v", ack)
	}

	applied := repository.lastAppliedTemplate
	if applied.OperationSection.SelectedOperationTemplateID != "joalheria-fluxo-rapido" {
		t.Fatalf("expected fluxo rapido selected, got %q", applied.OperationSection.SelectedOperationTemplateID)
	}
	if applied.OperationSection.CoreSettings.MaxConcurrentServices != 12 {
		t.Fatalf("expected template max concurrent 12, got %d", applied.OperationSection.CoreSettings.MaxConcurrentServices)
	}
	if !applied.OperationSection.CoreSettings.TestModeEnabled || !applied.OperationSection.CoreSettings.AutoFillFinishModal {
		t.Fatalf("expected test/auto-fill flags to be preserved")
	}
	if applied.OperationSection.AlertSettings.AlertMinConversionRate != 42 {
		t.Fatalf("expected alert settings to be preserved, got %+v", applied.OperationSection.AlertSettings)
	}
	if applied.ModalSection.ModalConfig.Title != "Titulo customizado" {
		t.Fatalf("expected custom modal title to be preserved, got %q", applied.ModalSection.ModalConfig.Title)
	}
	if applied.ModalSection.ModalConfig.ShowEmailField {
		t.Fatalf("expected fluxo rapido to disable email field")
	}
	if len(applied.VisitReasonOptions) != len(resolveTemplate("joalheria-fluxo-rapido").VisitReasonOptions) {
		t.Fatalf("expected visit reasons from template, got %d", len(applied.VisitReasonOptions))
	}
	if len(applied.CustomerSourceOptions) != len(defaultCustomerSourceOptions()) {
		t.Fatalf("expected customer sources from template, got %d", len(applied.CustomerSourceOptions))
	}
}

func TestApplyOperationTemplateRejectsUnknownTemplate(t *testing.T) {
	service := NewService(&fakeRepository{}, nil)

	_, err := service.ApplyOperationTemplate(context.Background(), auth.Principal{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     auth.RoleOwner,
	}, OperationTemplateApplyInput{
		TemplateID: "template-inexistente",
	})
	if err != ErrValidation {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestSaveOptionItemSeedsDefaultsBeforeFirstInsert(t *testing.T) {
	repository := &fakeRepository{
		operationSections: map[string]OperationSectionRecord{
			"tenant-1": defaultOperationSectionRecord("tenant-1", "joalheria-fluxo-rapido"),
		},
	}
	service := NewService(repository, nil)

	_, err := service.SaveOptionItem(context.Background(), auth.Principal{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     auth.RoleOwner,
	}, optionKindVisitReason, OptionItem{
		ID:    "novo-motivo",
		Label: "Novo motivo",
	}, "")
	if err != nil {
		t.Fatalf("SaveOptionItem returned error: %v", err)
	}

	if repository.upsertOptionCalls != 0 {
		t.Fatalf("expected no granular upsert when defaults need materialization, got %d calls", repository.upsertOptionCalls)
	}
	if repository.replaceOptionCalls != 1 {
		t.Fatalf("expected one replace call to materialize defaults, got %d", repository.replaceOptionCalls)
	}

	expectedDefaults, err := defaultOptionGroupItems("joalheria-fluxo-rapido", optionKindVisitReason)
	if err != nil {
		t.Fatalf("defaultOptionGroupItems returned error: %v", err)
	}
	if len(repository.lastReplacedOptionItems) != len(expectedDefaults)+1 {
		t.Fatalf("expected %d items after seeding defaults, got %d", len(expectedDefaults)+1, len(repository.lastReplacedOptionItems))
	}
	if repository.lastReplacedOptionItems[len(repository.lastReplacedOptionItems)-1].ID != "novo-motivo" {
		t.Fatalf("expected new item to be appended after defaults")
	}
}

func TestSaveProductItemUsesGranularUpsertWhenCatalogAlreadyExists(t *testing.T) {
	repository := &fakeRepository{
		productCatalogs: map[string][]ProductItem{
			"tenant-1": {
				{ID: "produto-existente", Name: "Produto existente", Code: "PROD-001", Category: "Teste", BasePrice: 10},
			},
		},
	}
	service := NewService(repository, nil)

	_, err := service.SaveProductItem(context.Background(), auth.Principal{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     auth.RoleOwner,
	}, ProductItemInput{
		Item: ProductItem{
			ID:        "produto-novo",
			Name:      "Produto novo",
			Code:      "prod-002",
			Category:  "Teste",
			BasePrice: 20,
		},
	})
	if err != nil {
		t.Fatalf("SaveProductItem returned error: %v", err)
	}

	if repository.replaceProductCalls != 0 {
		t.Fatalf("expected no replace when catalog already exists, got %d", repository.replaceProductCalls)
	}
	if repository.upsertProductCalls != 1 {
		t.Fatalf("expected one granular upsert, got %d", repository.upsertProductCalls)
	}
	if repository.lastUpsertedProduct.Code != "PROD-002" {
		t.Fatalf("expected code to be normalized to upper case, got %q", repository.lastUpsertedProduct.Code)
	}
}
