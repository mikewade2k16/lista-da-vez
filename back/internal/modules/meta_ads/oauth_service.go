package metaads

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	oauthStateBytes       = 32
	oauthStateTTL         = 10 * time.Minute
	metaOAuthCallbackPath = "/v1/public/meta-ads/oauth/callback"
	defaultOAuthDialogURL = "https://www.facebook.com/v24.0/dialog/oauth"
	maxOAuthCodeLength    = 4096
)

var (
	ErrOAuthNotConfigured = errors.New("meta_ads: oauth nao configurado")
	ErrOAuthInvalidConfig = errors.New("meta_ads: configuracao oauth invalida")
	ErrOAuthInvalidState  = errors.New("meta_ads: state oauth invalido, expirado ou consumido")
	ErrOAuthDenied        = errors.New("meta_ads: autorizacao oauth negada")
	ErrOAuthPermissions   = errors.New("meta_ads: permissoes oauth obrigatorias nao concedidas")
)

var defaultOAuthScopes = []string{
	"ads_management",
	"ads_read",
	"business_management",
	"pages_show_list",
	"pages_read_engagement",
	"instagram_basic",
}

// OAuthPermissionsError diferencia scopes ausentes daqueles explicitamente
// nao concedidos sem incluir esses detalhes na mensagem potencialmente
// logavel/exibivel. Os campos servem para diagnostico tipado e testes.
type OAuthPermissionsError struct {
	Missing    []string
	NotGranted []string
}

func (e *OAuthPermissionsError) Error() string {
	return ErrOAuthPermissions.Error()
}

func (e *OAuthPermissionsError) Unwrap() error {
	return ErrOAuthPermissions
}

type oauthStateRepository interface {
	CreateOAuthState(
		ctx context.Context,
		accountID string,
		createdByUserID string,
		stateHash []byte,
		redirectURI string,
		expiresAt time.Time,
	) error
	ConsumeOAuthState(ctx context.Context, stateHash []byte) (OAuthPendingState, error)
}

type oauthConnectionSaver interface {
	SaveOAuthConnection(
		ctx context.Context,
		accountID, token string,
		tokenExpiresAt *time.Time,
	) (ConnectionView, error)
}

// OAuthService liga o state persistido a troca server-side e ao caminho
// canonico de SaveConnection (validacao Graph + cifra + cache de ad accounts).
type OAuthService struct {
	states      oauthStateRepository
	connections oauthConnectionSaver
	exchanger   oauthTokenExchanger
	config      OAuthConfig
	now         func() time.Time
	random      io.Reader
}

func NewOAuthService(
	states oauthStateRepository,
	connections oauthConnectionSaver,
	exchanger oauthTokenExchanger,
	config OAuthConfig,
) *OAuthService {
	return &OAuthService{
		states:      states,
		connections: connections,
		exchanger:   exchanger,
		config: OAuthConfig{
			AppID:       strings.TrimSpace(config.AppID),
			AppSecret:   strings.TrimSpace(config.AppSecret),
			RedirectURI: strings.TrimSpace(config.RedirectURI),
		},
		now:    time.Now,
		random: rand.Reader,
	}
}

