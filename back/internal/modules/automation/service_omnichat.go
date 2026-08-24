package automation

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	ErrOmniChatDisabled              = errors.New("automation: omni chat desativado")
	ErrOmniChatCredentialUnavailable = errors.New("automation: credencial do omni chat indisponivel")
	ErrOmniChatInvalidConfig         = errors.New("automation: configuracao do omni chat invalida")
)

// OmniChatCredentialResolver e a fronteira com o cofre global de IA. Automation
// conhece somente a interface; a composicao injeta a fachada do Omnichannel.
type OmniChatCredentialResolver interface {
	ResolveCredential(ctx context.Context, accountID, credentialID string) (OmniChatRuntimeCredential, error)
}

type OmniChatRuntimeCredential struct {
	ID       string
	Provider string
	APIKey   string
}

type OmniChatConfigView struct {
	Enabled        bool                         `json:"enabled"`
	SystemPrompt   string                       `json:"systemPrompt"`
	IsDefault      bool                         `json:"isDefault"`
	Inherited      bool                         `json:"inherited"`
	InheritedFrom  string                       `json:"inheritedFrom,omitempty"`
	CredentialID   string                       `json:"credentialId"`
	Provider       string                       `json:"provider"`
	Model          string                       `json:"model"`
	Temperature    float64                      `json:"temperature"`
	HistoryWindow  int                          `json:"historyWindow"`
	SurfaceModules map[string]map[string]string `json:"surfaceModules"`
}

type OmniChatConfigInput struct {
	Enabled        bool
	SystemPrompt   string
	CredentialID   string
	Model          string
	Temperature    float64
	HistoryWindow  int
	SurfaceModules map[string]map[string]string
	UpdatedBy      string
}

// OmniChatRuntimeConfig e a leitura interna usada pelo motor compartilhado do
// assistente. A API key nunca e serializada para HTTP; ela cruza apenas a fronteira
// Go->n8n no momento da execucao.
type OmniChatRuntimeConfig struct {
	Enabled        bool
	SystemPrompt   string
	Provider       string
	Model          string
	APIKey         string
	Temperature    float64
	HistoryWindow  int
	SurfaceModules map[string]map[string]string
}

// OmniChatResultView e a resposta do Omni Chat para o painel de Operacao.
type OmniChatResultView struct {
	Answer   string       `json:"answer"`
	Topic    string       `json:"topic,omitempty"`
	Products []ProductHit `json:"products,omitempty"`
}

const (
	defaultHistoryWindow = 5
	minHistoryWindow     = 1
	maxHistoryWindow     = 20
)

var assistantSurfaceDefaults = map[string]map[string]string{
	"calendar": {"calendar": "write", "tasks": "write", "meta_ads": "off", "users": "read"},
	"meta_ads": {"calendar": "off", "tasks": "off", "meta_ads": "write", "users": "off"},
	"global":   {"calendar": "read", "tasks": "read", "meta_ads": "read", "users": "read"},
}

func defaultAssistantSurfaceModules() map[string]map[string]string {
	return normalizeAssistantSurfaceModules(assistantSurfaceDefaults)
}

func normalizeAssistantSurfaceModules(input map[string]map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string, len(assistantSurfaceDefaults))
	for surface, defaults := range assistantSurfaceDefaults {
		configured := input[surface]
		modes := make(map[string]string, len(defaults))
		for module, fallback := range defaults {
			mode := strings.ToLower(strings.TrimSpace(configured[module]))
			if mode != "off" && mode != "read" && mode != "write" {
				mode = fallback
			}
			modes[module] = mode
		}
		out[surface] = modes
	}
	return out
}

