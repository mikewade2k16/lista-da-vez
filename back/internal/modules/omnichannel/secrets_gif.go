package omnichannel

import (
	"context"
	"errors"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// GifService orquestra a busca de GIF (Tenor) e a chave GLOBAL do provedor (F12 C2/C5). A
// chave e da PLATAFORMA (um app Tenor para o produto), gravada CIFRADA em core.platform_settings
// via secretbox (prefixo v1:), NUNCA em env e NUNCA crua de volta ao front (so {set,last4}).
// Modelo de saida: calendar/secrets.go (mask); cifragem: platform/secretbox (F3), corrigindo o
// gap do calendario que grava a chave crua.

// gifDefaultProvider e o unico provedor suportado hoje. provider != tenor => search soft-error.
const gifDefaultProvider = "tenor"

// gifReplyPermission gateia search/media: compor/anexar GIF e parte de responder conversa.
// Enforcement no service (hasEffectivePermission), como React/Forward (service_actions_messages).
const gifReplyPermission = "omnichannel.conversations.reply"

// Mensagens ACIONAVEIS do soft-error (C2/C5): apontam o painel, NUNCA citam env var.
const (
	gifErrNoKey    = "Busca de GIF indisponivel: configure a chave do Tenor no painel de administracao (Administracao > Omnichannel > GIF)."
	gifErrProvider = "Provedor de GIF nao suportado. Selecione o Tenor no painel de administracao."
	gifErrUpstream = "Nao foi possivel consultar o provedor de GIF agora. Tente novamente em instantes."
)

// ErrGifSecretBox: cifragem indisponivel (secretBox nil). Em producao o boot faz fail-fast do
// OMNI_SECRETS_KEY antes de injetar, entao isto so acontece em configuracao degradada.
var ErrGifSecretBox = errors.New("omnichannel: cifragem de segredo indisponivel")

// GifSettingsStatus e a saida MASCARADA das rotas de settings (C5): nunca carrega a chave crua.
type GifSettingsStatus struct {
	Set      bool   `json:"set"`
	Last4    string `json:"last4"`
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
}

// GifSearchResponse e o envelope SOFT-ERROR do /gif/search (C2). Error e OMITIDO quando ok.
type GifSearchResponse struct {
	Provider string    `json:"provider"`
	Query    string    `json:"query"`
	Items    []GifItem `json:"items"`
	Error    string    `json:"error,omitempty"`
}

// gifSecretStore e a fatia de persistencia que o GifService consome. hasEffectivePermission ja
// existe no *Store (store_postgres_routing.go) e e reusado para o gate de reply.
type gifSecretStore interface {
	GetGifSecret(ctx context.Context) (gifSecretConfig, error)
	PutGifSecret(ctx context.Context, cfg gifSecretConfig, updatedBy string) error
	hasEffectivePermission(ctx context.Context, accountID, userID, key string) (bool, error)
}

// GifService reune persistencia da chave (store), cifragem (box) e o client Tenor.
type GifService struct {
	store gifSecretStore
	box   *secretbox.Box
	tenor *tenorClient
}

// NewGifService constroi o service. box pode ser nil (cifragem indisponivel): gravar chave
// falha com ErrGifSecretBox e o search cai no soft-error (sem chave legivel).
func NewGifService(store gifSecretStore, box *secretbox.Box) *GifService {
	return &GifService{store: store, box: box, tenor: newTenorClient()}
}

// Status devolve o status MASCARADO da chave global (C5): {set,last4,provider,baseUrl}. Decifra
// server-side so para computar o last4; a chave crua nunca sai.
func (s *GifService) Status(ctx context.Context) (GifSettingsStatus, error) {
	cfg, err := s.store.GetGifSecret(ctx)
	if err != nil {
		return GifSettingsStatus{}, err
	}
	st := secretbox.Mask(s.decrypt(cfg.APIKey))
	return GifSettingsStatus{
		Set:      st.Set,
		Last4:    st.Last4,
		Provider: gifProviderOrDefault(cfg.Provider),
		BaseURL:  strings.TrimSpace(cfg.BaseURL),
	}, nil
}

// Put grava (ou limpa) a chave global (C5). apiKey vazio => LIMPA (apiKey=""), preservando
// provider/baseUrl. provider/baseUrl nil => mantem o atual; presentes => sobrescrevem. A chave
// e cifrada (secretbox v1:) antes de persistir. So platform_admin (gate no handler).
func (s *GifService) Put(ctx context.Context, apiKey string, provider, baseURL *string, updatedBy string) error {
	cfg, err := s.store.GetGifSecret(ctx)
	if err != nil {
		return err
	}
	if provider != nil {
		cfg.Provider = gifProviderOrDefault(*provider)
	}
	if cfg.Provider == "" {
		cfg.Provider = gifDefaultProvider
	}
	if baseURL != nil {
		cfg.BaseURL = strings.TrimSpace(*baseURL)
	}
	key := strings.TrimSpace(apiKey)
	if key == "" {
		cfg.APIKey = ""
		return s.store.PutGifSecret(ctx, cfg, updatedBy)
	}
	enc, err := s.encrypt(key)
	if err != nil {
		return err
	}
	cfg.APIKey = enc
	return s.store.PutGifSecret(ctx, cfg, updatedBy)
}

// Search resolve a chave global e consulta o Tenor. SOFT-ERROR (C2): sem chave / provider !=
// tenor / upstream falhou => items:[] + Error acionavel, SEM erro Go (o handler responde 200).
// q vazio => items:[] sem chamar o Tenor. NUNCA loga a URL (tem key=) nem o payload.
func (s *GifService) Search(ctx context.Context, query string, limit int) GifSearchResponse {
	query = strings.TrimSpace(query)
	cfg, cfgErr := s.store.GetGifSecret(ctx)
	provider := gifDefaultProvider
	if cfgErr == nil {
		provider = gifProviderOrDefault(cfg.Provider)
	}
	resp := GifSearchResponse{Provider: provider, Query: query, Items: []GifItem{}}
	if query == "" {
		return resp
	}
	switch {
	case cfgErr != nil:
		// Falha ao ler a config: nao vaza detalhe, pede retry (soft-error, nunca 5xx).
		resp.Error = gifErrUpstream
	case provider != gifDefaultProvider:
		resp.Error = gifErrProvider
	default:
		apiKey := s.decrypt(cfg.APIKey)
		if apiKey == "" {
			resp.Error = gifErrNoKey
			break
		}
		items, err := s.tenor.search(ctx, cfg.BaseURL, apiKey, query, limit)
		if err != nil {
			resp.Error = gifErrUpstream
			break
		}
		resp.Items = items
	}
	return resp
}

// ensureReply exige omnichannel.conversations.reply na conta (403 se faltar). platform_admin
// passa; cai tambem no Principal.Permissions global quando resolvido. Espelha requireAgentPerm.
func (s *GifService) ensureReply(ctx context.Context, accountID string, p auth.Principal) error {
	if strings.TrimSpace(accountID) == "" {
		return ErrForbidden
	}
	if p.Role == auth.RolePlatformAdmin {
		return nil
	}
	ok, err := s.store.hasEffectivePermission(ctx, accountID, p.UserID, gifReplyPermission)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	if p.PermissionsResolved && containsPermission(p.Permissions, gifReplyPermission) {
		return nil
	}
	return ErrForbidden
}

// encrypt cifra a chave crua (secretbox v1:). box nil => ErrGifSecretBox (nao grava chave crua).
func (s *GifService) encrypt(plain string) (string, error) {
	if s.box == nil {
		return "", ErrGifSecretBox
	}
	return s.box.Encrypt(plain)
}

// decrypt reverte o ciphertext para uso server-side (last4/dispatch). Vazio, box nil ou falha
// de decifragem => "" (tratado como "sem chave"). NUNCA logar o retorno.
func (s *GifService) decrypt(cipher string) string {
	cipher = strings.TrimSpace(cipher)
	if cipher == "" || s.box == nil {
		return ""
	}
	plain, err := s.box.Decrypt(cipher)
	if err != nil {
		return ""
	}
	return plain
}

// gifProviderOrDefault normaliza o provider (minusculas); vazio => tenor.
func gifProviderOrDefault(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return gifDefaultProvider
	}
	return p
}
