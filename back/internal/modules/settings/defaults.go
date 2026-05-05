package settings

const defaultTemplateID = "joalheria-padrao"

var defaultOperationTemplates = []OperationTemplate{
	{
		ID:          "joalheria-padrao",
		Label:       "Joalheria padrao",
		Description: "Equilibrio entre qualidade de atendimento, captura de lead e disciplina de fila.",
		Settings: AppSettings{
			MaxConcurrentServices:              10,
			MaxConcurrentServicesPerConsultant: 1,
			TimingFastCloseMinutes:             5,
			TimingLongServiceMinutes:           25,
			TimingLowSaleAmount:                1200,
			ServiceCancelWindowSeconds:         30,
		},
		ModalConfig: ModalConfig{
			ShowCustomerNameField:           true,
			ShowCustomerPhoneField:          true,
			ShowEmailField:                  true,
			ShowProfessionField:             true,
			ShowNotesField:                  true,
			ShowProductSeenField:            true,
			ShowProductSeenNotesField:       true,
			ShowProductClosedField:          true,
			ShowVisitReasonField:            true,
			ShowCustomerSourceField:         true,
			ShowExistingCustomerField:       true,
			ShowQueueJumpReasonField:        true,
			ShowLossReasonField:             true,
			ShowCancelReasonField:           true,
			ShowStopReasonField:             true,
			AllowProductSeenNone:            true,
			VisitReasonSelectionMode:        "multiple",
			VisitReasonDetailMode:           "shared",
			CustomerSourceSelectionMode:     "single",
			CustomerSourceDetailMode:        "shared",
			CancelReasonInputMode:           "text",
			StopReasonInputMode:             "text",
			RequireCustomerNameField:        true,
			RequireCustomerPhoneField:       true,
			RequireEmailField:               false,
			RequireProfessionField:          false,
			RequireNotesField:               false,
			RequireProduct:                  true,
			RequireProductSeenField:         true,
			RequireProductSeenNotesField:    false,
			RequireProductClosedField:       true,
			RequireVisitReason:              true,
			RequireCustomerSource:           true,
			RequireCustomerNamePhone:        true,
			RequireProductSeenNotesWhenNone: true,
			ProductSeenNotesMinChars:        20,
			RequireQueueJumpReasonField:     true,
			RequireLossReasonField:          true,
			RequireCancelReasonField:        false,
			RequireStopReasonField:          false,
		},
		VisitReasonOptions: []OptionItem{
			{ID: "aniversario-casamento", Label: "Aniversario de casamento"},
			{ID: "pedido-noivado", Label: "Pedido de namoro ou noivado"},
			{ID: "casamento", Label: "Casamento"},
			{ID: "aniversario", Label: "Aniversario"},
			{ID: "quinze-anos", Label: "15 anos"},
			{ID: "formatura", Label: "Formatura"},
			{ID: "evento", Label: "Evento especial"},
			{ID: "promocao", Label: "Promocao ou conquista"},
			{ID: "presente", Label: "Presente"},
			{ID: "auto-presente", Label: "Auto presente"},
			{ID: "retirada", Label: "Retirada de reserva"},
			{ID: "consulta", Label: "Consulta ou pesquisa de preco"},
			{ID: "data-especial", Label: "Outra data especial"},
		},
		CustomerSourceOptions: defaultCustomerSourceOptions(),
	},
	{
		ID:          "joalheria-relacionamento",
		Label:       "Joalheria relacionamento",
		Description: "Mais foco em relacao de longo prazo e coleta completa de dados do cliente.",
		Settings: AppSettings{
			MaxConcurrentServices:              8,
			MaxConcurrentServicesPerConsultant: 1,
			TimingFastCloseMinutes:             7,
			TimingLongServiceMinutes:           35,
			TimingLowSaleAmount:                1500,
			ServiceCancelWindowSeconds:         30,
		},
		ModalConfig: ModalConfig{
			ShowCustomerNameField:           true,
			ShowCustomerPhoneField:          true,
			ShowEmailField:                  true,
			ShowProfessionField:             true,
			ShowNotesField:                  true,
			ShowProductSeenField:            true,
			ShowProductSeenNotesField:       true,
			ShowProductClosedField:          true,
			ShowVisitReasonField:            true,
			ShowCustomerSourceField:         true,
			ShowExistingCustomerField:       true,
			ShowQueueJumpReasonField:        true,
			ShowLossReasonField:             true,
			ShowCancelReasonField:           true,
			ShowStopReasonField:             true,
			AllowProductSeenNone:            true,
			VisitReasonSelectionMode:        "multiple",
			VisitReasonDetailMode:           "shared",
			CustomerSourceSelectionMode:     "single",
			CustomerSourceDetailMode:        "shared",
			CancelReasonInputMode:           "text",
			StopReasonInputMode:             "text",
			RequireCustomerNameField:        true,
			RequireCustomerPhoneField:       true,
			RequireEmailField:               false,
			RequireProfessionField:          false,
			RequireNotesField:               false,
			RequireProduct:                  true,
			RequireProductSeenField:         true,
			RequireProductSeenNotesField:    false,
			RequireProductClosedField:       true,
			RequireVisitReason:              true,
			RequireCustomerSource:           true,
			RequireCustomerNamePhone:        true,
			RequireProductSeenNotesWhenNone: true,
			ProductSeenNotesMinChars:        20,
			RequireQueueJumpReasonField:     true,
			RequireLossReasonField:          true,
			RequireCancelReasonField:        false,
			RequireStopReasonField:          false,
		},
		VisitReasonOptions: []OptionItem{
			{ID: "aniversario-casamento", Label: "Aniversario de casamento"},
			{ID: "noivado", Label: "Noivado"},
			{ID: "casamento", Label: "Casamento"},
			{ID: "presente", Label: "Presente"},
			{ID: "evento-corporativo", Label: "Evento corporativo"},
			{ID: "cliente-recorrente", Label: "Relacionamento com cliente recorrente"},
			{ID: "retirada", Label: "Retirada de reserva"},
			{ID: "consulta", Label: "Consulta ou pesquisa de preco"},
			{ID: "data-especial", Label: "Outra data especial"},
		},
		CustomerSourceOptions: defaultCustomerSourceOptions(),
	},
	{
		ID:          "joalheria-fluxo-rapido",
		Label:       "Joalheria fluxo rapido",
		Description: "Operacao de alto fluxo com fechamento mais objetivo e formulario mais leve.",
		Settings: AppSettings{
			MaxConcurrentServices:              12,
			MaxConcurrentServicesPerConsultant: 1,
			TimingFastCloseMinutes:             3,
			TimingLongServiceMinutes:           18,
			TimingLowSaleAmount:                900,
			ServiceCancelWindowSeconds:         30,
		},
		ModalConfig: ModalConfig{
			ShowCustomerNameField:           true,
			ShowCustomerPhoneField:          true,
			ShowEmailField:                  false,
			ShowProfessionField:             false,
			ShowNotesField:                  false,
			ShowProductSeenField:            true,
			ShowProductSeenNotesField:       true,
			ShowProductClosedField:          true,
			ShowVisitReasonField:            true,
			ShowCustomerSourceField:         true,
			ShowExistingCustomerField:       true,
			ShowQueueJumpReasonField:        true,
			ShowLossReasonField:             true,
			ShowCancelReasonField:           true,
			ShowStopReasonField:             true,
			AllowProductSeenNone:            true,
			VisitReasonSelectionMode:        "multiple",
			VisitReasonDetailMode:           "off",
			CustomerSourceSelectionMode:     "single",
			CustomerSourceDetailMode:        "off",
			CancelReasonInputMode:           "text",
			StopReasonInputMode:             "text",
			RequireCustomerNameField:        true,
			RequireCustomerPhoneField:       true,
			RequireEmailField:               false,
			RequireProfessionField:          false,
			RequireNotesField:               false,
			RequireProduct:                  true,
			RequireProductSeenField:         true,
			RequireProductSeenNotesField:    false,
			RequireProductClosedField:       true,
			RequireVisitReason:              true,
			RequireCustomerSource:           false,
			RequireCustomerNamePhone:        true,
			RequireProductSeenNotesWhenNone: true,
			ProductSeenNotesMinChars:        20,
			RequireQueueJumpReasonField:     true,
			RequireLossReasonField:          true,
			RequireCancelReasonField:        false,
			RequireStopReasonField:          false,
		},
		VisitReasonOptions: []OptionItem{
			{ID: "presente", Label: "Presente"},
			{ID: "auto-presente", Label: "Auto presente"},
			{ID: "promocao", Label: "Promocao ou conquista"},
			{ID: "aniversario", Label: "Aniversario"},
			{ID: "troca", Label: "Troca"},
			{ID: "retirada", Label: "Retirada de reserva"},
			{ID: "consulta", Label: "Consulta ou pesquisa de preco"},
		},
		CustomerSourceOptions: defaultCustomerSourceOptions(),
	},
}

