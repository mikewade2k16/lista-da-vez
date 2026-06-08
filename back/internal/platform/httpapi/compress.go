package httpapi

import (
	"bufio"
	"compress/gzip"
	"net"
	"net/http"
	"strings"
	"sync"
)

var gzipWriterPool = sync.Pool{
	New: func() any { return gzip.NewWriter(nil) },
}

// Gzip comprime respostas quando o cliente aceita gzip (P1.16). Pula:
//   - requests sem `Accept-Encoding: gzip`;
//   - upgrades de WebSocket (precisam de Hijack na conexao crua);
//   - respostas ja codificadas, de tipo nao-comprimivel (imagens/binarios) ou
//     sem corpo (204/304/1xx) — decidido no WriteHeader pelo Content-Type.
//
// O writer preserva Flush (SSE) e Hijack (WebSocket) por seguranca.
func Gzip() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWebSocketUpgrade(r) || !acceptsGzip(r) {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Add("Vary", "Accept-Encoding")

			gz := gzipWriterPool.Get().(*gzip.Writer)
			gz.Reset(w)
			gw := &gzipResponseWriter{ResponseWriter: w, gz: gz, compress: true}
			defer func() {
				if gw.wroteHeader && gw.compress {
					_ = gz.Close()
				}
				gz.Reset(nil)
				gzipWriterPool.Put(gz)
			}()

			next.ServeHTTP(gw, r)
		})
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
	compress    bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	h := w.Header()
	// Nao comprime se ja codificado, tipo nao-comprimivel, ou resposta sem corpo.
	if h.Get("Content-Encoding") != "" || !compressibleType(h.Get("Content-Type")) ||
		status == http.StatusNoContent || status == http.StatusNotModified || status < http.StatusOK {
		w.compress = false
	}

	if w.compress {
		h.Del("Content-Length") // tamanho muda apos comprimir
		h.Set("Content-Encoding", "gzip")
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		// Handler escreveu sem WriteHeader explicito: define Content-Type por
		// sniff (como o net/http faz) antes de decidir compressao.
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(b))
		}
		w.WriteHeader(http.StatusOK)
	}

	if w.compress {
		return w.gz.Write(b)
	}

	return w.ResponseWriter.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if w.compress && w.wroteHeader {
		_ = w.gz.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.EqualFold(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]), "gzip") {
			return true
		}
	}
	return false
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// compressibleType retorna true para content-types que valem a pena comprimir
// (texto/JSON/XML/SVG). Binarios ja comprimidos (imagens, video, zip) ficam fora.
func compressibleType(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}

	if strings.HasPrefix(ct, "text/") {
		return true
	}

	switch ct {
	case "application/json",
		"application/ld+json",
		"application/x-ndjson",
		"application/javascript",
		"application/xml",
		"application/rss+xml",
		"image/svg+xml":
		return true
	default:
		return false
	}
}
