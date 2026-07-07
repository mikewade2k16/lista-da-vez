package calendar

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// maxCallbackBody limita o corpo do callback do n8n (content do plano pode ter
// varias ideias por cliente). Teto de 2 MiB conforme contrato C4.
const maxCallbackBody = 2 << 20 // 2 MiB

// RegisterAIPlanRoutes monta os endpoints de IA do calendario (contrato C4):
//   - painel (/v1/calendar/ai/*): RequireAuth + accountScope, como as demais rotas;
//   - callback publico (/v1/public/calendar-ai/...): SEM JWT, autenticado por
//     X-Service-Token (comparacao constant-time com o env do servico). O prefixo
//     /v1/public fica FORA do gate de modulo (moduleGatingRules em app.go).
func RegisterAIPlanRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	// RequireAuthWithAccount valida membership na account do header: o plano resolve e USA
	// a API key da conta no dispatch (server->n8n). Sem o gate, a conta A dispararia plano
	// gastando a key da conta B via X-Account-Id forjado. O callback publico segue por token.
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}

	mux.Handle("POST /v1/calendar/ai/plan", wrap(handleCreateAIPlan(svc)))
	mux.Handle("GET /v1/calendar/ai/plans", wrap(handleListAIPlans(svc)))
	mux.Handle("GET /v1/calendar/ai/plans/{id}", wrap(handleGetAIPlan(svc)))
	mux.Handle("POST /v1/calendar/ai/plans/{id}/applied", wrap(handleMarkAIPlanApplied(svc)))
	mux.Handle("DELETE /v1/calendar/ai/plans/{id}", wrap(handleDeleteAIPlan(svc)))

	// Callback publico (n8n -> api). SEM RequireAuth: autentica por X-Service-Token.
	mux.Handle("POST /v1/public/calendar-ai/plans/{id}/result", handleAIPlanCallback(svc))
}

func handleCreateAIPlan(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var req AIPlanRequest
		if err := decodeJSONBody(w, r, &req); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.CreateAIPlan(r.Context(), accountID, req, principalLabel(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"id": view.ID, "status": view.Status})
	}
}

func handleListAIPlans(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		month := strings.TrimSpace(r.URL.Query().Get("month"))
		plans, err := svc.ListAIPlans(r.Context(), accountID, month)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"plans": plans})
	}
}

func handleGetAIPlan(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.GetAIPlan(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleMarkAIPlanApplied(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.MarkAIPlanApplied(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDeleteAIPlan(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		if err := svc.DeleteAIPlan(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleAIPlanCallback recebe o resultado do n8n. Autentica por X-Service-Token
// (constant-time). Sem token de servico configurado no env => 503 (nao aceita
// callback anonimo). Token errado => 403. Contrato C4.
func handleAIPlanCallback(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !svc.aiCallbackConfigured() {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "ai_not_configured",
				"IA do calendario nao configurada (CALENDAR_AI_SERVICE_TOKEN ausente).")
			return
		}
		if !svc.checkServiceToken(r.Header.Get("X-Service-Token")) {
			httpapi.WriteError(w, r, http.StatusForbidden, "invalid_token", "Token de servico invalido.")
			return
		}
		var cb AIPlanCallback
		if err := decodeCallbackBody(w, r, &cb); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if err := svc.ApplyPlanResult(r.Context(), r.PathValue("id"), cb); err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}
}

// checkServiceToken compara o header com o token do env em tempo constante
// (crypto/subtle). Token do env vazio => sempre falso (verificado antes no handler
// via aiCallbackConfigured, mas defesa em profundidade aqui tambem).
func (s *Service) checkServiceToken(header string) bool {
	want := strings.TrimSpace(s.ai.ServiceToken)
	got := strings.TrimSpace(header)
	if want == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// aiCallbackConfigured indica se o env do token de servico esta presente.
func (s *Service) aiCallbackConfigured() bool {
	return strings.TrimSpace(s.ai.ServiceToken) != ""
}

// decodeCallbackBody decodifica o corpo do callback com teto de 2 MiB (C4).
func decodeCallbackBody(w http.ResponseWriter, r *http.Request, dst *AIPlanCallback) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCallbackBody)).Decode(dst)
}
