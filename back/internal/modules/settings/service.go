package settings

import (
	"context"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// RealtimePublisher e o contrato leve com o modulo realtime.
// Settings agora publica apenas no canal de contexto (tenant-wide), porque a
// configuracao deixou de ser por loja: qualquer loja do tenant deve revalidar.
type RealtimePublisher interface {
	PublishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string, savedAt time.Time)
}

type Service struct {
	repository Repository
	notifier   RealtimePublisher
}

func NewService(repository Repository, notifier RealtimePublisher) *Service {
	return &Service{repository: repository, notifier: notifier}
}

func (service *Service) GetBundle(ctx context.Context, principal auth.Principal, requestedTenantID string) (Bundle, error) {
	if !canViewSettings(principal) {
		return Bundle{}, ErrForbidden
	}

	tenantID, err := service.resolveTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return Bundle{}, err
	}

	record, found, err := service.repository.GetByTenant(ctx, tenantID)
	if err != nil {
		return Bundle{}, err
	}

	if !found {
		return DefaultBundle(tenantID, defaultTemplateID), nil
	}

	return materializeBundleDefaults(recordToBundle(record)), nil
}

func (service *Service) SaveBundle(ctx context.Context, principal auth.Principal, input Bundle) (MutationAck, error) {
	if !canEditSettings(principal) {
		return MutationAck{}, ErrForbidden
	}

	tenantID, err := service.resolveTenantID(ctx, principal, input.TenantID)
	if err != nil {
		return MutationAck{}, err
	}

	normalized := service.normalizeBundle(tenantID, input)
	savedRecord, err := service.repository.Upsert(ctx, bundleToRecord(normalized))
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: savedRecord.TenantID,
		SavedAt:  savedRecord.UpdatedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) SaveOperationSection(ctx context.Context, principal auth.Principal, input OperationSectionInput) (MutationAck, error) {
	tenantID, currentSection, err := service.loadWritableOperationSection(ctx, principal, input.TenantID)
	if err != nil {
		return MutationAck{}, err
	}

	if input.SelectedOperationTemplateID != nil {
		selectedTemplateID := strings.TrimSpace(*input.SelectedOperationTemplateID)
		if selectedTemplateID != "" {
			currentSection.SelectedOperationTemplateID = selectedTemplateID
		}
	}

	if input.Settings != nil {
		corePatch, alertPatch := splitAppSettingsPatch(*input.Settings)
		currentSection.CoreSettings = applyOperationCoreSettingsPatch(currentSection.CoreSettings, corePatch)
		currentSection.AlertSettings = applyAlertSettingsPatch(currentSection.AlertSettings, alertPatch)
	}

	savedSection, err := service.repository.UpsertOperationSection(ctx, normalizeOperationSectionRecord(currentSection))
	if err != nil {
		return MutationAck{}, err
	}

	return service.finalizeMutation(ctx, newMutationAck(tenantID, savedSection.UpdatedAt), nil)
}

func (service *Service) SaveModalSection(ctx context.Context, principal auth.Principal, input ModalSectionInput) (MutationAck, error) {
	tenantID, currentSection, err := service.loadWritableModalSection(ctx, principal, input.TenantID)
	if err != nil {
		return MutationAck{}, err
	}

	if input.ModalConfig != nil {
		currentSection.ModalConfig = applyModalConfigPatch(currentSection.ModalConfig, *input.ModalConfig)
	}

	savedSection, err := service.repository.UpsertModalSection(ctx, normalizeModalSectionRecord(currentSection))
	if err != nil {
		return MutationAck{}, err
	}

	return service.finalizeMutation(ctx, newMutationAck(tenantID, savedSection.UpdatedAt), nil)
}

func (service *Service) ApplyOperationTemplate(ctx context.Context, principal auth.Principal, input OperationTemplateApplyInput) (MutationAck, error) {
	tenantID, err := service.resolveWritableTenantID(ctx, principal, input.TenantID)
	if err != nil {
		return MutationAck{}, err
	}

	templateID := strings.TrimSpace(input.TemplateID)
	template, found := findOperationTemplate(templateID)
	if !found {
		return MutationAck{}, ErrValidation
	}

	operationSection, found, err := service.repository.GetOperationSection(ctx, tenantID)
	if err != nil {
		return MutationAck{}, err
	}
	if !found {
		operationSection = defaultOperationSectionRecord(tenantID, template.ID)
	}

	modalSection, found, err := service.repository.GetModalSection(ctx, tenantID)
	if err != nil {
		return MutationAck{}, err
	}
	if !found {
		modalSection = defaultModalSectionRecord(tenantID, template.ID)
	}

	templateBundle := DefaultBundle(tenantID, template.ID)
	templateCore, _ := splitAppSettings(templateBundle.Settings)
	operationSection.SelectedOperationTemplateID = template.ID
	operationSection.CoreSettings = applyOperationTemplateCoreSettings(operationSection.CoreSettings, templateCore)

	modalSection.SelectedOperationTemplateID = template.ID
	modalSection.ModalConfig = mergeModalConfig(modalSection.ModalConfig, templateBundle.ModalConfig)

	savedAt, err := service.repository.ApplyOperationTemplate(ctx, OperationTemplateApplyRecord{
		TenantID:              tenantID,
		OperationSection:      operationSection,
		ModalSection:          modalSection,
		VisitReasonOptions:    cloneOptions(templateBundle.VisitReasonOptions),
		CustomerSourceOptions: cloneOptions(templateBundle.CustomerSourceOptions),
	})
	if err != nil {
		return MutationAck{}, err
	}

	return service.finalizeMutation(ctx, newMutationAck(tenantID, savedAt), nil)
}

