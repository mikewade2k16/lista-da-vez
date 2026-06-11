package settings

import (
	"encoding/json"
	"strings"
)

func splitAppSettings(settings AppSettings) (OperationCoreSettings, AlertSettings) {
	return OperationCoreSettings{
			MaxConcurrentServices:              settings.MaxConcurrentServices,
			MaxConcurrentServicesPerConsultant: settings.MaxConcurrentServicesPerConsultant,
			TimingFastCloseMinutes:             settings.TimingFastCloseMinutes,
			TimingLongServiceMinutes:           settings.TimingLongServiceMinutes,
			TimingLowSaleAmount:                settings.TimingLowSaleAmount,
			ServiceCancelWindowSeconds:         settings.ServiceCancelWindowSeconds,
			TestModeEnabled:                    settings.TestModeEnabled,
			AutoFillFinishModal:                settings.AutoFillFinishModal,
			ScoreWeightConversion:              settings.ScoreWeightConversion,
			ScoreWeightSoldValue:               settings.ScoreWeightSoldValue,
			ScoreWeightQuality:                 settings.ScoreWeightQuality,
			ScoreWeightPa:                      settings.ScoreWeightPa,
			ScoreWeightQueueDiscipline:         settings.ScoreWeightQueueDiscipline,
			CRMListUsageTiers:                  settings.CRMListUsageTiers,
			CRMListUsageMinOrdersForHighlight:  settings.CRMListUsageMinOrdersForHighlight,
			CRMGoalPayoutPolicy:                settings.CRMGoalPayoutPolicy,
		}, AlertSettings{
			AlertMinConversionRate: settings.AlertMinConversionRate,
			AlertMaxQueueJumpRate:  settings.AlertMaxQueueJumpRate,
			AlertMinPaScore:        settings.AlertMinPaScore,
			AlertMinTicketAverage:  settings.AlertMinTicketAverage,
		}
}

func composeAppSettings(core OperationCoreSettings, alerts AlertSettings) AppSettings {
	return AppSettings{
		MaxConcurrentServices:              core.MaxConcurrentServices,
		MaxConcurrentServicesPerConsultant: core.MaxConcurrentServicesPerConsultant,
		TimingFastCloseMinutes:             core.TimingFastCloseMinutes,
		TimingLongServiceMinutes:           core.TimingLongServiceMinutes,
		TimingLowSaleAmount:                core.TimingLowSaleAmount,
		ServiceCancelWindowSeconds:         core.ServiceCancelWindowSeconds,
		TestModeEnabled:                    core.TestModeEnabled,
		AutoFillFinishModal:                core.AutoFillFinishModal,
		ScoreWeightConversion:              core.ScoreWeightConversion,
		ScoreWeightSoldValue:               core.ScoreWeightSoldValue,
		ScoreWeightQuality:                 core.ScoreWeightQuality,
		ScoreWeightPa:                      core.ScoreWeightPa,
		ScoreWeightQueueDiscipline:         core.ScoreWeightQueueDiscipline,
		CRMListUsageTiers:                  core.CRMListUsageTiers,
		CRMListUsageMinOrdersForHighlight:  core.CRMListUsageMinOrdersForHighlight,
		CRMGoalPayoutPolicy:                core.CRMGoalPayoutPolicy,
		AlertMinConversionRate:             alerts.AlertMinConversionRate,
		AlertMaxQueueJumpRate:              alerts.AlertMaxQueueJumpRate,
		AlertMinPaScore:                    alerts.AlertMinPaScore,
		AlertMinTicketAverage:              alerts.AlertMinTicketAverage,
	}
}