func (s *Service) OmniChatAsk(
	ctx context.Context,
	scope ContextScope,
	question string,
	topic string,
	conversationID string,
	history []OmniChatHistoryMessage,
) (OmniChatResultView, error) {
	config, err := s.OmniChatConfig(ctx, scope.AccountID)
	if err != nil {
		return OmniChatResultView{}, err
	}
	if !config.Enabled {
		return OmniChatResultView{}, ErrOmniChatDisabled
	}
	if strings.TrimSpace(config.CredentialID) == "" || s.omniChatCredentials == nil {
		return OmniChatResultView{}, ErrOmniChatCredentialUnavailable
	}
	credential, err := s.omniChatCredentials.ResolveCredential(ctx, scope.AccountID, config.CredentialID)
	if err != nil || strings.TrimSpace(credential.APIKey) == "" {
		return OmniChatResultView{}, ErrOmniChatCredentialUnavailable
	}
	if normalizeOmniChatProvider(credential.Provider) == "" {
		return OmniChatResultView{}, ErrOmniChatInvalidConfig
	}

	contextToken, issueErr := s.ctxMgr.Issue(scope)
	if issueErr != nil {
		contextToken = ""
	}

	history = normalizeOmniChatHistory(history, config.HistoryWindow)
	topic = strings.TrimSpace(topic)
	result, err := s.n8n.Ask(ctx, OmniChatRunRequest{
		Question:      question,
		Topic:         topic,
		SystemMessage: config.SystemPrompt,
		SessionRef:    "omni-chat-" + scope.AccountID,
		ContextToken:  contextToken,
		SessionKey:    omniChatSessionKey(scope, conversationID),
		HistoryWindow: config.HistoryWindow,
		History:       history,
		AI: OmniChatAIExecution{
			Provider:    normalizeOmniChatProvider(credential.Provider),
			Model:       config.Model,
			APIKey:      credential.APIKey,
			Temperature: config.Temperature,
		},
	})
	if err != nil {
		return OmniChatResultView{}, err
	}

	return OmniChatResultView{Answer: result.Answer, Topic: topic, Products: result.Products}, nil
}

func omniChatSessionKey(scope ContextScope, conversationID string) string {
	base := scope.AccountID + "|" + scope.UserID
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return base
	}
	return base + "|" + conversationID
}

func clampHistoryWindow(n int) int {
	switch {
	case n <= 0:
		return defaultHistoryWindow
	case n < minHistoryWindow:
		return minHistoryWindow
	case n > maxHistoryWindow:
		return maxHistoryWindow
	default:
		return n
	}
}

func clampOmniChatTemperature(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func normalizeOmniChatProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "openai":
		return "openai"
	case "gemini":
		return "gemini"
	case "glm":
		return "glm"
	case "anthropic":
		return "anthropic"
	default:
		return ""
	}
}

func normalizeOmniChatHistory(history []OmniChatHistoryMessage, window int) []OmniChatHistoryMessage {
	limit := clampHistoryWindow(window) * 2
	if len(history) > limit {
		history = history[len(history)-limit:]
	}
	out := make([]OmniChatHistoryMessage, 0, len(history))
	for _, message := range history {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		content := strings.TrimSpace(message.Content)
		if (role != "user" && role != "assistant") || content == "" {
			continue
		}
		if len(content) > 6000 {
			content = content[:6000]
		}
		out = append(out, OmniChatHistoryMessage{Role: role, Content: content})
	}
	return out
}

func (s *Service) OmniChatConfig(ctx context.Context, accountID string) (OmniChatConfigView, error) {
	config, err := s.store.GetOmniChatConfig(ctx, accountID)
	if err != nil {
		return OmniChatConfigView{}, err
	}
	return omniChatConfigView(config), nil
}

// OmniChatSurfaceModuleModeTx devolve o modo efetivo da matriz canonica no
// snapshot da transacao chamadora. Nao resolve segredo/credencial: confirmacoes
// precisam revalidar autorizacao, sem depender da disponibilidade do provedor IA.
func (s *Service) OmniChatSurfaceModuleModeTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID, surface, module string,
) (string, error) {
	if s == nil || s.store == nil || tx == nil {
		return "off", ErrOmniChatInvalidConfig
	}
	config, err := s.store.GetOmniChatConfigTx(ctx, tx, strings.TrimSpace(accountID))
	if err != nil {
		return "off", err
	}
	if !config.Enabled {
		return "off", nil
	}
	surface = strings.ToLower(strings.TrimSpace(surface))
	module = strings.ToLower(strings.TrimSpace(module))
	mode := normalizeAssistantSurfaceModules(config.SurfaceModules)[surface][module]
	if mode != "read" && mode != "write" {
		return "off", nil
	}
	return mode, nil
}

