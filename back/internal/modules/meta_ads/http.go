package metaads

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// maxBodyBytes limita os corpos pequenos de mutacao do modulo.
const maxBodyBytes = 1 << 16

// RegisterRoutes monta os endpoints do painel de Meta Ads (/v1/meta-ads*). O
// gating por modulo (account_modules) e aplicado globalmente via path no Chain;
// aqui validamos membership e a permissao efetiva da account. accountID vem do
// Principal hidratado por RequireAuthWithAccount, NUNCA diretamente do header/body.
func RegisterRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(permission string, h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireMetaAdsPermission(permission, h))
	}

	mux.Handle("GET /v1/meta-ads/overview", wrap("meta_ads.view", handleOverview(svc)))
	mux.Handle("POST /v1/meta-ads/connection", wrap("meta_ads.connect", handleConnectionCreate(svc)))
	mux.Handle("DELETE /v1/meta-ads/connection", wrap("meta_ads.connect", handleConnectionDelete(svc)))
	mux.Handle("GET /v1/meta-ads/ad-accounts", wrap("meta_ads.view", handleAdAccountsList(svc)))
	mux.Handle("PATCH /v1/meta-ads/ad-accounts/{id}/client", wrap("meta_ads.manage", handleAdAccountClientUpdate(svc)))
	mux.Handle("POST /v1/meta-ads/sync", wrap("meta_ads.manage", handleSync(svc)))
	mux.Handle("GET /v1/meta-ads/campaigns", wrap("meta_ads.view", handleCampaignsList(svc)))
	mux.Handle("GET /v1/meta-ads/insights", wrap("meta_ads.view", handleInsights(svc)))
	registerInstagramIdentityRoutes(mux, svc, wrap)
	registerActionRoutes(mux, svc, wrap)
}

func handleAdAccountClientUpdate(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			ClientAccountID string `json:"clientAccountId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.SetAdAccountClient(r.Context(), accountID, r.PathValue("id"), body.ClientAccountID)
		if errors.Is(err, ErrInvalidClientAccount) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_client_account", "Conta de anuncio ou cliente invalido.")
			return
		}
		if err != nil {
			writeServiceError(w, r, err, "Falha ao vincular a conta de anuncio ao cliente.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// requireMetaAdsPermission consome a RBAC efetiva que RequireAuthWithAccount
// resolveu para a account ativa. Lista resolvida e vazia e fail-closed. Os papeis
// administrativos preservam o bypass global ja usado pelos demais modulos.
func requireMetaAdsPermission(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || strings.TrimSpace(principal.AccountID) == "" {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		if principal.Role != auth.RolePlatformAdmin && principal.Role != auth.RoleOwner &&
			(!principal.PermissionsResolved || !containsMetaAdsPermission(principal.Permissions, permission)) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para esta acao no Meta Ads.")
			return
		}
		next(w, r)
	}
}

func containsMetaAdsPermission(permissions []string, wanted string) bool {
	for _, permission := range permissions {
		if strings.TrimSpace(permission) == wanted {
			return true
		}
	}
	return false
}

func handleOverview(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		adAccountID := strings.TrimSpace(r.URL.Query().Get("adAccountId"))
		view, err := svc.Overview(r.Context(), accountID, adAccountID)
		if err != nil {
			writeServiceError(w, r, err, "Falha ao carregar o panorama.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleConnectionCreate(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if strings.TrimSpace(body.Token) == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_token", "Informe o token da Meta.")
			return
		}
		view, err := svc.SaveConnection(r.Context(), accountID, body.Token)
		if err != nil {
			if errors.Is(err, ErrCryptoKeyMissing) {
				httpapi.WriteError(w, r, http.StatusServiceUnavailable, "crypto_not_configured",
					"META_ADS_CRYPTO_KEY nao configurado no servidor.")
				return
			}
			writeServiceError(w, r, err, "Nao foi possivel conectar a conta Meta.")
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleConnectionDelete(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		if err := svc.DeleteConnection(r.Context(), accountID); err != nil {
			writeServiceError(w, r, err, "Falha ao remover a conexao.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleAdAccountsList(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		views, err := svc.ListAdAccounts(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err, "Falha ao listar contas de anuncio.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, views)
	}
}

// ============================================================================
// Helpers compartilhados
// ============================================================================

// writeServiceError mapeia erros de service para HTTP:
//   - ErrNotConnected      -> 404 not_connected
//   - pgx.ErrNoRows        -> 404 not_found (recurso de outra account/inexistente)
//   - ErrConnectionChanged -> 409 connection_changed (snapshot deve ser refeito)
//   - ErrOAuthPermissions  -> 422 missing_permissions (sem detalhes do grant)
//   - falhas da Graph      -> 502 meta_error
//   - resto                -> 500 internal_error
func writeServiceError(w http.ResponseWriter, r *http.Request, err error, internalMsg string) {
	switch {
	case errors.Is(err, ErrNotConnected):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_connected", "Conecte uma conta Meta primeiro.")
	case errors.Is(err, pgx.ErrNoRows):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrConnectionChanged):
		httpapi.WriteError(w, r, http.StatusConflict, "connection_changed", "A conexao Meta mudou durante a operacao. Tente novamente.")
	case errors.Is(err, ErrOAuthPermissions):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "missing_permissions",
			"O token nao concedeu todas as permissoes obrigatorias do Meta Ads.")
	case isGraphError(err):
		httpapi.WriteError(w, r, http.StatusBadGateway, "meta_error", "A Meta recusou a requisicao. Verifique o token e as permissoes.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", internalMsg)
	}
}

// isGraphError detecta erros vindos da Graph API (mapeados em meta_client.go).
func isGraphError(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "meta graph:")
}

func writeNoAccount(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
}

// accountIDFromContext devolve somente a account que RequireAuthWithAccount ja
// validou e gravou no Principal. Nao rele o X-Account-Id cru: isso impediria que
// um handler futuro, por engano, voltasse a confiar num header nao validado.
func accountIDFromContext(r *http.Request) (string, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return "", false
	}
	accountID := strings.TrimSpace(principal.AccountID)
	return accountID, accountID != ""
}
