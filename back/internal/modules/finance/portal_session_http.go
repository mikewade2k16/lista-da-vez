package finance

import (
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"net/http"
)

func RegisterPortalSessionRoutes(mux *http.ServeMux, svc *FiscalService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireFinancePermission("finance.sheets.view", h))
	}
	mux.Handle("GET /v1/finance/fiscal/portal-session", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.PortalSessionStatus(r.Context(), p.AccountID, p.UserID)
		fiscalResult(w, r, out, err)
	}))
	mux.Handle("POST /v1/finance/fiscal/portal-session", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
		}
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.ConnectPortal(r.Context(), p.AccountID, p.UserID, in.ExpectedUpdatedAt)
		fiscalResult(w, r, out, err)
	}))
}
