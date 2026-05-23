package settings

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

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

