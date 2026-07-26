package communications

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const maxJSONBodyBytes = 32 << 10

func accessFromRequest(r *http.Request) (AccessContext, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return AccessContext{}, false
	}
	accountID := strings.TrimSpace(principal.AccountID)
	if accountID == "" {
		accountID = strings.TrimSpace(principal.TenantID)
	}
	return AccessContext{
		UserID:              principal.UserID,
		AccountID:           accountID,
		Role:                string(principal.Role),
		StoreIDs:            append([]string{}, principal.StoreIDs...),
		Permissions:         append([]string{}, principal.Permissions...),
		PermissionsResolved: principal.PermissionsResolved,
	}, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	requireAccount := middleware.RequireAuthWithAccount

	mux.Handle("GET /v1/operations/communications", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			response, err := service.List(r.Context(), access, ListFilter{
				StoreID:       strings.TrimSpace(r.URL.Query().Get("storeId")),
				PublishedOnly: r.URL.Query().Get("publishedOnly") == "true",
			})
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, response)
		},
	)))

	mux.Handle("POST /v1/operations/communications", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			var input UpsertInput
			if err := decodeJSON(w, r, &input); err != nil {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
				return
			}
			item, err := service.Create(r.Context(), access, input)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"communication": item})
		},
	)))

	mux.Handle("PUT /v1/operations/communications/{communicationId}", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			var input UpsertInput
			if err := decodeJSON(w, r, &input); err != nil {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
				return
			}
			item, err := service.Update(
				r.Context(),
				access,
				r.PathValue("communicationId"),
				input,
			)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, map[string]any{"communication": item})
		},
	)))

	mux.Handle("DELETE /v1/operations/communications/{communicationId}", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			if err := service.Delete(
				r.Context(),
				access,
				r.PathValue("communicationId"),
			); err != nil {
				writeError(w, r, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		},
	)))
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para administrar comunicados.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "communication_not_found", "Comunicado nao encontrado.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados e as lojas do comunicado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Nao foi possivel processar o comunicado.")
	}
}