func DefaultBundle(tenantID string, selectedTemplateID string) Bundle {
	template := resolveTemplate(selectedTemplateID)

	return Bundle{
		TenantID:                    tenantID,
		OperationTemplates:          DefaultOperationTemplates(),
		SelectedOperationTemplateID: template.ID,
		Settings: AppSettings{
			MaxConcurrentServices:              template.Settings.MaxConcurrentServices,
			MaxConcurrentServicesPerConsultant: template.Settings.MaxConcurrentServicesPerConsultant,
			TimingFastCloseMinutes:             template.Settings.TimingFastCloseMinutes,
			TimingLongServiceMinutes:           template.Settings.TimingLongServiceMinutes,
			TimingLowSaleAmount:                template.Settings.TimingLowSaleAmount,
			ServiceCancelWindowSeconds:         template.Settings.ServiceCancelWindowSeconds,
			TestModeEnabled:                    false,
			AutoFillFinishModal:                false,
			AlertMinConversionRate:             0,
			AlertMaxQueueJumpRate:              0,
			AlertMinPaScore:                    0,
			AlertMinTicketAverage:              0,
		},
		ModalConfig:            mergeModalConfig(defaultBaseModalConfig(), template.ModalConfig),
		VisitReasonOptions:     cloneOptions(template.VisitReasonOptions),
		CustomerSourceOptions:  cloneOptions(template.CustomerSourceOptions),
		PauseReasonOptions:     defaultPauseReasonOptions(),
		CancelReasonOptions:    defaultCancelReasonOptions(),
		StopReasonOptions:      defaultStopReasonOptions(),
		QueueJumpReasonOptions: defaultQueueJumpReasonOptions(),
		LossReasonOptions:      defaultLossReasonOptions(),
		ProfessionOptions:      defaultProfessionOptions(),
		ProductCatalog:         defaultProductCatalog(),
	}
}

