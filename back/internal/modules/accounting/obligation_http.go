package accounting

import (
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"net/http"
)

func RegisterObligationRoutes(mux *http.ServeMux, s *Service, m *auth.Middleware) {
	const root = "/v1/accounting/issuers/{issuer}/obligations"
	mux.Handle("GET "+root+"/{obligation}/history", m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, l, e := pageLimit(r)
		if e != nil {
			result(w, r, 0, nil, e)
			return
		}
		v, e := s.ObligationHistory(r.Context(), scope(r), r.PathValue("obligation"), p, l)
		result(w, r, 200, v, e)
	})))
	mux.Handle("GET "+root, m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, l, e := pageLimit(r)
		if e != nil {
			result(w, r, 0, nil, e)
			return
		}
		v, e := s.ReadObligations(r.Context(), scope(r), r.URL.Query().Get("period"), "", p, l)
		result(w, r, 200, v, e)
	})))
	mux.Handle("GET "+root+"/{obligation}", m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, e := s.ReadObligations(r.Context(), scope(r), "", r.PathValue("obligation"), 1, 20)
		result(w, r, 200, v, e)
	})))
	mux.Handle("POST "+root, m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in ObligationInput
		if e := decode(w, r, &in); e != nil {
			result(w, r, 0, nil, e)
			return
		}
		v, e := s.SaveObligation(r.Context(), scope(r), r.Header.Get("Idempotency-Key"), in)
		result(w, r, 200, v, e)
	})))
	const rules = "/v1/accounting/issuers/{issuer}/obligation-rules"
	mux.Handle("GET "+rules, m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, e := s.ListObligationRules(r.Context(), scope(r), r.URL.Query().Get("code"))
		result(w, r, 200, v, e)
	})))
	mux.Handle("POST "+rules, m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in ObligationRuleInput
		if e := decode(w, r, &in); e != nil {
			result(w, r, 0, nil, e)
			return
		}
		v, e := s.CreateObligationRule(r.Context(), scope(r), r.Header.Get("Idempotency-Key"), in)
		result(w, r, 200, v, e)
	})))
	for _, action := range []string{"activate", "retire"} {
		mux.Handle("POST "+rules+"/{rule}/"+action, m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var in ObligationRuleTransition
			if e := decode(w, r, &in); e != nil {
				result(w, r, 0, nil, e)
				return
			}
			v, e := s.TransitionObligationRule(r.Context(), scope(r), r.PathValue("rule"), action, r.Header.Get("Idempotency-Key"), in)
			result(w, r, 200, v, e)
		})))
	}
	for _, operation := range []string{"review", "reopen", "prepare-finance", "report-payment"} {
		mux.Handle("POST "+root+"/{obligation}/"+operation, m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var in ObligationAction
			if e := decode(w, r, &in); e != nil {
				result(w, r, 0, nil, e)
				return
			}
			v, e := s.ObligationCommand(r.Context(), scope(r), r.PathValue("obligation"), operation, r.Header.Get("Idempotency-Key"), in)
			result(w, r, 200, v, e)
		})))
	}
}
