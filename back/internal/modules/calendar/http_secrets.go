package calendar

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// secretInput e o body dos PUT de key (SEC). apiKey vazio = limpar. A key crua NUNCA
// volta numa resposta (o GET/PUT respondem so com KeyStatusView mascarado).
type secretInput struct {
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
}

// RegisterSecretRoutes monta os endpoints de keys de IA do calendario (Wave 3, SEC).
// Conta (accountScope): GET/PUT /v1/calendar/ai-keys. Global (SO platform_admin):
// GET/PUT /v1/calendar/ai-keys/global. Ficam sob /v1/calendar (gate de modulo). O GET
// devolve SO status mascarado {set,last4}; a key crua nunca sai do server.
func RegisterSecretRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	// wrapAcct valida MEMBERSHIP na account do header X-Account-Id (RequireAuthWithAccount:
	// core.account_users/org, com bypass de platform_admin). Sem isso, um usuario da conta A
	// mandaria X-Account-Id: B e leria/gravaria a API key da conta B (secrets sao a superficie
	// mais sensivel do modulo). O global e platform_admin no proprio handler.
	wrapAcct := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuthWithAccount(h) }
	wrap := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuth(h) }

	mux.Handle("GET /v1/calendar/ai-keys", wrapAcct(handleGetAIKeys(svc)))
	mux.Handle("PUT /v1/calendar/ai-keys", wrapAcct(handlePutAIKey(svc)))
	mux.Handle("GET /v1/calendar/ai-keys/global", wrap(handleGetGlobalAIKeys(svc)))
	mux.Handle("PUT /v1/calendar/ai-keys/global", wrap(handlePutGlobalAIKey(svc)))
}

// handleGetAIKeys devolve o status mascarado da FONTE ATIVA da conta (global ou
// conta, conforme ai.useGlobalKeys).
func handleGetAIKeys(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		status, err := svc.GetAccountKeyStatus(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, status)
	}
}

// handlePutAIKey grava/limpa a key DESTA conta e devolve o status mascarado atualizado.
func handlePutAIKey(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body secretInput
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if err := svc.PutAccountKey(r.Context(), accountID, body.Provider, body.APIKey, principalLabel(r)); err != nil {
			writeServiceError(w, r, err)
			return
		}
		status, err := svc.GetAccountKeyStatus(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, status)
	}
}

// handleGetGlobalAIKeys devolve o status mascarado das keys GLOBAIS (so platform_admin).
func handleGetGlobalAIKeys(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePlatformAdmin(w, r) {
			return
		}
		status, err := svc.GetGlobalKeyStatus(r.Context())
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, status)
	}
}

// handlePutGlobalAIKey grava/limpa a key GLOBAL da plataforma (so platform_admin).
func handlePutGlobalAIKey(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePlatformAdmin(w, r) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		var body secretInput
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if err := svc.PutGlobalKey(r.Context(), body.Provider, body.APIKey, principal.UserID); err != nil {
			writeServiceError(w, r, err)
			return
		}
		status, err := svc.GetGlobalKeyStatus(r.Context())
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, status)
	}
}

// requirePlatformAdmin garante que o Principal e platform_admin; senao escreve 403 e
// devolve false. As keys globais valem para todas as contas, entao so o admin edita.
func requirePlatformAdmin(w http.ResponseWriter, r *http.Request) bool {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || principal.Role != auth.RolePlatformAdmin {
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Apenas platform_admin.")
		return false
	}
	return true
}
