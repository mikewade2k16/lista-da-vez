package alerts

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type listResponse struct {
	Alerts []AlertView `json:"alerts"`
}

type overviewResponse struct {
	Overview Overview `json:"overview"`
}

type alertResponse struct {
	Alert AlertView `json:"alert"`
}

type rulesResponse struct {
	Rules RulesView `json:"rules"`
}

type updateRulesRequest struct {
	LongOpenServiceMinutes   *int  `json:"longOpenServiceMinutes"`
	IdleStoreMinutes         *int  `json:"idleStoreMinutes"`
	AfterClosingGraceMinutes *int  `json:"afterClosingGraceMinutes"`
	NotifyDashboard          *bool `json:"notifyDashboard"`
	NotifyOperationContext   *bool `json:"notifyOperationContext"`
	NotifyExternal           *bool `json:"notifyExternal"`
}

type actionRequest struct {
	Note string `json:"note"`
}

type respondRequest struct {
	Response string `json:"response"`
}

type respondResponse struct {
	Alert           AlertView `json:"alert"`
	OpenFinishModal bool      `json:"openFinishModal"`
	ServiceID       string    `json:"serviceId"`
}

type createRuleRequest struct {
	TenantID               string           `json:"tenantId"`
	Name                   string           `json:"name"`
	Description            string           `json:"description"`
	IsActive               *bool            `json:"isActive"`
	TriggerType            string           `json:"triggerType"`
	ThresholdMinutes       int              `json:"thresholdMinutes"`
	Severity               string           `json:"severity"`
	DisplayKind            string           `json:"displayKind"`
	ColorTheme             string           `json:"colorTheme"`
	TitleTemplate          string           `json:"titleTemplate"`
	BodyTemplate           string           `json:"bodyTemplate"`
	InteractionKind        string           `json:"interactionKind"`
	ResponseOptions        []ResponseOption `json:"responseOptions"`
	IsMandatory            bool             `json:"isMandatory"`
	NotifyDashboard        bool             `json:"notifyDashboard"`
	NotifyOperationContext bool             `json:"notifyOperationContext"`
	NotifyExternal         bool             `json:"notifyExternal"`
	ExternalChannel        string           `json:"externalChannel"`
}

type updateRuleRequest struct {
	Name                   *string          `json:"name"`
	Description            *string          `json:"description"`
	IsActive               *bool            `json:"isActive"`
	TriggerType            *string          `json:"triggerType"`
	ThresholdMinutes       *int             `json:"thresholdMinutes"`
	Severity               *string          `json:"severity"`
	DisplayKind            *string          `json:"displayKind"`
	ColorTheme             *string          `json:"colorTheme"`
	TitleTemplate          *string          `json:"titleTemplate"`
	BodyTemplate           *string          `json:"bodyTemplate"`
	InteractionKind        *string          `json:"interactionKind"`
	ResponseOptions        []ResponseOption `json:"responseOptions"`
	IsMandatory            *bool            `json:"isMandatory"`
	NotifyDashboard        *bool            `json:"notifyDashboard"`
	NotifyOperationContext *bool            `json:"notifyOperationContext"`
	NotifyExternal         *bool            `json:"notifyExternal"`
	ExternalChannel        *string          `json:"externalChannel"`
}

type ruleResponse struct {
	Rule RuleDefinitionView `json:"rule"`
}

type rulesListResponse struct {
	Rules []RuleDefinitionView `json:"rules"`
}