func (s *OAuthService) Start(ctx context.Context, accountID, userID string) (OAuthStartResult, error) {
	redirectURI, err := s.validatedRedirectURI()
	if err != nil {
		return OAuthStartResult{}, err
	}
	accountID = strings.TrimSpace(accountID)
	userID = strings.TrimSpace(userID)
	if !metaAdsUUIDRe.MatchString(accountID) || !metaAdsUUIDRe.MatchString(userID) {
		return OAuthStartResult{}, ErrOAuthInvalidState
	}

	rawState := make([]byte, oauthStateBytes)
	if _, err := io.ReadFull(s.random, rawState); err != nil {
		return OAuthStartResult{}, fmt.Errorf("gerar state oauth: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(rawState)
	stateHash := sha256.Sum256([]byte(state))
	expiresAt := s.now().UTC().Add(oauthStateTTL)
	if err := s.states.CreateOAuthState(
		ctx,
		accountID,
		userID,
		stateHash[:],
		redirectURI,
		expiresAt,
	); err != nil {
		return OAuthStartResult{}, err
	}

	dialog, err := url.Parse(defaultOAuthDialogURL)
	if err != nil {
		return OAuthStartResult{}, ErrOAuthInvalidConfig
	}
	query := dialog.Query()
	query.Set("client_id", s.config.AppID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(defaultOAuthScopes, ","))
	query.Set("state", state)
	dialog.RawQuery = query.Encode()

	return OAuthStartResult{
		AuthorizationURL: dialog.String(),
		ExpiresAt:        expiresAt.Format(time.RFC3339),
	}, nil
}

// Complete consome o state antes de interpretar code/erro do provider. Assim
// ate um consentimento negado e single-use e nunca pode ser reaproveitado.
func (s *OAuthService) Complete(ctx context.Context, rawState, code string, providerDenied bool) error {
	if _, err := s.validatedRedirectURI(); err != nil {
		return err
	}
	stateHash, err := oauthStateHash(rawState)
	if err != nil {
		return err
	}
	pending, err := s.states.ConsumeOAuthState(ctx, stateHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOAuthInvalidState
		}
		return err
	}
	if providerDenied {
		return ErrOAuthDenied
	}
	code = strings.TrimSpace(code)
	if code == "" || len(code) > maxOAuthCodeLength {
		return ErrOAuthDenied
	}
	if _, err := validateOAuthRedirectURI(pending.RedirectURI); err != nil {
		return ErrOAuthInvalidConfig
	}

	token, err := s.exchanger.ExchangeCode(
		ctx,
		s.config.AppID,
		s.config.AppSecret,
		pending.RedirectURI,
		code,
	)
	if err != nil {
		return err
	}
	permissions, err := s.exchanger.ListPermissions(ctx, token.Value)
	if err != nil {
		return err
	}
	if err := validateOAuthPermissions(permissions); err != nil {
		return err
	}
	var expiresAt *time.Time
	if !token.ExpiresAt.IsZero() {
		value := token.ExpiresAt.UTC()
		expiresAt = &value
	}
	_, err = s.connections.SaveOAuthConnection(ctx, pending.AccountID, token.Value, expiresAt)
	return err
}

func validateOAuthPermissions(permissions []OAuthPermission) error {
	seen := make(map[string]bool, len(permissions))
	granted := make(map[string]bool, len(permissions))
	for _, permission := range permissions {
		name := strings.TrimSpace(permission.Name)
		if name == "" {
			continue
		}
		seen[name] = true
		if strings.EqualFold(strings.TrimSpace(permission.Status), "granted") {
			granted[name] = true
		}
	}

	result := &OAuthPermissionsError{}
	for _, required := range defaultOAuthScopes {
		switch {
		case !seen[required]:
			result.Missing = append(result.Missing, required)
		case !granted[required]:
			result.NotGranted = append(result.NotGranted, required)
		}
	}
	if len(result.Missing) == 0 && len(result.NotGranted) == 0 {
		return nil
	}
	return result
}

func (s *OAuthService) validatedRedirectURI() (string, error) {
	if s == nil || strings.TrimSpace(s.config.AppID) == "" ||
		strings.TrimSpace(s.config.AppSecret) == "" || strings.TrimSpace(s.config.RedirectURI) == "" {
		return "", ErrOAuthNotConfigured
	}
	return validateOAuthRedirectURI(s.config.RedirectURI)
}

func validateOAuthRedirectURI(raw string) (string, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != metaOAuthCallbackPath {
		return "", ErrOAuthInvalidConfig
	}
	if parsed.Scheme == "https" {
		return parsed.String(), nil
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" && (host == "localhost" || net.ParseIP(host).IsLoopback()) {
		return parsed.String(), nil
	}
	return "", ErrOAuthInvalidConfig
}

func oauthStateHash(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != oauthStateBytes {
		return nil, ErrOAuthInvalidState
	}
	hash := sha256.Sum256([]byte(raw))
	return hash[:], nil
}
