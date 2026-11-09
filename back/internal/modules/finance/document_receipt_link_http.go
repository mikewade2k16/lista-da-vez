package finance

import (
	"errors"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func receiptLinkResult(w http.ResponseWriter, r *http.Request, data any, err error) {
	switch {
	case errors.Is(err, ErrReceiptAlreadyLinked):
		httpapi.WriteError(w, r, http.StatusConflict, "receipt_already_linked", "Este comprovante já está vinculado a outro lançamento.")
	case errors.Is(err, ErrSheetClosed), errors.Is(err, ErrSheetConflict), errors.Is(err, ErrSheetNotFound):
		writeFinanceError(w, r, err)
	default:
		fiscalResult(w, r, data, err)
	}
}

// Reading needs sheet view; writing an entry needs sheet manage, like editing the sheet.
func RegisterReceiptLinkRoutes(mux *http.ServeMux, svc *DocumentService, middleware *auth.Middleware) {
	wrap := func(permission string, h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireFinancePermission(permission, h))
	}
	mux.Handle("GET /v1/finance/documents/{id}/receipt-link", wrap("finance.sheets.view", func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.GetReceiptLink(r.Context(), p.AccountID, p.UserID, r.PathValue("id"))
		receiptLinkResult(w, r, data, err)
	}))
	mux.Handle("POST /v1/finance/documents/{id}/receipt-link", wrap("finance.sheets.manage", func(w http.ResponseWriter, r *http.Request) {
		var in ReceiptLinkInput
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.LinkReceipt(r.Context(), p.AccountID, p.UserID, r.PathValue("id"), in)
		receiptLinkResult(w, r, data, err)
	}))
	mux.Handle("DELETE /v1/finance/documents/{id}/receipt-link", wrap("finance.sheets.manage", func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		err := svc.UnlinkReceipt(r.Context(), p.AccountID, p.UserID, r.PathValue("id"))
		receiptLinkResult(w, r, map[string]bool{"unlinked": err == nil}, err)
	}))
}
