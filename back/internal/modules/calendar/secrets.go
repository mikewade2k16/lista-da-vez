package calendar

import (
	"context"
	"strings"
)

// Secrets de IA do calendario (Wave 3, contrato SEC). A API key CRUA so existe
// server-side (resolver/dispatch). O front recebe SO o status mascarado {set,last4}
// via KeyStatus — NUNCA a key crua. Por conta: calendar.ai_secrets. Global (da
// plataforma, so platform_admin): core.platform_settings key 'calendar_ai_secrets'.

// secretProviders sao os provedores que tem slot de key nos secrets (SEC).
var secretProviders = []string{"gemini", "glm", "openai"}

// secretProviderSet e o mesmo conjunto para lookup O(1) na sanitizacao.
var secretProviderSet = map[string]bool{"gemini": true, "glm": true, "openai": true}

// KeyStatus e o status MASCARADO de uma key (write-only). Set = ha key gravada;
// Last4 = os ultimos 4 caracteres (para o usuario reconhecer). A key crua nunca sai.
type KeyStatus struct {
	Set   bool   `json:"set"`
	Last4 string `json:"last4"`
}

// KeyStatusView e a resposta dos GET de status: a FONTE ATIVA (global|account) e o
// status mascarado por provider. Nunca carrega key crua.
type KeyStatusView struct {
	Scope string               `json:"scope"` // global | account
	Keys  map[string]KeyStatus `json:"keys"`
}

// GlobalSecrets e o conjunto de keys GLOBAIS da plataforma (raw, server-side apenas).
// Persistido em core.platform_settings key 'calendar_ai_secrets' (mesmo padrao do
// media_limits). NUNCA serializado para o front (o front recebe KeyStatusView).
type GlobalSecrets struct {
	Gemini string `json:"gemini"`
	GLM    string `json:"glm"`
	OpenAI string `json:"openai"`
}

// mask converte a key crua em status mascarado: set=key!="" e last4 = ultimos 4
// caracteres. NUNCA devolve a key crua. Usada em TODAS as respostas de status.
func mask(key string) KeyStatus {
	key = strings.TrimSpace(key)
	if key == "" {
		return KeyStatus{Set: false, Last4: ""}
	}
	last4 := key
	if len(key) > 4 {
		last4 = key[len(key)-4:]
	}
	return KeyStatus{Set: true, Last4: last4}
}

// normalizeSecretProvider devolve o provider em minusculas se for um slot de secret
// valido (gemini|glm|openai); senao "".
func normalizeSecretProvider(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if secretProviderSet[p] {
		return p
	}
	return ""
}

// globalKeyFor extrai a key crua do provider do conjunto global (server-side apenas).
func globalKeyFor(s GlobalSecrets, provider string) string {
	switch provider {
	case "gemini":
		return s.Gemini
	case "glm":
		return s.GLM
	case "openai":
		return s.OpenAI
	}
	return ""
}

// GetAccountKeyStatus devolve o status MASCARADO da FONTE ATIVA da conta. Le a config:
// se ai.useGlobalKeys => status das chaves globais (scope "global"); senao => status
// das chaves DESTA conta (scope "account"). Nunca devolve key crua.
func (s *Service) GetAccountKeyStatus(ctx context.Context, accountID string) (KeyStatusView, error) {
	account := strings.TrimSpace(accountID)
	cfg, err := s.store.GetConfig(ctx, account)
	if err != nil {
		return KeyStatusView{}, err
	}
	if cfg.AI.UseGlobalKeys {
		view, err := s.GetGlobalKeyStatus(ctx)
		if err != nil {
			return KeyStatusView{}, err
		}
		// O endpoint de conta NAO expoe o last4 da chave GLOBAL compartilhada (so o
		// platform_admin ve o last4 em /ai-keys/global); aqui mostra apenas se esta setada.
		for k, st := range view.Keys {
			view.Keys[k] = KeyStatus{Set: st.Set}
		}
		return view, nil
	}
	keys := make(map[string]KeyStatus, len(secretProviders))
	for _, p := range secretProviders {
		raw, err := s.store.GetAccountSecret(ctx, account, p)
		if err != nil {
			return KeyStatusView{}, err
		}
		keys[p] = mask(raw)
	}
	return KeyStatusView{Scope: "account", Keys: keys}, nil
}

// GetGlobalKeyStatus devolve o status MASCARADO das chaves globais da plataforma.
func (s *Service) GetGlobalKeyStatus(ctx context.Context) (KeyStatusView, error) {
	secrets, err := s.store.GetGlobalSecrets(ctx)
	if err != nil {
		return KeyStatusView{}, err
	}
	keys := map[string]KeyStatus{
		"gemini": mask(secrets.Gemini),
		"glm":    mask(secrets.GLM),
		"openai": mask(secrets.OpenAI),
	}
	return KeyStatusView{Scope: "global", Keys: keys}, nil
}

// PutAccountKey grava (ou limpa) a key da conta para o provider. apiKey vazio =
// limpar (remove a linha). provider fora do enum => ErrInvalidProvider. A key crua
// NAO e logada nem devolvida (o handler responde so com o status mascarado).
func (s *Service) PutAccountKey(ctx context.Context, accountID, provider, apiKey, updatedBy string) error {
	prov := normalizeSecretProvider(provider)
	if prov == "" {
		return ErrInvalidProvider
	}
	return s.store.PutAccountSecret(ctx, strings.TrimSpace(accountID), prov,
		strings.TrimSpace(apiKey), strings.TrimSpace(updatedBy))
}

// PutGlobalKey grava (ou limpa) a key GLOBAL da plataforma para o provider (so
// platform_admin, gate no HTTP). apiKey vazio = limpar. provider fora do enum =>
// ErrInvalidProvider. A key crua NAO e logada nem devolvida.
func (s *Service) PutGlobalKey(ctx context.Context, provider, apiKey, updatedBy string) error {
	prov := normalizeSecretProvider(provider)
	if prov == "" {
		return ErrInvalidProvider
	}
	return s.store.PutGlobalSecret(ctx, prov, strings.TrimSpace(apiKey), strings.TrimSpace(updatedBy))
}

// resolveAIKey devolve a KEY CRUA do provider para a conta (server-side apenas, para
// o dispatch da IA). Le a config: ai.useGlobalKeys => chave global; senao => chave da
// conta. Provider sem slot de secret ou sem key gravada => "" (o caller decide o erro
// acionavel ai_key_missing). NUNCA logar o retorno.
func (s *Service) resolveAIKey(ctx context.Context, accountID, provider string) (string, error) {
	account := strings.TrimSpace(accountID)
	prov := normalizeSecretProvider(provider)
	if prov == "" {
		return "", nil
	}
	cfg, err := s.store.GetConfig(ctx, account)
	if err != nil {
		return "", err
	}
	if cfg.AI.UseGlobalKeys {
		secrets, err := s.store.GetGlobalSecrets(ctx)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(globalKeyFor(secrets, prov)), nil
	}
	raw, err := s.store.GetAccountSecret(ctx, account, prov)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(raw), nil
}
