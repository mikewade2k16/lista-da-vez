package omnichannel

import (
	"errors"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func RegisterRolloutRoutes(mux *http.ServeMux, svc *RolloutService, middleware *auth.Middleware) {
	wrap := func(handler http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(handler)
	}
	mux.Handle("GET /v1/omnichannel/settings/rollout", wrap(handleGetRollout(svc)))
	mux.Handle("PUT /v1/omnichannel/settings/rollout", wrap(handlePutRollout(svc)))
}

func handleGetRollout(svc *RolloutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		view, err := svc.Get(r.Context(), accountID, caller.UserID)
		if err != nil {
			writeRolloutError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handlePutRollout(svc *RolloutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in RolloutConfigInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.Update(r.Context(), accountID, caller.UserID, in)
		if err != nil {
			writeRolloutError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func writeRolloutError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrRolloutRevisionConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "rollout_revision_conflict",
			"A configuracao mudou em outra sessao. Recarregue antes de salvar.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "rollout_invalid",
			"Configuracao de rollout invalida.")
	default:
		writeServiceError(w, r, err)
	}
}
