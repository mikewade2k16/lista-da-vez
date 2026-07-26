package customerdata

import "net/http"

func registerOfflineRoutes(mux *http.ServeMux, service *Service, wrap func(http.Handler) http.Handler) {
	route(mux, "GET /v1/customer-data/relationships/{relationshipId}/offline-interactions", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.ListOfflineInteractions(r.Context(), principal, r.PathValue("relationshipId"), queryLimit(r))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"items": out})
	})
	route(mux, "POST /v1/customer-data/relationships/{relationshipId}/offline-interactions", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input OfflineInteractionInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, replayed, err := service.CreateOfflineInteraction(r.Context(), principal, r.PathValue("relationshipId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusCreated, map[string]any{"interaction": out, "replayed": replayed})
	})
	route(mux, "PATCH /v1/customer-data/offline-interactions/{interactionId}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var patch OfflineInteractionPatch
		if !decodeStrictJSON(w, r, &patch) {
			return
		}
		out, err := service.UpdateOfflineInteraction(r.Context(), principal, r.PathValue("interactionId"), patch)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "POST /v1/customer-data/offline-interactions/{interactionId}/archive", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		revision, err := queryExpectedRevision(r)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		out, err := service.ArchiveOfflineInteraction(r.Context(), principal, r.PathValue("interactionId"), revision)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
}