func DefaultOperationTemplates() []OperationTemplate {
	templates := make([]OperationTemplate, 0, len(defaultOperationTemplates))
	for _, template := range defaultOperationTemplates {
		templates = append(templates, OperationTemplate{
			ID:                    template.ID,
			Label:                 template.Label,
			Description:           template.Description,
			Settings:              template.Settings,
			ModalConfig:           mergeModalConfig(defaultBaseModalConfig(), template.ModalConfig),
			VisitReasonOptions:    cloneOptions(template.VisitReasonOptions),
			CustomerSourceOptions: cloneOptions(template.CustomerSourceOptions),
		})
	}

	return templates
}

func resolveTemplate(templateID string) OperationTemplate {
	if template, found := findOperationTemplate(templateID); found {
		return template
	}

	return defaultOperationTemplates[0]
}

func findOperationTemplate(templateID string) (OperationTemplate, bool) {
	for _, template := range defaultOperationTemplates {
		if template.ID == templateID {
			return template, true
		}
	}

	return OperationTemplate{}, false
}

func defaultBaseModalConfig() ModalConfig {
	return ModalConfig{
		Title:                                 "Fechar atendimento",
		FinishFlowMode:                        "legacy",
		ProductSeenLabel:                      "Interesses do cliente",
		ProductSeenPlaceholder:                "Busque e selecione interesses",
		ProductClosedLabel:                    "",
		ProductClosedPlaceholder:              "Busque e selecione o produto fechado",
		PurchaseCodeLabel:                     "Codigo da compra",
		PurchaseCodePlaceholder:               "Informe o codigo da compra para conciliacao posterior",
		NotesLabel:                            "Observações",
		NotesPlaceholder:                      "Detalhes adicionais do atendimento",
		QueueJumpReasonLabel:                  "Motivo do atendimento fora da vez",
		QueueJumpReasonPlaceholder:            "Busque e selecione o motivo fora da vez",
		LossReasonLabel:                       "Motivo da perda",
		LossReasonPlaceholder:                 "Busque e selecione o motivo da perda",
		CustomerSectionLabel:                  "Dados do cliente",
		CustomerNameLabel:                     "Nome do cliente",
		CustomerPhoneLabel:                    "Telefone",
		CustomerEmailLabel:                    "E-mail",
		CustomerProfessionLabel:               "Profissão",
		ExistingCustomerLabel:                 "Já era cliente",
		ProductSeenNotesLabel:                 "Observação dos interesses",
		ProductSeenNotesPlaceholder:           "Descreva referência, pedido específico, contexto do cliente ou justificativa quando não houver interesse identificado.",
		VisitReasonLabel:                      "Motivo da visita",
		CustomerSourceLabel:                   "Origem do cliente",
		CancelReasonLabel:                     "Motivo do cancelamento",
		CancelReasonPlaceholder:               "Informe ou selecione o motivo do cancelamento",
		CancelReasonOtherLabel:                "Detalhe do cancelamento",
		CancelReasonOtherPlaceholder:          "Explique por que o atendimento foi cancelado",
		StopReasonLabel:                       "Motivo da parada",
		StopReasonPlaceholder:                 "Informe ou selecione o motivo da parada",
		StopReasonOtherLabel:                  "Detalhe da parada",
		StopReasonOtherPlaceholder:            "Explique por que o atendimento foi parado",
		ShowCustomerNameField:                 true,
		ShowCustomerPhoneField:                true,
		ShowEmailField:                        true,
		ShowProfessionField:                   true,
		ShowNotesField:                        true,
		ShowProductSeenField:                  true,
		ShowProductSeenNotesField:             true,
		ShowProductClosedField:                true,
		ShowPurchaseCodeField:                 true,
		ShowVisitReasonField:                  true,
		ShowCustomerSourceField:               true,
		ShowExistingCustomerField:             true,
		ShowQueueJumpReasonField:              true,
		ShowLossReasonField:                   true,
		ShowCancelReasonField:                 true,
		ShowStopReasonField:                   true,
		AllowProductSeenNone:                  true,
		VisitReasonSelectionMode:              "multiple",
		VisitReasonDetailMode:                 "shared",
		LossReasonSelectionMode:               "single",
		LossReasonDetailMode:                  "off",
		CustomerSourceSelectionMode:           "single",
		CustomerSourceDetailMode:              "shared",
		CancelReasonInputMode:                 "text",
		StopReasonInputMode:                   "text",
		RequireCustomerNameField:              true,
		RequireCustomerPhoneField:             true,
		RequireEmailField:                     false,
		RequireProfessionField:                false,
		RequireNotesField:                     false,
		RequireProduct:                        true,
		RequireProductSeenField:               true,
		RequireProductSeenNotesField:          false,
		RequireProductClosedField:             true,
		RequirePurchaseCodeField:              true,
		RequireVisitReason:                    true,
		RequireCustomerSource:                 true,
		RequireCustomerNamePhone:              true,
		RequireCustomerNameJustification:      false,
		CustomerNameJustificationMinChars:     20,
		RequireCustomerPhoneJustification:     false,
		CustomerPhoneJustificationMinChars:    20,
		RequireEmailJustification:             false,
		EmailJustificationMinChars:            20,
		RequireProfessionJustification:        false,
		ProfessionJustificationMinChars:       20,
		RequireExistingCustomerJustification:  false,
		ExistingCustomerJustificationMinChars: 20,
		RequireNotesJustification:             false,
		NotesJustificationMinChars:            20,
		RequireProductSeenJustification:       false,
		ProductSeenJustificationMinChars:      20,
		RequireProductSeenNotesJustification:  false,
		ProductSeenNotesJustificationMinChars: 20,
		RequireProductClosedJustification:     false,
		ProductClosedJustificationMinChars:    20,
		RequirePurchaseCodeJustification:      false,
		PurchaseCodeJustificationMinChars:     20,
		RequireVisitReasonJustification:       false,
		VisitReasonJustificationMinChars:      20,
		RequireCustomerSourceJustification:    false,
		CustomerSourceJustificationMinChars:   20,
		RequireProductSeenNotesWhenNone:       true,
		ProductSeenNotesMinChars:              20,
		RequireQueueJumpReasonJustification:   false,
		QueueJumpReasonJustificationMinChars:  20,
		RequireLossReasonJustification:        false,
		LossReasonJustificationMinChars:       20,
		RequireQueueJumpReasonField:           true,
		RequireLossReasonField:                true,
		RequireCancelReasonField:              false,
		RequireStopReasonField:                false,
	}
}

