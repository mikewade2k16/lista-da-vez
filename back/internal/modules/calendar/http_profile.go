package calendar

import (
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterProfileRoutes monta os endpoints do perfil estrategico do cliente
// (/v1/calendar/client-profile*). Como as demais rotas do painel: RequireAuth +
// accountScope; o gating por modulo e global (RequireModuleByPath no Chain). O
// accountID vem SEMPRE do Principal (nunca do body); o clientId vem da query e e
// validado como UUID. Contrato C3.
func RegisterProfileRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	mux.Handle("GET /v1/calendar/client-profile", wrap(handleGetClientProfile(svc)))
	mux.Handle("PUT /v1/calendar/client-profile", wrap(handlePutClientProfile(svc)))
	mux.Handle("GET /v1/calendar/client-profiles", wrap(handleListClientProfiles(svc)))
}

func handleGetClientProfile(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
		view, err := svc.GetClientProfile(r.Context(), accountID, clientID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handlePutClientProfile(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
		var in ProfileInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.PutClientProfile(r.Context(), accountID, clientID, in, principalLabel(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleListClientProfiles(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		profiles, err := svc.ListClientProfiles(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
	}
}
