package settings

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

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
