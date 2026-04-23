package stores

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type listResponse struct {
	Stores []StoreView `json:"stores"`
}

type storeResponse struct {
	Store StoreView `json:"store"`
}

type deleteResponse struct {
	StoreID string `json:"storeId"`
	Deleted bool   `json:"deleted"`
	SavedAt string `json:"savedAt"`
}

type createRequest struct {
	TenantID          string   `json:"tenantId"`
	Code              string   `json:"code"`
	Name              string   `json:"name"`
	City              string   `json:"city"`
	DefaultTemplateID string   `json:"defaultTemplateId"`
	MonthlyGoal       *float64 `json:"monthlyGoal"`
	WeeklyGoal        *float64 `json:"weeklyGoal"`
	AvgTicketGoal     *float64 `json:"avgTicketGoal"`
	ConversionGoal    *float64 `json:"conversionGoal"`
	PAGoal            *float64 `json:"paGoal"`
	IsActive          *bool    `json:"isActive"`
}

type updateRequest struct {
	Name              *string  `json:"name"`
	Code              *string  `json:"code"`
	City              *string  `json:"city"`
	DefaultTemplateID *string  `json:"defaultTemplateId"`
	MonthlyGoal       *float64 `json:"monthlyGoal"`
	WeeklyGoal        *float64 `json:"weeklyGoal"`
	AvgTicketGoal     *float64 `json:"avgTicketGoal"`
	ConversionGoal    *float64 `json:"conversionGoal"`
	PAGoal            *float64 `json:"paGoal"`
	IsActive          *bool    `json:"isActive"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/stores", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		stores, err := service.ListAccessible(r.Context(), principal, ListInput{
			TenantID:        strings.TrimSpace(r.URL.Query().Get("tenantId")),
			IncludeInactive: readBoolQuery(r, "includeInactive"),
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, listResponse{
			Stores: stores,
		})
	})))

	mux.Handle("POST /v1/stores", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request createRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		store, err := service.Create(r.Context(), principal, CreateInput{
			TenantID:          request.TenantID,
			Code:              request.Code,
			Name:              request.Name,
			City:              request.City,
			DefaultTemplateID: request.DefaultTemplateID,
			MonthlyGoal:       derefFloat64(request.MonthlyGoal),
			WeeklyGoal:        derefFloat64(request.WeeklyGoal),
			AvgTicketGoal:     derefFloat64(request.AvgTicketGoal),
			ConversionGoal:    derefFloat64(request.ConversionGoal),
			PAGoal:            derefFloat64(request.PAGoal),
			IsActive:          request.IsActive,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, storeResponse{
			Store: store,
		})
	})))

	mux.Handle("PATCH /v1/stores/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		storeID := strings.TrimSpace(r.PathValue("id"))
		if storeID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_store_id", "Loja invalida.")
			return
		}

		var request updateRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		store, err := service.Update(r.Context(), principal, UpdateInput{
			ID:                storeID,
			Name:              request.Name,
			Code:              request.Code,
			City:              request.City,
			DefaultTemplateID: request.DefaultTemplateID,
			MonthlyGoal:       request.MonthlyGoal,
			WeeklyGoal:        request.WeeklyGoal,
			AvgTicketGoal:     request.AvgTicketGoal,
			ConversionGoal:    request.ConversionGoal,
			PAGoal:            request.PAGoal,
			IsActive:          request.IsActive,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, storeResponse{
			Store: store,
		})
	})))

	mux.Handle("POST /v1/stores/{id}/archive", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		storeID := strings.TrimSpace(r.PathValue("id"))
		if storeID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_store_id", "Loja invalida.")
			return
		}

		store, err := service.Archive(r.Context(), principal, storeID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, storeResponse{Store: store})
	})))

	mux.Handle("POST /v1/stores/{id}/restore", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		storeID := strings.TrimSpace(r.PathValue("id"))
		if storeID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_store_id", "Loja invalida.")
			return
		}

		store, err := service.Restore(r.Context(), principal, storeID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, storeResponse{Store: store})
	})))

	mux.Handle("DELETE /v1/stores/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		storeID := strings.TrimSpace(r.PathValue("id"))
		if storeID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_store_id", "Loja invalida.")
			return
		}

		result, err := service.Delete(r.Context(), principal, storeID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, deleteResponse{
			StoreID: result.StoreID,
			Deleted: result.Deleted,
			SavedAt: result.SavedAt.Format(time.RFC3339Nano),
		})
	})))
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var deleteBlocked *DeleteBlockedError

	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrTenantForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrValidation), errors.Is(err, ErrTenantRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados da loja.")
	case errors.Is(err, ErrStoreNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "store_not_found", "Loja nao encontrada.")
	case errors.Is(err, ErrStoreConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "store_conflict", "Ja existe uma loja com este codigo no tenant.")
	case errors.As(err, &deleteBlocked):
		httpapi.WriteErrorWithDetails(
			w,
			r,
			http.StatusConflict,
			"store_delete_blocked",
			"Esta loja ainda possui vinculos e nao pode ser removida.",
			map[string]any{
				"dependencies": deleteBlocked.Dependencies,
			},
		)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar a loja.")
	}
}

func readBoolQuery(r *http.Request, key string) bool {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func derefFloat64(value *float64) float64 {
	if value == nil {
		return 0
	}

	return *value
}
