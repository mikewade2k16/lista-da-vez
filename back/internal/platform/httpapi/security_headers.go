package httpapi

import "net/http"

// SecurityHeaders adiciona cabecalhos de seguranca padrao a TODAS as respostas
// (P1.10). HSTS so e emitido quando enableHSTS=true (producao/HTTPS): em dev/HTTP
// o HSTS forcaria o browser a so falar HTTPS com o host, quebrando o acesso local.
//
// CSP usa `default-src 'none'` porque a API responde JSON/uploads, nao documentos
// HTML que carregam recursos — `frame-ancestors 'none'` blinda contra clickjacking
// (redundante com X-Frame-Options, mas cobre browsers que so respeitam CSP).
func SecurityHeaders(enableHSTS bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
			if enableHSTS {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}