func (service *Service) SaveOptionSection(ctx context.Context, principal auth.Principal, optionGroup string, input OptionSectionInput) (MutationAck, error) {
	tenantID, currentItems, _, err := service.loadWritableOptionGroup(ctx, principal, input.TenantID, optionGroup)
	if err != nil {
		return MutationAck{}, err
	}

	nextItems := normalizeOptions(input.Items, currentItems)

	if !isValidOptionGroup(optionGroup) {
		return MutationAck{}, ErrValidation
	}

	savedAt, err := service.repository.ReplaceOptionGroup(ctx, tenantID, optionGroup, nextItems)
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) SaveOptionItem(ctx context.Context, principal auth.Principal, optionGroup string, item OptionItem, requestedTenantID string) (MutationAck, error) {
	tenantID, currentItems, seededDefaults, err := service.loadWritableOptionGroup(ctx, principal, requestedTenantID, optionGroup)
	if err != nil {
		return MutationAck{}, err
	}

	if !isValidOptionGroup(optionGroup) {
		return MutationAck{}, ErrValidation
	}

	normalizedItems := normalizeOptions([]OptionItem{item}, nil)
	if len(normalizedItems) != 1 {
		return MutationAck{}, ErrValidation
	}

	var savedAt time.Time
	if seededDefaults {
		nextItems, _ := upsertOptionGroupItem(currentItems, normalizedItems[0])
		savedAt, err = service.repository.ReplaceOptionGroup(ctx, tenantID, optionGroup, nextItems)
	} else {
		savedAt, err = service.repository.UpsertOption(ctx, tenantID, optionGroup, normalizedItems[0])
	}
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) DeleteOptionItem(ctx context.Context, principal auth.Principal, optionGroup string, optionID string, requestedTenantID string) (MutationAck, error) {
	if !isValidOptionGroup(optionGroup) {
		return MutationAck{}, ErrValidation
	}

	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return MutationAck{}, err
	}

	normalizedOptionID := strings.TrimSpace(optionID)
	if normalizedOptionID == "" {
		return MutationAck{}, ErrValidation
	}

	savedAt, err := service.repository.DeleteOption(ctx, tenantID, optionGroup, normalizedOptionID)
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) SaveProductSection(ctx context.Context, principal auth.Principal, input ProductSectionInput) (MutationAck, error) {
	tenantID, currentProducts, _, err := service.loadWritableProductCatalog(ctx, principal, input.TenantID)
	if err != nil {
		return MutationAck{}, err
	}

	savedAt, err := service.repository.ReplaceProducts(ctx, tenantID, normalizeProducts(input.Items, currentProducts))
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) SaveProductItem(ctx context.Context, principal auth.Principal, input ProductItemInput) (MutationAck, error) {
	tenantID, currentProducts, seededDefaults, err := service.loadWritableProductCatalog(ctx, principal, input.TenantID)
	if err != nil {
		return MutationAck{}, err
	}

	normalizedProducts := normalizeProducts([]ProductItem{input.Item}, nil)
	if len(normalizedProducts) != 1 {
		return MutationAck{}, ErrValidation
	}

	var savedAt time.Time
	if seededDefaults {
		nextProducts, _ := upsertProductCatalogItem(currentProducts, normalizedProducts[0])
		savedAt, err = service.repository.ReplaceProducts(ctx, tenantID, nextProducts)
	} else {
		savedAt, err = service.repository.UpsertProduct(ctx, tenantID, normalizedProducts[0])
	}
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) DeleteProductItem(ctx context.Context, principal auth.Principal, productID string, requestedTenantID string) (MutationAck, error) {
	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return MutationAck{}, err
	}

	normalizedProductID := strings.TrimSpace(productID)
	if normalizedProductID == "" {
		return MutationAck{}, ErrValidation
	}

	savedAt, err := service.repository.DeleteProduct(ctx, tenantID, normalizedProductID)
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

// resolveTenantID usa um tenant explicito quando a UI envia activeTenantId.
// Sem tenant explicito, principals globais ainda usam apenas o fallback seguro
// de tenant unico; zero ou multiplos tenants acessiveis seguem ambiguos.
func (service *Service) resolveTenantID(ctx context.Context, principal auth.Principal, requestedTenantID string) (string, error) {
	requestedTenantID = strings.TrimSpace(requestedTenantID)
	if requestedTenantID != "" {
		if tenantID := strings.TrimSpace(principal.TenantID); tenantID != "" && tenantID != requestedTenantID {
			return "", ErrForbidden
		}

		allowed, err := service.repository.CanAccessTenant(ctx, principal, requestedTenantID)
		if err != nil {
			return "", err
		}

		if !allowed {
			return "", ErrForbidden
		}

		return requestedTenantID, nil
	}

	tenantID := strings.TrimSpace(principal.TenantID)
	if tenantID != "" {
		return tenantID, nil
	}

	return service.repository.ResolveDefaultTenantID(ctx, principal)
}

func (service *Service) resolveWritableTenantID(ctx context.Context, principal auth.Principal, requestedTenantID string) (string, error) {
	if !canEditSettings(principal) {
		return "", ErrForbidden
	}

	return service.resolveTenantID(ctx, principal, requestedTenantID)
}

func (service *Service) loadWritableBundle(ctx context.Context, principal auth.Principal, requestedTenantID string) (string, Bundle, error) {
	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return "", Bundle{}, err
	}

	record, found, err := service.repository.GetByTenant(ctx, tenantID)
	if err != nil {
		return "", Bundle{}, err
	}

	if !found {
		return tenantID, DefaultBundle(tenantID, defaultTemplateID), nil
	}

	return tenantID, materializeBundleDefaults(recordToBundle(record)), nil
}

func (service *Service) loadWritableOperationSection(ctx context.Context, principal auth.Principal, requestedTenantID string) (string, OperationSectionRecord, error) {
	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return "", OperationSectionRecord{}, err
	}

	section, found, err := service.repository.GetOperationSection(ctx, tenantID)
	if err != nil {
		return "", OperationSectionRecord{}, err
	}

	if !found {
		return tenantID, defaultOperationSectionRecord(tenantID, defaultTemplateID), nil
	}

	return tenantID, normalizeOperationSectionRecord(section), nil
}

func (service *Service) loadWritableModalSection(ctx context.Context, principal auth.Principal, requestedTenantID string) (string, ModalSectionRecord, error) {
	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return "", ModalSectionRecord{}, err
	}

	section, found, err := service.repository.GetModalSection(ctx, tenantID)
	if err != nil {
		return "", ModalSectionRecord{}, err
	}

	if !found {
		selectedTemplateID, err := service.loadSelectedOperationTemplateID(ctx, tenantID)
		if err != nil {
			return "", ModalSectionRecord{}, err
		}

		return tenantID, defaultModalSectionRecord(tenantID, selectedTemplateID), nil
	}

	return tenantID, normalizeModalSectionRecord(section), nil
}

