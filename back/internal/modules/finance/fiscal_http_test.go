package finance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type fiscalScopeStore struct {
	FiscalStore
	account, user string
}

func (s *fiscalScopeStore) GetProfile(_ context.Context, account, user string) (FiscalProfileView, error) {
	s.account = account
	s.user = user
	return FiscalProfileView{}, nil
}
func TestFiscalHTTPUsesActorAndRejectsInjectedIdentity(t *testing.T) {
	user := auth.User{ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", Email: "fiscal@example.invalid", Role: auth.RoleDirector, Active: true}
	tokens := auth.NewHMACTokenManager("fiscal-tests-only", time.Hour)
	session, err := tokens.Issue("", user)
	if err != nil {
		t.Fatal(err)
	}
	service := auth.NewService(financeAuthUsers{user: user}, nil, tokens, nil, nil, nil, nil)
	middleware := auth.NewMiddleware(service)
	middleware.SetAccountChecker(financeMembership{[]string{"finance.sheets.view"}})
	store := &fiscalScopeStore{}
	mux := http.NewServeMux()
	RegisterFiscalRoutes(mux, NewFiscalService(store, nil), middleware)
	for _, test := range []struct {
		method, path, body, account string
		want                        int
	}{
		{"GET", "/v1/finance/fiscal/profile", "", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 200},
		{"GET", "/v1/finance/fiscal/profile", "", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", 403},
		{"PUT", "/v1/finance/fiscal/profile", `{"userId":"someone-else","profile":{}}`, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 400},
		{"PUT", "/v1/finance/fiscal/profile", `{"profile":{"password":"secret"}}`, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 400},
		{"PUT", "/v1/finance/fiscal/profile", `{"profile":{}} {"profile":{}}`, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 400},
		{"GET", "/v1/finance/fiscal/credentials", "", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 405},
	} {
		request := httptest.NewRequestWithContext(context.Background(), test.method, test.path, strings.NewReader(test.body))
		request.Header.Set("Authorization", "Bearer "+session.AccessToken)
		request.Header.Set("X-Account-Id", test.account)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("%s %s: %d want %d", test.method, test.path, response.Code, test.want)
		}
	}
	if store.account != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" || store.user != user.ID {
		t.Fatal("scope did not come from principal")
	}
}