func mergeModalConfig(base ModalConfig, override ModalConfig) ModalConfig {
	base.ShowCustomerNameField = override.ShowCustomerNameField
	base.ShowCustomerPhoneField = override.ShowCustomerPhoneField
	base.ShowEmailField = override.ShowEmailField
	base.ShowProfessionField = override.ShowProfessionField
	base.ShowNotesField = override.ShowNotesField
	base.ShowProductSeenField = override.ShowProductSeenField
	base.ShowProductSeenNotesField = override.ShowProductSeenNotesField
	base.ShowProductClosedField = override.ShowProductClosedField
	base.ShowPurchaseCodeField = override.ShowPurchaseCodeField
	base.ShowVisitReasonField = override.ShowVisitReasonField
	base.ShowCustomerSourceField = override.ShowCustomerSourceField
	base.ShowExistingCustomerField = override.ShowExistingCustomerField
	base.ShowQueueJumpReasonField = override.ShowQueueJumpReasonField
	base.ShowLossReasonField = override.ShowLossReasonField
	base.ShowCancelReasonField = override.ShowCancelReasonField
	base.ShowStopReasonField = override.ShowStopReasonField
	base.AllowProductSeenNone = override.AllowProductSeenNone
	base.FinishFlowMode = normalizeEnum(override.FinishFlowMode, []string{"legacy", "erp-reconciliation"}, base.FinishFlowMode)
	base.VisitReasonSelectionMode = override.VisitReasonSelectionMode
	base.VisitReasonDetailMode = override.VisitReasonDetailMode
	base.CustomerSourceSelectionMode = override.CustomerSourceSelectionMode
	base.CustomerSourceDetailMode = override.CustomerSourceDetailMode
	base.RequireCustomerNameField = override.RequireCustomerNameField
	base.RequireCustomerPhoneField = override.RequireCustomerPhoneField
	base.RequireEmailField = override.RequireEmailField
	base.RequireProfessionField = override.RequireProfessionField
	base.RequireNotesField = override.RequireNotesField
	base.RequireProduct = override.RequireProduct
	base.RequireProductSeenField = override.RequireProductSeenField
	base.RequireProductSeenNotesField = override.RequireProductSeenNotesField
	base.RequireProductClosedField = override.RequireProductClosedField
	base.RequirePurchaseCodeField = override.RequirePurchaseCodeField
	base.RequireVisitReason = override.RequireVisitReason
	base.RequireCustomerSource = override.RequireCustomerSource
	base.RequireCustomerNamePhone = override.RequireCustomerNamePhone
	base.RequireCustomerNameJustification = override.RequireCustomerNameJustification
	if override.CustomerNameJustificationMinChars > 0 {
		base.CustomerNameJustificationMinChars = override.CustomerNameJustificationMinChars
	}
	base.RequireCustomerPhoneJustification = override.RequireCustomerPhoneJustification
	if override.CustomerPhoneJustificationMinChars > 0 {
		base.CustomerPhoneJustificationMinChars = override.CustomerPhoneJustificationMinChars
	}
	base.RequireEmailJustification = override.RequireEmailJustification
	if override.EmailJustificationMinChars > 0 {
		base.EmailJustificationMinChars = override.EmailJustificationMinChars
	}
	base.RequireProfessionJustification = override.RequireProfessionJustification
	if override.ProfessionJustificationMinChars > 0 {
		base.ProfessionJustificationMinChars = override.ProfessionJustificationMinChars
	}
	base.RequireExistingCustomerJustification = override.RequireExistingCustomerJustification
	if override.ExistingCustomerJustificationMinChars > 0 {
		base.ExistingCustomerJustificationMinChars = override.ExistingCustomerJustificationMinChars
	}
	base.RequireNotesJustification = override.RequireNotesJustification
	if override.NotesJustificationMinChars > 0 {
		base.NotesJustificationMinChars = override.NotesJustificationMinChars
	}
	base.RequireProductSeenJustification = override.RequireProductSeenJustification
	if override.ProductSeenJustificationMinChars > 0 {
		base.ProductSeenJustificationMinChars = override.ProductSeenJustificationMinChars
	}
	base.RequireProductSeenNotesJustification = override.RequireProductSeenNotesJustification
	if override.ProductSeenNotesJustificationMinChars > 0 {
		base.ProductSeenNotesJustificationMinChars = override.ProductSeenNotesJustificationMinChars
	}
	base.RequireProductClosedJustification = override.RequireProductClosedJustification
	if override.ProductClosedJustificationMinChars > 0 {
		base.ProductClosedJustificationMinChars = override.ProductClosedJustificationMinChars
	}
	base.RequirePurchaseCodeJustification = override.RequirePurchaseCodeJustification
	if override.PurchaseCodeJustificationMinChars > 0 {
		base.PurchaseCodeJustificationMinChars = override.PurchaseCodeJustificationMinChars
	}
	base.RequireVisitReasonJustification = override.RequireVisitReasonJustification
	if override.VisitReasonJustificationMinChars > 0 {
		base.VisitReasonJustificationMinChars = override.VisitReasonJustificationMinChars
	}
	base.RequireCustomerSourceJustification = override.RequireCustomerSourceJustification
	if override.CustomerSourceJustificationMinChars > 0 {
		base.CustomerSourceJustificationMinChars = override.CustomerSourceJustificationMinChars
	}
	base.RequireProductSeenNotesWhenNone = override.RequireProductSeenNotesWhenNone
	if override.ProductSeenNotesMinChars > 0 {
		base.ProductSeenNotesMinChars = override.ProductSeenNotesMinChars
	}
	base.RequireQueueJumpReasonJustification = override.RequireQueueJumpReasonJustification
	if override.QueueJumpReasonJustificationMinChars > 0 {
		base.QueueJumpReasonJustificationMinChars = override.QueueJumpReasonJustificationMinChars
	}
	base.RequireLossReasonJustification = override.RequireLossReasonJustification
	if override.LossReasonJustificationMinChars > 0 {
		base.LossReasonJustificationMinChars = override.LossReasonJustificationMinChars
	}
	base.RequireQueueJumpReasonField = override.RequireQueueJumpReasonField
	base.RequireLossReasonField = override.RequireLossReasonField
	base.RequireCancelReasonField = override.RequireCancelReasonField
	base.RequireStopReasonField = override.RequireStopReasonField
	return base
}

