package settings

import (
	"context"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) GetBundle(ctx context.Context, principal auth.Principal, storeID string) (Bundle, error) {
	resolvedStoreID, err := service.resolveStoreID(ctx, principal, storeID)
	if err != nil {
		return Bundle{}, err
	}

	record, found, err := service.repository.GetByStore(ctx, resolvedStoreID)
	if err != nil {
		return Bundle{}, err
	}

	if !found {
		return DefaultBundle(resolvedStoreID, defaultTemplateID), nil
	}

	return recordToBundle(record), nil
}

func (service *Service) SaveBundle(ctx context.Context, principal auth.Principal, input Bundle) (MutationAck, error) {
	if principal.Role != auth.RoleOwner && principal.Role != auth.RolePlatformAdmin {
		return MutationAck{}, ErrForbidden
	}

	resolvedStoreID, err := service.resolveStoreID(ctx, principal, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	normalized := service.normalizeBundle(resolvedStoreID, input)
	savedRecord, err := service.repository.Upsert(ctx, bundleToRecord(normalized))
	if err != nil {
		return MutationAck{}, err
	}

	return MutationAck{
		OK:      true,
		StoreID: savedRecord.StoreID,
		SavedAt: savedRecord.UpdatedAt,
	}, nil
}

func (service *Service) SaveOperationSection(ctx context.Context, principal auth.Principal, input OperationSectionInput) (MutationAck, error) {
	resolvedStoreID, currentBundle, err := service.loadWritableBundle(ctx, principal, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	if input.SelectedOperationTemplateID != nil {
		selectedTemplateID := strings.TrimSpace(*input.SelectedOperationTemplateID)
		if selectedTemplateID != "" {
			currentBundle.SelectedOperationTemplateID = selectedTemplateID
		}
	}

	if input.Settings != nil {
		currentBundle.Settings = applyAppSettingsPatch(currentBundle.Settings, *input.Settings)
	}

	return service.persistConfig(ctx, currentBundle, resolvedStoreID)
}

func (service *Service) SaveModalSection(ctx context.Context, principal auth.Principal, input ModalSectionInput) (MutationAck, error) {
	resolvedStoreID, currentBundle, err := service.loadWritableBundle(ctx, principal, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	if input.ModalConfig != nil {
		currentBundle.ModalConfig = applyModalConfigPatch(currentBundle.ModalConfig, *input.ModalConfig)
	}

	return service.persistConfig(ctx, currentBundle, resolvedStoreID)
}

func (service *Service) SaveOptionSection(ctx context.Context, principal auth.Principal, storeID string, optionGroup string, items []OptionItem) (MutationAck, error) {
	resolvedStoreID, err := service.resolveWritableStoreID(ctx, principal, storeID)
	if err != nil {
		return MutationAck{}, err
	}

	var savedAt time.Time

	switch optionGroup {
	case optionKindVisitReason:
		savedAt, err = service.repository.ReplaceOptionGroup(ctx, resolvedStoreID, optionKindVisitReason, normalizeOptions(items, nil))
	case optionKindCustomerSource:
		savedAt, err = service.repository.ReplaceOptionGroup(ctx, resolvedStoreID, optionKindCustomerSource, normalizeOptions(items, nil))
	case optionKindQueueJump:
		savedAt, err = service.repository.ReplaceOptionGroup(ctx, resolvedStoreID, optionKindQueueJump, normalizeOptions(items, nil))
	case optionKindLossReason:
		savedAt, err = service.repository.ReplaceOptionGroup(ctx, resolvedStoreID, optionKindLossReason, normalizeOptions(items, nil))
	case optionKindProfession:
		savedAt, err = service.repository.ReplaceOptionGroup(ctx, resolvedStoreID, optionKindProfession, normalizeOptions(items, nil))
	default:
		return MutationAck{}, ErrValidation
	}

	if err != nil {
		return MutationAck{}, err
	}

	return MutationAck{
		OK:      true,
		StoreID: resolvedStoreID,
		SavedAt: savedAt,
	}, nil
}

func (service *Service) SaveOptionItem(ctx context.Context, principal auth.Principal, storeID string, optionGroup string, item OptionItem) (MutationAck, error) {
	resolvedStoreID, err := service.resolveWritableStoreID(ctx, principal, storeID)
	if err != nil {
		return MutationAck{}, err
	}

	normalizedItems := normalizeOptions([]OptionItem{item}, nil)
	if len(normalizedItems) != 1 {
		return MutationAck{}, ErrValidation
	}

	savedAt, err := service.repository.UpsertOption(ctx, resolvedStoreID, optionGroup, normalizedItems[0])
	if err != nil {
		return MutationAck{}, err
	}

	return MutationAck{
		OK:      true,
		StoreID: resolvedStoreID,
		SavedAt: savedAt,
	}, nil
}

func (service *Service) DeleteOptionItem(ctx context.Context, principal auth.Principal, storeID string, optionGroup string, optionID string) (MutationAck, error) {
	resolvedStoreID, err := service.resolveWritableStoreID(ctx, principal, storeID)
	if err != nil {
		return MutationAck{}, err
	}

	normalizedOptionID := strings.TrimSpace(optionID)
	if normalizedOptionID == "" {
		return MutationAck{}, ErrValidation
	}

	savedAt, err := service.repository.DeleteOption(ctx, resolvedStoreID, optionGroup, normalizedOptionID)
	if err != nil {
		return MutationAck{}, err
	}

	return MutationAck{
		OK:      true,
		StoreID: resolvedStoreID,
		SavedAt: savedAt,
	}, nil
}

func (service *Service) SaveProductSection(ctx context.Context, principal auth.Principal, input ProductSectionInput) (MutationAck, error) {
	resolvedStoreID, err := service.resolveWritableStoreID(ctx, principal, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	savedAt, err := service.repository.ReplaceProducts(ctx, resolvedStoreID, normalizeProducts(input.Items, nil))
	if err != nil {
		return MutationAck{}, err
	}

	return MutationAck{
		OK:      true,
		StoreID: resolvedStoreID,
		SavedAt: savedAt,
	}, nil
}

func (service *Service) SaveProductItem(ctx context.Context, principal auth.Principal, input ProductItemInput) (MutationAck, error) {
	resolvedStoreID, err := service.resolveWritableStoreID(ctx, principal, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	normalizedItems := normalizeProducts([]ProductItem{input.Item}, nil)
	if len(normalizedItems) != 1 {
		return MutationAck{}, ErrValidation
	}

	savedAt, err := service.repository.UpsertProduct(ctx, resolvedStoreID, normalizedItems[0])
	if err != nil {
		return MutationAck{}, err
	}

	return MutationAck{
		OK:      true,
		StoreID: resolvedStoreID,
		SavedAt: savedAt,
	}, nil
}

func (service *Service) DeleteProductItem(ctx context.Context, principal auth.Principal, storeID string, productID string) (MutationAck, error) {
	resolvedStoreID, err := service.resolveWritableStoreID(ctx, principal, storeID)
	if err != nil {
		return MutationAck{}, err
	}

	normalizedProductID := strings.TrimSpace(productID)
	if normalizedProductID == "" {
		return MutationAck{}, ErrValidation
	}

	savedAt, err := service.repository.DeleteProduct(ctx, resolvedStoreID, normalizedProductID)
	if err != nil {
		return MutationAck{}, err
	}

	return MutationAck{
		OK:      true,
		StoreID: resolvedStoreID,
		SavedAt: savedAt,
	}, nil
}

func (service *Service) resolveStoreID(ctx context.Context, principal auth.Principal, storeID string) (string, error) {
	trimmedStoreID := strings.TrimSpace(storeID)
	if trimmedStoreID == "" {
		return "", ErrStoreRequired
	}

	exists, err := service.repository.StoreExists(ctx, trimmedStoreID)
	if err != nil {
		return "", err
	}

	if !exists {
		return "", ErrStoreNotFound
	}

	if principal.Role == auth.RolePlatformAdmin {
		return trimmedStoreID, nil
	}

	for _, accessibleStoreID := range principal.StoreIDs {
		if accessibleStoreID == trimmedStoreID {
			return trimmedStoreID, nil
		}
	}

	return "", ErrForbidden
}

func (service *Service) loadWritableBundle(ctx context.Context, principal auth.Principal, storeID string) (string, Bundle, error) {
	resolvedStoreID, err := service.resolveWritableStoreID(ctx, principal, storeID)
	if err != nil {
		return "", Bundle{}, err
	}

	record, found, err := service.repository.GetByStore(ctx, resolvedStoreID)
	if err != nil {
		return "", Bundle{}, err
	}

	if !found {
		return resolvedStoreID, DefaultBundle(resolvedStoreID, defaultTemplateID), nil
	}

	return resolvedStoreID, recordToBundle(record), nil
}

func (service *Service) persistBundle(ctx context.Context, bundle Bundle, resolvedStoreID string) (MutationAck, error) {
	bundle.StoreID = resolvedStoreID

	savedRecord, err := service.repository.Upsert(ctx, bundleToRecord(bundle))
	if err != nil {
		return MutationAck{}, err
	}

	return MutationAck{
		OK:      true,
		StoreID: savedRecord.StoreID,
		SavedAt: savedRecord.UpdatedAt,
	}, nil
}

func (service *Service) persistConfig(ctx context.Context, bundle Bundle, resolvedStoreID string) (MutationAck, error) {
	bundle.StoreID = resolvedStoreID

	savedRecord, err := service.repository.UpsertConfig(ctx, bundleToRecord(bundle))
	if err != nil {
		return MutationAck{}, err
	}

	return MutationAck{
		OK:      true,
		StoreID: savedRecord.StoreID,
		SavedAt: savedRecord.UpdatedAt,
	}, nil
}

func (service *Service) resolveWritableStoreID(ctx context.Context, principal auth.Principal, storeID string) (string, error) {
	if principal.Role != auth.RoleOwner && principal.Role != auth.RolePlatformAdmin {
		return "", ErrForbidden
	}

	return service.resolveStoreID(ctx, principal, storeID)
}

func (service *Service) normalizeBundle(storeID string, input Bundle) Bundle {
	base := DefaultBundle(storeID, input.SelectedOperationTemplateID)
	base.Settings = normalizeAppSettings(input.Settings, base.Settings)
	base.ModalConfig = normalizeModalConfig(base.ModalConfig, input.ModalConfig)
	base.VisitReasonOptions = normalizeOptions(input.VisitReasonOptions, base.VisitReasonOptions)
	base.CustomerSourceOptions = normalizeOptions(input.CustomerSourceOptions, base.CustomerSourceOptions)
	base.QueueJumpReasonOptions = normalizeOptions(input.QueueJumpReasonOptions, base.QueueJumpReasonOptions)
	base.LossReasonOptions = normalizeOptions(input.LossReasonOptions, base.LossReasonOptions)
	base.ProfessionOptions = normalizeOptions(input.ProfessionOptions, base.ProfessionOptions)
	base.ProductCatalog = normalizeProducts(input.ProductCatalog, base.ProductCatalog)

	return base
}

func normalizeAppSettings(input AppSettings, fallback AppSettings) AppSettings {
	fallback.MaxConcurrentServices = maxInt(input.MaxConcurrentServices, 1)
	fallback.TimingFastCloseMinutes = maxInt(input.TimingFastCloseMinutes, 1)
	fallback.TimingLongServiceMinutes = maxInt(input.TimingLongServiceMinutes, 1)
	fallback.TimingLowSaleAmount = maxFloat(input.TimingLowSaleAmount, 0)
	fallback.TestModeEnabled = input.TestModeEnabled
	fallback.AutoFillFinishModal = input.AutoFillFinishModal
	fallback.AlertMinConversionRate = maxFloat(input.AlertMinConversionRate, 0)
	fallback.AlertMaxQueueJumpRate = maxFloat(input.AlertMaxQueueJumpRate, 0)
	fallback.AlertMinPaScore = maxFloat(input.AlertMinPaScore, 0)
	fallback.AlertMinTicketAverage = maxFloat(input.AlertMinTicketAverage, 0)
	return fallback
}

func applyAppSettingsPatch(base AppSettings, patch AppSettingsPatch) AppSettings {
	if patch.MaxConcurrentServices != nil {
		base.MaxConcurrentServices = maxInt(*patch.MaxConcurrentServices, 1)
	}
	if patch.TimingFastCloseMinutes != nil {
		base.TimingFastCloseMinutes = maxInt(*patch.TimingFastCloseMinutes, 1)
	}
	if patch.TimingLongServiceMinutes != nil {
		base.TimingLongServiceMinutes = maxInt(*patch.TimingLongServiceMinutes, 1)
	}
	if patch.TimingLowSaleAmount != nil {
		base.TimingLowSaleAmount = maxFloat(*patch.TimingLowSaleAmount, 0)
	}
	if patch.TestModeEnabled != nil {
		base.TestModeEnabled = *patch.TestModeEnabled
	}
	if patch.AutoFillFinishModal != nil {
		base.AutoFillFinishModal = *patch.AutoFillFinishModal
	}
	if patch.AlertMinConversionRate != nil {
		base.AlertMinConversionRate = maxFloat(*patch.AlertMinConversionRate, 0)
	}
	if patch.AlertMaxQueueJumpRate != nil {
		base.AlertMaxQueueJumpRate = maxFloat(*patch.AlertMaxQueueJumpRate, 0)
	}
	if patch.AlertMinPaScore != nil {
		base.AlertMinPaScore = maxFloat(*patch.AlertMinPaScore, 0)
	}
	if patch.AlertMinTicketAverage != nil {
		base.AlertMinTicketAverage = maxFloat(*patch.AlertMinTicketAverage, 0)
	}

	return base
}

func applyModalConfigPatch(base ModalConfig, patch ModalConfigPatch) ModalConfig {
	if patch.Title != nil {
		base.Title = fallbackString(*patch.Title, base.Title)
	}
	if patch.ProductSeenLabel != nil {
		base.ProductSeenLabel = fallbackString(*patch.ProductSeenLabel, base.ProductSeenLabel)
	}
	if patch.ProductSeenPlaceholder != nil {
		base.ProductSeenPlaceholder = fallbackString(*patch.ProductSeenPlaceholder, base.ProductSeenPlaceholder)
	}
	if patch.ProductClosedLabel != nil {
		base.ProductClosedLabel = fallbackString(*patch.ProductClosedLabel, base.ProductClosedLabel)
	}
	if patch.ProductClosedPlaceholder != nil {
		base.ProductClosedPlaceholder = fallbackString(*patch.ProductClosedPlaceholder, base.ProductClosedPlaceholder)
	}
	if patch.NotesLabel != nil {
		base.NotesLabel = fallbackString(*patch.NotesLabel, base.NotesLabel)
	}
	if patch.NotesPlaceholder != nil {
		base.NotesPlaceholder = fallbackString(*patch.NotesPlaceholder, base.NotesPlaceholder)
	}
	if patch.QueueJumpReasonLabel != nil {
		base.QueueJumpReasonLabel = fallbackString(*patch.QueueJumpReasonLabel, base.QueueJumpReasonLabel)
	}
	if patch.QueueJumpReasonPlaceholder != nil {
		base.QueueJumpReasonPlaceholder = fallbackString(*patch.QueueJumpReasonPlaceholder, base.QueueJumpReasonPlaceholder)
	}
	if patch.LossReasonLabel != nil {
		base.LossReasonLabel = fallbackString(*patch.LossReasonLabel, base.LossReasonLabel)
	}
	if patch.LossReasonPlaceholder != nil {
		base.LossReasonPlaceholder = fallbackString(*patch.LossReasonPlaceholder, base.LossReasonPlaceholder)
	}
	if patch.CustomerSectionLabel != nil {
		base.CustomerSectionLabel = fallbackString(*patch.CustomerSectionLabel, base.CustomerSectionLabel)
	}
	if patch.ShowEmailField != nil {
		base.ShowEmailField = *patch.ShowEmailField
	}
	if patch.ShowProfessionField != nil {
		base.ShowProfessionField = *patch.ShowProfessionField
	}
	if patch.ShowNotesField != nil {
		base.ShowNotesField = *patch.ShowNotesField
	}
	if patch.VisitReasonSelectionMode != nil {
		base.VisitReasonSelectionMode = normalizeEnum(*patch.VisitReasonSelectionMode, []string{"single", "multiple"}, base.VisitReasonSelectionMode)
	}
	if patch.VisitReasonDetailMode != nil {
		base.VisitReasonDetailMode = normalizeEnum(*patch.VisitReasonDetailMode, []string{"off", "shared", "per-item"}, base.VisitReasonDetailMode)
	}
	if patch.LossReasonSelectionMode != nil {
		base.LossReasonSelectionMode = normalizeEnum(*patch.LossReasonSelectionMode, []string{"single", "multiple"}, base.LossReasonSelectionMode)
	}
	if patch.LossReasonDetailMode != nil {
		base.LossReasonDetailMode = normalizeEnum(*patch.LossReasonDetailMode, []string{"off", "shared", "per-item"}, base.LossReasonDetailMode)
	}
	if patch.CustomerSourceSelectionMode != nil {
		base.CustomerSourceSelectionMode = normalizeEnum(*patch.CustomerSourceSelectionMode, []string{"single", "multiple"}, base.CustomerSourceSelectionMode)
	}
	if patch.CustomerSourceDetailMode != nil {
		base.CustomerSourceDetailMode = normalizeEnum(*patch.CustomerSourceDetailMode, []string{"off", "shared", "per-item"}, base.CustomerSourceDetailMode)
	}
	if patch.RequireProduct != nil {
		base.RequireProduct = *patch.RequireProduct
	}
	if patch.RequireVisitReason != nil {
		base.RequireVisitReason = *patch.RequireVisitReason
	}
	if patch.RequireCustomerSource != nil {
		base.RequireCustomerSource = *patch.RequireCustomerSource
	}
	if patch.RequireCustomerNamePhone != nil {
		base.RequireCustomerNamePhone = *patch.RequireCustomerNamePhone
	}

	return base
}

func recordToBundle(record Record) Bundle {
	bundle := DefaultBundle(record.StoreID, record.SelectedOperationTemplateID)
	bundle.SelectedOperationTemplateID = record.SelectedOperationTemplateID
	bundle.Settings = record.Settings
	bundle.ModalConfig = record.ModalConfig
	bundle.VisitReasonOptions = cloneOptions(record.VisitReasonOptions)
	bundle.CustomerSourceOptions = cloneOptions(record.CustomerSourceOptions)
	bundle.QueueJumpReasonOptions = cloneOptions(record.QueueJumpReasonOptions)
	bundle.LossReasonOptions = cloneOptions(record.LossReasonOptions)
	bundle.ProfessionOptions = cloneOptions(record.ProfessionOptions)
	bundle.ProductCatalog = cloneProducts(record.ProductCatalog)
	bundle.OperationTemplates = DefaultOperationTemplates()
	return bundle
}

func bundleToRecord(bundle Bundle) Record {
	return Record{
		StoreID:                     bundle.StoreID,
		SelectedOperationTemplateID: bundle.SelectedOperationTemplateID,
		Settings:                    bundle.Settings,
		ModalConfig:                 bundle.ModalConfig,
		VisitReasonOptions:          cloneOptions(bundle.VisitReasonOptions),
		CustomerSourceOptions:       cloneOptions(bundle.CustomerSourceOptions),
		QueueJumpReasonOptions:      cloneOptions(bundle.QueueJumpReasonOptions),
		LossReasonOptions:           cloneOptions(bundle.LossReasonOptions),
		ProfessionOptions:           cloneOptions(bundle.ProfessionOptions),
		ProductCatalog:              cloneProducts(bundle.ProductCatalog),
	}
}

func normalizeOptions(options []OptionItem, fallback []OptionItem) []OptionItem {
	if options == nil {
		return cloneOptions(fallback)
	}

	normalized := make([]OptionItem, 0, len(options))
	seen := make(map[string]struct{})
	for _, option := range options {
		id := strings.TrimSpace(option.ID)
		label := strings.TrimSpace(option.Label)
		if id == "" || label == "" {
			continue
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, OptionItem{
			ID:    id,
			Label: label,
		})
	}

	return normalized
}

func normalizeProducts(products []ProductItem, fallback []ProductItem) []ProductItem {
	if products == nil {
		return cloneProducts(fallback)
	}

	normalized := make([]ProductItem, 0, len(products))
	seen := make(map[string]struct{})
	for _, product := range products {
		id := strings.TrimSpace(product.ID)
		name := strings.TrimSpace(product.Name)
		if id == "" || name == "" {
			continue
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, ProductItem{
			ID:        id,
			Name:      name,
			Code:      strings.ToUpper(strings.TrimSpace(product.Code)),
			Category:  fallbackCategory(product.Category),
			BasePrice: maxFloat(product.BasePrice, 0),
		})
	}

	return normalized
}

func normalizeModalConfig(base ModalConfig, input ModalConfig) ModalConfig {
	base.Title = fallbackString(input.Title, base.Title)
	base.ProductSeenLabel = fallbackString(input.ProductSeenLabel, base.ProductSeenLabel)
	base.ProductSeenPlaceholder = fallbackString(input.ProductSeenPlaceholder, base.ProductSeenPlaceholder)
	base.ProductClosedLabel = fallbackString(input.ProductClosedLabel, base.ProductClosedLabel)
	base.ProductClosedPlaceholder = fallbackString(input.ProductClosedPlaceholder, base.ProductClosedPlaceholder)
	base.NotesLabel = fallbackString(input.NotesLabel, base.NotesLabel)
	base.NotesPlaceholder = fallbackString(input.NotesPlaceholder, base.NotesPlaceholder)
	base.QueueJumpReasonLabel = fallbackString(input.QueueJumpReasonLabel, base.QueueJumpReasonLabel)
	base.QueueJumpReasonPlaceholder = fallbackString(input.QueueJumpReasonPlaceholder, base.QueueJumpReasonPlaceholder)
	base.LossReasonLabel = fallbackString(input.LossReasonLabel, base.LossReasonLabel)
	base.LossReasonPlaceholder = fallbackString(input.LossReasonPlaceholder, base.LossReasonPlaceholder)
	base.CustomerSectionLabel = fallbackString(input.CustomerSectionLabel, base.CustomerSectionLabel)
	base.ShowEmailField = input.ShowEmailField
	base.ShowProfessionField = input.ShowProfessionField
	base.ShowNotesField = input.ShowNotesField
	base.VisitReasonSelectionMode = normalizeEnum(input.VisitReasonSelectionMode, []string{"single", "multiple"}, base.VisitReasonSelectionMode)
	base.VisitReasonDetailMode = normalizeEnum(input.VisitReasonDetailMode, []string{"off", "shared", "per-item"}, base.VisitReasonDetailMode)
	base.LossReasonSelectionMode = normalizeEnum(input.LossReasonSelectionMode, []string{"single", "multiple"}, base.LossReasonSelectionMode)
	base.LossReasonDetailMode = normalizeEnum(input.LossReasonDetailMode, []string{"off", "shared", "per-item"}, base.LossReasonDetailMode)
	base.CustomerSourceSelectionMode = normalizeEnum(input.CustomerSourceSelectionMode, []string{"single", "multiple"}, base.CustomerSourceSelectionMode)
	base.CustomerSourceDetailMode = normalizeEnum(input.CustomerSourceDetailMode, []string{"off", "shared", "per-item"}, base.CustomerSourceDetailMode)
	base.RequireProduct = input.RequireProduct
	base.RequireVisitReason = input.RequireVisitReason
	base.RequireCustomerSource = input.RequireCustomerSource
	base.RequireCustomerNamePhone = input.RequireCustomerNamePhone
	return base
}

func normalizeEnum(value string, allowed []string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	for _, candidate := range allowed {
		if candidate == trimmed {
			return trimmed
		}
	}

	return fallback
}

func fallbackString(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}

	return trimmed
}

func fallbackCategory(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "Sem categoria"
	}

	return trimmed
}

func maxFloat(value float64, minimum float64) float64 {
	if value < minimum {
		return minimum
	}

	return value
}

func maxInt(value int, minimum int) int {
	if value < minimum {
		return minimum
	}

	return value
}