type applyRuleResponse struct {
	AppliedCount int `json:"appliedCount"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/alerts/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		alert, err := service.FindByID(r.Context(), principal, strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, alertResponse{Alert: *alert})
	})))

	mux.Handle("GET /v1/alerts", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		alerts, err := service.List(r.Context(), principal, ListInput{
			TenantID: strings.TrimSpace(r.URL.Query().Get("tenantId")),
			StoreID:  strings.TrimSpace(r.URL.Query().Get("storeId")),
			Status:   strings.TrimSpace(r.URL.Query().Get("status")),
			Type:     strings.TrimSpace(r.URL.Query().Get("type")),
			Category: strings.TrimSpace(r.URL.Query().Get("category")),
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, listResponse{Alerts: alerts})
	})))

	mux.Handle("GET /v1/alerts/overview", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		overview, err := service.Overview(r.Context(), principal, OverviewInput{
			TenantID: strings.TrimSpace(r.URL.Query().Get("tenantId")),
			StoreID:  strings.TrimSpace(r.URL.Query().Get("storeId")),
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, overviewResponse{Overview: overview})
	})))

	mux.Handle("GET /v1/alerts/rules", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		if r.URL.Query().Get("format") == "definitions" {
			rules, err := service.ListRules(r.Context(), principal, ListRulesInput{
				TenantID:    strings.TrimSpace(r.URL.Query().Get("tenantId")),
				TriggerType: strings.TrimSpace(r.URL.Query().Get("triggerType")),
				OnlyActive:  r.URL.Query().Get("onlyActive") == "true",
			})
			if err != nil {
				writeServiceError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, rulesListResponse{Rules: rules})
			return
		}

		rules, err := service.Rules(r.Context(), principal, strings.TrimSpace(r.URL.Query().Get("tenantId")))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, rulesResponse{Rules: rules})
	})))

	mux.Handle("PATCH /v1/alerts/rules", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request updateRulesRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		rules, err := service.UpdateRules(r.Context(), principal, strings.TrimSpace(r.URL.Query().Get("tenantId")), UpdateRulesInput{
			LongOpenServiceMinutes:   request.LongOpenServiceMinutes,
			IdleStoreMinutes:         request.IdleStoreMinutes,
			AfterClosingGraceMinutes: request.AfterClosingGraceMinutes,
			NotifyDashboard:          request.NotifyDashboard,
			NotifyOperationContext:   request.NotifyOperationContext,
			NotifyExternal:           request.NotifyExternal,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, rulesResponse{Rules: rules})
	})))

	mux.Handle("POST /v1/alerts/{id}/acknowledge", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request actionRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		alert, err := service.Acknowledge(r.Context(), principal, strings.TrimSpace(r.PathValue("id")), request.Note)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, alertResponse{Alert: *alert})
	})))

	mux.Handle("POST /v1/alerts/{id}/respond", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request respondRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		result, err := service.RespondToAlert(r.Context(), principal, strings.TrimSpace(r.PathValue("id")), request.Response)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		view := result.Alert.View()
		httpapi.WriteJSON(w, http.StatusOK, respondResponse{
			Alert:           view,
			OpenFinishModal: result.OpenFinishModal,
			ServiceID:       result.ServiceID,
		})
	})))

	mux.Handle("POST /v1/alerts/{id}/resolve", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request actionRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		alert, err := service.Resolve(r.Context(), principal, strings.TrimSpace(r.PathValue("id")), request.Note)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, alertResponse{Alert: *alert})
	})))

	// Rule management endpoints
	mux.Handle("POST /v1/alerts/rules", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request createRuleRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		isActive := true
		if request.IsActive != nil {
			isActive = *request.IsActive
		}
		tenantID := strings.TrimSpace(request.TenantID)
		if tenantID == "" {
			tenantID = strings.TrimSpace(principal.TenantID)
		}

		rule, err := service.CreateRule(r.Context(), principal, CreateRuleInput{
			TenantID:               tenantID,
			Name:                   request.Name,
			Description:            request.Description,
			IsActive:               isActive,
			TriggerType:            request.TriggerType,
			ThresholdMinutes:       request.ThresholdMinutes,
			Severity:               request.Severity,
			DisplayKind:            request.DisplayKind,
			ColorTheme:             request.ColorTheme,
			TitleTemplate:          request.TitleTemplate,
			BodyTemplate:           request.BodyTemplate,
			InteractionKind:        request.InteractionKind,
			ResponseOptions:        request.ResponseOptions,
			IsMandatory:            request.IsMandatory,
			NotifyDashboard:        request.NotifyDashboard,
			NotifyOperationContext: request.NotifyOperationContext,
			NotifyExternal:         request.NotifyExternal,
			ExternalChannel:        request.ExternalChannel,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, ruleResponse{Rule: *rule})
	})))

	mux.Handle("GET /v1/alerts/rules/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		rule, err := service.GetRule(r.Context(), principal, strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ruleResponse{Rule: *rule})
	})))

	mux.Handle("PATCH /v1/alerts/rules/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request updateRuleRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		rule, err := service.UpdateRule(r.Context(), principal, strings.TrimSpace(r.PathValue("id")), UpdateRuleInput{
			Name:                   request.Name,
			Description:            request.Description,
			IsActive:               request.IsActive,
			TriggerType:            request.TriggerType,
			ThresholdMinutes:       request.ThresholdMinutes,
			Severity:               request.Severity,
			DisplayKind:            request.DisplayKind,
			ColorTheme:             request.ColorTheme,
			TitleTemplate:          request.TitleTemplate,
			BodyTemplate:           request.BodyTemplate,
			InteractionKind:        request.InteractionKind,
			ResponseOptions:        request.ResponseOptions,
			IsMandatory:            request.IsMandatory,
			NotifyDashboard:        request.NotifyDashboard,
			NotifyOperationContext: request.NotifyOperationContext,
			NotifyExternal:         request.NotifyExternal,
			ExternalChannel:        request.ExternalChannel,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ruleResponse{Rule: *rule})
	})))

	mux.Handle("DELETE /v1/alerts/rules/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		err := service.DeleteRule(r.Context(), principal, strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusNoContent, nil)
	})))

	mux.Handle("POST /v1/alerts/rules/{id}/apply-now", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		count, err := service.ApplyRuleNow(r.Context(), principal, strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, applyRuleResponse{AppliedCount: count})
	})))
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "alert_not_found", "Alerta nao encontrado.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrTenantRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "tenant_required", "Tenant obrigatorio para este recurso.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Payload invalido para alertas.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar alertas.")
	}
}
