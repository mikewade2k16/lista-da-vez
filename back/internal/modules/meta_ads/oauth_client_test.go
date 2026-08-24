package metaads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestMetaOAuthClientKeepsSecretsOutOfURLAndUsesLongLivedToken(t *testing.T) {
	var requests []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("secret-bearing query = %q", r.URL.RawQuery)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests = append(requests, r.PostForm)
		w.Header().Set("Content-Type", "application/json")
		if r.PostForm.Get("grant_type") == "fb_exchange_token" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "long-lived-token",
				"expires_in":   3600,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "short-token",
			"expires_in":   300,
		})
	}))
	defer server.Close()

	client := NewMetaOAuthClient(server.URL)
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }
	token, err := client.ExchangeCode(
		context.Background(),
		"app-id",
		"app-secret",
		"https://api.example.com/v1/public/meta-ads/oauth/callback",
		"one-time-code",
	)
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if token.Value != "long-lived-token" || !token.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("token = %#v", token)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d", len(requests))
	}
	if requests[0].Get("code") != "one-time-code" || requests[0].Get("client_secret") != "app-secret" {
		t.Fatalf("short exchange form = %#v", requests[0])
	}
	if requests[1].Get("fb_exchange_token") != "short-token" {
		t.Fatalf("long exchange form = %#v", requests[1])
	}
}

func TestMetaOAuthClientDoesNotReturnProviderTokenBodyInError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid grant","code":190},"access_token":"must-not-leak"}`))
	}))
	defer server.Close()

	client := NewMetaOAuthClient(server.URL)
	_, err := client.ExchangeCode(context.Background(), "app-id", "app-secret", "https://api.example.com/callback", "code")
	if err == nil {
		t.Fatal("expected exchange error")
	}
	if strings.Contains(err.Error(), "must-not-leak") || strings.Contains(err.Error(), "app-secret") {
		t.Fatalf("secret leaked in error: %v", err)
	}
}

func TestMetaOAuthClientListPermissionsUsesBearerWithoutTokenInURL(t *testing.T) {
	const token = "permission-check-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/me/permissions" || r.URL.RawQuery != "" {
			t.Errorf("permissions URL = %q", r.URL.RequestURI())
		}
		if strings.Contains(r.RequestURI, token) {
			t.Errorf("token leaked in request URI = %q", r.RequestURI)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"permission":"ads_management","status":"granted"},{"permission":"ads_read","status":"declined"}]}`))
	}))
	defer server.Close()

	client := NewMetaOAuthClient(server.URL)
	permissions, err := client.ListPermissions(context.Background(), token)
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	if len(permissions) != 2 || permissions[0].Name != "ads_management" ||
		permissions[0].Status != "granted" || permissions[1].Status != "declined" {
		t.Fatalf("permissions = %#v", permissions)
	}
}

func TestMetaOAuthClientListPermissionsDoesNotExposeProviderBody(t *testing.T) {
	const token = "must-not-leak-permission-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RequestURI(), token) {
			t.Errorf("token leaked in URL = %q", r.URL.RequestURI())
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"token must-not-leak-permission-token rejected"}}`))
	}))
	defer server.Close()

	client := NewMetaOAuthClient(server.URL)
	_, err := client.ListPermissions(context.Background(), token)
	if err == nil {
		t.Fatal("expected permissions error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("token leaked in error: %v", err)
	}
}
