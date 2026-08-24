package metaads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type oauthRepositoryFake struct {
	createdAccountID string
	createdUserID    string
	createdHash      []byte
	createdRedirect  string
	createdExpiry    time.Time
	pending          OAuthPendingState
	consumeErr       error
	consumed         bool
}

func (f *oauthRepositoryFake) CreateOAuthState(
	_ context.Context,
	accountID string,
	createdByUserID string,
	stateHash []byte,
	redirectURI string,
	expiresAt time.Time,
) error {
	f.createdAccountID = accountID
	f.createdUserID = createdByUserID
	f.createdHash = append([]byte(nil), stateHash...)
	f.createdRedirect = redirectURI
	f.createdExpiry = expiresAt
	return nil
}

func (f *oauthRepositoryFake) ConsumeOAuthState(_ context.Context, _ []byte) (OAuthPendingState, error) {
	if f.consumeErr != nil {
		return OAuthPendingState{}, f.consumeErr
	}
	if f.consumed {
		return OAuthPendingState{}, pgx.ErrNoRows
	}
	f.consumed = true
	return f.pending, nil
}

type oauthConnectionSaverFake struct {
	accountID string
	token     string
	expiresAt *time.Time
	err       error
	calls     int
}

func (f *oauthConnectionSaverFake) SaveOAuthConnection(
	_ context.Context,
	accountID string,
	token string,
	expiresAt *time.Time,
) (ConnectionView, error) {
	f.calls++
	f.accountID = accountID
	f.token = token
	f.expiresAt = expiresAt
	return ConnectionView{Connected: f.err == nil}, f.err
}

type oauthExchangerFake struct {
	appID           string
	appSecret       string
	redirectURI     string
	code            string
	token           OAuthAccessToken
	err             error
	calls           int
	permissions     []OAuthPermission
	permissionsErr  error
	permissionCalls int
	permissionToken string
}

func (f *oauthExchangerFake) ExchangeCode(
	_ context.Context,
	appID string,
	appSecret string,
	redirectURI string,
	code string,
) (OAuthAccessToken, error) {
	f.calls++
	f.appID = appID
	f.appSecret = appSecret
	f.redirectURI = redirectURI
	f.code = code
	return f.token, f.err
}

func (f *oauthExchangerFake) ListPermissions(
	_ context.Context,
	token string,
) ([]OAuthPermission, error) {
	f.permissionCalls++
	f.permissionToken = token
	if f.permissionsErr != nil {
		return nil, f.permissionsErr
	}
	if f.permissions != nil {
		return append([]OAuthPermission(nil), f.permissions...), nil
	}
	permissions := make([]OAuthPermission, 0, len(defaultOAuthScopes))
	for _, scope := range defaultOAuthScopes {
		permissions = append(permissions, OAuthPermission{Name: scope, Status: "granted"})
	}
	return permissions, nil
}

func newOAuthServiceTest(
	repository *oauthRepositoryFake,
	saver *oauthConnectionSaverFake,
	exchanger *oauthExchangerFake,
) *OAuthService {
	return NewOAuthService(repository, saver, exchanger, OAuthConfig{
		AppID:       "123456",
		AppSecret:   "app-secret",
		RedirectURI: "https://api.example.com/v1/public/meta-ads/oauth/callback",
	})
}

func TestDefaultOAuthScopesCoverInstagramMediaThroughFacebookPages(t *testing.T) {
	required := map[string]bool{
		"pages_show_list":       false,
		"pages_read_engagement": false,
		"instagram_basic":       false,
	}
	for _, scope := range defaultOAuthScopes {
		if _, tracked := required[scope]; tracked {
			required[scope] = true
		}
	}
	for scope, present := range required {
		if !present {
			t.Fatalf("defaultOAuthScopes nao inclui %s", scope)
		}
	}
}

func TestOAuthStartPersistsOnlyHashAndBuildsAuthorizationURL(t *testing.T) {
	repository := &oauthRepositoryFake{}
	service := newOAuthServiceTest(repository, &oauthConnectionSaverFake{}, &oauthExchangerFake{})
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	service.random = bytes.NewReader(bytes.Repeat([]byte{0x7a}, oauthStateBytes))

	result, err := service.Start(
		context.Background(),
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
	)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	parsed, err := url.Parse(result.AuthorizationURL)
	if err != nil {
		t.Fatalf("authorization URL: %v", err)
	}
	rawState := parsed.Query().Get("state")
	if rawState == "" {
		t.Fatal("authorization URL sem state")
	}
	expectedHash := sha256.Sum256([]byte(rawState))
	if !bytes.Equal(repository.createdHash, expectedHash[:]) {
		t.Fatal("repository nao recebeu SHA-256 do state")
	}
	if bytes.Equal(repository.createdHash, []byte(rawState)) {
		t.Fatal("state bruto foi persistido")
	}
	if parsed.Query().Get("client_secret") != "" || strings.Contains(result.AuthorizationURL, "app-secret") {
		t.Fatal("app secret apareceu na authorization URL")
	}
	if repository.createdExpiry != now.Add(oauthStateTTL) {
		t.Fatalf("expiry = %s", repository.createdExpiry)
	}
	if parsed.Query().Get("scope") != strings.Join(defaultOAuthScopes, ",") {
		t.Fatalf("scopes = %q", parsed.Query().Get("scope"))
	}
}

