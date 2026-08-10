package planning

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type configurationRequest struct {
	StoreID       string          `json:"storeId"`
	Configuration json.RawMessage `json:"configuration"`
}

type scheduleRequest struct {
	StoreID         string  `json:"storeId"`
	WeekStart       string  `json:"weekStart"`
	TargetMonth     string  `json:"targetMonth"`
	GoalWeek        int     `json:"goalWeek"`
	Status          string  `json:"status"`
	Shifts          []Shift `json:"shifts"`
	ExpectedVersion *int64  `json:"expectedVersion"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/operations/planning", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		result, err := service.Get(r.Context(), principal, GetInput{StoreID: r.URL.Query().Get("storeId"), WeekStart: r.URL.Query().Get("weekStart")})
		if err != nil {
			writeError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
	})))

	mux.Handle("PUT /v1/operations/planning/configuration", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		var request configurationRequest
		if httpapi.ReadJSON(r, &request) != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		result, err := service.SaveConfiguration(r.Context(), principal, SaveConfigurationInput{StoreID: strings.TrimSpace(request.StoreID), Configuration: request.Configuration})
		if err != nil {
			writeError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"configuration": result})
	})))

	mux.Handle("PUT /v1/operations/planning/schedule", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		var request scheduleRequest
		if httpapi.ReadJSON(r, &request) != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		result, err := service.SaveSchedule(r.Context(), principal, SaveScheduleInput{
			StoreID: strings.TrimSpace(request.StoreID), WeekStart: strings.TrimSpace(request.WeekStart),
			TargetMonth: strings.TrimSpace(request.TargetMonth), GoalWeek: request.GoalWeek,
			Status: strings.TrimSpace(request.Status), Shifts: request.Shifts, ExpectedVersion: request.ExpectedVersion,
		})
		if err != nil {
			writeError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"schedule": result})
	})))

	mux.Handle("POST /v1/operations/planning/schedule/generate", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		var request scheduleRequest
		if httpapi.ReadJSON(r, &request) != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		result, err := service.GenerateSchedule(r.Context(), principal, GenerateScheduleInput{
			StoreID: strings.TrimSpace(request.StoreID), WeekStart: strings.TrimSpace(request.WeekStart),
			TargetMonth: strings.TrimSpace(request.TargetMonth), GoalWeek: request.GoalWeek, ExpectedVersion: request.ExpectedVersion,
		})
		if err != nil {
			writeError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"schedule": result})
	})))

	mux.Handle("POST /v1/operations/planning/schedule/reopen", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		var request scheduleRequest
		if httpapi.ReadJSON(r, &request) != nil || request.ExpectedVersion == nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		result, err := service.ReopenSchedule(r.Context(), principal, strings.TrimSpace(request.StoreID), strings.TrimSpace(request.WeekStart), *request.ExpectedVersion)
		if err != nil {
			writeError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"schedule": result})
	})))
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para alterar o planejamento.")
	case errors.Is(err, ErrStoreNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "store_not_found", "Loja nao encontrada.")
	case errors.Is(err, ErrScheduleNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "schedule_not_found", "Escala nao encontrada.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados do planejamento.")
	case errors.Is(err, ErrVersionConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "version_conflict", "A escala foi alterada por outra pessoa. Recarregue os dados.")
	case errors.Is(err, ErrPublished):
		httpapi.WriteError(w, r, http.StatusConflict, "schedule_published", "A escala publicada precisa ser reaberta antes de editar.")
	case errors.Is(err, ErrScheduleRestrictions):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "schedule_restrictions", "Resolva as restricoes obrigatorias antes de publicar.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar o planejamento.")
	}
}
