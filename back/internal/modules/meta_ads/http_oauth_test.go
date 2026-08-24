package metaads

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOAuthCallbackNeverReflectsProviderSecrets(t *testing.T) {
	rawState := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x6e}, oauthStateBytes))
	repository := &oauthRepositoryFake{pending: OAuthPendingState{
		AccountID:   "11111111-1111-4111-8111-111111111111",
		RedirectURI: "https://api.example.com/v1/public/meta-ads/oauth/callback",
	}}
	service := newOAuthServiceTest(repository, &oauthConnectionSaverFake{}, &oauthExchangerFake{})
	query := url.Values{
		"state":             {rawState},
		"code":              {"provider-secret-code"},
		"error":             {"access_denied"},
		"error_description": {"provider-secret-description"},
	}
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		metaOAuthCallbackPath+"?"+query.Encode(),
		nil,
	)
	recorder := httptest.NewRecorder()

	handleOAuthCallback(service).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache control = %q", recorder.Header().Get("Cache-Control"))
	}
	body := recorder.Body.String()
	for _, secret := range []string{rawState, "provider-secret-code", "provider-secret-description"} {
		if strings.Contains(body, secret) {
			t.Fatalf("callback reflected secret %q: %s", secret, body)
		}
	}
}

func TestOAuthCallbackMissingPermissionsReturnsConstantPageWithoutSaving(t *testing.T) {
	rawState := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x7f}, oauthStateBytes))
	repository := &oauthRepositoryFake{pending: OAuthPendingState{
		AccountID:   "11111111-1111-4111-8111-111111111111",
		RedirectURI: "https://api.example.com/v1/public/meta-ads/oauth/callback",
	}}
	saver := &oauthConnectionSaverFake{}
	exchanger := &oauthExchangerFake{
		token:       OAuthAccessToken{Value: "provider-secret-token"},
		permissions: []OAuthPermission{},
	}
	service := newOAuthServiceTest(repository, saver, exchanger)
	query := url.Values{
		"state": {rawState},
		"code":  {"provider-secret-code"},
	}
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		metaOAuthCallbackPath+"?"+query.Encode(),
		nil,
	)
	recorder := httptest.NewRecorder()

	handleOAuthCallback(service).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "Permissoes necessarias nao concedidas") ||
		!strings.Contains(body, "Nenhuma conexao foi criada") {
		t.Fatalf("unexpected callback page: %s", body)
	}
	for _, secret := range []string{rawState, "provider-secret-code", "provider-secret-token", "instagram_basic"} {
		if strings.Contains(body, secret) {
			t.Fatalf("callback reflected detail %q: %s", secret, body)
		}
	}
	if saver.calls != 0 {
		t.Fatal("connection saved without required permissions")
	}
}
