package observability

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// queryTracer implementa pgx.QueryTracer: mede cada consulta, conta por
// requisição (detecção de N+1) e registra as lentas com o SQL normalizado.
type queryTracer struct {
	rt *Runtime
}

type queryStart struct {
	at  time.Time
	sql string
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if tracingSkipped(ctx) {
		return ctx
	}
	return context.WithValue(ctx, queryStartKey, queryStart{at: time.Now(), sql: data.SQL})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	start, ok := ctx.Value(queryStartKey).(queryStart)
	if !ok {
		return
	}
	duration := time.Since(start.at)
	durationMS := float64(duration.Microseconds()) / 1000
	sql := t.rt.normalizer.Normalize(start.sql)
	failed := data.Err != nil && !errors.Is(data.Err, pgx.ErrNoRows)

	route := "background"
	correlationID := ""
	if state := stateFrom(ctx); state != nil {
		state.addQuery(sql.Fingerprint, sql.Statement, duration)
		route = state.routeName()
		correlationID = state.correlationID
	}
	now := time.Now().UTC()
	t.rt.recorder.aggregateQuery(now, route, sql, durationMS, failed)

	if duration >= t.rt.cfg.SlowQuery || failed {
		event := slowQueryEvent{
			OccurredAt: now, CorrelationID: correlationID, Route: route,
			Fingerprint: sql.Fingerprint, Statement: sql.Statement, DurationMS: durationMS,
		}
		if failed {
			event.Error = SanitizeText(data.Err.Error(), 500)
		}
		t.rt.recorder.enqueue(event)
	}
}

// WrapTransport instrumenta chamadas HTTP de saída (integrações externas).
func (rt *Runtime) WrapTransport(base http.RoundTripper) http.RoundTripper {
	if rt == nil {
		return base
	}
	if base == nil {
		base = http.DefaultTransport
	}
	return &tracingTransport{base: base, rt: rt}
}

// InstrumentDefaultTransport cobre todo http.Client que não define Transport próprio.
func (rt *Runtime) InstrumentDefaultTransport() {
	if rt == nil {
		return
	}
	if _, already := http.DefaultTransport.(*tracingTransport); already {
		return
	}
	http.DefaultTransport = rt.WrapTransport(http.DefaultTransport)
}

type tracingTransport struct {
	base http.RoundTripper
	rt   *Runtime
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	startedAt := time.Now()
	resp, err := t.base.RoundTrip(req)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	t.rt.recordExternal(req.Context(), req, status, time.Since(startedAt), isTimeout(err), err)
	return resp, err
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
