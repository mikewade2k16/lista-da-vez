package observability

import (
	"bufio"
	"net"
	"net/http"
	"strings"
	"time"
)

// RouteResolver é satisfeito por *http.ServeMux: Handler(r) devolve o padrão que
// vai atender a requisição sem executá-la (rota de baixa cardinalidade).
type RouteResolver interface {
	Handler(r *http.Request) (http.Handler, string)
}

const unmatchedRoute = "UNMATCHED"

// Middleware mede toda requisição: deve ficar DEPOIS do RequestID (lê o
// X-Request-ID já definido) e ANTES de auth/Recover, que enriquecem o estado.
func (rt *Runtime) Middleware(router RouteResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if rt == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipInstrumentation(r) {
				next.ServeHTTP(w, r)
				return
			}
			startedAt := time.Now()
			state := &requestState{
				correlationID: w.Header().Get("X-Request-ID"),
				route:         resolveRoute(router, r),
				source:        "live",
			}
			if strings.EqualFold(r.Header.Get("X-Obs-Source"), "runner") {
				state.source = "runner"
			}
			writer := &countingWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(writer, r.WithContext(withState(r.Context(), state)))
			rt.finishRequest(r, state, writer, time.Since(startedAt))
		})
	}
}

func skipInstrumentation(r *http.Request) bool {
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return true // conexões longas distorcem latência; medidas só no handshake falho
	}
	return r.URL.Path == "/v1/obs/collect" || r.Method == http.MethodOptions
}

func resolveRoute(router RouteResolver, r *http.Request) string {
	if router == nil {
		return unmatchedRoute
	}
	_, pattern := router.Handler(r)
	if pattern == "" {
		return unmatchedRoute
	}
	if !strings.Contains(pattern, " ") {
		pattern = r.Method + " " + pattern
	}
	return truncate(pattern, 300)
}

func (rt *Runtime) finishRequest(r *http.Request, state *requestState, writer *countingWriter, duration time.Duration) {
	state.mu.Lock()
	event := httpEvent{
		OccurredAt:    time.Now().UTC(),
		CorrelationID: state.correlationID,
		Method:        r.Method,
		Route:         state.route,
		PathModule:    pathModule(r.URL.Path),
		Status:        writer.status,
		DurationMS:    float64(duration.Microseconds()) / 1000,
		ResponseBytes: writer.bytes,
		UserID:        state.userID,
		AccountID:     state.accountID,
		QueryCount:    state.queryCount,
		QueryMS:       float64(state.queryDuration.Microseconds()) / 1000,
		ExternalCount: state.externalCount,
		ErrorCode:     state.errorCode,
		ErrorMessage:  state.errorMessage,
		ErrorStack:    state.errorStack,
		Source:        state.source,
	}
	var suspects []nPlusOneEvent
	for fingerprint, entry := range state.fingerprints {
		if entry.count >= rt.cfg.NPlusOneThreshold {
			suspects = append(suspects, nPlusOneEvent{
				OccurredAt: event.OccurredAt, CorrelationID: state.correlationID, Route: state.route,
				Fingerprint: fingerprint, Statement: entry.statement, Repetitions: entry.count,
			})
		}
	}
	state.mu.Unlock()

	if event.Route == unmatchedRoute && event.Status == http.StatusNotFound {
		event.PathModule = "" // varredura de URL inexistente não polui os módulos
	}
	rt.recorder.aggregateRequest(event)
	for _, suspect := range suspects {
		rt.recorder.enqueue(suspect)
	}
	if rt.shouldSample(event.Status, duration, event.Source) {
		rt.recorder.enqueue(event)
	}
}

// countingWriter conta bytes/status e preserva Flush/Hijack/Unwrap do writer real.
type countingWriter struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func (w *countingWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *countingWriter) Write(body []byte) (int, error) {
	w.wroteHeader = true
	written, err := w.ResponseWriter.Write(body)
	w.bytes += int64(written)
	return written, err
}

func (w *countingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *countingWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *countingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}
