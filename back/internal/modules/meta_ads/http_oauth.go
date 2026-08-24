package metaads

import (
	"errors"
	"html"
	"io"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterOAuthRoutes separa o inicio autenticado do callback publico. O
// callback fica sob /v1/public para nao cair no module gate nem exigir JWT; a
// posse do state imprevisivel, persistido apenas como hash, autentica o fluxo.
func RegisterOAuthRoutes(
	mux *http.ServeMux,
	svc *OAuthService,
	middleware *auth.Middleware,
) {
	start := middleware.RequireAuthWithAccount(
		requireMetaAdsPermission("meta_ads.connect", handleOAuthStart(svc)),
	)
	mux.Handle("POST /v1/meta-ads/oauth/start", start)
	mux.Handle("GET /v1/public/meta-ads/oauth/callback", handleOAuthCallback(svc))
}

func handleOAuthStart(svc *OAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || strings.TrimSpace(principal.AccountID) == "" || strings.TrimSpace(principal.UserID) == "" {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		result, err := svc.Start(r.Context(), principal.AccountID, principal.UserID)
		if err != nil {
			writeOAuthStartError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
	}
}

func handleOAuthCallback(svc *OAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		err := svc.Complete(
			r.Context(),
			query.Get("state"),
			query.Get("code"),
			strings.TrimSpace(query.Get("error")) != "",
		)
		if err != nil {
			writeOAuthCallbackError(w, err)
			return
		}
		writeOAuthCallbackPage(
			w,
			http.StatusOK,
			"Conexao concluida",
			"A conta Meta foi conectada ao Omni. Esta janela pode ser fechada.",
		)
	}
}

func writeOAuthStartError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrOAuthNotConfigured):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "oauth_not_configured",
			"Facebook Login nao configurado no servidor. Use a conexao manual ou configure o app Meta.")
	case errors.Is(err, ErrOAuthInvalidConfig):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "oauth_invalid_config",
			"A URL de callback do Facebook Login esta invalida.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error",
			"Nao foi possivel iniciar o Facebook Login.")
	}
}

func writeOAuthCallbackError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrOAuthNotConfigured), errors.Is(err, ErrOAuthInvalidConfig), errors.Is(err, ErrCryptoKeyMissing):
		writeOAuthCallbackPage(w, http.StatusServiceUnavailable, "Conexao indisponivel",
			"O Facebook Login nao esta configurado corretamente no servidor.")
	case errors.Is(err, ErrOAuthInvalidState):
		writeOAuthCallbackPage(w, http.StatusBadRequest, "Link expirado",
			"Este login expirou ou ja foi usado. Volte ao Omni e inicie uma nova conexao.")
	case errors.Is(err, ErrOAuthDenied):
		writeOAuthCallbackPage(w, http.StatusBadRequest, "Autorizacao cancelada",
			"Nenhuma conexao foi criada. Volte ao Omni quando quiser tentar novamente.")
	case errors.Is(err, ErrOAuthPermissions):
		writeOAuthCallbackPage(w, http.StatusForbidden, "Permissoes necessarias nao concedidas",
			"Nenhuma conexao foi criada. Volte ao Omni e autorize todas as permissoes solicitadas.")
	case isGraphError(err):
		writeOAuthCallbackPage(w, http.StatusBadGateway, "Falha na autorizacao",
			"A Meta nao concluiu a troca segura. Volte ao Omni e tente novamente.")
	default:
		writeOAuthCallbackPage(w, http.StatusInternalServerError, "Falha na conexao",
			"Nao foi possivel concluir a conexao. Volte ao Omni e inicie novamente.")
	}
}

// writeOAuthCallbackPage escreve somente texto constante. Nenhum code, state,
// token, account id ou mensagem crua da Meta e refletido no browser.
func writeOAuthCallbackPage(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	escapedTitle := html.EscapeString(title)
	escapedMessage := html.EscapeString(message)
	_, _ = io.WriteString(w, `<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>`+escapedTitle+`</title><style>body{font-family:system-ui,sans-serif;max-width:36rem;margin:4rem auto;padding:0 1.5rem;line-height:1.5}h1{font-size:1.5rem}</style></head><body><h1>`+escapedTitle+`</h1><p>`+escapedMessage+`</p></body></html>`)
}
