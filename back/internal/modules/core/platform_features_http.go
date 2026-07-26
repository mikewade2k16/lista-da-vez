package core

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterExperimentalFeaturesRoutes expõe o rollout global. Todos os usuários
// autenticados podem ler para gatear o produto; somente platform_admin escreve.
//
//	GET /v1/platform/experimental-features
//	PUT /v1/platform/experimental-features
func RegisterExperimentalFeaturesRoutes(
	mux *http.ServeMux,
	svc *ExperimentalFeaturesService,
	middleware *auth.Middleware,
) {
	mux.Handle(
		"GET /v1/platform/experimental-features",
		middleware.RequireAuth(handleGetExperimentalFeatures(svc)),
	)
	mux.Handle(
		"PUT /v1/platform/experimental-features",
		middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !requirePlatformAdmin(w, r) {
				return
			}
			handlePutExperimentalFeatures(svc)(w, r)
		})),
	)
}

type putExperimentalFeaturesRequest struct {
	Features ExperimentalFeatures `json:"features"`
}

func handleGetExperimentalFeatures(svc *ExperimentalFeaturesService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := svc.Get(r.Context())
		if err != nil {
			writeExperimentalFeaturesError(w, r)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func handlePutExperimentalFeatures(svc *ExperimentalFeaturesService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input putExperimentalFeaturesRequest
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		response, err := svc.Save(r.Context(), input.Features, principal.UserID)
		if err != nil {
			writeExperimentalFeaturesError(w, r)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func writeExperimentalFeaturesError(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(
		w,
		r,
		http.StatusInternalServerError,
		"internal_error",
		"Erro interno ao processar a requisicao.",
	)
}
