package omnichannel

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Instagram routes are account-scoped and intentionally live beside the
// WhatsApp Cloud configuration routes. They manage credentials, moderation
// state and comments; sending is performed only by the outbox worker.
func registerInstagramRoutes(mux *http.ServeMux, svc *InstagramService, middleware *auth.Middleware) {
	if svc == nil {
		return
	}
	wrap := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuthWithAccount(h) }
	mux.Handle("PUT /v1/omnichannel/tenant/instagram/accounts", wrap(handleConfigureInstagram(svc)))
	mux.Handle("GET /v1/omnichannel/tenant/instagram/accounts", wrap(handleListInstagramAccounts(svc)))
	mux.Handle("GET /v1/omnichannel/tenant/instagram/comments", wrap(handleListInstagramComments(svc)))
	mux.Handle("GET /v1/omnichannel/tenant/instagram/comments/{commentId}/actions", wrap(handleListInstagramActions(svc)))
	mux.Handle("POST /v1/omnichannel/tenant/instagram/comments/{commentId}/actions/{actionId}/decide", wrap(handleDecideInstagramAction(svc)))
}

func handleConfigureInstagram(svc *InstagramService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in InstagramAccountInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.Configure(r.Context(), accountID, caller, in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleListInstagramAccounts(svc *InstagramService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		items, err := svc.Accounts(r.Context(), accountID, caller)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func handleListInstagramComments(svc *InstagramService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		items, err := svc.Comments(r.Context(), accountID, caller, r.URL.Query().Get("instagramAccountId"))
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func handleListInstagramActions(svc *InstagramService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		items, err := svc.Actions(r.Context(), accountID, caller, r.PathValue("commentId"))
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func handleDecideInstagramAction(svc *InstagramService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in InstagramActionDecisionInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.DecideAction(r.Context(), accountID, caller, r.PathValue("commentId"), r.PathValue("actionId"), in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}
