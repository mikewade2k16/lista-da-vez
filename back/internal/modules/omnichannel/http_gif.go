package omnichannel

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Rotas de GIF + chave do Tenor (F12 C2/C3/C5), sob /v1/omnichannel/* (gate de modulo no Chain).
//
//   - GET  /gif/search   RequireAuthWithAccount + conversations.reply. Erro e SOFT (HTTP 200 +
//     items:[] + error) — um 4xx/5xx apagaria a mensagem acionavel no catch do front (C2).
//   - GET  /gif/media     RequireAuthWithAccount + conversations.reply. Allowlist de host + guarda
//     anti-SSRF da ssrf.go (F6) + stream do binario. Host fora/URL invalida => 400; upstream => 502.
//   - GET|PUT /gif/settings   RequireAuth + platform_admin (no handler). A chave e GLOBAL da
//     plataforma; platform_admin tem bypass do gate de modulo mesmo sem X-Account-Id.

const (
	// gifSearchDefaultLimit e o default do /gif/search (C2: 1..40 default 24 — NAO o do sticker).
	gifSearchDefaultLimit = 24
	gifSearchMaxLimit     = 40
	// gifMediaTimeout limita o proxy /gif/media (C3). O legado nao tinha timeout aqui.
	gifMediaTimeout = 10 * time.Second
)

// gifMediaHosts e a allowlist de host do proxy /gif/media (C3). Camada EXTRA, alem do guarda
// anti-SSRF da ssrf.go — nao o substitui.
var gifMediaHosts = map[string]bool{
	"media.tenor.com":      true,
	"c.tenor.com":          true,
	"tenor.googleapis.com": true,
	"www.tenor.com":        true,
}

// RegisterGifRoutes monta as rotas de GIF/chave do Tenor. Chamado pelo module.go (costura).
func RegisterGifRoutes(mux *http.ServeMux, svc *GifService, middleware *auth.Middleware) {
	wrapAcct := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuthWithAccount(h) }
	wrap := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuth(h) }

	mux.Handle("GET /v1/omnichannel/gif/search", wrapAcct(handleGifSearch(svc)))
	mux.Handle("GET /v1/omnichannel/gif/media", wrapAcct(handleGifMedia(svc)))
	mux.Handle("GET /v1/omnichannel/gif/settings", wrap(handleGetGifSettings(svc)))
	mux.Handle("PUT /v1/omnichannel/gif/settings", wrap(handlePutGifSettings(svc)))
}

// gifSettingsInput e o body do PUT /gif/settings (C5). provider/baseUrl sao PONTEIROS: ausentes
// mantem o valor atual; presentes (mesmo "") sobrescrevem. apiKey vazio = limpar a chave.
type gifSettingsInput struct {
	APIKey   string  `json:"apiKey"`
	Provider *string `json:"provider"`
	BaseURL  *string `json:"baseUrl"`
}

// ============================================================================
// Busca de GIF (C2)
// ============================================================================

func handleGifSearch(svc *GifService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		if err := svc.ensureReply(r.Context(), accountID, principal); err != nil {
			writeServiceError(w, r, err)
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		limit := parseGifLimit(r.URL.Query().Get("limit"))
		// Soft-error e sucesso saem ambos com HTTP 200: o front le response.error (C2).
		httpapi.WriteJSON(w, http.StatusOK, svc.Search(r.Context(), q, limit))
	}
}

// parseGifLimit implementa min(40, max(1, limit ?? 24)) (C2). Ausente/invalido => default 24.
func parseGifLimit(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return gifSearchDefaultLimit
	}
	if n < 1 {
		return 1
	}
	if n > gifSearchMaxLimit {
		return gifSearchMaxLimit
	}
	return n
}

// ============================================================================
// Proxy de midia do Tenor (C3)
// ============================================================================

func handleGifMedia(svc *GifService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		if err := svc.ensureReply(r.Context(), accountID, principal); err != nil {
			writeServiceError(w, r, err)
			return
		}

		raw := strings.TrimSpace(r.URL.Query().Get("url"))
		target, err := url.Parse(raw)
		if err != nil || raw == "" || (target.Scheme != "http" && target.Scheme != "https") ||
			!gifMediaHosts[strings.ToLower(target.Hostname())] {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_url",
				"URL de GIF invalida ou de host nao permitido.")
			return
		}
		// Guarda anti-SSRF da F6 (IP resolvido, sem redirect): camada ALEM da allowlist de host.
		if err := validatePublicURL(r.Context(), raw); err != nil {
			httpapi.WriteError(w, r, http.StatusBadGateway, "gif_upstream",
				"Nao foi possivel acessar o recurso de GIF.")
			return
		}
		// #nosec G704 -- raw passou pela allowlist de host e por validatePublicURL.
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, raw, nil)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadGateway, "gif_upstream",
				"Nao foi possivel acessar o recurso de GIF.")
			return
		}
		req.Header.Set("User-Agent", tenorUserAgent)
		// #nosec G704 -- o transport revalida o IP conectado e bloqueia redirects.
		resp, err := ssrfSafeClient(gifMediaTimeout).Do(req)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadGateway, "gif_upstream",
				"Nao foi possivel acessar o recurso de GIF.")
			return
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			httpapi.WriteError(w, r, http.StatusBadGateway, "gif_upstream",
				"O provedor de GIF respondeu com erro.")
			return
		}
		if ct := strings.TrimSpace(resp.Header.Get("Content-Type")); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		// Conteudo publico de terceiro (nao e dado de conversa): pode ser cacheado.
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, resp.Body)
	}
}

// ============================================================================
// Chave do Tenor (C5) — so platform_admin
// ============================================================================

func handleGetGifSettings(svc *GifService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGifPlatformAdmin(w, r) {
			return
		}
		status, err := svc.Status(r.Context())
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, status)
	}
}

func handlePutGifSettings(svc *GifService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGifPlatformAdmin(w, r) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		var body gifSettingsInput
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if err := svc.Put(r.Context(), body.APIKey, body.Provider, body.BaseURL, principal.UserID); err != nil {
			writeServiceError(w, r, err)
			return
		}
		status, err := svc.Status(r.Context())
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, status)
	}
}

// requireGifPlatformAdmin garante platform_admin (a chave e global; vale para todas as contas).
// Espelha calendar/http_secrets.go requirePlatformAdmin (nome proprio para nao colidir).
func requireGifPlatformAdmin(w http.ResponseWriter, r *http.Request) bool {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || principal.Role != auth.RolePlatformAdmin {
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Apenas platform_admin.")
		return false
	}
	return true
}
