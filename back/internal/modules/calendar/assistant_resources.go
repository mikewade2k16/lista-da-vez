package calendar

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const (
	assistantResourceInstagramPost = "instagram_post"
	assistantResourceMetaCampaign  = "meta_campaign"
	assistantResourceMetaAdAccount = "meta_ad_account"

	maxAssistantResources       = 20
	maxAssistantResourceIDRunes = 192
	maxAssistantResourceTitle   = 180
	maxAssistantResourceText    = 240
	maxAssistantResourceURL     = 2048
	maxAssistantResourceMeta    = 8
)

var assistantResourceSuffixRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// AssistantResource e um snapshot read-only seguro para cards do chat. O LLM
// escolhe somente o ID; todos os outros campos vem deste registry autoritativo.
type AssistantResource struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Title     string            `json:"title"`
	Subtitle  string            `json:"subtitle,omitempty"`
	Status    string            `json:"status,omitempty"`
	ImageURL  string            `json:"imageUrl,omitempty"`
	Permalink string            `json:"permalink,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// MetaAssistantContextResult separa o contexto que o modelo pode consultar do
// registry de cards que o backend pode persistir. O provider nunca concede
// autorizacao: o motor ja resolveu a capability e repete a intersecao por ID.
type MetaAssistantContextResult struct {
	Context              any
	Resources            []AssistantResource
	ActionAdAccounts     []MetaAssistantActionAdAccount
	ActionCampaigns      []MetaAssistantActionCampaign
	ActionInstagramPosts []MetaAssistantActionInstagramPost
}

func sanitizeAssistantResources(resources []AssistantResource) []AssistantResource {
	out := make([]AssistantResource, 0, min(len(resources), maxAssistantResources))
	seen := make(map[string]struct{}, min(len(resources), maxAssistantResources))
	for _, resource := range resources {
		if len(out) >= maxAssistantResources {
			break
		}
		clean, ok := sanitizeAssistantResource(resource)
		if !ok {
			continue
		}
		if _, duplicate := seen[clean.ID]; duplicate {
			continue
		}
		seen[clean.ID] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func sanitizeAssistantResource(resource AssistantResource) (AssistantResource, bool) {
	kind := strings.ToLower(strings.TrimSpace(resource.Kind))
	if !validAssistantResourceKind(kind) {
		return AssistantResource{}, false
	}
	id := strings.TrimSpace(resource.ID)
	prefix := kind + ":"
	suffix := strings.TrimPrefix(id, prefix)
	if suffix == id || suffix == "" || len([]rune(id)) > maxAssistantResourceIDRunes ||
		!assistantResourceSuffixRe.MatchString(suffix) {
		return AssistantResource{}, false
	}
	title := assistantResourceText(resource.Title, maxAssistantResourceTitle)
	if title == "" {
		title = assistantResourceKindLabel(kind)
	}
	return AssistantResource{
		ID:        id,
		Kind:      kind,
		Title:     title,
		Subtitle:  assistantResourceText(resource.Subtitle, maxAssistantResourceText),
		Status:    assistantResourceText(resource.Status, maxAssistantResourceText),
		ImageURL:  sanitizeAssistantHTTPSURL(resource.ImageURL),
		Permalink: sanitizeAssistantHTTPSURL(resource.Permalink),
		Metadata:  sanitizeAssistantResourceMetadata(resource.Metadata),
	}, true
}

// selectAuthorizedAssistantResources cruza a lista nao confiavel do modelo com
// o registry sanitizado. Preserva a ordem pedida, deduplica e limita os cards.
func selectAuthorizedAssistantResources(registry []AssistantResource, requestedIDs []string) []AssistantResource {
	cleanRegistry := sanitizeAssistantResources(registry)
	byID := make(map[string]AssistantResource, len(cleanRegistry))
	for _, resource := range cleanRegistry {
		byID[resource.ID] = resource
	}
	out := make([]AssistantResource, 0, min(len(requestedIDs), maxAssistantResources))
	seen := make(map[string]struct{}, min(len(requestedIDs), maxAssistantResources))
	for _, rawID := range requestedIDs {
		if len(out) >= maxAssistantResources {
			break
		}
		id := strings.TrimSpace(rawID)
		resource, exists := byID[id]
		if !exists {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, resource)
	}
	return out
}

func validAssistantResourceKind(kind string) bool {
	switch kind {
	case assistantResourceInstagramPost, assistantResourceMetaCampaign, assistantResourceMetaAdAccount:
		return true
	default:
		return false
	}
}

func assistantResourceKindLabel(kind string) string {
	switch kind {
	case assistantResourceInstagramPost:
		return "Post do Instagram"
	case assistantResourceMetaCampaign:
		return "Campanha Meta"
	case assistantResourceMetaAdAccount:
		return "Conta de anuncios Meta"
	default:
		return "Recurso"
	}
}

func assistantResourceText(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" || limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func sanitizeAssistantHTTPSURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxAssistantResourceURL {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	return parsed.String()
}

func sanitizeAssistantResourceMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]string, min(len(keys), maxAssistantResourceMeta))
	for _, key := range keys {
		if len(out) >= maxAssistantResourceMeta {
			break
		}
		cleanKey := assistantResourceText(key, 40)
		cleanValue := assistantResourceText(metadata[key], maxAssistantResourceText)
		if cleanKey == "" || cleanValue == "" || !assistantResourceSuffixRe.MatchString(cleanKey) {
			continue
		}
		out[cleanKey] = cleanValue
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// filterChatMessagesForCapabilities impede que uma revogacao continue expondo
// ou realimentando respostas geradas com qualquer contexto hoje indisponivel.
// ContextModules cobre respostas novas mesmo sem cards; para legado, requisitos
// sao derivados apenas de artefatos estruturados (nunca do texto). A mensagem do
// assistente e removida inteira, pois o texto pode repetir os mesmos dados.
func filterChatMessagesForCapabilities(messages []ChatMessage, capabilities []AssistantCapability) []ChatMessage {
	out := make([]ChatMessage, 0, len(messages))
	for _, message := range messages {
		required := assistantMessageRequiredModules(message)
		if message.Role == chatRoleAssistant && !assistantModulesAllowed(required, capabilities) {
			continue
		}
		if assistantCapabilityMode(capabilities, "meta_ads") == assistantModeOff {
			message.Resources = nil
		}
		if assistantCapabilityMode(capabilities, "calendar") == assistantModeOff {
			message.CalendarItems = nil
		}
		message.Proposals = filterStoredProposalsForCapabilities(message.Proposals, capabilities)
		message.ContextModules = sanitizeAssistantContextModules(message.ContextModules)
		out = append(out, message)
	}
	return out
}

func filterStoredProposalsForCapabilities(proposals []StoredProposal, capabilities []AssistantCapability) []StoredProposal {
	out := make([]StoredProposal, 0, len(proposals))
	for _, proposal := range proposals {
		if assistantModulesAllowed(assistantProposalContextModules(proposal.Kind, proposal.Fields), capabilities) {
			out = append(out, proposal)
		}
	}
	return out
}