func TestOAuthCompleteUsesStateAccountAndIsSingleUse(t *testing.T) {
	raw := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x3c}, oauthStateBytes))
	expiresAt := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	repository := &oauthRepositoryFake{pending: OAuthPendingState{
		AccountID:   "11111111-1111-4111-8111-111111111111",
		RedirectURI: "https://api.example.com/v1/public/meta-ads/oauth/callback",
	}}
	saver := &oauthConnectionSaverFake{}
	exchanger := &oauthExchangerFake{token: OAuthAccessToken{Value: "long-token", ExpiresAt: expiresAt}}
	service := newOAuthServiceTest(repository, saver, exchanger)

	if err := service.Complete(context.Background(), raw, "provider-code", false); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if saver.accountID != repository.pending.AccountID || saver.token != "long-token" {
		t.Fatalf("conexao salva fora do state: account=%q token=%q", saver.accountID, saver.token)
	}
	if exchanger.permissionCalls != 1 || exchanger.permissionToken != "long-token" {
		t.Fatalf("permission check = calls %d token %q", exchanger.permissionCalls, exchanger.permissionToken)
	}
	if saver.expiresAt == nil || !saver.expiresAt.Equal(expiresAt) {
		t.Fatal("expiracao nao foi persistida na conexao correta")
	}
	if err := service.Complete(context.Background(), raw, "provider-code", false); !errors.Is(err, ErrOAuthInvalidState) {
		t.Fatalf("reuso do state = %v", err)
	}
	if exchanger.calls != 1 {
		t.Fatalf("exchange calls = %d, want 1", exchanger.calls)
	}
}

func TestOAuthCompleteRejectsMissingOrDeclinedPermissionsBeforeSaving(t *testing.T) {
	tests := []struct {
		name           string
		permissions    []OAuthPermission
		wantMissing    []string
		wantNotGranted []string
	}{
		{
			name: "missing",
			permissions: []OAuthPermission{
				{Name: "ads_management", Status: "granted"},
				{Name: "ads_read", Status: "granted"},
				{Name: "business_management", Status: "granted"},
				{Name: "pages_show_list", Status: "granted"},
				{Name: "pages_read_engagement", Status: "granted"},
			},
			wantMissing: []string{"instagram_basic"},
		},
		{
			name: "declined",
			permissions: []OAuthPermission{
				{Name: "ads_management", Status: "granted"},
				{Name: "ads_read", Status: "declined"},
				{Name: "business_management", Status: "granted"},
				{Name: "pages_show_list", Status: "granted"},
				{Name: "pages_read_engagement", Status: "granted"},
				{Name: "instagram_basic", Status: "granted"},
			},
			wantNotGranted: []string{"ads_read"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x5a}, oauthStateBytes))
			repository := &oauthRepositoryFake{pending: OAuthPendingState{
				AccountID:   "11111111-1111-4111-8111-111111111111",
				RedirectURI: "https://api.example.com/v1/public/meta-ads/oauth/callback",
			}}
			saver := &oauthConnectionSaverFake{}
			exchanger := &oauthExchangerFake{
				token:       OAuthAccessToken{Value: "must-not-be-saved"},
				permissions: tt.permissions,
			}
			service := newOAuthServiceTest(repository, saver, exchanger)

			err := service.Complete(context.Background(), raw, "provider-code", false)
			if !errors.Is(err, ErrOAuthPermissions) {
				t.Fatalf("Complete error = %v", err)
			}
			var permissionErr *OAuthPermissionsError
			if !errors.As(err, &permissionErr) {
				t.Fatalf("error type = %T", err)
			}
			if strings.Join(permissionErr.Missing, ",") != strings.Join(tt.wantMissing, ",") {
				t.Fatalf("missing = %#v", permissionErr.Missing)
			}
			if strings.Join(permissionErr.NotGranted, ",") != strings.Join(tt.wantNotGranted, ",") {
				t.Fatalf("not granted = %#v", permissionErr.NotGranted)
			}
			if saver.calls != 0 {
				t.Fatal("conexao ou expiracao persistida antes de validar permissoes")
			}
			if strings.Contains(err.Error(), "must-not-be-saved") || strings.Contains(err.Error(), "ads_read") {
				t.Fatalf("erro logavel contem detalhe sensivel: %v", err)
			}
		})
	}
}

func TestOAuthDeniedConsumesStateBeforeReturning(t *testing.T) {
	raw := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x4d}, oauthStateBytes))
	repository := &oauthRepositoryFake{pending: OAuthPendingState{
		AccountID:   "11111111-1111-4111-8111-111111111111",
		RedirectURI: "https://api.example.com/v1/public/meta-ads/oauth/callback",
	}}
	exchanger := &oauthExchangerFake{}
	service := newOAuthServiceTest(repository, &oauthConnectionSaverFake{}, exchanger)

	if err := service.Complete(context.Background(), raw, "", true); !errors.Is(err, ErrOAuthDenied) {
		t.Fatalf("denied = %v", err)
	}
	if !repository.consumed || exchanger.calls != 0 {
		t.Fatal("state negado nao foi consumido antes da troca")
	}
}

func TestOAuthRedirectValidationFailsClosed(t *testing.T) {
	invalid := []string{
		"",
		"http://example.com/v1/public/meta-ads/oauth/callback",
		"https://api.example.com/other",
		"https://api.example.com/v1/public/meta-ads/oauth/callback?next=x",
	}
	for _, raw := range invalid {
		if _, err := validateOAuthRedirectURI(raw); !errors.Is(err, ErrOAuthInvalidConfig) {
			t.Fatalf("redirect %q = %v", raw, err)
		}
	}
	if got, err := validateOAuthRedirectURI("http://127.0.0.1:8080/v1/public/meta-ads/oauth/callback"); err != nil || got == "" {
		t.Fatalf("loopback dev redirect = %q, %v", got, err)
	}
}
