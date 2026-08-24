package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type connectionSnapshotRepositoryFake struct {
	hasCryptoKey bool
	saves        int
	token        string
	adAccounts   []AdAccount
}

func (f *connectionSnapshotRepositoryFake) HasCryptoKey() bool {
	return f.hasCryptoKey
}

func (f *connectionSnapshotRepositoryFake) SaveConnectionSnapshot(
	_ context.Context,
	accountID string,
	_ string,
	name string,
	token string,
	_ *time.Time,
	adAccounts []AdAccount,
) (Connection, error) {
	f.saves++
	f.token = token
	f.adAccounts = append([]AdAccount(nil), adAccounts...)
	return Connection{
		ID: "connection-id", AccountID: accountID, Name: name,
		Status: connectionActive, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}, nil
}

func TestManualConnectionRequiresSamePermissionsAsOAuthBeforeSaving(t *testing.T) {
	const token = "manual-sensitive-token"
	tests := []struct {
		name               string
		permissionsStatus  int
		permissionsPayload any
		wantPermissionErr  bool
		wantGraphErr       bool
		wantSave           bool
	}{
		{
			name:               "missing",
			permissionsStatus:  http.StatusOK,
			permissionsPayload: permissionPayloadWithout("instagram_basic", ""),
			wantPermissionErr:  true,
		},
		{
			name:               "declined",
			permissionsStatus:  http.StatusOK,
			permissionsPayload: permissionPayloadWithout("", "ads_management"),
			wantPermissionErr:  true,
		},
		{
			name:               "provider error is sanitized",
			permissionsStatus:  http.StatusBadGateway,
			permissionsPayload: map[string]any{"error": map[string]any{"message": "provider-body-secret"}},
			wantGraphErr:       true,
		},
		{
			name:               "complete",
			permissionsStatus:  http.StatusOK,
			permissionsPayload: permissionPayloadWithout("", ""),
			wantSave:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			permissionCalls := 0
			adAccountCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.String(), token) || r.URL.Query().Get("access_token") != "" {
					t.Errorf("token leaked in URL: %s", r.URL.String())
				}
				if authorization := r.Header.Get("Authorization"); authorization != "Bearer "+token {
					t.Errorf("Authorization=%q", authorization)
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/me/permissions":
					permissionCalls++
					w.WriteHeader(tt.permissionsStatus)
					if err := json.NewEncoder(w).Encode(tt.permissionsPayload); err != nil {
						t.Errorf("encode permissions: %v", err)
					}
				case "/me/adaccounts":
					adAccountCalls++
					_, _ = w.Write([]byte(`{"data":[{"account_id":"123","name":"Conta","currency":"BRL","account_status":1}]}`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			repository := &connectionSnapshotRepositoryFake{hasCryptoKey: true}
			service := &Service{
				connectionSnapshots: repository,
				client:              NewMetaClient(server.URL),
			}
			view, err := service.SaveConnection(
				context.Background(), "11111111-1111-4111-8111-111111111111", token,
			)
			switch {
			case tt.wantPermissionErr && !errors.Is(err, ErrOAuthPermissions):
				t.Fatalf("SaveConnection error=%v want ErrOAuthPermissions", err)
			case tt.wantGraphErr && (err == nil || !strings.HasPrefix(err.Error(), "meta graph:")):
				t.Fatalf("SaveConnection error=%v want sanitized Graph error", err)
			case tt.wantSave && err != nil:
				t.Fatalf("SaveConnection: %v", err)
			}
			if err != nil && (strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "provider-body-secret")) {
				t.Fatalf("error exposed token/provider body: %v", err)
			}
			if permissionCalls != 1 {
				t.Fatalf("permission calls=%d want=1", permissionCalls)
			}
			if tt.wantSave {
				if repository.saves != 1 || adAccountCalls != 1 || !view.Connected ||
					repository.token != token || len(repository.adAccounts) != 1 {
					t.Fatalf("complete grant did not save one snapshot: saves=%d adCalls=%d view=%#v accounts=%#v",
						repository.saves, adAccountCalls, view, repository.adAccounts)
				}
			} else if repository.saves != 0 || adAccountCalls != 0 {
				t.Fatalf("invalid grants reached discovery/save: saves=%d adCalls=%d", repository.saves, adAccountCalls)
			}
		})
	}
}

func TestManualConnectionPermissionErrorMapsToSanitized422(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		context.Background(), http.MethodPost, "/v1/meta-ads/connection", nil,
	)
	writeServiceError(recorder, request, &OAuthPermissionsError{
		Missing: []string{"instagram_basic"}, NotGranted: []string{"ads_management"},
	}, "must-not-appear")

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want=%d", recorder.Code, http.StatusUnprocessableEntity)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "missing_permissions") {
		t.Fatalf("response without stable code: %s", body)
	}
	for _, forbidden := range []string{"instagram_basic", "ads_management", "must-not-appear"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed internal detail %q: %s", forbidden, body)
		}
	}
}

func permissionPayloadWithout(missing string, declined string) map[string]any {
	data := make([]map[string]string, 0, len(defaultOAuthScopes))
	for _, scope := range defaultOAuthScopes {
		if scope == missing {
			continue
		}
		status := "granted"
		if scope == declined {
			status = "declined"
		}
		data = append(data, map[string]string{
			"permission": scope,
			"status":     status,
		})
	}
	return map[string]any{"data": data}
}
