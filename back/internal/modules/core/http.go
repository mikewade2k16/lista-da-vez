package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterRoutes registra os endpoints v2 do core. Deve ser chamado APENAS
// quando cfg.CoreV2Enabled e true. Quando off, o mux nem conhece as rotas
// (callers recebem 404 padrao).
//
// Endpoints expostos:
//   GET  /v2/me/accounts                 → lista accounts do user (lean)
//   GET  /v2/me/context?accountId=<id>   → contexto completo do account
func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v2/me/accounts", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		response, err := service.MeAccounts(r.Context(), principal.UserID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	})))

	mux.Handle("GET /v2/me/context", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
		if accountID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_account_id", "Informe accountId na query string.")
			return
		}

		response, err := service.MeContext(r.Context(), principal.UserID, accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	})))
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUserNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "user_not_found", "Usuario nao encontrado no schema core.")
	case errors.Is(err, ErrAccountNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "account_not_found", "Account nao encontrada.")
	case errors.Is(err, ErrAccountNotMember):
		// Mesmo codigo do not_found para nao revelar existencia.
		httpapi.WriteError(w, r, http.StatusNotFound, "account_not_found", "Account nao encontrada para o usuario.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao carregar o contexto do core.")
	}
}
