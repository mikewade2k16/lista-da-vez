package finance

import (
	"errors"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Public projection for Finance readers; never leaks model, credential or instructions.
func RegisterPlusStatusRoute(mux *http.ServeMux, svc *PlusService, middleware *auth.Middleware) {
	mux.Handle("GET /v1/finance/assistant/plus/status", middleware.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		sc := PlusScope{AccountID: p.AccountID, ActorID: p.UserID, IssuerID: r.URL.Query().Get("issuerId")}
		e := svc.Authorize(r.Context(), sc)
		var denied *PlusError
		if errors.As(e, &denied) && denied.Code == "plus_disabled" {
			httpapi.WriteJSON(w, 200, map[string]any{"status": "success", "data": map[string]any{"enabled": false, "mode": "legacy", "toolPolicyVersion": 1}})
			return
		}
		if e != nil {
			status, code := 500, "plus_error"
			if errors.As(e, &denied) {
				status, code = denied.Status, denied.Code
			}
			httpapi.WriteError(w, r, status, code, "Plus indisponível para este contexto.")
			return
		}
		config, e := svc.assistant.Config(r.Context(), sc.AccountID)
		if e != nil {
			httpapi.WriteError(w, r, 503, "plus_unavailable", "Configuração indisponível.")
			return
		}
		tools, e := svc.Catalog(r.Context(), sc)
		if e != nil {
			httpapi.WriteError(w, r, 503, "plus_unavailable", "Ferramentas indisponíveis.")
			return
		}
		httpapi.WriteJSON(w, 200, map[string]any{"status": "success", "data": map[string]any{"enabled": true, "mode": "plus", "explanationsEnabled": config.Plus.Explanation.Enabled, "toolPolicyVersion": 1, "tools": tools}})
	})))
}