func splitAppSettingsPatch(patch AppSettingsPatch) (OperationCoreSettingsPatch, AlertSettingsPatch) {
	return OperationCoreSettingsPatch{
			MaxConcurrentServices:              patch.MaxConcurrentServices,
			MaxConcurrentServicesPerConsultant: patch.MaxConcurrentServicesPerConsultant,
			TimingFastCloseMinutes:             patch.TimingFastCloseMinutes,
			TimingLongServiceMinutes:           patch.TimingLongServiceMinutes,
			TimingLowSaleAmount:                patch.TimingLowSaleAmount,
			ServiceCancelWindowSeconds:         patch.ServiceCancelWindowSeconds,
			TestModeEnabled:                    patch.TestModeEnabled,
			AutoFillFinishModal:                patch.AutoFillFinishModal,
			ScoreWeightConversion:              patch.ScoreWeightConversion,
			ScoreWeightSoldValue:               patch.ScoreWeightSoldValue,
			ScoreWeightQuality:                 patch.ScoreWeightQuality,
			ScoreWeightPa:                      patch.ScoreWeightPa,
			ScoreWeightQueueDiscipline:         patch.ScoreWeightQueueDiscipline,
			CRMListUsageTiers:                  patch.CRMListUsageTiers,
			CRMListUsageMinOrdersForHighlight:  patch.CRMListUsageMinOrdersForHighlight,
			CRMGoalPayoutPolicy:                patch.CRMGoalPayoutPolicy,
		}, AlertSettingsPatch{
			AlertMinConversionRate: patch.AlertMinConversionRate,
			AlertMaxQueueJumpRate:  patch.AlertMaxQueueJumpRate,
			AlertMinPaScore:        patch.AlertMinPaScore,
			AlertMinTicketAverage:  patch.AlertMinTicketAverage,
		}
}

func defaultOperationSectionRecord(tenantID string, selectedTemplateID string) OperationSectionRecord {
	bundle := DefaultBundle(tenantID, selectedTemplateID)
	coreSettings, alertSettings := splitAppSettings(bundle.Settings)

	return OperationSectionRecord{
		TenantID:                    tenantID,
		SelectedOperationTemplateID: bundle.SelectedOperationTemplateID,
		CoreSettings:                coreSettings,
		AlertSettings:               alertSettings,
	}
}

func defaultModalSectionRecord(tenantID string, selectedTemplateID string) ModalSectionRecord {
	bundle := DefaultBundle(tenantID, selectedTemplateID)

	return ModalSectionRecord{
		TenantID:                    tenantID,
		SelectedOperationTemplateID: bundle.SelectedOperationTemplateID,
		ModalConfig:                 bundle.ModalConfig,
	}
}

func defaultAppearanceSectionRecord(tenantID string) AppearanceSectionRecord {
	bundle := DefaultBundle(tenantID, defaultTemplateID)

	return AppearanceSectionRecord{
		TenantID:   tenantID,
		Appearance: cloneAppearanceConfig(bundle.Appearance),
	}
}

func defaultOptionGroupItems(selectedTemplateID string, optionGroup string) ([]OptionItem, error) {
	return getOptionGroupItems(DefaultBundle("", selectedTemplateID), optionGroup)
}

func defaultProductCatalogItems() []ProductItem {
	return cloneProducts(defaultProductCatalog())
}

func recordToOperationSection(record Record) OperationSectionRecord {
	coreSettings, alertSettings := splitAppSettings(record.Settings)

	return OperationSectionRecord{
		TenantID:                    record.TenantID,
		SelectedOperationTemplateID: record.SelectedOperationTemplateID,
		CoreSettings:                coreSettings,
		AlertSettings:               alertSettings,
		CreatedAt:                   record.CreatedAt,
		UpdatedAt:                   record.UpdatedAt,
	}
}

func recordToModalSection(record Record) ModalSectionRecord {
	return ModalSectionRecord{
		TenantID:                    record.TenantID,
		SelectedOperationTemplateID: record.SelectedOperationTemplateID,
		ModalConfig:                 record.ModalConfig,
		CreatedAt:                   record.CreatedAt,
		UpdatedAt:                   record.UpdatedAt,
	}
}

