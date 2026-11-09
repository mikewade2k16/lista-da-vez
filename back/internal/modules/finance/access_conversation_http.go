package finance

import (
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
	"net/http"
)

func RegisterAccessConversationRoutes(mux *http.ServeMux, svc *AccessConversationService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireFinancePermission("finance.sheets.view", h))
	}
	mux.Handle("GET /v1/finance/conversation", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.List(r.Context(), p.AccountID, p.UserID)
		conversationResult(w, r, out, err)
	}))
	mux.Handle("POST /v1/finance/conversation", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in AccessConversationInput
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.Send(r.Context(), p.AccountID, p.UserID, in)
		conversationResult(w, r, out, err)
	}))
}

func conversationResult(w http.ResponseWriter, r *http.Request, data any, err error) {
	var domain *PlusError
	switch {
	case errors.Is(err, ErrSheetNotFound):
		fiscalResult(w, r, nil, ErrFiscalNotFound)
	case errors.As(err, &domain):
		httpapi.WriteError(w, r, domain.Status, domain.Code, "Não foi possível atender nesse contexto. Confira as permissões e tente novamente.")
	default:
		fiscalResult(w, r, data, err)
	}
}
