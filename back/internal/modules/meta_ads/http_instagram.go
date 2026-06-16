package metaads

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// registerBridgeRoutes monta o BRIDGE INTERNO do runner (/internal/meta-ads/*).
//
// Estas rotas NAO passam pelo middleware JWT (sao internas, fora do prefixo
// /v1/meta-ads gateado por modulo): a seguranca e o bearer de servico
// (META_ADS_RUNNER_BRIDGE_TOKEN) + a rede interna. O accountId vem da query e e
// confiavel APENAS porque a rota esta atras do bearer interno — ainda assim
// validamos formato (nao-vazio) e a existencia da conexao (404 not_connected).
func registerBridgeRoutes(mux *http.ServeMux, svc *Service, bridgeToken string) {
	mux.Handle("GET /internal/meta-ads/runner/instagram/accounts", handleBridgeInstagramAccounts(svc, bridgeToken))
	mux.Handle("GET /internal/meta-ads/runner/instagram/media", handleBridgeInstagramMedia(svc, bridgeToken))
}

func handleBridgeInstagramAccounts(svc *Service, bridgeToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !bridgeAuthorized(w, r, bridgeToken) {
			return
		}
		accountID, ok := bridgeAccountID(w, r)
		if !ok {
			return
		}
		accounts, err := svc.InstagramAccounts(r.Context(), accountID)
		if err != nil {
			writeBridgeError(w, err)
			return
		}
		if accounts == nil {
			accounts = []InstagramAccountView{}
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
	}
}

func handleBridgeInstagramMedia(svc *Service, bridgeToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !bridgeAuthorized(w, r, bridgeToken) {
			return
		}
		accountID, ok := bridgeAccountID(w, r)
		if !ok {
			return
		}
		igUserID := strings.TrimSpace(r.URL.Query().Get("igUserId"))
		// limit invalido/ausente vira 0 -> default/clamp no service.
		limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
		media, err := svc.InstagramMedia(r.Context(), accountID, igUserID, limit)
		if err != nil {
			writeBridgeError(w, err)
			return
		}
		if media == nil {
			media = []InstagramMediaView{}
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"media": media})
	}
}

// bridgeAuthorized aplica o guard de bearer de servico do bridge (shape de erro
// FLAT do contrato, distinto do envelope do painel). Env vazia -> 503
// bridge_not_configured; token errado -> 401 unauthorized (comparacao em tempo
// constante). Retorna false (e ja escreveu a resposta) quando barra.
func bridgeAuthorized(w http.ResponseWriter, r *http.Request, bridgeToken string) bool {
	if strings.TrimSpace(bridgeToken) == "" {
		writeBridgeErrorCode(w, http.StatusServiceUnavailable, "bridge_not_configured")
		return false
	}
	if !bridgeBearerEquals(r.Header.Get("Authorization"), bridgeToken) {
		writeBridgeErrorCode(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	return true
}

// bridgeAccountID le e valida o accountId da query (nao-vazio). Ausente -> 400
// missing_account_id.
func bridgeAccountID(w http.ResponseWriter, r *http.Request) (string, bool) {
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if accountID == "" {
		writeBridgeErrorCode(w, http.StatusBadRequest, "missing_account_id")
		return "", false
	}
	return accountID, true
}

// bridgeBearerEquals compara o header Authorization com o token de servico em
// tempo constante (mesmo padrao de bearerEquals do modulo automation).
func bridgeBearerEquals(header, token string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimSpace(header[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

// writeBridgeError mapeia os erros de service para o shape FLAT do bridge:
//   - ErrNotConnected -> 404 not_connected
//   - falha da Graph  -> 502 graph_error + message (SEM token)
//   - resto           -> 500 internal_error
func writeBridgeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotConnected):
		writeBridgeErrorCode(w, http.StatusNotFound, "not_connected")
	case isGraphError(err):
		// err.Error() vem de graphError (meta_client.go): "meta graph: http N:
		// <mensagem da Graph> (code C)". A URL com access_token NUNCA entra nessa
		// string — so a mensagem da Graph e o status. Seguro ecoar ao runner.
		httpapi.WriteJSON(w, http.StatusBadGateway, map[string]string{
			"error":   "graph_error",
			"message": err.Error(),
		})
	default:
		writeBridgeErrorCode(w, http.StatusInternalServerError, "internal_error")
	}
}

// writeBridgeErrorCode escreve o shape FLAT { "error": code } do contrato do
// bridge (distinto do envelope { error: { code, message } } do painel).
func writeBridgeErrorCode(w http.ResponseWriter, status int, code string) {
	httpapi.WriteJSON(w, status, map[string]string{"error": code})
}
