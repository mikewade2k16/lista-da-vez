package finance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type financeAuthUsers struct {
	auth.UserRepository
	user auth.User
}

func (s financeAuthUsers) LoadUserForAuth(context.Context, string) (auth.User, error) {
	return s.user, nil
}

type financeMembership struct{ permissions []string }

func (s financeMembership) IsMember(_ context.Context, accountID, _ string) (bool, error) {
	return accountID == "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil
}
func (s financeMembership) ResolveAccountPermissions(context.Context, string, string) ([]string, error) {
	return s.permissions, nil
}

type accessConfigStore struct{ ConfigStore }

func (s accessConfigStore) GetConfig(context.Context, string, string) (ConfigData, error) {
	return ConfigData{Categories: []Category{}, FixedAccounts: []FixedAccount{}, RecurringEntries: []RecurringEntry{}}, nil
}
func (s accessConfigStore) SaveConfig(context.Context, string, ConfigData) (ConfigData, error) {
	return ConfigData{}, nil
}

func TestFinanceRouteAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, account string
		permissions                 []string
		role                        auth.Role
		want                        int
	}{
		{"reader can read", "GET", "/v1/finance/config", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", []string{"finance.config.view"}, auth.RoleDirector, 200},
		{"reader cannot write", "PUT", "/v1/finance/config", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", []string{"finance.config.view"}, auth.RoleDirector, 403},
		{"forged account rejected", "GET", "/v1/finance/config", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", []string{"finance.config.view"}, auth.RoleDirector, 403},
		{"missing account rejected", "GET", "/v1/finance/config", "", []string{"finance.config.view"}, auth.RoleDirector, 400},
		{"empty permissions rejected", "GET", "/v1/finance/config", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil, auth.RoleDirector, 403},
		{"owner preserves access", "GET", "/v1/finance/config", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", []string{"core.account.manage"}, auth.RoleOwner, 200},
		{"owner role cannot bypass account RBAC", "GET", "/v1/finance/config", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil, auth.RoleOwner, 403},
		{"platform admin preserves access", "GET", "/v1/finance/config", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil, auth.RolePlatformAdmin, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := auth.User{ID: "audit-user", Email: "audit@finance.test", Role: tc.role, Active: true}
			tokens := auth.NewHMACTokenManager("finance-test-only", time.Hour)
			session, err := tokens.Issue("", user)
			if err != nil {
				t.Fatal(err)
			}
			svc := auth.NewService(financeAuthUsers{user: user}, nil, tokens, nil, nil, nil, nil)
			middleware := auth.NewMiddleware(svc)
			middleware.SetAccountChecker(financeMembership{tc.permissions})
			mux := http.NewServeMux()
			RegisterRoutes(mux, NewService(nil, accessConfigStore{}), middleware)
			req := httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+session.AccessToken)
			req.Header.Set("X-Account-Id", tc.account)
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("status %d, want %d: %s", res.Code, tc.want, res.Body.String())
			}
		})
	}
}