func defaultCustomerSourceOptions() []OptionItem {
	return []OptionItem{
		{ID: "instagram", Label: "Instagram"},
		{ID: "trafego-pago", Label: "Trafego pago"},
		{ID: "google", Label: "Google"},
		{ID: "whatsapp", Label: "WhatsApp"},
		{ID: "site", Label: "Site"},
		{ID: "indicacao", Label: "Indicacao de amigo"},
		{ID: "cliente-recorrente", Label: "Cliente recorrente"},
		{ID: "vitrine", Label: "Vitrine ou passagem na frente"},
		{ID: "evento-parceria", Label: "Evento ou parceria"},
		{ID: "outro", Label: "Outro"},
	}
}

func defaultQueueJumpReasonOptions() []OptionItem {
	return []OptionItem{
		{ID: "cliente-fixo", Label: "Cliente fixo"},
		{ID: "troca", Label: "Troca"},
		{ID: "retirada", Label: "Retirada"},
		{ID: "cliente-chamado-consultor", Label: "Cliente chamado pelo consultor"},
		{ID: "atendimento-agendado", Label: "Atendimento agendado"},
	}
}

func defaultPauseReasonOptions() []OptionItem {
	return []OptionItem{
		{ID: "almoco", Label: "Almoco"},
		{ID: "atendimento-externo", Label: "Atendimento externo"},
		{ID: "suporte-interno", Label: "Suporte interno"},
		{ID: "treinamento", Label: "Treinamento"},
		{ID: "reuniao", Label: "Reuniao"},
	}
}

