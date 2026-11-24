package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type accountTestUsers struct {
	auth.UserRepository
	user auth.User
}

func (s accountTestUsers) LoadUserForAuth(context.Context, string) (auth.User, error) {
	return s.user, nil
}

type accountTestChecker struct{ member string }

func (c accountTestChecker) IsMember(_ context.Context, accountID, _ string) (bool, error) {
	return accountID == c.member, nil
}

func TestRequireAuthWithAccountForeignAccountIsNotFound(t *testing.T) {
	user := auth.User{ID: "u1", Email: "u1@test.local", Role: auth.RoleDirector, Active: true}
	tokens := auth.NewHMACTokenManager("middleware-account-test", time.Hour)
	session, err := tokens.Issue("", user)
	if err != nil {
		t.Fatal(err)
	}
	svc := auth.NewService(accountTestUsers{user: user}, nil, tokens, nil, nil, nil, nil)
	middleware := auth.NewMiddleware(svc)
	middleware.SetAccountChecker(accountTestChecker{member: "own-account"})
	handler := middleware.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	do := func(accountID string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer "+session.AccessToken)
		req.Header.Set("X-Account-Id", accountID)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}

	if res := do("own-account"); res.Code != http.StatusNoContent {
		t.Fatalf("conta propria: status %d", res.Code)
	}
	res := do("foreign-account")
	if res.Code != http.StatusNotFound {
		t.Fatalf("conta alheia: status %d, want 404", res.Code)
	}
	if !strings.Contains(res.Body.String(), "not_found") || strings.Contains(res.Body.String(), "account_not_member") {
		t.Fatalf("corpo deve ser generico: %s", res.Body.String())
	}
	// Conta inexistente deve ser indistinguivel de conta alheia.
	if other := do("does-not-exist"); other.Code != res.Code {
		t.Fatalf("inexistente %d != alheia %d", other.Code, res.Code)
	}
}
