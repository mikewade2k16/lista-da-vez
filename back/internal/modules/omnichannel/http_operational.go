package omnichannel

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func RegisterOperationalRoutes(mux *http.ServeMux, svc *OperationalService, middleware *auth.Middleware) {
	wrap := func(handler http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(handler)
	}
	mux.Handle("GET /v1/omnichannel/operations/health", wrap(handleOperationalHealth(svc)))
}

func handleOperationalHealth(svc *OperationalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		view, err := svc.Health(r.Context(), accountID, caller.UserID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}
