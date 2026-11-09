package finance

import (
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"net/http"
)

func RegisterDocumentPreparationRoutes(mux *http.ServeMux, svc *DocumentService, middleware *auth.Middleware) {
	mux.Handle("POST /v1/finance/documents/{id}/prepare", middleware.RequireAuthWithAccount(requireFinancePermission("finance.sheets.view", func(w http.ResponseWriter, r *http.Request) {
		var in DocumentPrepareInput
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.Prepare(r.Context(), p.AccountID, p.UserID, r.PathValue("id"), in.ExpectedUpdatedAt)
		fiscalResult(w, r, out, err)
	})))
	mux.Handle("GET /v1/finance/documents/{id}", middleware.RequireAuthWithAccount(requireFinancePermission("finance.sheets.view", func(w http.ResponseWriter, r *http.Request) {
		if !isUUID(r.PathValue("id")) {
			fiscalResult(w, r, nil, ErrFiscalNotFound)
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.Get(r.Context(), p.AccountID, p.UserID, r.PathValue("id"))
		fiscalResult(w, r, out, err)
	})))
}
