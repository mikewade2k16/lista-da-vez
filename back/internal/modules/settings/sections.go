package settings

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

func operationSectionToRecord(section OperationSectionRecord) Record {
	return Record{
		TenantID:                    section.TenantID,
		SelectedOperationTemplateID: section.SelectedOperationTemplateID,
		Settings:                    composeAppSettings(section.CoreSettings, section.AlertSettings),
		CreatedAt:                   section.CreatedAt,
		UpdatedAt:                   section.UpdatedAt,
	}
}

func modalSectionToRecord(section ModalSectionRecord) Record {
	return Record{
		TenantID:                    section.TenantID,
		SelectedOperationTemplateID: section.SelectedOperationTemplateID,
		ModalConfig:                 section.ModalConfig,
		CreatedAt:                   section.CreatedAt,
		UpdatedAt:                   section.UpdatedAt,
	}
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
