package settings

const defaultTemplateID = "joalheria-padrao"

var defaultOperationTemplates = []OperationTemplate{
	{
		ID:          "joalheria-padrao",
		Label:       "Joalheria padrao",
		Description: "Equilibrio entre qualidade de atendimento, captura de lead e disciplina de fila.",
		Settings: AppSettings{
			MaxConcurrentServices:    10,
			TimingFastCloseMinutes:   5,
			TimingLongServiceMinutes: 25,
			TimingLowSaleAmount:      1200,
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
			AllowProductSeenNone:            true,
			VisitReasonSelectionMode:        "multiple",
			VisitReasonDetailMode:           "shared",
			CustomerSourceSelectionMode:     "single",
			CustomerSourceDetailMode:        "shared",
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
			MaxConcurrentServices:    8,
			TimingFastCloseMinutes:   7,
			TimingLongServiceMinutes: 35,
			TimingLowSaleAmount:      1500,
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
			AllowProductSeenNone:            true,
			VisitReasonSelectionMode:        "multiple",
			VisitReasonDetailMode:           "shared",
			CustomerSourceSelectionMode:     "single",
			CustomerSourceDetailMode:        "shared",
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
			MaxConcurrentServices:    12,
			TimingFastCloseMinutes:   3,
			TimingLongServiceMinutes: 18,
			TimingLowSaleAmount:      900,
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
			AllowProductSeenNone:            true,
			VisitReasonSelectionMode:        "multiple",
			VisitReasonDetailMode:           "off",
			CustomerSourceSelectionMode:     "single",
			CustomerSourceDetailMode:        "off",
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
			MaxConcurrentServices:    template.Settings.MaxConcurrentServices,
			TimingFastCloseMinutes:   template.Settings.TimingFastCloseMinutes,
			TimingLongServiceMinutes: template.Settings.TimingLongServiceMinutes,
			TimingLowSaleAmount:      template.Settings.TimingLowSaleAmount,
			TestModeEnabled:          false,
			AutoFillFinishModal:      false,
			AlertMinConversionRate:   0,
			AlertMaxQueueJumpRate:    0,
			AlertMinPaScore:          0,
			AlertMinTicketAverage:    0,
		},
		ModalConfig:            mergeModalConfig(defaultBaseModalConfig(), template.ModalConfig),
		VisitReasonOptions:     cloneOptions(template.VisitReasonOptions),
		CustomerSourceOptions:  cloneOptions(template.CustomerSourceOptions),
		PauseReasonOptions:     defaultPauseReasonOptions(),
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
			ModalConfig:           template.ModalConfig,
			VisitReasonOptions:    cloneOptions(template.VisitReasonOptions),
			CustomerSourceOptions: cloneOptions(template.CustomerSourceOptions),
		})
	}

	return templates
}

func resolveTemplate(templateID string) OperationTemplate {
	for _, template := range defaultOperationTemplates {
		if template.ID == templateID {
			return template
		}
	}

	return defaultOperationTemplates[0]
}

func defaultBaseModalConfig() ModalConfig {
	return ModalConfig{
		Title:                           "Fechar atendimento",
		ProductSeenLabel:                "Interesses do cliente",
		ProductSeenPlaceholder:          "Busque e selecione interesses",
		ProductClosedLabel:              "",
		ProductClosedPlaceholder:        "Busque e selecione o produto fechado",
		NotesLabel:                      "Observações",
		NotesPlaceholder:                "Detalhes adicionais do atendimento",
		QueueJumpReasonLabel:            "Motivo do atendimento fora da vez",
		QueueJumpReasonPlaceholder:      "Busque e selecione o motivo fora da vez",
		LossReasonLabel:                 "Motivo da perda",
		LossReasonPlaceholder:           "Busque e selecione o motivo da perda",
		CustomerSectionLabel:            "Dados do cliente",
		CustomerNameLabel:               "Nome do cliente",
		CustomerPhoneLabel:              "Telefone",
		CustomerEmailLabel:              "E-mail",
		CustomerProfessionLabel:         "Profissão",
		ExistingCustomerLabel:           "Já era cliente",
		ProductSeenNotesLabel:           "Observação dos interesses",
		ProductSeenNotesPlaceholder:     "Descreva referência, pedido específico, contexto do cliente ou justificativa quando não houver interesse identificado.",
		VisitReasonLabel:                "Motivo da visita",
		CustomerSourceLabel:             "Origem do cliente",
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
		AllowProductSeenNone:            true,
		VisitReasonSelectionMode:        "multiple",
		VisitReasonDetailMode:           "shared",
		LossReasonSelectionMode:         "single",
		LossReasonDetailMode:            "off",
		CustomerSourceSelectionMode:     "single",
		CustomerSourceDetailMode:        "shared",
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
	base.ShowVisitReasonField = override.ShowVisitReasonField
	base.ShowCustomerSourceField = override.ShowCustomerSourceField
	base.ShowExistingCustomerField = override.ShowExistingCustomerField
	base.ShowQueueJumpReasonField = override.ShowQueueJumpReasonField
	base.ShowLossReasonField = override.ShowLossReasonField
	base.AllowProductSeenNone = override.AllowProductSeenNone
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
	base.RequireVisitReason = override.RequireVisitReason
	base.RequireCustomerSource = override.RequireCustomerSource
	base.RequireCustomerNamePhone = override.RequireCustomerNamePhone
	base.RequireProductSeenNotesWhenNone = override.RequireProductSeenNotesWhenNone
	if override.ProductSeenNotesMinChars > 0 {
		base.ProductSeenNotesMinChars = override.ProductSeenNotesMinChars
	}
	base.RequireQueueJumpReasonField = override.RequireQueueJumpReasonField
	base.RequireLossReasonField = override.RequireLossReasonField
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
