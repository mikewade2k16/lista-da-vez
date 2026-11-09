package accounting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type httpAuthUsers struct {
	auth.UserRepository
	user auth.User
}

func (u httpAuthUsers) LoadUserForAuth(context.Context, string) (auth.User, error) {
	return u.user, nil
}

type httpMembership struct{ account string }

func (m httpMembership) IsMember(_ context.Context, account, _ string) (bool, error) {
	return account == m.account, nil
}
func TestHTTPAuthenticationStrictPayloadAndScopedReads(t *testing.T) {
	s, sc, _ := fixture(t)
	user := auth.User{ID: sc.ActorID, Email: "synthetic@accounting.test", Role: auth.RoleDirector, Active: true}
	tokens := auth.NewHMACTokenManager("synthetic-accounting-test-only", time.Hour)
	session, e := tokens.Issue("", user)
	if e != nil {
		t.Fatal(e)
	}
	middleware := auth.NewMiddleware(auth.NewService(httpAuthUsers{user: user}, nil, tokens, nil, nil, nil, nil))
	middleware.SetAccountChecker(httpMembership{account: sc.AccountID})
	mux := http.NewServeMux()
	RegisterRoutes(mux, s, middleware)
	for _, tc := range []struct {
		method, path, body, account string
		authenticated               bool
		status                      int
	}{
		{"GET", "/v1/accounting/issuers", "", sc.AccountID, false, 401},
		{"GET", "/v1/accounting/issuers", "", "00000000-0000-4000-8000-000000000099", true, 403},
		{"POST", "/v1/accounting/issuers", `{"accountId":"forged"}`, sc.AccountID, true, 400},
		{"POST", "/v1/accounting/issuers", `{} {}`, sc.AccountID, true, 400},
		{"POST", "/v1/accounting/issuers", `{"expectedVersion":0,"profile":{"legalName":"Synthetic"},"grants":["issue"]}`, sc.AccountID, true, 400},
		{"GET", "/v1/accounting/issuers/00000000-0000-4000-8000-000000000099", "", sc.AccountID, true, 404},
		{"GET", "/v1/accounting/issuers/" + sc.IssuerID, "", sc.AccountID, true, 200},
		{"GET", "/v1/accounting/issuers?limit=999", "", sc.AccountID, true, 400},
	} {
		req := httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-Account-Id", tc.account)
		req.Header.Set("Idempotency-Key", "http-fixture-key")
		if tc.authenticated {
			req.Header.Set("Authorization", "Bearer "+session.AccessToken)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s %s got %d want %d: %s", tc.method, tc.path, rec.Code, tc.status, rec.Body.String())
		}
	}
}