func (service *Service) loadWritableOptionGroup(ctx context.Context, principal auth.Principal, requestedTenantID string, optionGroup string) (string, []OptionItem, bool, error) {
	if !isValidOptionGroup(optionGroup) {
		return "", nil, false, ErrValidation
	}

	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return "", nil, false, err
	}

	items, err := service.repository.GetOptionGroup(ctx, tenantID, optionGroup)
	if err != nil {
		return "", nil, false, err
	}

	if len(items) > 0 {
		return tenantID, cloneOptions(items), false, nil
	}

	selectedTemplateID, err := service.loadSelectedOperationTemplateID(ctx, tenantID)
	if err != nil {
		return "", nil, false, err
	}

	defaultItems, err := defaultOptionGroupItems(selectedTemplateID, optionGroup)
	if err != nil {
		return "", nil, false, err
	}

	return tenantID, defaultItems, true, nil
}

func (service *Service) loadWritableProductCatalog(ctx context.Context, principal auth.Principal, requestedTenantID string) (string, []ProductItem, bool, error) {
	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return "", nil, false, err
	}

	items, err := service.repository.GetProductCatalog(ctx, tenantID)
	if err != nil {
		return "", nil, false, err
	}

	if len(items) > 0 {
		return tenantID, cloneProducts(items), false, nil
	}

	return tenantID, defaultProductCatalogItems(), true, nil
}

func (service *Service) loadSelectedOperationTemplateID(ctx context.Context, tenantID string) (string, error) {
	section, found, err := service.repository.GetOperationSection(ctx, tenantID)
	if err != nil {
		return "", err
	}

	if !found {
		return defaultTemplateID, nil
	}

	return normalizeOperationSectionRecord(section).SelectedOperationTemplateID, nil
}

func (service *Service) persistBundle(ctx context.Context, bundle Bundle, tenantID string) (MutationAck, error) {
	bundle.TenantID = tenantID

	savedRecord, err := service.repository.Upsert(ctx, bundleToRecord(bundle))
	if err != nil {
		return MutationAck{}, err
	}

	return MutationAck{
		OK:       true,
		TenantID: savedRecord.TenantID,
		SavedAt:  savedRecord.UpdatedAt,
	}, nil
}

func (service *Service) persistConfig(ctx context.Context, bundle Bundle, tenantID string) (MutationAck, error) {
	return service.persistBundle(ctx, bundle, tenantID)
}

func (service *Service) finalizeMutation(ctx context.Context, ack MutationAck, err error) (MutationAck, error) {
	if err != nil {
		return MutationAck{}, err
	}

	service.publishSettingsEvent(ctx, ack.TenantID, "updated", ack.SavedAt)
	return ack, nil
}

func (service *Service) publishSettingsEvent(ctx context.Context, tenantID string, action string, savedAt time.Time) {
	if service.notifier == nil {
		return
	}

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return
	}

	service.notifier.PublishContextEvent(ctx, tenantID, "settings", strings.TrimSpace(action), tenantID, savedAt)
}

func canViewSettings(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionSettingsView) ||
			accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionOperationsView)
	}

	defaultPermissions := accesscontrol.DefaultRolePermissions(principal.Role)
	return accesscontrol.HasPermission(defaultPermissions, accesscontrol.PermissionSettingsView) ||
		accesscontrol.HasPermission(defaultPermissions, accesscontrol.PermissionOperationsView)
}

func canEditSettings(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionSettingsEdit)
	}

	return principal.Role == auth.RoleOwner || principal.Role == auth.RolePlatformAdmin
}

func newMutationAck(tenantID string, savedAt time.Time) MutationAck {
	return MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}
}

func isValidOptionGroup(optionGroup string) bool {
	switch optionGroup {
	case optionKindVisitReason, optionKindCustomerSource, optionKindPauseReason,
		optionKindCancelReason, optionKindStopReason, optionKindQueueJump, optionKindLossReason, optionKindProfession:
		return true
	default:
		return false
	}
}

func (service *Service) normalizeBundle(tenantID string, input Bundle) Bundle {
	base := DefaultBundle(tenantID, input.SelectedOperationTemplateID)
	base.Settings = normalizeAppSettings(input.Settings, base.Settings)
	base.ModalConfig = normalizeModalConfig(base.ModalConfig, input.ModalConfig)
	base.VisitReasonOptions = normalizeOptions(input.VisitReasonOptions, base.VisitReasonOptions)
	base.CustomerSourceOptions = normalizeOptions(input.CustomerSourceOptions, base.CustomerSourceOptions)
	base.PauseReasonOptions = normalizeOptions(input.PauseReasonOptions, base.PauseReasonOptions)
	base.CancelReasonOptions = normalizeOptions(input.CancelReasonOptions, base.CancelReasonOptions)
	base.StopReasonOptions = normalizeOptions(input.StopReasonOptions, base.StopReasonOptions)
	base.QueueJumpReasonOptions = normalizeOptions(input.QueueJumpReasonOptions, base.QueueJumpReasonOptions)
	base.LossReasonOptions = normalizeOptions(input.LossReasonOptions, base.LossReasonOptions)
	base.ProfessionOptions = normalizeOptions(input.ProfessionOptions, base.ProfessionOptions)
	base.ProductCatalog = normalizeProducts(input.ProductCatalog, base.ProductCatalog)

	return base
}

func normalizeAppSettings(input AppSettings, fallback AppSettings) AppSettings {
	inputCore, inputAlerts := splitAppSettings(input)
	fallbackCore, fallbackAlerts := splitAppSettings(fallback)
	return composeAppSettings(
		normalizeOperationCoreSettings(inputCore, fallbackCore),
		normalizeAlertSettings(inputAlerts, fallbackAlerts),
	)
}