func recordToAppearanceSection(record Record) AppearanceSectionRecord {
	return AppearanceSectionRecord{
		TenantID:   record.TenantID,
		Appearance: cloneAppearanceConfig(record.Appearance),
		CreatedAt:  record.CreatedAt,
		UpdatedAt:  record.UpdatedAt,
	}
}

func cloneAppearanceOverrides(input AppearanceOverrides) AppearanceOverrides {
	if input == nil {
		return AppearanceOverrides{}
	}

	cloned := make(AppearanceOverrides, len(input))
	for rawTheme, values := range input {
		theme := strings.TrimSpace(rawTheme)
		if theme == "" || values == nil {
			continue
		}

		nextValues := make(map[string]string, len(values))
		for rawKey, rawValue := range values {
			key := strings.TrimSpace(rawKey)
			value := strings.TrimSpace(rawValue)
			if key == "" || value == "" {
				continue
			}

			nextValues[key] = value
		}

		if len(nextValues) == 0 {
			continue
		}

		cloned[theme] = nextValues
	}

	return cloned
}

func cloneAppearanceConfig(input AppearanceConfig) AppearanceConfig {
	return AppearanceConfig{
		ActiveTheme:     strings.TrimSpace(input.ActiveTheme),
		CustomThemeName: strings.TrimSpace(input.CustomThemeName),
		Overrides:       cloneAppearanceOverrides(input.Overrides),
	}
}

func normalizeRawJSON(input json.RawMessage, fallback json.RawMessage) json.RawMessage {
	if len(input) > 0 && json.Valid(input) {
		return append(json.RawMessage(nil), input...)
	}
	if len(fallback) > 0 && json.Valid(fallback) {
		return append(json.RawMessage(nil), fallback...)
	}
	return json.RawMessage(`null`)
}

func normalizeAppearanceTheme(value string, fallback string) string {
	switch strings.TrimSpace(value) {
	case "light", "dark", "apple", "custom":
		return strings.TrimSpace(value)
	default:
		if fallback == "dark" || fallback == "apple" || fallback == "custom" {
			return fallback
		}
		return "light"
	}
}

func normalizeAppearanceConfig(input AppearanceConfig, fallback AppearanceConfig) AppearanceConfig {
	next := cloneAppearanceConfig(fallback)
	next.ActiveTheme = normalizeAppearanceTheme(input.ActiveTheme, next.ActiveTheme)

	customThemeName := strings.TrimSpace(input.CustomThemeName)
	if customThemeName == "" {
		customThemeName = strings.TrimSpace(next.CustomThemeName)
	}
	if customThemeName == "" {
		customThemeName = defaultAppearanceConfig().CustomThemeName
	}
	next.CustomThemeName = customThemeName

	if input.Overrides != nil {
		next.Overrides = cloneAppearanceOverrides(input.Overrides)
	}

	if next.Overrides == nil {
		next.Overrides = AppearanceOverrides{}
	}

	return next
}

func applyAppearanceConfigPatch(base AppearanceConfig, patch AppearanceConfigPatch) AppearanceConfig {
	next := normalizeAppearanceConfig(base, defaultAppearanceConfig())

	if patch.ActiveTheme != nil {
		next.ActiveTheme = normalizeAppearanceTheme(*patch.ActiveTheme, next.ActiveTheme)
	}

	if patch.CustomThemeName != nil {
		name := strings.TrimSpace(*patch.CustomThemeName)
		if name == "" {
			name = defaultAppearanceConfig().CustomThemeName
		}
		next.CustomThemeName = name
	}

	if patch.Overrides != nil {
		next.Overrides = cloneAppearanceOverrides(*patch.Overrides)
	}

	return normalizeAppearanceConfig(next, defaultAppearanceConfig())
}

