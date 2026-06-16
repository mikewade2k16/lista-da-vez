package site

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// maxProductImageUploadBytes limita o corpo do upload de imagem de produto.
const maxProductImageUploadBytes int64 = 15 << 20 // 15 MB

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
	mux.Handle("GET /v1/admin/products/source", wrap(handleGetProductSource(svc)))
	mux.Handle("PATCH /v1/admin/products/source", wrap(handleSetProductSource(svc)))
	mux.Handle("POST /v1/admin/products/sync", wrap(handleSyncProducts(svc)))
	mux.Handle("POST /v1/admin/products/erp-match", wrap(handleErpMatch(svc)))
	mux.Handle("GET /v1/admin/products/erp-unmatched", wrap(handleErpUnmatched(svc)))
	mux.Handle("POST /v1/admin/products/from-erp", wrap(handleCreateProductFromErp(svc)))
	mux.Handle("GET /v1/admin/products/{id}", wrap(handleGetProduct(svc)))
	mux.Handle("PATCH /v1/admin/products/{id}", wrap(handleUpdateProduct(svc)))
	mux.Handle("POST /v1/admin/products/{id}/image", wrap(handleUploadProductImage(svc)))
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
			Campaign:  strings.TrimSpace(q.Get("campaign")),
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

// handleUploadProductImage recebe um multipart (campo `file`) e troca a imagem do
// produto, salvando localmente em /uploads/site/products/... (sem hotlink).
func handleUploadProductImage(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxProductImageUploadBytes+(1<<20))
		//nolint:gosec // G120: corpo ja limitado pelo MaxBytesReader acima
		if err := r.ParseMultipartForm(maxProductImageUploadBytes); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_upload", "Arquivo invalido ou muito grande.")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "file_required", "Campo 'file' obrigatorio.")
			return
		}
		defer func() { _ = file.Close() }()
		content, err := io.ReadAll(io.LimitReader(file, maxProductImageUploadBytes))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_upload", "Falha ao ler o arquivo.")
			return
		}
		view, err := svc.UploadProductImage(
			r.Context(), accountID, r.PathValue("id"),
			header.Filename, header.Header.Get("Content-Type"), content,
		)
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

// handleSyncProducts dispara a sync das fontes externas da account.
// Aceita ?accountId= opcional; o escopo e validado contra o principal:
// platform_admin pode sincronizar qualquer account; demais papeis so a propria.
func handleSyncProducts(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctxAccountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		accountID, ok := resolveSyncAccountScope(r, ctxAccountID)
		if !ok {
			// Recurso fora do escopo: 404 (nao 403) para nao vazar existencia.
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Account nao encontrada.")
			return
		}
		resp, err := svc.SyncProducts(r.Context(), accountID)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

// resolveSyncAccountScope decide a account-alvo do sync: se vier ?accountId= e
// for diferente do contexto, so platform_admin pode alvejar outra account.
func resolveSyncAccountScope(r *http.Request, ctxAccountID string) (string, bool) {
	requested := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if requested == "" || requested == ctxAccountID {
		return ctxAccountID, true
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if ok && principal.Role == auth.RolePlatformAdmin {
		return requested, true
	}
	return "", false
}

// handleGetProductSource le a fonte external_api da account e devolve o modo
// (local/online/custom) derivado do base_url.
func handleGetProductSource(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		view, err := svc.GetProductSource(r.Context(), accountID)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// handleSetProductSource troca a fonte external_api da account entre local e
// online (base_url conhecido por modo). NAO re-sincroniza.
func handleSetProductSource(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		var input ProductSourceModeInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		view, err := svc.SetProductSourceMode(r.Context(), accountID, input.Mode)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// handleErpMatch cruza os produtos da account com o ERP e materializa os links.
func handleErpMatch(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		resp, err := svc.MatchERP(r.Context(), accountID)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

// handleErpUnmatched lista itens do ERP da account ainda sem produto.
func handleErpUnmatched(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		perPage, _ := strconv.Atoi(q.Get("perPage"))
		filter := ErpUnmatchedFilter{
			AccountID: accountID,
			Q:         strings.TrimSpace(q.Get("q")),
			Page:      page,
			PerPage:   perPage,
		}
		resp, err := svc.ListUnmatchedErp(r.Context(), filter)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

// handleCreateProductFromErp cria um produto a partir de um sku do ERP.
func handleCreateProductFromErp(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
			return
		}
		var input ProductFromErpInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		view, err := svc.CreateProductFromErp(r.Context(), accountID, input)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
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
	case errors.Is(err, ErrNoProductSource):
		httpapi.WriteError(w, r, http.StatusNotFound, "no_product_source", "Nenhuma fonte de produtos configurada para esta account.")
	case errors.Is(err, ErrErpItemNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "erp_item_not_found", "Item nao encontrado no ERP desta account.")
	case errors.Is(err, ErrInvalidProductSourceMode):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_source_mode", "mode deve ser 'local' ou 'online'.")
	case errors.Is(err, ErrProductSyncUnavailable):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "product_sync_unavailable", "Sincronizacao de produtos indisponivel.")
	case errors.Is(err, ErrInvalidSignature):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "invalid_signature", "X-Signature ausente ou invalido.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
