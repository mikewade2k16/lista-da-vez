package finance

import (
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"net/http"
)

func RegisterDocumentReviewRoutes(mux *http.ServeMux, svc *DocumentService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireFinancePermission("finance.sheets.view", h))
	}
	mux.Handle("GET /v1/finance/documents/{id}/review", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.Review(r.Context(), p.AccountID, p.UserID, r.PathValue("id"))
		fiscalResult(w, r, out, err)
	}))
	mutate := func(approve bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var in DocumentReviewInput
			if !decodeFiscal(w, r, &in) {
				return
			}
			p, _ := auth.PrincipalFromContext(r.Context())
			var out DocumentReview
			var err error
			if approve {
				out, err = svc.ApproveReview(r.Context(), p.AccountID, p.UserID, r.PathValue("id"), in)
			} else {
				out, err = svc.RevokeReview(r.Context(), p.AccountID, p.UserID, r.PathValue("id"), in)
			}
			fiscalResult(w, r, out, err)
		}
	}
	mux.Handle("POST /v1/finance/documents/{id}/review/approval", wrap(mutate(true)))
	mux.Handle("DELETE /v1/finance/documents/{id}/review/approval", wrap(mutate(false)))
}
