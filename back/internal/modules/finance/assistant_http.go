package finance

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func RegisterAssistantRoutes(mux *http.ServeMux, svc *AssistantService, middleware *auth.Middleware) {
	mux.Handle("GET /v1/finance/assistant/config", middleware.RequireAuthWithAccount(requireFinancePermission("finance.config.view", func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.Config(r.Context(), p.AccountID)
		fiscalResult(w, r, out, err)
	})))
	mux.Handle("PUT /v1/finance/assistant/config", middleware.RequireAuthWithAccount(requireFinancePermission("finance.config.manage", func(w http.ResponseWriter, r *http.Request) {
		var in AssistantConfigInput
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.SaveConfig(r.Context(), p.AccountID, in)
		fiscalResult(w, r, out, err)
	})))
}
