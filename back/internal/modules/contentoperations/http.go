package contentoperations

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/content-operations/brief", middleware.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticação obrigatória.")
			return
		}
		brief, err := service.Brief(r.Context(), principal, strings.TrimSpace(principal.AccountID))
		if err != nil {
			switch {
			case errors.Is(err, ErrForbidden):
				httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissão para ver a operação de conteúdo.")
			case errors.Is(err, ErrNotReady):
				httpapi.WriteError(w, r, http.StatusServiceUnavailable, "not_ready", "Operação de conteúdo indisponível.")
			default:
				httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao calcular os alertas de conteúdo.")
			}
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, brief)
	})))
}
