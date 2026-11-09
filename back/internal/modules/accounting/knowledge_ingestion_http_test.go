package accounting

import (
	"context"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/knowledge"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestKnowledgeIngestionHTTPRejectsForgedURLAndScope(t *testing.T) {
	s, sc, p := fixture(t)
	user := auth.User{ID: sc.ActorID, Email: "synthetic@ingestion.test", Role: auth.RoleDirector, Active: true}
	tokens := auth.NewHMACTokenManager("synthetic-ingestion-only", time.Hour)
	session, e := tokens.Issue("", user)
	if e != nil {
		t.Fatal(e)
	}
	m := auth.NewMiddleware(auth.NewService(httpAuthUsers{user: user}, nil, tokens, nil, nil, nil, nil))
	m.SetAccountChecker(httpMembership{account: sc.AccountID})
	k := knowledge.NewService(knowledge.NewStore(p), knowledge.Ports{Authorize: s.AuthorizeKnowledge, IngestionSources: knowledge.OfficialIngestionSources()})
	mux := http.NewServeMux()
	RegisterKnowledgeRoutes(mux, k, m)
	root := "/v1/accounting/issuers/" + sc.IssuerID + "/knowledge/ingestion"
	for _, c := range []struct {
		method, path, body string
		status             int
	}{{"GET", root + "/catalog", "", 200}, {"PUT", root + "/subscriptions", `{"url":"https://127.0.0.1/private"}`, 400}, {"PUT", root + "/subscriptions", `{"sourceId":"forged","accountId":"forged"}`, 400}, {"GET", "/v1/accounting/issuers/00000000-0000-4000-8000-000000000099/knowledge/ingestion/catalog", "", 404}} {
		r := httptest.NewRequestWithContext(context.Background(), c.method, c.path, strings.NewReader(c.body))
		r.Header.Set("Authorization", "Bearer "+session.AccessToken)
		r.Header.Set("X-Account-Id", sc.AccountID)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != c.status {
			t.Fatalf("%s: %d %s", c.path, w.Code, w.Body.String())
		}
	}
}
