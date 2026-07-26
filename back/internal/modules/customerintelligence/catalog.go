package customerintelligence

import (
	"regexp"
	"sort"
	"strings"
)

const (
	ModuleID = "customer_intelligence"

	CapabilityProfile   = "customer_intelligence.profile"
	CapabilityRuntime   = "customer_intelligence.runtime"
	CapabilityPortfolio = "customer_intelligence.portfolio"

	// As operacoes abaixo compartilham o gate canonico de perfil. Fonte,
	// finalidade, writer e processo ainda precisam passar seus gates proprios.
	CapabilityContext        = CapabilityProfile
	CapabilitySourceSync     = CapabilityProfile
	CapabilityMemoryWrite    = CapabilityProfile
	CapabilityRecommendation = CapabilityProfile

	OutcomeNoReply = "no_reply"
	OutcomeReply   = "reply"
	OutcomeHandoff = "handoff"

	// Aliases de código preservam legibilidade interna; o valor público do Runtime
	// permanece estritamente reply|handoff|no_reply.
	OutcomeReplyDraft   = OutcomeReply
	OutcomeHumanHandoff = OutcomeHandoff
)

const (
	PermissionProfileView       = ModuleID + ".profile.view"
	PermissionProfileManage     = ModuleID + ".profile.manage"
	PermissionSourcesView       = ModuleID + ".sources.view"
	PermissionSourcesManage     = ModuleID + ".sources.manage"
	PermissionAgentsView        = ModuleID + ".agents.view"
	PermissionAgentsManage      = ModuleID + ".agents.manage"
	PermissionPromptsView       = ModuleID + ".prompts.view"
	PermissionPromptsManage     = ModuleID + ".prompts.manage"
	PermissionPromptsPublish    = ModuleID + ".prompts.publish"
	PermissionPromptsPlatform   = ModuleID + ".prompts.platform_manage"
	PermissionRunsView          = ModuleID + ".runs.view"
	PermissionAuditView         = ModuleID + ".audit.view"
	PermissionPortfolioView     = ModuleID + ".portfolio.view"
	PermissionPortfolioManage   = ModuleID + ".portfolio.manage"
	PermissionPortfolioPlatform = ModuleID + ".portfolio.platform_manage"
)

type SourceDescriptor struct {
	Key               string                        `json:"key"`
	Label             string                        `json:"label"`
	OwnerPackage      string                        `json:"ownerPackage"`
	Capabilities      []string                      `json:"capabilities"`
	Modes             []string                      `json:"modes"`
	RequiresModule    string                        `json:"requiresModule,omitempty"`
	AllowedConfigKeys []string                      `json:"allowedConfigKeys"`
	AllowedFields     []string                      `json:"allowedFields"`
	PurposeKeys       []string                      `json:"purposeKeys"`
	ConfigSchema      []SourceConfigFieldDescriptor `json:"configSchema"`
}

type SourceConfigFieldDescriptor struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required,omitempty"`
	Min         int      `json:"min,omitempty"`
	Max         int      `json:"max,omitempty"`
	Options     []string `json:"options,omitempty"`
	ElementKeys []string `json:"elementKeys,omitempty"`
}

