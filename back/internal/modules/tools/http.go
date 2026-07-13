package tools

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterRoutes monta /v1/tools/* no mux (RequireAuth; gating de modulo no Chain).
// O checker valida membership da account informada em X-Account-Id para usuarios
// comuns (platform_admin tem bypass e opera cross-conta).
func RegisterRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware, checker auth.AccountMemberChecker) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	mux.Handle("GET /v1/tools/short-links", wrap(handleListShortLinks(svc, checker)))
	mux.Handle("POST /v1/tools/short-links", wrap(handleCreateShortLink(svc, checker)))
	mux.Handle("PATCH /v1/tools/short-links/{id}", wrap(handleUpdateShortLink(svc, checker)))
	mux.Handle("DELETE /v1/tools/short-links/{id}", wrap(handleDeleteShortLink(svc, checker)))

	mux.Handle("GET /v1/tools/qr-codes", wrap(handleListQrCodes(svc, checker)))
	mux.Handle("POST /v1/tools/qr-codes", wrap(handleCreateQrCode(svc, checker)))
	mux.Handle("PATCH /v1/tools/qr-codes/{id}", wrap(handleUpdateQrCode(svc, checker)))
	mux.Handle("DELETE /v1/tools/qr-codes/{id}", wrap(handleDeleteQrCode(svc, checker)))
}

// scopeContext resolve o escopo de conta do request e aplica o isolamento
// multi-tenant NO handler (RequireAuthWithAccount rejeitaria platform_admin):
//   - platform_admin: confia no X-Account-Id (troca de conta); "" = todas as contas.
//   - usuario comum: exige X-Account-Id e valida membership; escopo = essa conta.
//
// Retorna (accountID, isAdmin, ok). Em erro, ja escreveu a resposta (ok=false).
func scopeContext(w http.ResponseWriter, r *http.Request, checker auth.AccountMemberChecker) (string, bool, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		return "", false, false
	}
	accountID := strings.TrimSpace(r.Header.Get("X-Account-Id"))
	if principal.Role == auth.RolePlatformAdmin {
		return accountID, true, true
	}
	if accountID == "" {
		httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
		return "", false, false
	}
	member, err := checker.IsMember(r.Context(), accountID, principal.UserID)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao validar membership.")
		return "", false, false
	}
	if !member {
		httpapi.WriteError(w, r, http.StatusForbidden, "account_not_member", "Sem acesso a esta account.")
		return "", false, false
	}
	return accountID, false, true
}

// listFilterFromQuery le page/limit/q/status da query string.
func listFilterFromQuery(r *http.Request) ListFilter {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	return ListFilter{
		Q:      q.Get("q"),
		Status: q.Get("status"),
		Page:   page,
		Limit:  limit,
	}
}

// createTargetAccount resolve a conta dona de um create.
//   - platform_admin: usa accountId do body (mira qualquer conta) ou o X-Account-Id.
//   - usuario comum: sempre a propria conta validada (accountId do body ignorado).
func createTargetAccount(scopeAccount string, isAdmin bool, bodyAccountID *string) string {
	if isAdmin && bodyAccountID != nil && strings.TrimSpace(*bodyAccountID) != "" {
		return strings.TrimSpace(*bodyAccountID)
	}
	return scopeAccount
}

func writeList(w http.ResponseWriter, data any, meta ListMeta) {
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "data": data, "meta": meta})
}

func writeSuccess(w http.ResponseWriter, status int, data any) {
	httpapi.WriteJSON(w, status, map[string]any{"status": "success", "data": data})
}

// writeToolsError mapeia sentinelas para 404/400; FK invalida vira 400; resto 500.
func writeToolsError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrShortLinkNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "short_link_not_found", "Link curto nao encontrado.")
	case errors.Is(err, ErrQrCodeNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "qr_code_not_found", "QR Code nao encontrado.")
	case errors.Is(err, ErrInvalidTargetURL):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_target_url", "Informe uma URL de destino valida.")
	case errors.Is(err, ErrAccountRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "account_required", "Selecione a conta dona do link.")
	case isForeignKeyViolation(err):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_account", "Conta invalida.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro interno.")
	}
}

// ============================================================================
// Short links
// ============================================================================

func handleListShortLinks(svc *Service, checker auth.AccountMemberChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, ok := scopeContext(w, r, checker)
		if !ok {
			return
		}
		items, meta, err := svc.ListShortLinks(r.Context(), accountID, listFilterFromQuery(r))
		if err != nil {
			writeToolsError(w, r, err)
			return
		}
		writeList(w, items, meta)
	}
}

func handleCreateShortLink(svc *Service, checker auth.AccountMemberChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, isAdmin, ok := scopeContext(w, r, checker)
		if !ok {
			return
		}
		var in ShortLinkInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		target := createTargetAccount(accountID, isAdmin, in.AccountID)
		item, err := svc.CreateShortLink(r.Context(), target, in)
		if err != nil {
			writeToolsError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, item)
	}
}

func handleUpdateShortLink(svc *Service, checker auth.AccountMemberChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, isAdmin, ok := scopeContext(w, r, checker)
		if !ok {
			return
		}
		var in ShortLinkInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		scope := accountID
		if isAdmin {
			scope = "" // admin edita por id (qualquer conta)
		}
		item, err := svc.UpdateShortLink(r.Context(), r.PathValue("id"), scope, in)
		if err != nil {
			writeToolsError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, item)
	}
}

func handleDeleteShortLink(svc *Service, checker auth.AccountMemberChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, isAdmin, ok := scopeContext(w, r, checker)
		if !ok {
			return
		}
		scope := accountID
		if isAdmin {
			scope = "" // admin remove por id (qualquer conta)
		}
		if err := svc.DeleteShortLink(r.Context(), r.PathValue("id"), scope); err != nil {
			writeToolsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "success"})
	}
}

// ============================================================================
// QR codes
// ============================================================================

func handleListQrCodes(svc *Service, checker auth.AccountMemberChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, ok := scopeContext(w, r, checker)
		if !ok {
			return
		}
		items, meta, err := svc.ListQrCodes(r.Context(), accountID, listFilterFromQuery(r))
		if err != nil {
			writeToolsError(w, r, err)
			return
		}
		writeList(w, items, meta)
	}
}

func handleCreateQrCode(svc *Service, checker auth.AccountMemberChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, isAdmin, ok := scopeContext(w, r, checker)
		if !ok {
			return
		}
		var in QrCodeInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		target := createTargetAccount(accountID, isAdmin, in.AccountID)
		item, err := svc.CreateQrCode(r.Context(), target, in)
		if err != nil {
			writeToolsError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, item)
	}
}

func handleUpdateQrCode(svc *Service, checker auth.AccountMemberChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, isAdmin, ok := scopeContext(w, r, checker)
		if !ok {
			return
		}
		var in QrCodeInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		scope := accountID
		if isAdmin {
			scope = "" // admin edita por id (qualquer conta)
		}
		item, err := svc.UpdateQrCode(r.Context(), r.PathValue("id"), scope, in)
		if err != nil {
			writeToolsError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusOK, item)
	}
}

func handleDeleteQrCode(svc *Service, checker auth.AccountMemberChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, isAdmin, ok := scopeContext(w, r, checker)
		if !ok {
			return
		}
		scope := accountID
		if isAdmin {
			scope = ""
		}
		if err := svc.DeleteQrCode(r.Context(), r.PathValue("id"), scope); err != nil {
			writeToolsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "success"})
	}
}
