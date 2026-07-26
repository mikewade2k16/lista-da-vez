package customerdata

import "net/http"

func registerNoteConsentRoutes(mux *http.ServeMux, service *Service, wrap func(http.Handler) http.Handler) {
	route(mux, "GET /v1/customer-data/relationships/{relationshipId}/notes", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.ListNotes(r.Context(), principal, r.PathValue("relationshipId"), queryLimit(r))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"items": out})
	})
	route(mux, "POST /v1/customer-data/relationships/{relationshipId}/notes", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input NoteInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, replayed, err := service.CreateNote(r.Context(), principal, r.PathValue("relationshipId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusCreated, map[string]any{"note": out, "replayed": replayed})
	})
	route(mux, "PATCH /v1/customer-data/notes/{noteId}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var patch NotePatch
		if !decodeStrictJSON(w, r, &patch) {
			return
		}
		out, err := service.UpdateNote(r.Context(), principal, r.PathValue("noteId"), patch)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "POST /v1/customer-data/notes/{noteId}/archive", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		revision, err := queryExpectedRevision(r)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		out, err := service.ArchiveNote(r.Context(), principal, r.PathValue("noteId"), revision)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})

	route(mux, "GET /v1/customer-data/relationships/{relationshipId}/consents", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.ListConsents(r.Context(), principal, r.PathValue("relationshipId"), queryLimit(r))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"items": out})
	})
	route(mux, "POST /v1/customer-data/relationships/{relationshipId}/consents", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input ConsentInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, replayed, err := service.RecordConsent(r.Context(), principal, r.PathValue("relationshipId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusCreated, map[string]any{"consent": out, "replayed": replayed})
	})
}
