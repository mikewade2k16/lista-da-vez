package calendar

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterRuntimeRoutes monta a rota de contexto compartilhado consumida por
// outros runtimes (chat do calendario, bot WhatsApp, Omni Chat) — chamada
// service-to-service. FORA do prefixo /v1/calendar (gateado por modulo +
// X-Account-Id): a autenticacao aqui e por TOKEN DE SERVICO (AUTOMATION_RUNTIME_TOKEN,
// o MESMO token do runtime do automation), nao por JWT de usuario. O prefixo
// /v1/runtime NAO esta em moduleGatingRules (app.go), entao fica fora do gate.
func RegisterRuntimeRoutes(mux *http.ServeMux, svc *Service, runtimeToken string) {
	mux.Handle("GET /v1/runtime/calendar/context", handleRuntimeContext(svc, runtimeToken))
}

// handleRuntimeContext devolve o agregado C9 (account/client/month/holidays/
// monthNotes/events/plans). Ordem dos checks: autentica ANTES de validar input
// (nao revela erro de input a chamador nao autenticado).
//   - token de servico ausente no env => 503 (nao aceita chamada anonima);
//   - Bearer errado/ausente => 401;
//   - accountId ausente ou nao-UUID => 400 (validado, nunca confiado como escopo cego).
//
// accountId vem do QUERY porque a chamada e service-to-service (sem JWT nem
// X-Account-Id): o isolamento vem do token de servico + do filtro por account_id
// em todas as queries do BuildAIContext. clientId e month sao opcionais (month
// vazio = mes corrente).
func handleRuntimeContext(svc *Service, runtimeToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(runtimeToken) == "" {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "runtime_not_configured",
				"AUTOMATION_RUNTIME_TOKEN nao configurado.")
			return
		}
		if !runtimeBearerEquals(r.Header.Get("Authorization"), runtimeToken) {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Token de servico invalido.")
			return
		}
		accountID := normalizeUUID(r.URL.Query().Get("accountId"))
		if accountID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_account", "accountId obrigatorio (UUID).")
			return
		}
		out, err := svc.BuildAIContext(r.Context(), accountID,
			strings.TrimSpace(r.URL.Query().Get("clientId")),
			strings.TrimSpace(r.URL.Query().Get("month")))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, out)
	}
}

// runtimeBearerEquals compara o header Authorization com o token de servico em
// tempo constante (crypto/subtle). Header vazio ou sem o prefixo Bearer => falso.
func runtimeBearerEquals(header, token string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimSpace(header[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}
