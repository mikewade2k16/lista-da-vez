package bio

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterPublicRoutes monta o endpoint publico GET /v1/public/bio/{slug}, sem
// JWT e fora do gating por modulo (prefixo /v1/public, diferente de /v1/bio).
// O front bio consome server-to-server. Se publicToken nao for vazio, exige
// Authorization: Bearer <token>.
func RegisterPublicRoutes(mux *http.ServeMux, svc *Service, publicToken string, broker *sseBroker) {
	mux.Handle("GET /v1/public/bio/{slug}", handlePublicBio(svc, strings.TrimSpace(publicToken)))
	// Stream SSE de tempo real: o front da bio escuta e refetcha ao receber
	// `updated`. Sem auth/gating (prefixo /v1/public); CORS vem do middleware.
	mux.Handle("GET /v1/public/bio/{slug}/stream", handleStream(broker))
}

func handlePublicBio(svc *Service, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token != "" && !bearerEquals(r.Header.Get("Authorization"), token) {
			// Token configurado e nao bateu: 404 uniforme (nao vaza existencia).
			writePublicNotFound(w, r)
			return
		}
		resolved, err := svc.Public(r.Context(), r.PathValue("slug"))
		if err != nil {
			writePublicNotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.WriteHeader(http.StatusOK)
		// Resposta de API em application/json (nao HTML); JSON da bio publicada.
		_, _ = w.Write(resolved) //nolint:gosec // G705: resposta JSON da API, nao HTML
	}
}

func writePublicNotFound(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Bio nao encontrada.")
}

// bearerEquals compara o header Authorization com o token de servico em tempo
// constante (anti timing attack).
func bearerEquals(header, token string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimSpace(header[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}