func defaultCancelReasonOptions() []OptionItem {
	return []OptionItem{
		{ID: "clique-incorreto", Label: "Clique incorreto"},
		{ID: "cliente-desistiu-imediato", Label: "Cliente desistiu imediatamente"},
		{ID: "troca-consultor", Label: "Troca de consultor"},
		{ID: "ajuste-fila", Label: "Ajuste de fila"},
	}
}

func defaultStopReasonOptions() []OptionItem {
	return []OptionItem{
		{ID: "cliente-saiu", Label: "Cliente saiu"},
		{ID: "pausa-consultor", Label: "Pausa do consultor"},
		{ID: "aguardando-retorno", Label: "Aguardando retorno"},
		{ID: "encaminhado", Label: "Encaminhado"},
	}
}

func defaultLossReasonOptions() []OptionItem {
	return []OptionItem{
		{ID: "preco", Label: "Preco"},
		{ID: "vai-pensar", Label: "Vai pensar"},
		{ID: "nao-encontrou-o-que-queria", Label: "Nao encontrou o que queria"},
		{ID: "tamanho-indisponivel", Label: "Tamanho indisponivel"},
		{ID: "comparando-precos", Label: "Comparando precos"},
		{ID: "volta-depois", Label: "Volta depois"},
		{ID: "so-pesquisando", Label: "So pesquisando"},
	}
}

