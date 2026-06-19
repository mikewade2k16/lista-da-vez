package automation

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// omniContextHeader e o header onde o n8n reenvia o context token HMAC opaco
// emitido no /ask. O escopo (account/tenant/store) das tools de dados do Omni
// Chat sai SO deste token assinado, nunca do query/body.
const omniContextHeader = "X-Omni-Context"

// registerOmniChatToolRoutes monta as tools de dados do Omni Chat (Fase 2),
// consumidas pelo n8n. Fora do prefixo /v1/automation (sem gating de modulo /
// X-Account-Id). Auth de transporte = token de servico (runtime token); escopo
// multi-tenant = context token assinado no header X-Omni-Context.
func registerOmniChatToolRoutes(mux *http.ServeMux, svc *Service, token string, ctxMgr *ContextTokenManager) {
	mux.Handle("GET /v1/runtime/omni-chat/catalog", handleOmniChatCatalogTool(svc, token, ctxMgr))
}

// handleOmniChatCatalogTool e a tool de catalogo do Omni Chat (Fase 2).
//
//	GET /v1/runtime/omni-chat/catalog?q=
//	Authorization: Bearer $AUTOMATION_RUNTIME_TOKEN   (transporte)
//	X-Omni-Context: <ctxv1 token>                     (escopo multi-tenant)
//
// Resposta: 200 { produtos: [{ name, code, price }], total } (produtos vazio se q vazio).
func handleOmniChatCatalogTool(svc *Service, token string, ctxMgr *ContextTokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) Transporte: token de servico (constant-time via bearerEquals).
		switch {
		case token == "":
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "runtime_not_configured",
				"AUTOMATION_RUNTIME_TOKEN nao configurado.")
			return
		case !bearerEquals(r.Header.Get("Authorization"), token):
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Token de servico invalido.")
			return
		}

		// 2) Escopo: context token assinado. account/tenant/store saem SO daqui.
		// Erro generico (invalido OU expirado) -> 401, sem vazar o motivo.
		scope, err := ctxMgr.Parse(strings.TrimSpace(r.Header.Get(omniContextHeader)))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Context token invalido.")
			return
		}

		// 3) Busca escopada pelo accountID do TOKEN (nunca do query). Tres intencoes
		// resolvidas em OmniChatCatalog (vazio/NONE, "LISTAR"=amostra, termo=busca com
		// fallback p/ amostra) para o bot nunca travar nem ficar "burro".
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		hits, mode, err := svc.OmniChatCatalog(r.Context(), scope.AccountID, query)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao buscar no catalogo.")
			return
		}
		// Observabilidade: account/q/mode/contagem (account UUID e q nao sao segredo) —
		// ajuda a diagnosticar "catalogo vazio" sem precisar reconstruir execucao do n8n.
		slog.Info("omni-chat catalog tool", "account", scope.AccountID, "q", query, "mode", mode, "results", len(hits))
		httpapi.WriteJSON(w, http.StatusOK, struct {
			Produtos []ProductHit `json:"produtos"`
			Total    int          `json:"total"`
			Mode     string       `json:"mode"`
		}{Produtos: hits, Total: len(hits), Mode: mode})
	}
}
