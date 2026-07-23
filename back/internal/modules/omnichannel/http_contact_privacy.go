package omnichannel

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func RegisterContactPrivacyRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuthWithAccount(h) }
	mux.Handle("GET /v1/omnichannel/privacy/hidden-contacts", wrap(handleListHiddenContacts(svc)))
	mux.Handle("POST /v1/omnichannel/conversations/{conversationId}/privacy/hide", wrap(handleHideContactConversation(svc)))
	mux.Handle("POST /v1/omnichannel/privacy/hidden-contacts/{contactId}/restore", wrap(handleRestoreHiddenContact(svc)))
	mux.Handle("GET /v1/omnichannel/conversations/{conversationId}/ai-restriction", wrap(handleGetContactAIRestriction(svc)))
	mux.Handle("PUT /v1/omnichannel/conversations/{conversationId}/ai-restriction", wrap(handlePutContactAIRestriction(svc)))
}

func handleGetContactAIRestriction(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.GetContactAIRestriction(r.Context(), p.AccountID, p, r.PathValue("conversationId"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, out)
	}
}

func handlePutContactAIRestriction(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in ContactAIRestrictionInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateContactAIRestriction(r.Context(), p.AccountID, p, r.PathValue("conversationId"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, out)
	}
}

func handleListHiddenContacts(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListHiddenContacts(r.Context(), p.AccountID, p)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, out)
	}
}

func handleHideContactConversation(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in HideContactInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.HideContactConversation(r.Context(), p.AccountID, p, r.PathValue("conversationId"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, out)
	}
}

func handleRestoreHiddenContact(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		if err := svc.RestoreHiddenContact(r.Context(), p.AccountID, p, r.PathValue("contactId")); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
