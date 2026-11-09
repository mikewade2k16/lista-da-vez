package accounting

import (
	"context"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestObligationHTTPStrictScopeHistoryAndRevocation(t *testing.T) {
	s, sc, _ := obligationFixture(t)
	user := auth.User{ID: sc.ActorID, Email: "synthetic@obligation.test", Role: auth.RoleDirector, Active: true}
	tokens := auth.NewHMACTokenManager("synthetic-obligation-only", time.Hour)
	session, e := tokens.Issue("", user)
	if e != nil {
		t.Fatal(e)
	}
	m := auth.NewMiddleware(auth.NewService(httpAuthUsers{user: user}, nil, tokens, nil, nil, nil, nil))
	m.SetAccountChecker(httpMembership{account: sc.AccountID})
	mux := http.NewServeMux()
	RegisterRoutes(mux, s, m)
	o := saveTestObligation(t, s, sc, "http-pending-obligation", ObligationInput{Code: "company.declaration_review", Period: "2026"})
	root := "/v1/accounting/issuers/" + sc.IssuerID + "/obligations"
	for _, c := range []struct {
		method, path, body string
		status             int
	}{{"GET", root, "", 200}, {"GET", root + "/" + o.ID + "/history", "", 200}, {"GET", root + "?limit=999", "", 400}, {"GET", "/v1/accounting/issuers/00000000-0000-4000-8000-000000000099/obligations/" + o.ID, "", 404}, {"POST", root, `{"code":"mei.das","period":"2026-11","amount":"0.00"}`, 400}, {"POST", root, `{"code":"mei.das","period":"2026-11","accountId":"forged"}`, 400}, {"POST", root + "/" + o.ID + "/review", `{"expectedVersion":1,"confirmed":true}`, 409}} {
		r := httptest.NewRequestWithContext(context.Background(), c.method, c.path, strings.NewReader(c.body))
		r.Header.Set("Authorization", "Bearer "+session.AccessToken)
		r.Header.Set("X-Account-Id", sc.AccountID)
		r.Header.Set("Idempotency-Key", "obligation-http-command")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != c.status {
			t.Fatalf("%s %s: %d %s", c.method, c.path, w.Code, w.Body.String())
		}
	}
	s.policy = &testPolicy{deny: "accounting.obligations.view"}
	_, e = s.ReadObligations(context.Background(), sc, "", o.ID, 1, 20)
	expectCode(t, e, "permission_denied")
}