func normalizeOperationCoreSettings(input OperationCoreSettings, fallback OperationCoreSettings) OperationCoreSettings {
	fallback.MaxConcurrentServices = maxInt(input.MaxConcurrentServices, 1)
	maxConcurrent := fallback.MaxConcurrentServices
	perConsultant := maxInt(input.MaxConcurrentServicesPerConsultant, 1)
	if perConsultant > maxConcurrent {
		perConsultant = maxConcurrent
	}
	fallback.MaxConcurrentServicesPerConsultant = perConsultant
	fallback.TimingFastCloseMinutes = maxInt(input.TimingFastCloseMinutes, 1)
	fallback.TimingLongServiceMinutes = maxInt(input.TimingLongServiceMinutes, 1)
	fallback.TimingLowSaleAmount = maxFloat(input.TimingLowSaleAmount, 0)
	fallback.ServiceCancelWindowSeconds = maxInt(input.ServiceCancelWindowSeconds, 0)
	fallback.TestModeEnabled = input.TestModeEnabled
	fallback.AutoFillFinishModal = input.AutoFillFinishModal
	fallback.ScoreWeightConversion = maxFloat(input.ScoreWeightConversion, 0)
	fallback.ScoreWeightSoldValue = maxFloat(input.ScoreWeightSoldValue, 0)
	fallback.ScoreWeightQuality = maxFloat(input.ScoreWeightQuality, 0)
	fallback.ScoreWeightPa = maxFloat(input.ScoreWeightPa, 0)
	fallback.ScoreWeightQueueDiscipline = maxFloat(input.ScoreWeightQueueDiscipline, 0)
	fallback.CRMListUsageTiers = normalizeRawJSON(input.CRMListUsageTiers, fallback.CRMListUsageTiers)
	fallback.CRMListUsageMinOrdersForHighlight = maxInt(input.CRMListUsageMinOrdersForHighlight, 1)
	fallback.CRMGoalPayoutPolicy = normalizeRawJSON(input.CRMGoalPayoutPolicy, fallback.CRMGoalPayoutPolicy)
	return fallback
}

func normalizeAlertSettings(input AlertSettings, fallback AlertSettings) AlertSettings {
	fallback.AlertMinConversionRate = maxFloat(input.AlertMinConversionRate, 0)
	fallback.AlertMaxQueueJumpRate = maxFloat(input.AlertMaxQueueJumpRate, 0)
	fallback.AlertMinPaScore = maxFloat(input.AlertMinPaScore, 0)
	fallback.AlertMinTicketAverage = maxFloat(input.AlertMinTicketAverage, 0)
	return fallback
}

