package omnichannel

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func registerWhatsAppCloudRoutes(mux *http.ServeMux, svc *WhatsAppCloudService, middleware *auth.Middleware) {
	if svc == nil {
		return
	}
	wrap := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuthWithAccount(h) }
	mux.Handle("PUT /v1/omnichannel/tenant/whatsapp/instances/{id}/meta-cloud", wrap(handleConfigureMetaCloud(svc)))
	mux.Handle("GET /v1/omnichannel/tenant/whatsapp/instances/{id}/meta-cloud", wrap(handleGetMetaCloud(svc)))
	mux.Handle("GET /v1/omnichannel/tenant/whatsapp/instances/{id}/templates", wrap(handleListMetaTemplates(svc)))
	mux.Handle("POST /v1/omnichannel/tenant/whatsapp/instances/{id}/templates/sync", wrap(handleSyncMetaTemplates(svc)))
}

func handleConfigureMetaCloud(svc *WhatsAppCloudService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in MetaCloudConfigInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.Configure(r.Context(), accountID, caller, r.PathValue("id"), in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleGetMetaCloud(svc *WhatsAppCloudService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		view, err := svc.GetConfig(r.Context(), accountID, caller, r.PathValue("id"))
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleListMetaTemplates(svc *WhatsAppCloudService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		items, err := svc.Templates(r.Context(), accountID, caller, r.PathValue("id"))
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func handleSyncMetaTemplates(svc *WhatsAppCloudService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		items, err := svc.SyncTemplates(r.Context(), accountID, caller, r.PathValue("id"))
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}
