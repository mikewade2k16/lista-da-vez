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

// maxBodyBytes limita o corpo dos POST (o unico body relevante e o token).
const maxBodyBytes = 1 << 16

// RegisterRoutes monta os endpoints do painel de Meta Ads (/v1/meta-ads*). O
// gating por modulo (account_modules) e aplicado globalmente via path no Chain;
// aqui so exigimos autenticacao. accountID vem do principal/header (X-Account-Id),
// NUNCA do body.
func RegisterRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	mux.Handle("GET /v1/meta-ads/overview", wrap(handleOverview(svc)))
	mux.Handle("POST /v1/meta-ads/connection", wrap(handleConnectionCreate(svc)))
	mux.Handle("DELETE /v1/meta-ads/connection", wrap(handleConnectionDelete(svc)))
	mux.Handle("GET /v1/meta-ads/ad-accounts", wrap(handleAdAccountsList(svc)))
	mux.Handle("POST /v1/meta-ads/sync", wrap(handleSync(svc)))
	mux.Handle("GET /v1/meta-ads/campaigns", wrap(handleCampaignsList(svc)))
	mux.Handle("GET /v1/meta-ads/insights", wrap(handleInsights(svc)))
	registerAssistantRoutes(mux, svc, wrap)
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
//   - falhas da Graph      -> 502 meta_error
//   - resto                -> 500 internal_error
func writeServiceError(w http.ResponseWriter, r *http.Request, err error, internalMsg string) {
	switch {
	case errors.Is(err, ErrNotConnected):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_connected", "Conecte uma conta Meta primeiro.")
	case errors.Is(err, pgx.ErrNoRows):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
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

// accountIDFromContext resolve o accountID do header X-Account-Id ou, na ausencia,
// do TenantID do principal. NUNCA do body.
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