var sourceCatalog = map[string]SourceDescriptor{
	"omnichannel": {
		Key: "omnichannel", Label: "Conversas online", Modes: []string{"event", "scheduled", "on_demand"},
		OwnerPackage: "omnichannel", Capabilities: []string{"subject_evidence"},
		RequiresModule: "omnichannel", AllowedConfigKeys: []string{"lookbackDays", "includeMediaMetadata"},
		PurposeKeys: []string{"customer_service", "customer_profile"},
		ConfigSchema: []SourceConfigFieldDescriptor{
			{Key: "lookbackDays", Label: "Janela em dias", Type: "integer", Min: 1, Max: 365},
			{Key: "includeMediaMetadata", Label: "Incluir metadados de midia", Type: "boolean"},
		},
		AllowedFields: []string{
			"message_id", "conversation_id", "contact_source_id", "channel", "direction",
			"message_type", "content", "status", "occurred_at", "media_mime_type",
			"media_file_name", "media_caption", "media_duration_seconds",
		},
	},
	"manual.offline": {
		Key: "manual.offline", Label: "Registros offline", Modes: []string{"manual"},
		OwnerPackage: "customerdata", Capabilities: []string{"subject_evidence"},
		AllowedConfigKeys: []string{"defaultSensitivity"},
		PurposeKeys:       []string{"customer_profile"},
		ConfigSchema: []SourceConfigFieldDescriptor{{
			Key: "defaultSensitivity", Label: "Sensibilidade padrao", Type: "select",
			Options: []string{"internal", "personal", "sensitive", "restricted"},
		}},
		AllowedFields: []string{
			"value", "note", "interaction_type", "occurred_at", "timezone", "title",
			"status", "duration_seconds", "content", "source_external_ref",
		},
	},
	"erp": {
		Key: "erp", Label: "ERP configurado", Modes: []string{"scheduled", "on_demand"},
		OwnerPackage: "crm/erp", Capabilities: []string{"subject_evidence"},
		AllowedConfigKeys: []string{"connectionId", "entityTypes", "lookbackDays"},
		PurposeKeys:       []string{"customer_profile", "customer_service", "marketing"},
		ConfigSchema: []SourceConfigFieldDescriptor{
			{Key: "connectionId", Label: "Conexao ERP registrada", Type: "safe_key", Required: true},
			{
				Key: "entityTypes", Label: "Entidades", Type: "string_list",
				ElementKeys: []string{"customer", "order", "order_canceled"},
			},
			{Key: "lookbackDays", Label: "Janela em dias", Type: "integer", Min: 1, Max: 3650},
		},
		AllowedFields: []string{
			"preferred_name", "registered_at", "birthday", "gender", "city", "state",
			"country", "tags", "source_batch_date", "order_date", "total_amount_cents",
			"product_return_cents", "skus", "quantity", "payment_type", "store_code",
			"cancelled",
		},
	},
	"calendar.client_profile": {
		Key: "calendar.client_profile", Label: "Perfil estrategico do cliente", Modes: []string{"on_demand"},
		OwnerPackage: "calendar", Capabilities: []string{"business_context"},
		RequiresModule: "calendar", AllowedConfigKeys: []string{"sections", "maxBytes"},
		PurposeKeys: []string{"customer_service", "customer_profile"},
		ConfigSchema: []SourceConfigFieldDescriptor{
			{Key: "sections", Label: "Secoes", Type: "string_list", ElementKeys: []string{"strategy", "presence", "voice", "brief"}},
			{Key: "maxBytes", Label: "Limite de bytes", Type: "integer", Min: 512, Max: 65536},
		},
		AllowedFields: []string{"strategy", "presence", "voice", "brief"},
	},
	"site": {
		Key: "site", Label: "Site e catalogo", Modes: []string{"scheduled", "on_demand"},
		OwnerPackage: "site", Capabilities: []string{"subject_evidence"},
		RequiresModule: "site", AllowedConfigKeys: []string{"siteId", "entityTypes"},
		PurposeKeys: []string{"customer_profile", "customer_service", "marketing"},
		ConfigSchema: []SourceConfigFieldDescriptor{
			{Key: "siteId", Label: "Site registrado", Type: "uuid", Required: true},
			{
				Key: "entityTypes", Label: "Entidades", Type: "string_list",
				ElementKeys: []string{"lead"},
			},
		},
		AllowedFields: []string{
			"source_label", "page", "coupon", "consent", "consent_label", "status",
			"created_at",
		},
	},
	"bi.perola": {
		Key: "bi.perola", Label: "BI Perola", Modes: []string{"on_demand"},
		OwnerPackage: "bi", Capabilities: []string{"business_context", "portfolio_aggregate"},
		AllowedConfigKeys: []string{"datasetId", "limit"},
		PurposeKeys:       []string{"customer_service", "customer_profile", "portfolio_analysis"},
		ConfigSchema: []SourceConfigFieldDescriptor{
			{Key: "datasetId", Label: "Dataset registrado", Type: "safe_key", Required: true},
			{Key: "limit", Label: "Limite agregado", Type: "integer", Min: 1, Max: 1000},
		},
	},
}

