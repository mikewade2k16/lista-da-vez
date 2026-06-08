package site

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterAdminRoutes monta /v1/admin/leads*, /v1/admin/products*,
// /v1/admin/tracking-events e /v1/admin/webhook-sources* no mux.
// AccountID vem do principal (X-Account-Id).
func RegisterAdminRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	// Leads
	mux.Handle("GET /v1/admin/leads", wrap(handleListLeads(svc)))
	mux.Handle("POST /v1/admin/leads", wrap(handleCreateLead(svc)))
	mux.Handle("GET /v1/admin/leads/{id}", wrap(handleGetLead(svc)))
	mux.Handle("PATCH /v1/admin/leads/{id}", wrap(handleUpdateLead(svc)))
	mux.Handle("DELETE /v1/admin/leads/{id}", wrap(handleDeleteLead(svc)))

	// Products
	mux.Handle("GET /v1/admin/products", wrap(handleListProducts(svc)))
	mux.Handle("POST /v1/admin/products", wrap(handleCreateProduct(svc)))
	mux.Handle("GET /v1/admin/products/{id}", wrap(handleGetProduct(svc)))
	mux.Handle("PATCH /v1/admin/products/{id}", wrap(handleUpdateProduct(svc)))
	mux.Handle("DELETE /v1/admin/products/{id}", wrap(handleDeleteProduct(svc)))

	// Tracking events
	mux.Handle("GET /v1/admin/tracking-events", wrap(handleListTrackingEvents(svc)))
	mux.Handle("GET /v1/admin/tracking-analytics", wrap(handleTrackingAnalytics(svc)))

	// Webhook sources
	mux.Handle("GET /v1/admin/webhook-sources", wrap(handleListSources(svc)))
	mux.Handle("POST /v1/admin/webhook-sources", wrap(handleCreateSource(svc)))
	mux.Handle("POST /v1/admin/webhook-sources/{id}/rotate", wrap(handleRotateSource(svc)))
	mux.Handle("DELETE /v1/admin/webhook-sources/{id}", wrap(handleDeleteSource(svc)))
}

func accountIDFromContext(r *http.Request) (string, bool) {
	if accountID := strings.TrimSpace(r.Header.Get("X-Account-Id")); accountID != "" {
		return accountID, true
	}
	// Fallback: TenantID do JWT para usuarios sem suporte a header ainda.
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return "", false
	}
	return principal.TenantID, principal.TenantID != ""
}

// ============================================================================
// Leads
// ============================================================================

func handleListLeads(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		perPage, _ := strconv.Atoi(q.Get("perPage"))
		filter := LeadListFilter{
			AccountID: accountID,
			Q:         strings.TrimSpace(q.Get("q")),
			Status:    strings.TrimSpace(q.Get("status")),
			SourceID:  strings.TrimSpace(q.Get("sourceId")),
			Page:      page,
			PerPage:   perPage,
		}
		resp, err := svc.ListLeads(r.Context(), filter)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleGetLead(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		view, err := svc.GetLead(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleCreateLead(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		var input LeadCreateInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		view, err := svc.CreateLead(r.Context(), accountID, input)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleUpdateLead(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		var input LeadUpdateInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		view, err := svc.UpdateLead(r.Context(), accountID, r.PathValue("id"), input)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDeleteLead(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		if err := svc.DeleteLead(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeSiteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================================================
// Products
// ============================================================================

func handleListProducts(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		perPage, _ := strconv.Atoi(q.Get("perPage"))
		filter := ProductListFilter{
			AccountID: accountID,
			Q:         strings.TrimSpace(q.Get("q")),
			Status:    strings.TrimSpace(q.Get("status")),
			Category:  strings.TrimSpace(q.Get("category")),
			Page:      page,
			PerPage:   perPage,
		}
		resp, err := svc.ListProducts(r.Context(), filter)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleGetProduct(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		view, err := svc.GetProduct(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleCreateProduct(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		var input ProductCreateInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		view, err := svc.CreateProduct(r.Context(), accountID, input)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleUpdateProduct(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		var input ProductUpdateInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		view, err := svc.UpdateProduct(r.Context(), accountID, r.PathValue("id"), input)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDeleteProduct(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		if err := svc.DeleteProduct(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeSiteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================================================
// Tracking events
// ============================================================================

func handleListTrackingEvents(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		perPage, _ := strconv.Atoi(q.Get("perPage"))
		filter := TrackingEventListFilter{
			AccountID: accountID,
			Q:         strings.TrimSpace(q.Get("q")),
			Source:    strings.TrimSpace(q.Get("source")),
			EventType: strings.TrimSpace(q.Get("eventType")),
			PagePath:  strings.TrimSpace(q.Get("pagePath")),
			Page:      page,
			PerPage:   perPage,
		}
		resp, err := svc.ListTrackingEvents(r.Context(), filter)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleTrackingAnalytics(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		q := r.URL.Query()
		days, _ := strconv.Atoi(q.Get("days"))
		filter := TrackingAnalyticsFilter{
			AccountID: accountID,
			Source:    strings.TrimSpace(q.Get("source")),
			Days:      days,
		}
		resp, err := svc.GetTrackingAnalytics(r.Context(), filter)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

// ============================================================================
// Webhook sources
// ============================================================================

func handleListSources(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		sources, err := svc.ListSources(r.Context(), accountID)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"sources": sources})
	}
}

func handleCreateSource(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		var input WebhookSourceCreateInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		resp, err := svc.CreateSource(r.Context(), accountID, input)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, resp)
	}
}

func handleRotateSource(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		resp, err := svc.RotateSecret(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleDeleteSource(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		if err := svc.DeleteSource(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeSiteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================================================
// Errors
// ============================================================================

func writeSiteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrLeadNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "lead_not_found", "Lead nao encontrado.")
	case errors.Is(err, ErrProductNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "product_not_found", "Produto nao encontrado.")
	case errors.Is(err, ErrSourceNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "source_not_found", "Webhook source nao encontrada.")
	case errors.Is(err, ErrSourceSlugConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "slug_conflict", "Ja existe uma webhook source com este slug.")
	case errors.Is(err, ErrInvalidEntityType):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_entity_type", "entityType deve ser 'leads', 'products' ou 'tracking'.")
	case errors.Is(err, ErrInvalidSignature):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "invalid_signature", "X-Signature ausente ou invalido.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