func defaultProfessionOptions() []OptionItem {
	return []OptionItem{
		{ID: "profissao-advogada", Label: "Advogada"},
		{ID: "profissao-arquiteta", Label: "Arquiteta"},
		{ID: "profissao-dentista", Label: "Dentista"},
		{ID: "profissao-empresaria", Label: "Empresaria"},
		{ID: "profissao-engenheira", Label: "Engenheira"},
		{ID: "profissao-medica", Label: "Medica"},
	}
}

func defaultProductCatalog() []ProductItem {
	return []ProductItem{
		{ID: "produto-1", Name: "Anel Solitario Ouro 18k", Code: "ANE-OURO-001", Category: "Aneis", BasePrice: 3900},
		{ID: "produto-2", Name: "Alianca Slim Diamantada", Code: "ALI-OURO-002", Category: "Aliancas", BasePrice: 2200},
		{ID: "produto-3", Name: "Brinco Gota Safira", Code: "BRI-PEDRA-003", Category: "Brincos", BasePrice: 1750},
		{ID: "produto-4", Name: "Colar Riviera Prata", Code: "COL-PRATA-004", Category: "Colares", BasePrice: 1480},
		{ID: "produto-5", Name: "Pulseira Cartier Ouro", Code: "PUL-OURO-005", Category: "Pulseiras", BasePrice: 2850},
		{ID: "produto-6", Name: "Relogio Classico Feminino", Code: "REL-CLASS-006", Category: "Relogios", BasePrice: 4200},
		{ID: "produto-7", Name: "Anel Formatura Esmeralda", Code: "ANE-FORM-007", Category: "Aneis", BasePrice: 2600},
		{ID: "produto-8", Name: "Escapulario Ouro Branco", Code: "COL-OURO-008", Category: "Colares", BasePrice: 1950},
		{ID: "produto-9", Name: "Brinco Argola Premium", Code: "BRI-PRATA-009", Category: "Brincos", BasePrice: 1320},
		{ID: "produto-10", Name: "Pulseira Tennis Zirconia", Code: "PUL-PRATA-010", Category: "Pulseiras", BasePrice: 1680},
	}
}

func cloneOptions(options []OptionItem) []OptionItem {
	cloned := make([]OptionItem, 0, len(options))
	for _, option := range options {
		cloned = append(cloned, OptionItem{
			ID:    option.ID,
			Label: option.Label,
		})
	}

	return cloned
}

func cloneProducts(products []ProductItem) []ProductItem {
	cloned := make([]ProductItem, 0, len(products))
	for _, product := range products {
		cloned = append(cloned, ProductItem{
			ID:        product.ID,
			Name:      product.Name,
			Code:      product.Code,
			Category:  product.Category,
			BasePrice: product.BasePrice,
		})
	}

	return cloned
}
