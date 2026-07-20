package omnichannel

import (
	"context"
	"os"
	"strings"
	"time"
)

// Operacoes de instancia que nao sao CRUD: validar a config de endpoints e limpar o
// historico de conversas. Metodos no SessionService (mesmo escopo/admin da gestao).

// ============================================================================
// validate-endpoints
// ============================================================================

// EndpointValidationInput e o body de POST /tenant/whatsapp/validate-endpoints. O front manda
// instanceId e/ou instanceName; ambos vazios => a default/1a ativa da conta.
type EndpointValidationInput struct {
	InstanceID   string `json:"instanceId"`
	InstanceName string `json:"instanceName"`
}

// EndpointValidationView espelha WhatsAppEndpointValidationResponse (types/index.ts:256).
type EndpointValidationView struct {
	InstanceName string                    `json:"instanceName"`
	GeneratedAt  string                    `json:"generatedAt"`
	BaseURL      string                    `json:"baseUrl"`
	TimeoutMs    int                       `json:"timeoutMs"`
	Endpoints    []EndpointValidationEntry `json:"endpoints"`
	Summary      EndpointValidationSummary `json:"summary"`
}

// EndpointValidationEntry espelha WhatsAppEndpointValidationEntry (types/index.ts:236).
type EndpointValidationEntry struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	PathTemplate string  `json:"pathTemplate"`
	ResolvedPath *string `json:"resolvedPath"`
	Status       string  `json:"status"`
	Available    bool    `json:"available"`
	HTTPStatus   *int    `json:"httpStatus"`
	Message      string  `json:"message"`
}

// EndpointValidationSummary espelha WhatsAppEndpointValidationSummary (types/index.ts:247).
type EndpointValidationSummary struct {
	Total         int `json:"total"`
	Available     int `json:"available"`
	MissingRoute  int `json:"missingRoute"`
	AuthError     int `json:"authError"`
	ProviderError int `json:"providerError"`
	NetworkError  int `json:"networkError"`
}

// endpointDef e um dos 6 endpoints do envio Evolution (chave/label/rota) que o front tabula.
type endpointDef struct {
	key          string
	label        string
	pathTemplate string
}

// endpointCatalog sao os 6 endpoints de envio da Evolution (os mesmos que o front tipa em
// WhatsAppEndpointValidationEntry.key). {instance} resolve para o instance_name.
var endpointCatalog = []endpointDef{
	{"text", "Texto", "/message/sendText/{instance}"},
	{"media", "Midia", "/message/sendMedia/{instance}"},
	{"audio", "Audio", "/message/sendWhatsAppAudio/{instance}"},
	{"contact", "Contato", "/message/sendContact/{instance}"},
	{"sticker", "Sticker", "/message/sendSticker/{instance}"},
	{"reaction", "Reacao", "/message/sendReaction/{instance}"},
}

// endpointValidationTimeoutMs e o timeout informado ao painel (o front usa 120s na chamada).
const endpointValidationTimeoutMs = 120_000

// ValidateEndpoints valida a CONFIGURACAO de envio da instancia (LEGADO/parcial: nao ha probe
// de rede vivo aqui — isso e F6/envio). Deriva status HONESTO da config real: provider,
// baseURL (provider_config.baseURL -> EVOLUTION_BASE_URL) e presenca de credencial
// (credentials_ciphertext -> EVOLUTION_API_KEY). Ver docs/LEGADO.md e o AGENT.md do modulo.
func (s *SessionService) ValidateEndpoints(ctx context.Context, accountID string, caller Caller, in EndpointValidationInput) (EndpointValidationView, error) {
	if !caller.IsAdmin {
		return EndpointValidationView{}, ErrForbidden
	}
	row, err := s.store.ResolveInstanceForOps(ctx, accountID, in.InstanceID, in.InstanceName)
	if err != nil {
		if noRows(err) {
			return EndpointValidationView{}, ErrSessionUnavailable
		}
		return EndpointValidationView{}, err
	}

	baseURL := strings.TrimSpace(row.Config["baseURL"])
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("EVOLUTION_BASE_URL"))
	}
	hasCredential := row.HasCredentials || strings.TrimSpace(os.Getenv("EVOLUTION_API_KEY")) != ""

	// status uniforme por endpoint (a config e por instancia, nao por rota). Mensagem honesta
	// em cada caso — nunca "ok" mentindo que houve probe.
	status, available, message := classifyEndpointConfig(row.Provider, baseURL, hasCredential)

	entries := make([]EndpointValidationEntry, 0, len(endpointCatalog))
	summary := EndpointValidationSummary{Total: len(endpointCatalog)}
	for _, def := range endpointCatalog {
		var resolved *string
		if baseURL != "" {
			p := strings.TrimRight(baseURL, "/") +
				strings.ReplaceAll(def.pathTemplate, "{instance}", row.InstanceName)
			resolved = &p
		}
		entries = append(entries, EndpointValidationEntry{
			Key:          def.key,
			Label:        def.label,
			PathTemplate: def.pathTemplate,
			ResolvedPath: resolved,
			Status:       status,
			Available:    available,
			HTTPStatus:   nil,
			Message:      message,
		})
		switch status {
		case "ok":
			summary.Available++
		case "missing_route":
			summary.MissingRoute++
		case "auth_error":
			summary.AuthError++
		case "provider_error":
			summary.ProviderError++
		case "network_error":
			summary.NetworkError++
		}
	}

	return EndpointValidationView{
		InstanceName: row.InstanceName,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		BaseURL:      baseURL,
		TimeoutMs:    endpointValidationTimeoutMs,
		Endpoints:    entries,
		Summary:      summary,
	}, nil
}

