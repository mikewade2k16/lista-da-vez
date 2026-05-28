package roadmap

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type HTTPHandler struct {
	service *Service
}

type modulesResponse struct {
	Modules []ModuleRecord `json:"modules"`
}

type moduleResponse struct {
	Module ModuleRecord `json:"module"`
}

type rulesResponse struct {
	Rules []Rule `json:"rules"`
}

type ruleResponse struct {
	Rule Rule `json:"rule"`
}

type upsertModuleRequest struct {
	SourceID    string   `json:"sourceId"`
	Label       string   `json:"label"`
	Route       string   `json:"route"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Scope       []string `json:"scope"`
	DependsOn   []string `json:"dependsOn"`
	SortOrder   int      `json:"sortOrder"`
}

type upsertRuleRequest struct {
	SourceID    string `json:"sourceId"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Why         string `json:"why"`
	AppliesWhen string `json:"appliesWhen"`
	SortOrder   int    `json:"sortOrder"`
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	NewHTTPHandler(service).RegisterRoutes(mux, middleware)
}

func (handler *HTTPHandler) RegisterRoutes(mux *http.ServeMux, middleware *auth.Middleware) {
	mux.Handle("GET /v1/roadmap/modules", middleware.RequireAuth(handler.withPermission(PermRoadmapView, handler.listModules)))
	mux.Handle("POST /v1/roadmap/modules", middleware.RequireAuth(handler.withPermission(PermRoadmapManage, handler.createModule)))
	mux.Handle("PUT /v1/roadmap/modules/{id}", middleware.RequireAuth(handler.withPermission(PermRoadmapManage, handler.updateModule)))
	mux.Handle("DELETE /v1/roadmap/modules/{id}", middleware.RequireAuth(handler.withPermission(PermRoadmapManage, handler.deleteModule)))

	mux.Handle("GET /v1/roadmap/rules", middleware.RequireAuth(handler.withPermission(PermRoadmapView, handler.listRules)))
	mux.Handle("POST /v1/roadmap/rules", middleware.RequireAuth(handler.withPermission(PermRoadmapManage, handler.createRule)))
	mux.Handle("PUT /v1/roadmap/rules/{id}", middleware.RequireAuth(handler.withPermission(PermRoadmapManage, handler.updateRule)))
	mux.Handle("DELETE /v1/roadmap/rules/{id}", middleware.RequireAuth(handler.withPermission(PermRoadmapManage, handler.deleteRule)))

	mux.Handle("GET /v1/roadmap/rules.md", middleware.RequireAuth(handler.withPermission(PermRoadmapView, handler.exportMarkdown)))
	mux.Handle("GET /v1/roadmap/dashboard", middleware.RequireAuth(handler.withPermission(PermRoadmapView, handler.dashboard)))
}

type roadmapHTTPContext struct {
	Access AccessContext
}

func (handler *HTTPHandler) withPermission(permission string, next func(http.ResponseWriter, *http.Request, roadmapHTTPContext)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		accountID := strings.TrimSpace(r.Header.Get("X-Account-Id"))
		if accountID == "" {
			accountID = strings.TrimSpace(r.URL.Query().Get("accountId"))
		}
		if accountID == "" {
			accountID = strings.TrimSpace(principal.TenantID)
		}

		access, err := handler.service.ResolveAccessContext(r.Context(), principal, accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if !access.Has(permission) {
			writeServiceError(w, r, ErrForbidden)
			return
		}

		next(w, r, roadmapHTTPContext{Access: access})
	})
}

func (handler *HTTPHandler) listModules(w http.ResponseWriter, r *http.Request, ctx roadmapHTTPContext) {
	modules, err := handler.service.ListModules(r.Context(), ctx.Access)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, modulesResponse{Modules: modules})
}

func (handler *HTTPHandler) createModule(w http.ResponseWriter, r *http.Request, ctx roadmapHTTPContext) {
	var req upsertModuleRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	m, err := handler.service.CreateOrUpsertModule(r.Context(), ctx.Access, moduleInputFromRequest(req))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, moduleResponse{Module: *m})
}

func (handler *HTTPHandler) updateModule(w http.ResponseWriter, r *http.Request, ctx roadmapHTTPContext) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Id obrigatorio.")
		return
	}
	var req upsertModuleRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	m, err := handler.service.UpdateModule(r.Context(), ctx.Access, id, moduleInputFromRequest(req))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, moduleResponse{Module: *m})
}

func (handler *HTTPHandler) deleteModule(w http.ResponseWriter, r *http.Request, ctx roadmapHTTPContext) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Id obrigatorio.")
		return
	}
	if err := handler.service.DeleteModule(r.Context(), ctx.Access, id); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *HTTPHandler) listRules(w http.ResponseWriter, r *http.Request, ctx roadmapHTTPContext) {
	rules, err := handler.service.ListRules(r.Context(), ctx.Access)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, rulesResponse{Rules: rules})
}

func (handler *HTTPHandler) createRule(w http.ResponseWriter, r *http.Request, ctx roadmapHTTPContext) {
	var req upsertRuleRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	rule, err := handler.service.CreateOrUpsertRule(r.Context(), ctx.Access, ruleInputFromRequest(req))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, ruleResponse{Rule: *rule})
}

func (handler *HTTPHandler) updateRule(w http.ResponseWriter, r *http.Request, ctx roadmapHTTPContext) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Id obrigatorio.")
		return
	}
	var req upsertRuleRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	rule, err := handler.service.UpdateRule(r.Context(), ctx.Access, id, ruleInputFromRequest(req))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, ruleResponse{Rule: *rule})
}

func (handler *HTTPHandler) deleteRule(w http.ResponseWriter, r *http.Request, ctx roadmapHTTPContext) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Id obrigatorio.")
		return
	}
	if err := handler.service.DeleteRule(r.Context(), ctx.Access, id); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type dashboardResponse struct {
	Modules []DashboardModule `json:"modules"`
}

func (handler *HTTPHandler) dashboard(w http.ResponseWriter, r *http.Request, ctx roadmapHTTPContext) {
	items, err := handler.service.Dashboard(r.Context(), ctx.Access)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, dashboardResponse{Modules: items})
}

func (handler *HTTPHandler) exportMarkdown(w http.ResponseWriter, r *http.Request, ctx roadmapHTTPContext) {
	content, err := handler.service.ExportRulesMarkdown(r.Context(), ctx.Access)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", "inline; filename=AGENT_RULES.md")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

func moduleInputFromRequest(req upsertModuleRequest) UpsertModuleInput {
	scope := req.Scope
	if scope == nil {
		scope = []string{}
	}
	depends := req.DependsOn
	if depends == nil {
		depends = []string{}
	}
	return UpsertModuleInput{
		SourceID:    req.SourceID,
		Label:       req.Label,
		Route:       req.Route,
		Status:      req.Status,
		Priority:    req.Priority,
		Category:    req.Category,
		Description: req.Description,
		Scope:       scope,
		DependsOn:   depends,
		SortOrder:   req.SortOrder,
	}
}

func ruleInputFromRequest(req upsertRuleRequest) UpsertRuleInput {
	return UpsertRuleInput(req)
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrCannotDeleteGlobal):
		httpapi.WriteError(w, r, http.StatusForbidden, "cannot_delete_global", "Registros globais nao podem ser apagados; apenas editados (vira override por account).")
	case errors.Is(err, ErrInvalid), errors.Is(err, ErrAccountRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados enviados.")
	case errors.Is(err, ErrAccountNotFound), errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar roadmap.")
	}
}
