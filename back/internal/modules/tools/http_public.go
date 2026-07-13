package tools

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterPublicRoutes monta os redirects publicos GET /s/{slug} e GET /q/{slug},
// sem JWT e fora do gating por modulo (prefixos /s e /q nao estao em
// moduleGatingRules). A conta dona precisa estar ativa e com o modulo tools
// habilitado (checado na query do store); caso contrario 404 uniforme.
func RegisterPublicRoutes(mux *http.ServeMux, svc *Service) {
	mux.Handle("GET /s/{slug}", handleShortLinkRedirect(svc))
	mux.Handle("GET /q/{slug}", handleQrRedirect(svc))
}

func handleShortLinkRedirect(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target, err := svc.ResolveShortLink(r.Context(), r.PathValue("slug"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Link nao encontrado.")
			return
		}
		redirectTo(w, r, target)
	}
}

func handleQrRedirect(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target, err := svc.ResolveQrCode(r.Context(), r.PathValue("slug"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "QR nao encontrado.")
			return
		}
		redirectTo(w, r, target)
	}
}

// redirectTo emite um 302 sem cache (o destino pode mudar; e um redirect
// contabilizado, nao um recurso cacheavel).
func redirectTo(w http.ResponseWriter, r *http.Request, target string) {
	w.Header().Set("Cache-Control", "no-store")
	// G710 (open redirect) e o PROPRIO proposito de um encurtador: o destino e a
	// URL que o dono cadastrou (normalizada com esquema http/https no service),
	// lida do banco — nao um parametro cru do request.
	http.Redirect(w, r, target, http.StatusFound) //nolint:gosec // G710: destino cadastrado pelo dono do link
}
