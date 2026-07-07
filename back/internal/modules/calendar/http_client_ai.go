package calendar

import (
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterClientAIRoutes monta o override de IA por cliente (WAVE 3.1, SPEC-B3):
// GET/PUT /v1/calendar/ai-config/client?clientId=. A rota e account-scoped e SENSIVEL
// (muda o comportamento da IA que consome a key da conta) => RequireAuthWithAccount
// valida MEMBERSHIP na account do header X-Account-Id (mesmo gate dos secrets/chat da
// 3.0): sem isso, a conta A mandaria X-Account-Id: B e leria/gravaria o override de B.
// clientId UUID obrigatorio (senao 400 invalid_client); accountID vem do Principal,
// NUNCA do body. Fica sob /v1/calendar (gate de modulo aplicado no Chain).
func RegisterClientAIRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrapAcct := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuthWithAccount(h) }
	mux.Handle("GET /v1/calendar/ai-config/client", wrapAcct(handleGetClientAIOverride(svc)))
	mux.Handle("PUT /v1/calendar/ai-config/client", wrapAcct(handlePutClientAIOverride(svc)))
}

// handleGetClientAIOverride devolve o override do cliente (ou vazio, HasOverride=false).
func handleGetClientAIOverride(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
		view, err := svc.GetClientAIOverride(r.Context(), accountID, clientID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// handlePutClientAIOverride faz upsert do override do cliente e devolve o status atualizado.
func handlePutClientAIOverride(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
		var in ClientAIOverride
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.PutClientAIOverride(r.Context(), accountID, clientID, in, principalLabel(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}
