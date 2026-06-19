package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// gatewayCookieName e o cookie de sessao lido pelo gate SSO. Diferente do Bearer
// token do SPA (localStorage): este cookie e enviado pelo NAVEGADOR ao abrir
// direto n8n./waha.crowvisuals.com.br, permitindo que o Caddy (forward_auth)
// pergunte a API se a sessao do Omni e valida. Ver docs/automation/SSO_GATEWAY_PLAN.md.
const gatewayCookieName = "omni_gw"

// GatewayConfig parametriza o cookie de sessao e o destino de login do gate SSO.
type GatewayConfig struct {
	// CookieDomain e o dominio do cookie. Vazio = host-only (dev). Prod:
	// ".crowvisuals.com.br" para o cookie valer em omni./n8n./waha.
	CookieDomain string
	// LoginURL e a base do painel para onde redirecionar quem nao tem sessao
	// (ex.: https://omni.crowvisuals.com.br). Vazio = responde 401 (dev/curl).
	LoginURL string
}

// SetGatewayCookie grava o cookie de sessao do gate apos um login bem-sucedido.
func SetGatewayCookie(w http.ResponseWriter, cfg GatewayConfig, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     gatewayCookieName,
		Value:    token,
		Path:     "/",
		Domain:   cfg.CookieDomain,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearGatewayCookie expira o cookie de sessao do gate (logout).
func ClearGatewayCookie(w http.ResponseWriter, cfg GatewayConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     gatewayCookieName,
		Value:    "",
		Path:     "/",
		Domain:   cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// handleGatewayVerify e o endpoint consumido pelo Caddy via forward_auth.
//   - 200: sessao valida E papel platform_admin -> Caddy libera o upstream.
//   - 302: sem sessao -> redireciona pro login do Omni (ou 401 se LoginURL vazio).
//   - 403: logado, mas nao e platform_admin.
func handleGatewayVerify(service *Service, cfg GatewayConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if c, err := r.Cookie(gatewayCookieName); err == nil {
			token = strings.TrimSpace(c.Value)
		}
		// Fallback para teste via curl (sem navegador/cookie).
		if token == "" {
			if t, err := ExtractBearerToken(r.Header.Get("Authorization")); err == nil {
				token = t
			}
		}
		if token == "" {
			gatewayRedirectOrUnauthorized(w, r, cfg)
			return
		}

		principal, err := service.AuthenticateToken(r.Context(), token)
		if err != nil {
			gatewayRedirectOrUnauthorized(w, r, cfg)
			return
		}

		if principal.Role != RolePlatformAdmin {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden",
				"Acesso restrito a administradores da plataforma.")
			return
		}

		w.Header().Set("X-Gateway-User", principal.Email)
		w.WriteHeader(http.StatusOK)
	}
}

// gatewayRedirectOrUnauthorized manda o navegador pro login do Omni. Apos logar,
// o cookie omni_gw passa a existir e reabrir o subdominio (n8n./waha.) libera.
// Nao passa ?redirect= externo: o login do painel so aceita caminhos internos
// (login.vue exige redirect que comeca com "/"); bounce-back cross-subdominio
// fica como melhoria futura no front.
func gatewayRedirectOrUnauthorized(w http.ResponseWriter, r *http.Request, cfg GatewayConfig) {
	loginURL := strings.TrimSpace(cfg.LoginURL)
	if loginURL == "" {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Sessao do Omni obrigatoria.")
		return
	}

	http.Redirect(w, r, strings.TrimRight(loginURL, "/")+"/auth/login", http.StatusFound)
}
