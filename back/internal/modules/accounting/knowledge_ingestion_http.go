package accounting

import (
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/knowledge"
	"net/http"
)

func registerKnowledgeIngestionRoutes(mux *http.ServeMux, s *knowledge.Service, m *auth.Middleware) {
	const root = "/v1/accounting/issuers/{issuer}/knowledge/ingestion"
	scopeOf := func(r *http.Request) knowledge.Scope {
		v := scope(r)
		return knowledge.Scope{AccountID: v.AccountID, ActorID: v.ActorID, ResourceID: v.IssuerID}
	}
	mux.Handle("GET "+root+"/catalog", m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, e := s.IngestionCatalog(r.Context(), scopeOf(r))
		knowledgeResult(w, r, v, e)
	})))
	mux.Handle("GET "+root+"/subscriptions", m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, e := s.SourceSubscriptions(r.Context(), scopeOf(r))
		knowledgeResult(w, r, v, e)
	})))
	mux.Handle("PUT "+root+"/subscriptions", m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in knowledge.SubscriptionInput
		if e := decode(w, r, &in); e != nil {
			knowledgeResult(w, r, nil, e)
			return
		}
		v, e := s.SaveSourceSubscription(r.Context(), scopeOf(r), r.Header.Get("Idempotency-Key"), in)
		knowledgeResult(w, r, v, e)
	})))
	mux.Handle("GET "+root+"/runs", m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, l, e := pageLimit(r)
		if e != nil {
			knowledgeResult(w, r, nil, e)
			return
		}
		v, e := s.IngestionRuns(r.Context(), scopeOf(r), p, l)
		knowledgeResult(w, r, v, e)
	})))
}
