package httpapi

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Middleware func(http.Handler) http.Handler

type contextKey string

const requestIDContextKey contextKey = "request-id"

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *statusRecorder) Flush() {
	flusher, ok := recorder.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}

	flusher.Flush()
}

func (recorder *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := recorder.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}

	return hijacker.Hijack()
}

func (recorder *statusRecorder) Push(target string, options *http.PushOptions) error {
	pusher, ok := recorder.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}

	return pusher.Push(target, options)
}

func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	wrapped := handler

	for index := len(middlewares) - 1; index >= 0; index-- {
		wrapped = middlewares[index](wrapped)
	}

	return wrapped
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-ID", requestID)

		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey).(string)
	return value
}

func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			recorder := &statusRecorder{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			next.ServeHTTP(recorder, r)

			logger.Info(
				"http_request",
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", recorder.status),
				slog.Duration("duration", time.Since(startedAt)),
			)
		})
	}
}

func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				logger.Error(
					"http_panic",
					slog.String("request_id", RequestIDFromContext(r.Context())),
					slog.Any("panic", recovered),
				)

				WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro interno do servidor.")
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func CORS(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" {
				if OriginAllowed(origin, allowedOrigins) {
					if containsWildcardOrigin(allowedOrigins) {
						w.Header().Set("Access-Control-Allow-Origin", "*")
					} else {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Set("Vary", "Origin")
					}
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Requested-With")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func OriginAllowed(origin string, allowedOrigins []string) bool {
	normalizedOrigin := strings.TrimSpace(origin)
	if normalizedOrigin == "" {
		return false
	}

	allowed := make(map[string]struct{}, len(allowedOrigins))
	patterns := make([]string, 0, len(allowedOrigins))
	allowAny := false

	for _, configuredOrigin := range allowedOrigins {
		normalized := strings.TrimSpace(configuredOrigin)
		if normalized == "" {
			continue
		}

		if normalized == "*" {
			allowAny = true
		}

		if strings.Contains(normalized, "*") {
			patterns = append(patterns, normalized)
			continue
		}

		allowed[normalized] = struct{}{}
	}

	if allowAny {
		return true
	}

	if _, ok := allowed[normalizedOrigin]; ok {
		return true
	}

	return matchesOriginPattern(normalizedOrigin, patterns)
}

func containsWildcardOrigin(allowedOrigins []string) bool {
	for _, origin := range allowedOrigins {
		if strings.TrimSpace(origin) == "*" {
			return true
		}
	}

	return false
}

func matchesOriginPattern(origin string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	originScheme := strings.ToLower(strings.TrimSpace(originURL.Scheme))
	originHost := strings.ToLower(strings.TrimSpace(originURL.Hostname()))
	if originScheme == "" || originHost == "" {
		return false
	}

	for _, pattern := range patterns {
		patternURL, err := url.Parse(strings.ReplaceAll(pattern, ":*", ""))
		if err != nil {
			continue
		}

		patternScheme := strings.ToLower(strings.TrimSpace(patternURL.Scheme))
		patternHost := strings.ToLower(strings.TrimSpace(patternURL.Hostname()))
		if patternScheme == "" || patternHost == "" {
			continue
		}

		if originScheme == patternScheme && originHost == patternHost {
			return true
		}
	}

	return false
}

func newRequestID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}

	return hex.EncodeToString(buffer)
}