func omniChatConfigView(config OmniChatConfig) OmniChatConfigView {
	isDefault := strings.TrimSpace(config.SystemPrompt) == ""
	prompt := config.SystemPrompt
	if isDefault {
		prompt = omniChatPersona
	}
	return OmniChatConfigView{
		Enabled:        config.Enabled,
		SystemPrompt:   prompt,
		IsDefault:      isDefault,
		Inherited:      config.Inherited,
		InheritedFrom:  config.SourceAccountName,
		CredentialID:   config.CredentialID,
		Provider:       config.Provider,
		Model:          config.Model,
		Temperature:    config.Temperature,
		HistoryWindow:  clampHistoryWindow(config.HistoryWindow),
		SurfaceModules: normalizeAssistantSurfaceModules(config.SurfaceModules),
	}
}

func (s *Service) SetOmniChatConfig(ctx context.Context, accountID string, input OmniChatConfigInput) (OmniChatConfigView, error) {
	prompt := strings.TrimSpace(input.SystemPrompt)
	model := strings.TrimSpace(input.Model)
	credentialID := strings.TrimSpace(input.CredentialID)
	provider := ""

	if input.Enabled && (credentialID == "" || model == "") {
		return OmniChatConfigView{}, ErrOmniChatInvalidConfig
	}
	if credentialID != "" {
		if s.omniChatCredentials == nil {
			return OmniChatConfigView{}, ErrOmniChatCredentialUnavailable
		}
		credential, err := s.omniChatCredentials.ResolveCredential(ctx, accountID, credentialID)
		if err != nil {
			return OmniChatConfigView{}, ErrOmniChatCredentialUnavailable
		}
		provider = normalizeOmniChatProvider(credential.Provider)
		if provider == "" {
			return OmniChatConfigView{}, ErrOmniChatInvalidConfig
		}
	}
	if provider == "" {
		provider = "openai"
	}
	if model == "" {
		model = "gpt-4.1-mini"
	}

	saved, err := s.store.SaveOmniChatConfig(ctx, OmniChatConfig{
		AccountID: accountID, Enabled: input.Enabled, SystemPrompt: prompt,
		CredentialID: credentialID, Provider: provider, Model: model,
		Temperature:    clampOmniChatTemperature(input.Temperature),
		HistoryWindow:  clampHistoryWindow(input.HistoryWindow),
		SurfaceModules: normalizeAssistantSurfaceModules(input.SurfaceModules),
		UpdatedBy:      input.UpdatedBy,
	})
	if err != nil {
		return OmniChatConfigView{}, err
	}
	return omniChatConfigView(saved), nil
}

// OmniChatRuntime resolve configuracao + segredo no cofre para consumidores
// internos. Mantem uma unica fonte de provider/modelo/prompt para o chat 360.
func (s *Service) OmniChatRuntime(ctx context.Context, accountID string) (OmniChatRuntimeConfig, error) {
	config, err := s.OmniChatConfig(ctx, accountID)
	if err != nil {
		return OmniChatRuntimeConfig{}, err
	}
	if !config.Enabled {
		return OmniChatRuntimeConfig{}, ErrOmniChatDisabled
	}
	if strings.TrimSpace(config.CredentialID) == "" || s.omniChatCredentials == nil {
		return OmniChatRuntimeConfig{}, ErrOmniChatCredentialUnavailable
	}
	credential, err := s.omniChatCredentials.ResolveCredential(ctx, accountID, config.CredentialID)
	if err != nil || strings.TrimSpace(credential.APIKey) == "" {
		return OmniChatRuntimeConfig{}, ErrOmniChatCredentialUnavailable
	}
	provider := normalizeOmniChatProvider(credential.Provider)
	if provider == "" {
		return OmniChatRuntimeConfig{}, ErrOmniChatInvalidConfig
	}
	return OmniChatRuntimeConfig{
		Enabled: true, SystemPrompt: config.SystemPrompt, Provider: provider,
		Model: config.Model, APIKey: credential.APIKey, Temperature: config.Temperature,
		HistoryWindow:  config.HistoryWindow,
		SurfaceModules: normalizeAssistantSurfaceModules(config.SurfaceModules),
	}, nil
}
