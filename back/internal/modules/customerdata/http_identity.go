package customerdata

import "net/http"

func registerIdentityRoutes(mux *http.ServeMux, service *Service, wrap func(http.Handler) http.Handler) {
	route(mux, "GET /v1/customer-data/relationships/{relationshipId}/identities", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.ListIdentities(r.Context(), principal, r.PathValue("relationshipId"))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"items": out})
	})

	route(mux, "POST /v1/customer-data/relationships/{relationshipId}/identities", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input IdentityInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, replayed, err := service.AddIdentity(r.Context(), principal, r.PathValue("relationshipId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusCreated, map[string]any{"identity": out, "replayed": replayed})
	})

	identityState := func(state string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			principal, ok := requestPrincipal(w, r)
			if !ok {
				return
			}
			var input IdentityStateInput
			if !decodeStrictJSON(w, r, &input) {
				return
			}
			out, replayed, err := service.SetIdentityState(r.Context(), principal, r.PathValue("identityId"), state, input)
			if err != nil {
				writeCustomerDataError(w, err)
				return
			}
			writeCustomerDataJSON(w, http.StatusOK, map[string]any{"identity": out, "replayed": replayed})
		}
	}
	route(mux, "POST /v1/customer-data/identities/{identityId}/verify", wrap, identityState("verified"))
	route(mux, "POST /v1/customer-data/identities/{identityId}/revoke", wrap, identityState("revoked"))
}
