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
	wrap := func(permission string, h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireFinancePermission(permission, h))
	}

	mux.Handle("GET /v1/finance/sheets", wrap("finance.sheets.view", handleListSheets(svc)))
	mux.Handle("POST /v1/finance/sheets", wrap("finance.sheets.manage", handleCreateSheet(svc)))
	mux.Handle("GET /v1/finance/sheets/{id}", wrap("finance.sheets.view", handleGetSheet(svc)))
	mux.Handle("PUT /v1/finance/sheets/{id}", wrap("finance.sheets.manage", handleUpdateSheet(svc)))
	mux.Handle("DELETE /v1/finance/sheets/{id}", wrap("finance.sheets.manage", handleDeleteSheet(svc)))
	mux.Handle("PATCH /v1/finance/sheets/{id}/status", wrap("finance.sheets.manage", handleSheetStatus(svc)))
	mux.Handle("PATCH /v1/finance/sheets/{id}/lines/{lineId}", wrap("finance.sheets.manage", handlePatchLine(svc)))
	mux.Handle("GET /v1/finance/config", wrap("finance.config.view", handleGetConfig(svc)))
	mux.Handle("PUT /v1/finance/config", wrap("finance.config.manage", handleSaveConfig(svc)))
	mux.Handle("GET /v1/finance/config/recurring-clients", wrap("finance.config.view", handleRecurringClients(svc)))
}

// accountIDFromContext resolve a account do request (X-Account-Id ou TenantID do JWT).
func accountIDFromContext(r *http.Request) (string, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	return principal.AccountID, ok && principal.AccountID != ""
}

func requireFinancePermission(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		if !canFinance(principal, permission) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para esta operacao financeira.")
			return
		}
		next(w, r)
	}
}

func canFinance(principal auth.Principal, permission string) bool {
	if principal.Role == auth.RolePlatformAdmin {
		return true
	}
	if !principal.PermissionsResolved {
		return false
	}
	for _, granted := range principal.Permissions {
		if strings.TrimSpace(granted) == permission || (principal.Role == auth.RoleOwner && strings.TrimSpace(granted) == "core.account.manage") {
			return true
		}
	}
	return false
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
	case errors.Is(err, ErrSheetClosed):
		httpapi.WriteError(w, r, http.StatusConflict, "sheet_closed", "Planilha fechada. Reabra antes de editar.")
	case errors.Is(err, ErrSheetConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "sheet_conflict", "A planilha foi alterada. Recarregue antes de salvar.")
	case errors.Is(err, ErrInvalidStatus):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_status", "Status de planilha invalido.")
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
		principal, _ := auth.PrincipalFromContext(r.Context())
		if !canFinance(principal, "finance.recurring.manage") {
			current, err := svc.GetConfig(r.Context(), accountID, strVal(in.CoreTenantID))
			if err != nil {
				writeFinanceError(w, r, err)
				return
			}
			if !sameRecurring(current.RecurringEntries, in.RecurringEntries) {
				httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para alterar recorrencias.")
				return
			}
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

func handleSheetStatus(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			noAccount(w, r)
			return
		}
		var in struct {
			Status            string `json:"status"`
			ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
		}
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		d, err := svc.ChangeStatus(r.Context(), accountID, r.PathValue("id"), in.Status, in.ExpectedUpdatedAt)
		if err != nil {
			writeFinanceError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, d)
	}
}

// CanFinancePermission exposes the canonical Finance permission rule to adapters.
func CanFinancePermission(principal auth.Principal, permission string) bool {
	return canFinance(principal, permission)
}
