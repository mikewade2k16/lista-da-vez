package tools

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Permissoes de cada ferramenta. ".manage" inclui ".view".
const (
	permShortLinksView   = "tools.shortlinks.view"
	permShortLinksManage = "tools.shortlinks.manage"
	permQrView           = "tools.qr.view"
	permQrManage         = "tools.qr.manage"
)

// toolsHandler recebe a conta ja validada e se o principal e platform_admin.
type toolsHandler func(w http.ResponseWriter, r *http.Request, accountID string, isAdmin bool)

// RegisterRoutes monta /v1/tools/* no mux (gating de modulo no Chain). Cada rota
// exige a permissao da ferramenta, resolvida para a conta do X-Account-Id.
// O checker continua no contrato do modulo; a validacao de membership e das
// permissoes efetivas acontece no RequireAuthWithAccount.
func RegisterRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware, _ auth.AccountMemberChecker) {
	wrap := func(permission string, h toolsHandler) http.Handler {
		return requireToolsPermission(middleware, permission, h)
	}

	mux.Handle("GET /v1/tools/short-links", wrap(permShortLinksView, handleListShortLinks(svc)))
	mux.Handle("POST /v1/tools/short-links", wrap(permShortLinksManage, handleCreateShortLink(svc)))
	mux.Handle("PATCH /v1/tools/short-links/{id}", wrap(permShortLinksManage, handleUpdateShortLink(svc)))
	mux.Handle("DELETE /v1/tools/short-links/{id}", wrap(permShortLinksManage, handleDeleteShortLink(svc)))

	mux.Handle("GET /v1/tools/qr-codes", wrap(permQrView, handleListQrCodes(svc)))
	mux.Handle("POST /v1/tools/qr-codes", wrap(permQrManage, handleCreateQrCode(svc)))
	mux.Handle("PATCH /v1/tools/qr-codes/{id}", wrap(permQrManage, handleUpdateQrCode(svc)))
	mux.Handle("DELETE /v1/tools/qr-codes/{id}", wrap(permQrManage, handleDeleteQrCode(svc)))
}

// hasToolsPermission reporta se a lista efetiva concede a permissao pedida
// (".manage" tambem concede o ".view" da mesma ferramenta).
func hasToolsPermission(granted []string, required string) bool {
	manage := ""
	if strings.HasSuffix(required, ".view") {
		manage = strings.TrimSuffix(required, ".view") + ".manage"
	}
	for _, p := range granted {
		if p == required || (manage != "" && p == manage) {
			return true
		}
	}
	return false
}

// requireToolsPermission aplica autenticacao, escopo e permissao:
//   - platform_admin: confia no X-Account-Id (troca de conta); "" = todas as
//     contas (list) / por id em qualquer conta (mutations).
//   - demais: RequireAuthWithAccount valida membership da conta do X-Account-Id
//     e resolve as permissoes efetivas dela; sem a permissao = 403.
func requireToolsPermission(middleware *auth.Middleware, permission string, h toolsHandler) http.Handler {
	member := middleware.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		if !hasToolsPermission(principal.Permissions, permission) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para esta acao.")
			return
		}
		h(w, r, principal.AccountID, false)
	}))
	return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		if principal.Role == auth.RolePlatformAdmin {
			h(w, r, strings.TrimSpace(r.Header.Get("X-Account-Id")), true)
			return
		}
		member.ServeHTTP(w, r)
	}))
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
	case errors.Is(err, ErrInvalidSlug):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_slug", "Informe um slug com letras ou numeros.")
	case errors.Is(err, ErrQrSlugImmutable):
		httpapi.WriteError(w, r, http.StatusConflict, "qr_slug_immutable",
			"O slug do QR nao pode ser alterado: os QR Codes ja impressos parariam de funcionar.")
	case errors.Is(err, ErrSlugChangeNotConfirmed):
		httpapi.WriteError(w, r, http.StatusConflict, "slug_change_requires_confirmation",
			"Trocar o slug faz o link antigo parar de funcionar. Confirme a troca.")
	case errors.Is(err, ErrInvalidColor):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_color", "Cor invalida. Use o formato #rrggbb.")
	case errors.Is(err, ErrLowContrast):
		httpapi.WriteError(w, r, http.StatusBadRequest, "low_contrast",
			"Cores com pouco contraste: o QR precisa ser mais escuro que o fundo para ser lido pela camera.")
	case errors.Is(err, ErrConcurrentChange):
		httpapi.WriteError(w, r, http.StatusConflict, "concurrent_change",
			"Este QR foi alterado por outra pessoa. Recarregue e tente de novo.")
	case isForeignKeyViolation(err):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_account", "Conta invalida.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro interno.")
	}
}

// ============================================================================
// Short links
// ============================================================================

func handleListShortLinks(svc *Service) toolsHandler {
	return func(w http.ResponseWriter, r *http.Request, accountID string, isAdmin bool) {
		items, meta, err := svc.ListShortLinks(r.Context(), accountID, listFilterFromQuery(r))
		if err != nil {
			writeToolsError(w, r, err)
			return
		}
		writeList(w, items, meta)
	}
}

func handleCreateShortLink(svc *Service) toolsHandler {
	return func(w http.ResponseWriter, r *http.Request, accountID string, isAdmin bool) {
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

func handleUpdateShortLink(svc *Service) toolsHandler {
	return func(w http.ResponseWriter, r *http.Request, accountID string, isAdmin bool) {
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

func handleDeleteShortLink(svc *Service) toolsHandler {
	return func(w http.ResponseWriter, r *http.Request, accountID string, isAdmin bool) {
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

func handleListQrCodes(svc *Service) toolsHandler {
	return func(w http.ResponseWriter, r *http.Request, accountID string, isAdmin bool) {
		items, meta, err := svc.ListQrCodes(r.Context(), accountID, listFilterFromQuery(r))
		if err != nil {
			writeToolsError(w, r, err)
			return
		}
		writeList(w, items, meta)
	}
}

func handleCreateQrCode(svc *Service) toolsHandler {
	return func(w http.ResponseWriter, r *http.Request, accountID string, isAdmin bool) {
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

func handleUpdateQrCode(svc *Service) toolsHandler {
	return func(w http.ResponseWriter, r *http.Request, accountID string, isAdmin bool) {
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

func handleDeleteQrCode(svc *Service) toolsHandler {
	return func(w http.ResponseWriter, r *http.Request, accountID string, isAdmin bool) {
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
