package omnichannel

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Proxy PUBLICO do avatar do WhatsApp (F12 C4 — decisao do dono: rota publica). O front poe a URL
// do avatar direto no <img src>, que o navegador carrega SEM token; sob o gate (RequireAuth +
// moduleGatingRules) daria 401 em TODA foto. Por isso vai FORA do gate — precedente /v1/public/*
// (bio/cardapio) — com allowlist ESTRITA dos 4 hosts do WhatsApp + rate-limit por IP + o anti-SSRF
// da F6 (reusado, nao reimplementado). Exposicao real ~ zero: nenhum dado de conta na resposta, e
// quem chama ja tem a URL do CDN (abrir direto no browser da o mesmo). Rota:
//
//	GET /v1/public/omnichannel/avatar?url=<url do CDN do WhatsApp>

// avatarAllowedHosts espelha EXATAMENTE a allowlist do front (useAvatarProxy.ts): divergir = foto
// que o front proxia e o back recusa. Allowlist de host e camada ALEM do anti-SSRF, nao substituta.
var avatarAllowedHosts = map[string]bool{
	"pps.whatsapp.net":       true,
	"mmg.whatsapp.net":       true,
	"mmx.whatsapp.net":       true,
	"lookaside.whatsapp.com": true,
}

const (
	avatarRateLimit  = 600
	avatarRateWindow = time.Minute
	avatarTimeout    = 10 * time.Second
	// avatarUserAgent: UA proprio (o default do Go pode ser barrado por WAF/CDN).
	avatarUserAgent = "OmniAtendimento/1.0 (+avatar-proxy)"
)

// registerAvatarRoutes monta a rota PUBLICA do avatar. Chamada de dentro do modulo
// (handle.RegisterRoutes), na secao publica — como o webhook. Sem middleware de auth.
func registerAvatarRoutes(mux *http.ServeMux, limiter *rateLimiter) {
	mux.HandleFunc("GET /v1/public/omnichannel/avatar", handleAvatar(limiter))
}

func handleAvatar(limiter *rateLimiter) http.HandlerFunc {
	client := ssrfSafeClient(avatarTimeout)
	return func(w http.ResponseWriter, r *http.Request) {
		// Rate-limit por IP (rota publica): protege a banda do proxy aberto (para esses 4 hosts).
		if !limiter.allow("avatar", clientIP(r), avatarRateLimit, avatarRateWindow) {
			httpapi.WriteError(w, r, http.StatusTooManyRequests, "rate_limited",
				"Muitas requisicoes. Tente novamente em instantes.")
			return
		}

		raw := strings.TrimSpace(r.URL.Query().Get("url"))
		if raw == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_url", "Parametro url ausente.")
			return
		}
		u, err := url.Parse(raw)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_url", "URL invalida.")
			return
		}
		if !avatarAllowedHosts[strings.ToLower(u.Hostname())] {
			httpapi.WriteError(w, r, http.StatusForbidden, "host_not_allowed",
				"Host nao permitido para avatar.")
			return
		}
		// Anti-SSRF da F6: 422 esquema invalido, 403 IP interno; revalida o IP no dial e NAO segue
		// redirect (corrige o buraco do legado, que seguia redirect com allowlist so na URL inicial).
		if err := validatePublicURL(r.Context(), raw); err != nil {
			writeAvatarSSRFError(w, r, err)
			return
		}

		// #nosec G704 -- raw passou pela allowlist de host e por validatePublicURL.
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, raw, nil)
		if err != nil {
			avatarNoContent(w)
			return
		}
		req.Header.Set("User-Agent", avatarUserAgent)

		// #nosec G704 -- o transport revalida o IP conectado e bloqueia redirects.
		resp, err := client.Do(req)
		if err != nil {
			avatarNoContent(w)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			avatarNoContent(w)
			return
		}

		ct := resp.Header.Get("Content-Type")
		if i := strings.IndexByte(ct, ';'); i >= 0 { // so o tipo, sem ;charset
			ct = strings.TrimSpace(ct[:i])
		}
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=600")
		_, _ = io.Copy(w, resp.Body)
	}
}

// avatarNoContent: upstream falhou / vazio / != 2xx -> 204 + cache curto. O front trata como
// "sem avatar" e cai nas iniciais (spec C4).
func avatarNoContent(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=120")
	w.WriteHeader(http.StatusNoContent)
}

// writeAvatarSSRFError mapeia o erro do guarda da F6: esquema invalido -> 422, IP/host interno -> 403.
func writeAvatarSSRFError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrSSRFBadScheme):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "bad_scheme", "Protocolo nao permitido.")
	default: // ErrSSRFBlockedHost
		httpapi.WriteError(w, r, http.StatusForbidden, "host_not_allowed", "Destino nao permitido.")
	}
}
