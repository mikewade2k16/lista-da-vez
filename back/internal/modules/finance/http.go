package finance

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterRoutes monta /v1/finance/* no mux (RequireAuth; gating de modulo no Chain).
func RegisterRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	mux.Handle("GET /v1/finance/sheets", wrap(handleListSheets(svc)))
	mux.Handle("POST /v1/finance/sheets", wrap(handleCreateSheet(svc)))
	mux.Handle("GET /v1/finance/sheets/{id}", wrap(handleGetSheet(svc)))
	mux.Handle("PUT /v1/finance/sheets/{id}", wrap(handleUpdateSheet(svc)))
	mux.Handle("DELETE /v1/finance/sheets/{id}", wrap(handleDeleteSheet(svc)))
	mux.Handle("PATCH /v1/finance/sheets/{id}/lines/{lineId}", wrap(handlePatchLine(svc)))

	mux.Handle("GET /v1/finance/config", wrap(handleGetConfig(svc)))
	mux.Handle("PUT /v1/finance/config", wrap(handleSaveConfig(svc)))
	mux.Handle("GET /v1/finance/config/recurring-clients", wrap(handleRecurringClients(svc)))
}

// accountIDFromContext resolve a account do request (X-Account-Id ou TenantID do JWT).
func accountIDFromContext(r *http.Request) (string, bool) {
	if accountID := strings.TrimSpace(r.Header.Get("X-Account-Id")); accountID != "" {
		return accountID, true
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return "", false
	}
	return principal.TenantID, principal.TenantID != ""
}

// successList e o envelope de listagem.
type successList struct {
	Status string          `json:"status"`
	Data   []SheetListItem `json:"data"`
	Meta   ListMeta        `json:"meta"`
}

// successData e o envelope generico { status, data }.
func writeSuccess(w http.ResponseWriter, status int, data any) {
	httpapi.WriteJSON(w, status, map[string]any{"status": "success", "data": data})
}

// writeFinanceError mapeia sentinelas para 404; resto vira 500.
func writeFinanceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrSheetNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "sheet_not_found", "Planilha nao encontrada.")
	case errors.Is(err, ErrLineNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "line_not_found", "Linha nao encontrada.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro interno.")
	}
}

func noAccount(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
}

// ============================================================================
// Sheets
// ============================================================================

func handleListSheets(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			noAccount(w, r)
			return
		}
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		filter := ListFilter{
			CoreTenantID: q.Get("coreTenantId"),
			Period:       q.Get("period"),
			Q:            q.Get("q"),
			Page:         page,
			Limit:        limit,
		}
		items, meta, err := svc.ListSheets(r.Context(), accountID, filter)
		if err != nil {
			writeFinanceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, successList{Status: "success", Data: items, Meta: meta})
	}
}

func handleGetSheet(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			noAccount(w, r)
			return
		}
		detail, err := svc.GetSheet(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeFinanceError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, detail)
	}
}

func handleCreateSheet(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			noAccount(w, r)
			return
		}
		var in SheetInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		detail, err := svc.CreateSheet(r.Context(), accountID, in)
		if err != nil {
			writeFinanceError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, detail)
	}
}

func handleUpdateSheet(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			noAccount(w, r)
			return
		}
		var in SheetInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		detail, err := svc.UpdateSheet(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeFinanceError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, detail)
	}
}

func handleDeleteSheet(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			noAccount(w, r)
			return
		}
		if err := svc.DeleteSheet(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeFinanceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "success"})
	}
}

func handlePatchLine(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			noAccount(w, r)
			return
		}
		var in LinePatchInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		data, err := svc.PatchLine(r.Context(), accountID, r.PathValue("id"), r.PathValue("lineId"), in)
		if err != nil {
			writeFinanceError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, data)
	}
}

// ============================================================================
// Config
// ============================================================================

func handleGetConfig(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			noAccount(w, r)
			return
		}
		data, err := svc.GetConfig(r.Context(), accountID, r.URL.Query().Get("coreTenantId"))
		if err != nil {
			writeFinanceError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, data)
	}
}

func handleSaveConfig(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			noAccount(w, r)
			return
		}
		var in ConfigInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		data, err := svc.SaveConfig(r.Context(), accountID, in)
		if err != nil {
			writeFinanceError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, data)
	}
}

// handleRecurringClients: so platform_admin recebe a lista real (agencia
// acompanhando mensalidades). Caller comum recebe [] (idem ao mock, zero vazamento).
func handleRecurringClients(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := accountIDFromContext(r); !ok {
			noAccount(w, r)
			return
		}
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || principal.Role != auth.RolePlatformAdmin {
			writeSuccess(w, http.StatusOK, []RecurringClient{})
			return
		}
		data, err := svc.ListRecurringClients(r.Context())
		if err != nil {
			writeFinanceError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, data)
	}
}