var processCatalog = map[string]string{
	"conversation.triage":            "Triagem de conversa",
	"conversation.reply":             "Resposta de conversa",
	"conversation.handoff_summary":   "Resumo de handoff",
	"memory.extract":                 "Extracao de memoria",
	"profile.summary":                "Resumo de perfil",
	"recommendation.follow_up":       "Recomendacao de follow-up",
	"recommendation.offer":           "Recomendacao de oferta",
	"recommendation.important_dates": "Datas importantes",
	"source.suggest":                 "Sugestao de fonte",
	"portfolio.opportunity":          "Oportunidade de portfolio",
	"media.image_analysis":           "Analise de imagem",
	"media.document_analysis":        "Analise de documento",
	"quality.review":                 "Revisao de qualidade",
}

var capabilityCatalog = map[string][]string{
	CapabilityProfile:   {"off", "shadow", "on"},
	CapabilityRuntime:   {"off", "shadow", "canary", "on"},
	CapabilityPortfolio: {"off", "shadow", "on"},
}

var (
	uuidPattern       = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	safeKeyPattern    = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
	requestKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	templateVarRE     = regexp.MustCompile(`\{\{\s*([a-zA-Z][a-zA-Z0-9_.]*)\s*\}\}`)
)

func SourceCatalog() []SourceDescriptor {
	keys := make([]string, 0, len(sourceCatalog))
	for key := range sourceCatalog {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]SourceDescriptor, 0, len(keys))
	for _, key := range keys {
		item := sourceCatalog[key]
		item.Modes = append([]string(nil), item.Modes...)
		item.Capabilities = append([]string(nil), item.Capabilities...)
		item.AllowedConfigKeys = append([]string(nil), item.AllowedConfigKeys...)
		item.AllowedFields = append([]string(nil), item.AllowedFields...)
		item.PurposeKeys = append([]string(nil), item.PurposeKeys...)
		item.ConfigSchema = append([]SourceConfigFieldDescriptor(nil), item.ConfigSchema...)
		for index := range item.ConfigSchema {
			item.ConfigSchema[index].Options = append(
				[]string(nil), item.ConfigSchema[index].Options...,
			)
			item.ConfigSchema[index].ElementKeys = append(
				[]string(nil), item.ConfigSchema[index].ElementKeys...,
			)
		}
		out = append(out, item)
	}
	return out
}

func ProcessKeys() []string {
	keys := make([]string, 0, len(processCatalog))
	for key := range processCatalog {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func validUUID(value string) bool {
	return uuidPattern.MatchString(strings.TrimSpace(value))
}

func validProcessKey(value string) bool {
	_, ok := processCatalog[strings.TrimSpace(value)]
	return ok
}

func validSourceKey(value string) bool {
	_, ok := sourceCatalog[strings.TrimSpace(value)]
	return ok
}

func validCapability(key, mode string) bool {
	modes, ok := capabilityCatalog[strings.TrimSpace(key)]
	if !ok {
		return false
	}
	return validMode(mode, modes...)
}

func validCapabilityScope(key, scopeKey string) bool {
	scopeKey = strings.TrimSpace(scopeKey)
	if key != CapabilityRuntime {
		return scopeKey == ""
	}
	return scopeKey == "" ||
		(len(scopeKey) <= 80 && safeKeyPattern.MatchString(scopeKey))
}

func validMode(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
