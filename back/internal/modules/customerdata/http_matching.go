package customerdata

import "net/http"

func registerMatchingRoutes(mux *http.ServeMux, service *Service, wrap func(http.Handler) http.Handler) {
	route(mux, "GET /v1/customer-data/match-candidates", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		items, cursor, err := service.ListMatchCandidates(
			r.Context(), principal, r.URL.Query().Get("clientAccountId"),
			MatchCandidateFilter{
				Status: r.URL.Query().Get("status"), Cursor: r.URL.Query().Get("cursor"), Limit: queryLimit(r),
			},
		)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": cursor})
	})
	route(mux, "GET /v1/customer-data/match-candidates/{candidateId}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.GetMatchCandidate(r.Context(), principal, r.PathValue("candidateId"))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "POST /v1/customer-data/match-candidates/{candidateId}/decision", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input MatchDecisionInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, replayed, err := service.DecideMatchCandidate(r.Context(), principal, r.PathValue("candidateId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"candidate": out, "replayed": replayed})
	})
	route(mux, "POST /v1/customer-data/subjects/{subjectId}/merge", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input MergeInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, err := service.MergeSubjects(
			r.Context(), principal, r.PathValue("subjectId"),
			r.URL.Query().Get("clientAccountId"), input,
		)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusCreated, out)
	})
	route(mux, "POST /v1/customer-data/merges/{mergeId}/undo", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input UndoMergeInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, err := service.UndoMerge(r.Context(), principal, r.PathValue("mergeId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusCreated, out)
	})
}