// classifyEndpointConfig deriva (status, available, message) da config. Ordem: provider ->
// baseURL -> credencial -> ok. "ok" NAO afirma conectividade viva (F6 valida no envio).
func classifyEndpointConfig(provider, baseURL string, hasCredential bool) (status string, available bool, message string) {
	switch {
	case provider != "evolution":
		return "provider_error", false,
			"Validacao de endpoints disponivel apenas para o provedor Evolution."
	case baseURL == "":
		return "missing_route", false,
			"Base URL da Evolution nao configurada nesta instancia."
	case !hasCredential:
		return "auth_error", false,
			"Credencial da Evolution ausente. Configure a API key da instancia."
	default:
		return "ok", true,
			"Configuracao presente. A conectividade do envio e validada no momento do envio."
	}
}

// ============================================================================
// conversations/clear
// ============================================================================

// ConversationClearInput e o body de POST /tenant/whatsapp/conversations/clear. instanceId
// ausente/vazio => escopo tenant (toda a conta); presente => so aquela instancia.
type ConversationClearInput struct {
	InstanceID *string `json:"instanceId"`
}

// ConversationClearView espelha WhatsAppConversationHistoryClearResponse (types/index.ts:216).
type ConversationClearView struct {
	TenantID             string  `json:"tenantId"`
	Scope                string  `json:"scope"`
	InstanceID           *string `json:"instanceId"`
	InstanceName         *string `json:"instanceName"`
	DeletedAuditEvents   int64   `json:"deletedAuditEvents"`
	DeletedMessages      int64   `json:"deletedMessages"`
	DeletedConversations int64   `json:"deletedConversations"`
	Message              string  `json:"message"`
}

// ClearConversations apaga o historico de conversas da conta (escopo tenant) ou de UMA
// instancia. tenantId volta como o account_id (o Omni mapeia tenantId -> account_id).
func (s *SessionService) ClearConversations(ctx context.Context, accountID string, caller Caller, in ConversationClearInput) (ConversationClearView, error) {
	if !caller.IsAdmin {
		return ConversationClearView{}, ErrForbidden
	}

	instanceID := optTrim(in.InstanceID)
	scope := "tenant"
	var instanceName *string
	if instanceID != nil {
		row, err := s.store.ResolveInstanceForOps(ctx, accountID, *instanceID, "")
		if err != nil {
			if noRows(err) {
				return ConversationClearView{}, ErrSessionUnavailable
			}
			return ConversationClearView{}, err
		}
		scope = "instance"
		name := row.InstanceName
		instanceName = &name
	}

	audit, msgs, convs, err := s.store.ClearConversations(ctx, accountID, instanceID)
	if err != nil {
		return ConversationClearView{}, err
	}

	message := "Historico de conversas removido"
	if scope == "instance" {
		message = "Historico da instancia removido"
	}
	return ConversationClearView{
		TenantID:             accountID,
		Scope:                scope,
		InstanceID:           instanceID,
		InstanceName:         instanceName,
		DeletedAuditEvents:   audit,
		DeletedMessages:      msgs,
		DeletedConversations: convs,
		Message:              message,
	}, nil
}
