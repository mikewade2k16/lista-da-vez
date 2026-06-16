package automation

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// registerModelRoutes monta os endpoints do painel de modelos (A6). AccountID vem
// do principal (X-Account-Id); o gating por modulo ja roda no Chain.
func registerModelRoutes(mux *http.ServeMux, svc *Service, wrap func(http.HandlerFunc) http.Handler) {
	mux.Handle("GET /v1/automation/models", wrap(handleModelsGet(svc)))
	mux.Handle("PUT /v1/automation/models", wrap(handleModelsPut(svc)))
}

func handleModelsGet(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.Models(r.Context(), accountID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao carregar os modelos.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleModelsPut(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			Role     string          `json:"role"`
			Provider string          `json:"provider"`
			ModelID  string          `json:"modelId"`
			Params   json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		role := strings.TrimSpace(body.Role)
		provider := strings.TrimSpace(body.Provider)
		modelID := strings.TrimSpace(body.ModelID)
		if role == "" || provider == "" || modelID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_params", "role, provider e modelId sao obrigatorios.")
			return
		}
		view, err := svc.SetModel(r.Context(), accountID, role, provider, modelID, body.Params)
		if err != nil {
			if errors.Is(err, ErrInvalidModel) {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_model", "Modelo invalido para esta funcao.")
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao salvar o modelo.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}
