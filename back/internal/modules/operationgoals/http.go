package operationgoals

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type goalResponse struct {
	Goal GoalTargetView `json:"goal"`
}

type createRequest struct {
	StoreID        string  `json:"storeId"`
	ConsultantID   string  `json:"consultantId"`
	Month          string  `json:"month"`
	Week           int     `json:"week"`
	MonthlyGoal    float64 `json:"monthlyGoal"`
	AvgTicketGoal  float64 `json:"avgTicketGoal"`
	ConversionGoal float64 `json:"conversionGoal"`
	PAGoal         float64 `json:"paGoal"`
}

type updateRequest struct {
	MonthlyGoal    any `json:"monthlyGoal"`
	AvgTicketGoal  any `json:"avgTicketGoal"`
	ConversionGoal any `json:"conversionGoal"`
	PAGoal         any `json:"paGoal"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/operations/goals", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		response, err := service.List(r.Context(), principal, ListInput{
			TenantID: strings.TrimSpace(r.URL.Query().Get("tenantId")),
			StoreID:  strings.TrimSpace(r.URL.Query().Get("storeId")),
			Month:    strings.TrimSpace(r.URL.Query().Get("month")),
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "csv") {
			writeCSVResponse(w, response)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	})))

	mux.Handle("POST /v1/operations/goals", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		goal, err := service.Create(r.Context(), principal, CreateInput(request))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, goalResponse{Goal: goal})
	})))

	mux.Handle("PATCH /v1/operations/goals/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request updateRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		monthlyGoal, err := parseOptionalNumber(request.MonthlyGoal)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Campo monthlyGoal invalido.")
			return
		}

		avgTicketGoal, err := parseOptionalNumber(request.AvgTicketGoal)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Campo avgTicketGoal invalido.")
			return
		}

		conversionGoal, err := parseOptionalNumber(request.ConversionGoal)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Campo conversionGoal invalido.")
			return
		}

		paGoal, err := parseOptionalNumber(request.PAGoal)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Campo paGoal invalido.")
			return
		}

		goal, err := service.Update(r.Context(), principal, UpdateInput{
			ID:             strings.TrimSpace(r.PathValue("id")),
			MonthlyGoal:    monthlyGoal,
			AvgTicketGoal:  avgTicketGoal,
			ConversionGoal: conversionGoal,
			PAGoal:         paGoal,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, goalResponse{Goal: goal})
	})))

	mux.Handle("DELETE /v1/operations/goals/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		if err := service.Delete(r.Context(), principal, strings.TrimSpace(r.PathValue("id"))); err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})))
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrStoreRequired), errors.Is(err, ErrValidation), errors.Is(err, ErrTenantRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados da meta.")
	case errors.Is(err, ErrStoreNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "store_not_found", "Loja nao encontrada.")
	case errors.Is(err, ErrConsultantNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "consultant_not_found", "Consultor nao encontrado.")
	case errors.Is(err, ErrGoalNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "goal_not_found", "Meta nao encontrada.")
	case errors.Is(err, ErrGoalConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "goal_conflict", "Ja existe uma meta cadastrada para esse mes e escopo.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar as metas.")
	}
}

func writeCSVResponse(w http.ResponseWriter, response GoalTargetListView) {
	filename := fmt.Sprintf("metas-operacao-%s.csv", response.Month)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(buildCSV(response)))
}

func buildCSV(response GoalTargetListView) string {
	lines := []string{"Periodo;Escopo;Loja;Codigo loja;Consultor;Meta total;Ticket medio;Conversao;PA;Atualizado em"}
	for _, item := range response.Items {
		lines = append(lines, strings.Join([]string{
			escapeCSVCell(item.Month),
			escapeCSVCell(item.Scope),
			escapeCSVCell(item.StoreName),
			escapeCSVCell(item.StoreCode),
			escapeCSVCell(item.ConsultantName),
			escapeCSVCell(formatCSVNumber(item.MonthlyGoal)),
			escapeCSVCell(formatCSVNumber(item.AvgTicketGoal)),
			escapeCSVCell(formatCSVNumber(item.ConversionGoal)),
			escapeCSVCell(formatCSVNumber(item.PAGoal)),
			escapeCSVCell(item.UpdatedAt),
		}, ";"))
	}

	return "\uFEFF" + strings.Join(lines, "\n")
}

func escapeCSVCell(value string) string {
	if strings.ContainsAny(value, ";\n\"") {
		return "\"" + strings.ReplaceAll(value, "\"", "\"\"") + "\""
	}

	return value
}

func formatCSVNumber(value float64) string {
	return strings.TrimSpace(fmt.Sprintf("%.2f", value))
}

func parseOptionalNumber(raw any) (*float64, error) {
	if raw == nil {
		return nil, nil
	}

	switch value := raw.(type) {
	case float64:
		return &value, nil
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, nil
		}

		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return nil, err
		}

		return &parsed, nil
	default:
		return nil, fmt.Errorf("unsupported number type %T", raw)
	}
}