func applyAppSettingsPatch(base AppSettings, patch AppSettingsPatch) AppSettings {
	corePatch, alertPatch := splitAppSettingsPatch(patch)
	baseCore, baseAlerts := splitAppSettings(base)
	return composeAppSettings(
		applyOperationCoreSettingsPatch(baseCore, corePatch),
		applyAlertSettingsPatch(baseAlerts, alertPatch),
	)
}

func applyModalConfigPatch(base ModalConfig, patch ModalConfigPatch) ModalConfig {
	if patch.Title != nil {
		base.Title = fallbackString(*patch.Title, base.Title)
	}
	if patch.FinishFlowMode != nil {
		base.FinishFlowMode = normalizeEnum(*patch.FinishFlowMode, []string{"legacy", "erp-reconciliation"}, base.FinishFlowMode)
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
	if patch.PurchaseCodeLabel != nil {
		base.PurchaseCodeLabel = fallbackString(*patch.PurchaseCodeLabel, base.PurchaseCodeLabel)
	}
	if patch.PurchaseCodePlaceholder != nil {
		base.PurchaseCodePlaceholder = fallbackString(*patch.PurchaseCodePlaceholder, base.PurchaseCodePlaceholder)
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
	if patch.CustomerNameLabel != nil {
		base.CustomerNameLabel = fallbackString(*patch.CustomerNameLabel, base.CustomerNameLabel)
	}
	if patch.CustomerPhoneLabel != nil {
		base.CustomerPhoneLabel = fallbackString(*patch.CustomerPhoneLabel, base.CustomerPhoneLabel)
	}
	if patch.CustomerEmailLabel != nil {
		base.CustomerEmailLabel = fallbackString(*patch.CustomerEmailLabel, base.CustomerEmailLabel)
	}
	if patch.CustomerProfessionLabel != nil {
		base.CustomerProfessionLabel = fallbackString(*patch.CustomerProfessionLabel, base.CustomerProfessionLabel)
	}
	if patch.ExistingCustomerLabel != nil {
		base.ExistingCustomerLabel = fallbackString(*patch.ExistingCustomerLabel, base.ExistingCustomerLabel)
	}
	if patch.ProductSeenNotesLabel != nil {
		base.ProductSeenNotesLabel = fallbackString(*patch.ProductSeenNotesLabel, base.ProductSeenNotesLabel)
	}
	if patch.ProductSeenNotesPlaceholder != nil {
		base.ProductSeenNotesPlaceholder = fallbackString(*patch.ProductSeenNotesPlaceholder, base.ProductSeenNotesPlaceholder)
	}
	if patch.VisitReasonLabel != nil {
		base.VisitReasonLabel = fallbackString(*patch.VisitReasonLabel, base.VisitReasonLabel)
	}
	if patch.CustomerSourceLabel != nil {
		base.CustomerSourceLabel = fallbackString(*patch.CustomerSourceLabel, base.CustomerSourceLabel)
	}
	if patch.CancelReasonLabel != nil {
		base.CancelReasonLabel = fallbackString(*patch.CancelReasonLabel, base.CancelReasonLabel)
	}
	if patch.CancelReasonPlaceholder != nil {
		base.CancelReasonPlaceholder = fallbackString(*patch.CancelReasonPlaceholder, base.CancelReasonPlaceholder)
	}
	if patch.CancelReasonOtherLabel != nil {
		base.CancelReasonOtherLabel = fallbackString(*patch.CancelReasonOtherLabel, base.CancelReasonOtherLabel)
	}
	if patch.CancelReasonOtherPlaceholder != nil {
		base.CancelReasonOtherPlaceholder = fallbackString(*patch.CancelReasonOtherPlaceholder, base.CancelReasonOtherPlaceholder)
	}
	if patch.StopReasonLabel != nil {
		base.StopReasonLabel = fallbackString(*patch.StopReasonLabel, base.StopReasonLabel)
	}
	if patch.StopReasonPlaceholder != nil {
		base.StopReasonPlaceholder = fallbackString(*patch.StopReasonPlaceholder, base.StopReasonPlaceholder)
	}
	if patch.StopReasonOtherLabel != nil {
		base.StopReasonOtherLabel = fallbackString(*patch.StopReasonOtherLabel, base.StopReasonOtherLabel)
	}
	if patch.StopReasonOtherPlaceholder != nil {
		base.StopReasonOtherPlaceholder = fallbackString(*patch.StopReasonOtherPlaceholder, base.StopReasonOtherPlaceholder)
	}
	if patch.ShowCustomerNameField != nil {
		base.ShowCustomerNameField = *patch.ShowCustomerNameField
	}
	if patch.ShowCustomerPhoneField != nil {
		base.ShowCustomerPhoneField = *patch.ShowCustomerPhoneField
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
	if patch.ShowProductSeenField != nil {
		base.ShowProductSeenField = *patch.ShowProductSeenField
	}
	if patch.ShowProductSeenNotesField != nil {
		base.ShowProductSeenNotesField = *patch.ShowProductSeenNotesField
	}
	if patch.ShowProductClosedField != nil {
		base.ShowProductClosedField = *patch.ShowProductClosedField
	}
	if patch.ShowPurchaseCodeField != nil {
		base.ShowPurchaseCodeField = *patch.ShowPurchaseCodeField
	}
	if patch.ShowVisitReasonField != nil {
		base.ShowVisitReasonField = *patch.ShowVisitReasonField
	}
	if patch.ShowCustomerSourceField != nil {
		base.ShowCustomerSourceField = *patch.ShowCustomerSourceField
	}
	if patch.ShowExistingCustomerField != nil {
		base.ShowExistingCustomerField = *patch.ShowExistingCustomerField
	}
	if patch.ShowQueueJumpReasonField != nil {
		base.ShowQueueJumpReasonField = *patch.ShowQueueJumpReasonField
	}
	if patch.ShowLossReasonField != nil {
		base.ShowLossReasonField = *patch.ShowLossReasonField
	}
	if patch.ShowCancelReasonField != nil {
		base.ShowCancelReasonField = *patch.ShowCancelReasonField
	}
	if patch.ShowStopReasonField != nil {
		base.ShowStopReasonField = *patch.ShowStopReasonField
	}
	if patch.AllowProductSeenNone != nil {
		base.AllowProductSeenNone = *patch.AllowProductSeenNone
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
	if patch.CancelReasonInputMode != nil {
		base.CancelReasonInputMode = normalizeEnum(*patch.CancelReasonInputMode, []string{"text", "select", "select-with-other", "select_other", "select-other"}, base.CancelReasonInputMode)
	}
	if patch.StopReasonInputMode != nil {
		base.StopReasonInputMode = normalizeEnum(*patch.StopReasonInputMode, []string{"text", "select", "select-with-other", "select_other", "select-other"}, base.StopReasonInputMode)
	}
	if patch.RequireCustomerNameField != nil {
		base.RequireCustomerNameField = *patch.RequireCustomerNameField
	}
	if patch.RequireCustomerPhoneField != nil {
		base.RequireCustomerPhoneField = *patch.RequireCustomerPhoneField
	}
	if patch.RequireEmailField != nil {
		base.RequireEmailField = *patch.RequireEmailField
	}
	if patch.RequireProfessionField != nil {
		base.RequireProfessionField = *patch.RequireProfessionField
	}
	if patch.RequireNotesField != nil {
		base.RequireNotesField = *patch.RequireNotesField
	}
	if patch.RequireProduct != nil {
		base.RequireProduct = *patch.RequireProduct
	}
	if patch.RequireProductSeenField != nil {
		base.RequireProductSeenField = *patch.RequireProductSeenField
	}
	if patch.RequireProductSeenNotesField != nil {
		base.RequireProductSeenNotesField = *patch.RequireProductSeenNotesField
	}
	if patch.RequireProductClosedField != nil {
		base.RequireProductClosedField = *patch.RequireProductClosedField
	}
	if patch.RequirePurchaseCodeField != nil {
		base.RequirePurchaseCodeField = *patch.RequirePurchaseCodeField
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
	if patch.RequireCustomerNameJustification != nil {
		base.RequireCustomerNameJustification = *patch.RequireCustomerNameJustification
	}
	if patch.CustomerNameJustificationMinChars != nil {
		base.CustomerNameJustificationMinChars = maxInt(*patch.CustomerNameJustificationMinChars, 1)
	}
	if patch.RequireCustomerPhoneJustification != nil {
		base.RequireCustomerPhoneJustification = *patch.RequireCustomerPhoneJustification
	}
	if patch.CustomerPhoneJustificationMinChars != nil {
		base.CustomerPhoneJustificationMinChars = maxInt(*patch.CustomerPhoneJustificationMinChars, 1)
	}
	if patch.RequireEmailJustification != nil {
		base.RequireEmailJustification = *patch.RequireEmailJustification
	}
	if patch.EmailJustificationMinChars != nil {
		base.EmailJustificationMinChars = maxInt(*patch.EmailJustificationMinChars, 1)
	}
	if patch.RequireProfessionJustification != nil {
		base.RequireProfessionJustification = *patch.RequireProfessionJustification
	}
	if patch.ProfessionJustificationMinChars != nil {
		base.ProfessionJustificationMinChars = maxInt(*patch.ProfessionJustificationMinChars, 1)
	}
	if patch.RequireExistingCustomerJustification != nil {
		base.RequireExistingCustomerJustification = *patch.RequireExistingCustomerJustification
	}
	if patch.ExistingCustomerJustificationMinChars != nil {
		base.ExistingCustomerJustificationMinChars = maxInt(*patch.ExistingCustomerJustificationMinChars, 1)
	}
	if patch.RequireNotesJustification != nil {
		base.RequireNotesJustification = *patch.RequireNotesJustification
	}
	if patch.NotesJustificationMinChars != nil {
		base.NotesJustificationMinChars = maxInt(*patch.NotesJustificationMinChars, 1)
	}
	if patch.RequireProductSeenJustification != nil {
		base.RequireProductSeenJustification = *patch.RequireProductSeenJustification
	}
	if patch.ProductSeenJustificationMinChars != nil {
		base.ProductSeenJustificationMinChars = maxInt(*patch.ProductSeenJustificationMinChars, 1)
	}
	if patch.RequireProductSeenNotesJustification != nil {
		base.RequireProductSeenNotesJustification = *patch.RequireProductSeenNotesJustification
	}
	if patch.ProductSeenNotesJustificationMinChars != nil {
		base.ProductSeenNotesJustificationMinChars = maxInt(*patch.ProductSeenNotesJustificationMinChars, 1)
	}
	if patch.RequireProductClosedJustification != nil {
		base.RequireProductClosedJustification = *patch.RequireProductClosedJustification
	}
	if patch.ProductClosedJustificationMinChars != nil {
		base.ProductClosedJustificationMinChars = maxInt(*patch.ProductClosedJustificationMinChars, 1)
	}
	if patch.RequirePurchaseCodeJustification != nil {
		base.RequirePurchaseCodeJustification = *patch.RequirePurchaseCodeJustification
	}
	if patch.PurchaseCodeJustificationMinChars != nil {
		base.PurchaseCodeJustificationMinChars = maxInt(*patch.PurchaseCodeJustificationMinChars, 1)
	}
	if patch.RequireVisitReasonJustification != nil {
		base.RequireVisitReasonJustification = *patch.RequireVisitReasonJustification
	}
	if patch.VisitReasonJustificationMinChars != nil {
		base.VisitReasonJustificationMinChars = maxInt(*patch.VisitReasonJustificationMinChars, 1)
	}
	if patch.RequireCustomerSourceJustification != nil {
		base.RequireCustomerSourceJustification = *patch.RequireCustomerSourceJustification
	}
	if patch.CustomerSourceJustificationMinChars != nil {
		base.CustomerSourceJustificationMinChars = maxInt(*patch.CustomerSourceJustificationMinChars, 1)
	}
	if patch.RequireProductSeenNotesWhenNone != nil {
		base.RequireProductSeenNotesWhenNone = *patch.RequireProductSeenNotesWhenNone
	}
	if patch.ProductSeenNotesMinChars != nil {
		base.ProductSeenNotesMinChars = maxInt(*patch.ProductSeenNotesMinChars, 1)
	}
	if patch.RequireQueueJumpReasonJustification != nil {
		base.RequireQueueJumpReasonJustification = *patch.RequireQueueJumpReasonJustification
	}
	if patch.QueueJumpReasonJustificationMinChars != nil {
		base.QueueJumpReasonJustificationMinChars = maxInt(*patch.QueueJumpReasonJustificationMinChars, 1)
	}
	if patch.RequireLossReasonJustification != nil {
		base.RequireLossReasonJustification = *patch.RequireLossReasonJustification
	}
	if patch.LossReasonJustificationMinChars != nil {
		base.LossReasonJustificationMinChars = maxInt(*patch.LossReasonJustificationMinChars, 1)
	}
	if patch.RequireQueueJumpReasonField != nil {
		base.RequireQueueJumpReasonField = *patch.RequireQueueJumpReasonField
	}
	if patch.RequireLossReasonField != nil {
		base.RequireLossReasonField = *patch.RequireLossReasonField
	}
	if patch.RequireCancelReasonField != nil {
		base.RequireCancelReasonField = *patch.RequireCancelReasonField
	}
	if patch.RequireStopReasonField != nil {
		base.RequireStopReasonField = *patch.RequireStopReasonField
	}

	return base
}

func recordToBundle(record Record) Bundle {
	bundle := DefaultBundle(record.TenantID, record.SelectedOperationTemplateID)
	bundle.SelectedOperationTemplateID = record.SelectedOperationTemplateID
	bundle.Settings = record.Settings
	bundle.ModalConfig = record.ModalConfig
	bundle.VisitReasonOptions = cloneOptions(record.VisitReasonOptions)
	bundle.CustomerSourceOptions = cloneOptions(record.CustomerSourceOptions)
	bundle.PauseReasonOptions = cloneOptions(record.PauseReasonOptions)
	bundle.CancelReasonOptions = cloneOptions(record.CancelReasonOptions)
	bundle.StopReasonOptions = cloneOptions(record.StopReasonOptions)
	bundle.QueueJumpReasonOptions = cloneOptions(record.QueueJumpReasonOptions)
	bundle.LossReasonOptions = cloneOptions(record.LossReasonOptions)
	bundle.ProfessionOptions = cloneOptions(record.ProfessionOptions)
	bundle.ProductCatalog = cloneProducts(record.ProductCatalog)
	bundle.OperationTemplates = DefaultOperationTemplates()
	return bundle
}

func bundleToRecord(bundle Bundle) Record {
	return Record{
		TenantID:                    bundle.TenantID,
		SelectedOperationTemplateID: bundle.SelectedOperationTemplateID,
		Settings:                    bundle.Settings,
		ModalConfig:                 bundle.ModalConfig,
		VisitReasonOptions:          cloneOptions(bundle.VisitReasonOptions),
		CustomerSourceOptions:       cloneOptions(bundle.CustomerSourceOptions),
		PauseReasonOptions:          cloneOptions(bundle.PauseReasonOptions),
		CancelReasonOptions:         cloneOptions(bundle.CancelReasonOptions),
		StopReasonOptions:           cloneOptions(bundle.StopReasonOptions),
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
	base.FinishFlowMode = normalizeEnum(input.FinishFlowMode, []string{"legacy", "erp-reconciliation"}, base.FinishFlowMode)
	base.ProductSeenLabel = fallbackString(input.ProductSeenLabel, base.ProductSeenLabel)
	base.ProductSeenPlaceholder = fallbackString(input.ProductSeenPlaceholder, base.ProductSeenPlaceholder)
	base.ProductClosedLabel = fallbackString(input.ProductClosedLabel, base.ProductClosedLabel)
	base.ProductClosedPlaceholder = fallbackString(input.ProductClosedPlaceholder, base.ProductClosedPlaceholder)
	base.PurchaseCodeLabel = fallbackString(input.PurchaseCodeLabel, base.PurchaseCodeLabel)
	base.PurchaseCodePlaceholder = fallbackString(input.PurchaseCodePlaceholder, base.PurchaseCodePlaceholder)
	base.NotesLabel = fallbackString(input.NotesLabel, base.NotesLabel)
	base.NotesPlaceholder = fallbackString(input.NotesPlaceholder, base.NotesPlaceholder)
	base.QueueJumpReasonLabel = fallbackString(input.QueueJumpReasonLabel, base.QueueJumpReasonLabel)
	base.QueueJumpReasonPlaceholder = fallbackString(input.QueueJumpReasonPlaceholder, base.QueueJumpReasonPlaceholder)
	base.LossReasonLabel = fallbackString(input.LossReasonLabel, base.LossReasonLabel)
	base.LossReasonPlaceholder = fallbackString(input.LossReasonPlaceholder, base.LossReasonPlaceholder)
	base.CustomerSectionLabel = fallbackString(input.CustomerSectionLabel, base.CustomerSectionLabel)
	base.CustomerNameLabel = fallbackString(input.CustomerNameLabel, base.CustomerNameLabel)
	base.CustomerPhoneLabel = fallbackString(input.CustomerPhoneLabel, base.CustomerPhoneLabel)
	base.CustomerEmailLabel = fallbackString(input.CustomerEmailLabel, base.CustomerEmailLabel)
	base.CustomerProfessionLabel = fallbackString(input.CustomerProfessionLabel, base.CustomerProfessionLabel)
	base.ExistingCustomerLabel = fallbackString(input.ExistingCustomerLabel, base.ExistingCustomerLabel)
	base.ProductSeenNotesLabel = fallbackString(input.ProductSeenNotesLabel, base.ProductSeenNotesLabel)
	base.ProductSeenNotesPlaceholder = fallbackString(input.ProductSeenNotesPlaceholder, base.ProductSeenNotesPlaceholder)
	base.VisitReasonLabel = fallbackString(input.VisitReasonLabel, base.VisitReasonLabel)
	base.CustomerSourceLabel = fallbackString(input.CustomerSourceLabel, base.CustomerSourceLabel)
	base.CancelReasonLabel = fallbackString(input.CancelReasonLabel, base.CancelReasonLabel)
	base.CancelReasonPlaceholder = fallbackString(input.CancelReasonPlaceholder, base.CancelReasonPlaceholder)
	base.CancelReasonOtherLabel = fallbackString(input.CancelReasonOtherLabel, base.CancelReasonOtherLabel)
	base.CancelReasonOtherPlaceholder = fallbackString(input.CancelReasonOtherPlaceholder, base.CancelReasonOtherPlaceholder)
	base.StopReasonLabel = fallbackString(input.StopReasonLabel, base.StopReasonLabel)
	base.StopReasonPlaceholder = fallbackString(input.StopReasonPlaceholder, base.StopReasonPlaceholder)
	base.StopReasonOtherLabel = fallbackString(input.StopReasonOtherLabel, base.StopReasonOtherLabel)
	base.StopReasonOtherPlaceholder = fallbackString(input.StopReasonOtherPlaceholder, base.StopReasonOtherPlaceholder)
	base.ShowCustomerNameField = input.ShowCustomerNameField
	base.ShowCustomerPhoneField = input.ShowCustomerPhoneField
	base.ShowEmailField = input.ShowEmailField
	base.ShowProfessionField = input.ShowProfessionField
	base.ShowNotesField = input.ShowNotesField
	base.ShowProductSeenField = input.ShowProductSeenField
	base.ShowProductSeenNotesField = input.ShowProductSeenNotesField
	base.ShowProductClosedField = input.ShowProductClosedField
	base.ShowPurchaseCodeField = input.ShowPurchaseCodeField
	base.ShowVisitReasonField = input.ShowVisitReasonField
	base.ShowCustomerSourceField = input.ShowCustomerSourceField
	base.ShowExistingCustomerField = input.ShowExistingCustomerField
	base.ShowQueueJumpReasonField = input.ShowQueueJumpReasonField
	base.ShowLossReasonField = input.ShowLossReasonField
	base.ShowCancelReasonField = input.ShowCancelReasonField
	base.ShowStopReasonField = input.ShowStopReasonField
	base.AllowProductSeenNone = input.AllowProductSeenNone
	base.VisitReasonSelectionMode = normalizeEnum(input.VisitReasonSelectionMode, []string{"single", "multiple"}, base.VisitReasonSelectionMode)
	base.VisitReasonDetailMode = normalizeEnum(input.VisitReasonDetailMode, []string{"off", "shared", "per-item"}, base.VisitReasonDetailMode)
	base.LossReasonSelectionMode = normalizeEnum(input.LossReasonSelectionMode, []string{"single", "multiple"}, base.LossReasonSelectionMode)
	base.LossReasonDetailMode = normalizeEnum(input.LossReasonDetailMode, []string{"off", "shared", "per-item"}, base.LossReasonDetailMode)
	base.CustomerSourceSelectionMode = normalizeEnum(input.CustomerSourceSelectionMode, []string{"single", "multiple"}, base.CustomerSourceSelectionMode)
	base.CustomerSourceDetailMode = normalizeEnum(input.CustomerSourceDetailMode, []string{"off", "shared", "per-item"}, base.CustomerSourceDetailMode)
	base.CancelReasonInputMode = normalizeEnum(input.CancelReasonInputMode, []string{"text", "select", "select-with-other", "select_other", "select-other"}, base.CancelReasonInputMode)
	base.StopReasonInputMode = normalizeEnum(input.StopReasonInputMode, []string{"text", "select", "select-with-other", "select_other", "select-other"}, base.StopReasonInputMode)
	base.RequireCustomerNameField = input.RequireCustomerNameField
	base.RequireCustomerPhoneField = input.RequireCustomerPhoneField
	base.RequireEmailField = input.RequireEmailField
	base.RequireProfessionField = input.RequireProfessionField
	base.RequireNotesField = input.RequireNotesField
	base.RequireProduct = input.RequireProduct
	base.RequireProductSeenField = input.RequireProductSeenField
	base.RequireProductSeenNotesField = input.RequireProductSeenNotesField
	base.RequireProductClosedField = input.RequireProductClosedField
	base.RequirePurchaseCodeField = input.RequirePurchaseCodeField
	base.RequireVisitReason = input.RequireVisitReason
	base.RequireCustomerSource = input.RequireCustomerSource
	base.RequireCustomerNamePhone = input.RequireCustomerNamePhone
	base.RequireCustomerNameJustification = input.RequireCustomerNameJustification
	if input.CustomerNameJustificationMinChars > 0 {
		base.CustomerNameJustificationMinChars = input.CustomerNameJustificationMinChars
	}
	base.RequireCustomerPhoneJustification = input.RequireCustomerPhoneJustification
	if input.CustomerPhoneJustificationMinChars > 0 {
		base.CustomerPhoneJustificationMinChars = input.CustomerPhoneJustificationMinChars
	}
	base.RequireEmailJustification = input.RequireEmailJustification
	if input.EmailJustificationMinChars > 0 {
		base.EmailJustificationMinChars = input.EmailJustificationMinChars
	}
	base.RequireProfessionJustification = input.RequireProfessionJustification
	if input.ProfessionJustificationMinChars > 0 {
		base.ProfessionJustificationMinChars = input.ProfessionJustificationMinChars
	}
	base.RequireExistingCustomerJustification = input.RequireExistingCustomerJustification
	if input.ExistingCustomerJustificationMinChars > 0 {
		base.ExistingCustomerJustificationMinChars = input.ExistingCustomerJustificationMinChars
	}
	base.RequireNotesJustification = input.RequireNotesJustification
	if input.NotesJustificationMinChars > 0 {
		base.NotesJustificationMinChars = input.NotesJustificationMinChars
	}
	base.RequireProductSeenJustification = input.RequireProductSeenJustification
	if input.ProductSeenJustificationMinChars > 0 {
		base.ProductSeenJustificationMinChars = input.ProductSeenJustificationMinChars
	}
	base.RequireProductSeenNotesJustification = input.RequireProductSeenNotesJustification
	if input.ProductSeenNotesJustificationMinChars > 0 {
		base.ProductSeenNotesJustificationMinChars = input.ProductSeenNotesJustificationMinChars
	}
	base.RequireProductClosedJustification = input.RequireProductClosedJustification
	if input.ProductClosedJustificationMinChars > 0 {
		base.ProductClosedJustificationMinChars = input.ProductClosedJustificationMinChars
	}
	base.RequirePurchaseCodeJustification = input.RequirePurchaseCodeJustification
	if input.PurchaseCodeJustificationMinChars > 0 {
		base.PurchaseCodeJustificationMinChars = input.PurchaseCodeJustificationMinChars
	}
	base.RequireVisitReasonJustification = input.RequireVisitReasonJustification
	if input.VisitReasonJustificationMinChars > 0 {
		base.VisitReasonJustificationMinChars = input.VisitReasonJustificationMinChars
	}
	base.RequireCustomerSourceJustification = input.RequireCustomerSourceJustification
	if input.CustomerSourceJustificationMinChars > 0 {
		base.CustomerSourceJustificationMinChars = input.CustomerSourceJustificationMinChars
	}
	base.RequireProductSeenNotesWhenNone = input.RequireProductSeenNotesWhenNone
	if input.ProductSeenNotesMinChars > 0 {
		base.ProductSeenNotesMinChars = input.ProductSeenNotesMinChars
	}
	base.RequireQueueJumpReasonJustification = input.RequireQueueJumpReasonJustification
	if input.QueueJumpReasonJustificationMinChars > 0 {
		base.QueueJumpReasonJustificationMinChars = input.QueueJumpReasonJustificationMinChars
	}
	base.RequireLossReasonJustification = input.RequireLossReasonJustification
	if input.LossReasonJustificationMinChars > 0 {
		base.LossReasonJustificationMinChars = input.LossReasonJustificationMinChars
	}
	base.RequireQueueJumpReasonField = input.RequireQueueJumpReasonField
	base.RequireLossReasonField = input.RequireLossReasonField
	base.RequireCancelReasonField = input.RequireCancelReasonField
	base.RequireStopReasonField = input.RequireStopReasonField
	return base
}

func materializeBundleDefaults(bundle Bundle) Bundle {
	defaults := DefaultBundle(bundle.TenantID, bundle.SelectedOperationTemplateID)
	bundle.ModalConfig = normalizeModalConfig(defaults.ModalConfig, bundle.ModalConfig)

	if len(bundle.VisitReasonOptions) == 0 {
		bundle.VisitReasonOptions = cloneOptions(defaults.VisitReasonOptions)
	}
	if len(bundle.CustomerSourceOptions) == 0 {
		bundle.CustomerSourceOptions = cloneOptions(defaults.CustomerSourceOptions)
	}
	if len(bundle.PauseReasonOptions) == 0 {
		bundle.PauseReasonOptions = cloneOptions(defaults.PauseReasonOptions)
	}
	if len(bundle.CancelReasonOptions) == 0 {
		bundle.CancelReasonOptions = cloneOptions(defaults.CancelReasonOptions)
	}
	if len(bundle.StopReasonOptions) == 0 {
		bundle.StopReasonOptions = cloneOptions(defaults.StopReasonOptions)
	}
	if len(bundle.QueueJumpReasonOptions) == 0 {
		bundle.QueueJumpReasonOptions = cloneOptions(defaults.QueueJumpReasonOptions)
	}
	if len(bundle.LossReasonOptions) == 0 {
		bundle.LossReasonOptions = cloneOptions(defaults.LossReasonOptions)
	}
	if len(bundle.ProfessionOptions) == 0 {
		bundle.ProfessionOptions = cloneOptions(defaults.ProfessionOptions)
	}
	if len(bundle.ProductCatalog) == 0 {
		bundle.ProductCatalog = cloneProducts(defaults.ProductCatalog)
	}

	return bundle
}

func getOptionGroupItems(bundle Bundle, optionGroup string) ([]OptionItem, error) {
	switch optionGroup {
	case optionKindVisitReason:
		return cloneOptions(bundle.VisitReasonOptions), nil
	case optionKindCustomerSource:
		return cloneOptions(bundle.CustomerSourceOptions), nil
	case optionKindPauseReason:
		return cloneOptions(bundle.PauseReasonOptions), nil
	case optionKindCancelReason:
		return cloneOptions(bundle.CancelReasonOptions), nil
	case optionKindStopReason:
		return cloneOptions(bundle.StopReasonOptions), nil
	case optionKindQueueJump:
		return cloneOptions(bundle.QueueJumpReasonOptions), nil
	case optionKindLossReason:
		return cloneOptions(bundle.LossReasonOptions), nil
	case optionKindProfession:
		return cloneOptions(bundle.ProfessionOptions), nil
	default:
		return nil, ErrValidation
	}
}

func upsertOptionGroupItem(items []OptionItem, item OptionItem) ([]OptionItem, bool) {
	normalizedItems := normalizeOptions([]OptionItem{item}, nil)
	if len(normalizedItems) != 1 {
		return nil, false
	}

	nextItems := cloneOptions(items)
	nextItem := normalizedItems[0]

	for index, current := range nextItems {
		if current.ID == nextItem.ID {
			nextItems[index] = nextItem
			return nextItems, true
		}
	}

	return append(nextItems, nextItem), true
}

func removeOptionGroupItem(items []OptionItem, optionID string) []OptionItem {
	nextItems := make([]OptionItem, 0, len(items))
	for _, item := range items {
		if item.ID != optionID {
			nextItems = append(nextItems, item)
		}
	}

	return nextItems
}

func upsertProductCatalogItem(items []ProductItem, product ProductItem) ([]ProductItem, bool) {
	normalizedItems := normalizeProducts([]ProductItem{product}, nil)
	if len(normalizedItems) != 1 {
		return nil, false
	}

	nextProducts := cloneProducts(items)
	nextProduct := normalizedItems[0]

	for index, current := range nextProducts {
		if current.ID == nextProduct.ID {
			nextProducts[index] = nextProduct
			return nextProducts, true
		}
	}

	return append(nextProducts, nextProduct), true
}

func removeProductCatalogItem(items []ProductItem, productID string) []ProductItem {
	nextProducts := make([]ProductItem, 0, len(items))
	for _, item := range items {
		if item.ID != productID {
			nextProducts = append(nextProducts, item)
		}
	}

	return nextProducts
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
