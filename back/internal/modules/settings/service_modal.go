package settings

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