func applyOperationCoreSettingsPatch(base OperationCoreSettings, patch OperationCoreSettingsPatch) OperationCoreSettings {
	if patch.MaxConcurrentServices != nil {
		base.MaxConcurrentServices = maxInt(*patch.MaxConcurrentServices, 1)
	}
	if patch.MaxConcurrentServicesPerConsultant != nil {
		perConsultant := maxInt(*patch.MaxConcurrentServicesPerConsultant, 1)
		if perConsultant > base.MaxConcurrentServices {
			perConsultant = base.MaxConcurrentServices
		}
		base.MaxConcurrentServicesPerConsultant = perConsultant
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
	if patch.ServiceCancelWindowSeconds != nil {
		base.ServiceCancelWindowSeconds = maxInt(*patch.ServiceCancelWindowSeconds, 0)
	}
	if patch.TestModeEnabled != nil {
		base.TestModeEnabled = *patch.TestModeEnabled
	}
	if patch.AutoFillFinishModal != nil {
		base.AutoFillFinishModal = *patch.AutoFillFinishModal
	}
	if patch.ScoreWeightConversion != nil {
		base.ScoreWeightConversion = maxFloat(*patch.ScoreWeightConversion, 0)
	}
	if patch.ScoreWeightSoldValue != nil {
		base.ScoreWeightSoldValue = maxFloat(*patch.ScoreWeightSoldValue, 0)
	}
	if patch.ScoreWeightQuality != nil {
		base.ScoreWeightQuality = maxFloat(*patch.ScoreWeightQuality, 0)
	}
	if patch.ScoreWeightPa != nil {
		base.ScoreWeightPa = maxFloat(*patch.ScoreWeightPa, 0)
	}
	if patch.ScoreWeightQueueDiscipline != nil {
		base.ScoreWeightQueueDiscipline = maxFloat(*patch.ScoreWeightQueueDiscipline, 0)
	}
	if patch.CRMListUsageTiers != nil {
		base.CRMListUsageTiers = normalizeRawJSON(*patch.CRMListUsageTiers, base.CRMListUsageTiers)
	}
	if patch.CRMListUsageMinOrdersForHighlight != nil {
		base.CRMListUsageMinOrdersForHighlight = maxInt(*patch.CRMListUsageMinOrdersForHighlight, 1)
	}
	if patch.CRMGoalPayoutPolicy != nil {
		base.CRMGoalPayoutPolicy = normalizeRawJSON(*patch.CRMGoalPayoutPolicy, base.CRMGoalPayoutPolicy)
	}

	return base
}

func applyOperationTemplateCoreSettings(base OperationCoreSettings, template OperationCoreSettings) OperationCoreSettings {
	next := base
	next.MaxConcurrentServices = template.MaxConcurrentServices
	next.MaxConcurrentServicesPerConsultant = template.MaxConcurrentServicesPerConsultant
	next.TimingFastCloseMinutes = template.TimingFastCloseMinutes
	next.TimingLongServiceMinutes = template.TimingLongServiceMinutes
	next.TimingLowSaleAmount = template.TimingLowSaleAmount
	next.ServiceCancelWindowSeconds = template.ServiceCancelWindowSeconds
	next.ScoreWeightConversion = template.ScoreWeightConversion
	next.ScoreWeightSoldValue = template.ScoreWeightSoldValue
	next.ScoreWeightQuality = template.ScoreWeightQuality
	next.ScoreWeightPa = template.ScoreWeightPa
	next.ScoreWeightQueueDiscipline = template.ScoreWeightQueueDiscipline
	next.CRMListUsageTiers = normalizeRawJSON(template.CRMListUsageTiers, defaultCRMListUsageTiers())
	next.CRMListUsageMinOrdersForHighlight = maxInt(
		template.CRMListUsageMinOrdersForHighlight,
		defaultCRMListUsageMinOrdersForHighlight,
	)
	next.CRMGoalPayoutPolicy = normalizeRawJSON(template.CRMGoalPayoutPolicy, defaultCRMGoalPayoutPolicy())
	return normalizeOperationCoreSettings(next, next)
}

func applyAlertSettingsPatch(base AlertSettings, patch AlertSettingsPatch) AlertSettings {
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

func normalizeOperationSectionRecord(section OperationSectionRecord) OperationSectionRecord {
	defaults := defaultOperationSectionRecord(section.TenantID, section.SelectedOperationTemplateID)
	section.SelectedOperationTemplateID = defaults.SelectedOperationTemplateID
	section.CoreSettings = normalizeOperationCoreSettings(section.CoreSettings, defaults.CoreSettings)
	section.AlertSettings = normalizeAlertSettings(section.AlertSettings, defaults.AlertSettings)
	return section
}

func normalizeModalSectionRecord(section ModalSectionRecord) ModalSectionRecord {
	defaults := defaultModalSectionRecord(section.TenantID, section.SelectedOperationTemplateID)
	section.SelectedOperationTemplateID = defaults.SelectedOperationTemplateID
	section.ModalConfig = normalizeModalConfig(defaults.ModalConfig, section.ModalConfig)
	return section
}

func normalizeAppearanceSectionRecord(section AppearanceSectionRecord) AppearanceSectionRecord {
	defaults := defaultAppearanceSectionRecord(section.TenantID)
	section.Appearance = normalizeAppearanceConfig(section.Appearance, defaults.Appearance)
	return section
}
