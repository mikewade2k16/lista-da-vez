package finance

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// Recurring customers: listing, history and manual edit (new immutable version).
// The chat also writes when a draft is completed; account and user come from the session.
func RegisterFiscalCustomerRoutes(mux *http.ServeMux, svc *DocumentService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireFinancePermission("finance.sheets.view", h))
	}
	mux.Handle("GET /v1/finance/fiscal/customers", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.customers.List(r.Context(), p.AccountID, p.UserID)
		fiscalResult(w, r, data, err)
	}))
	mux.Handle("GET /v1/finance/fiscal/customers/{id}/versions", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.customers.Versions(r.Context(), p.AccountID, p.UserID, r.PathValue("id"))
		fiscalResult(w, r, data, err)
	}))
	mux.Handle("PUT /v1/finance/fiscal/customers/{id}", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in FiscalCustomerInput
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.customers.Update(r.Context(), p.AccountID, p.UserID, r.PathValue("id"), in)
		fiscalResult(w, r, data, err)
	}))
}
