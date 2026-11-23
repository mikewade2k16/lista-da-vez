package observability

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Runtime é o ponto único de instrumentação. Um *Runtime nil (instrumentação
// desligada) é válido em todos os métodos: nada é medido nem gravado.
type Runtime struct {
	cfg        Config
	logger     *slog.Logger
	recorder   *recorder
	normalizer *sqlNormalizer
	cancel     context.CancelFunc
	done       chan struct{}
}

// New devolve nil quando OBSERVABILITY_ENABLED=false.
func New(cfg Config, logger *slog.Logger) *Runtime {
	if !cfg.Enabled {
		return nil
	}
	return &Runtime{
		cfg:        cfg,
		logger:     logger,
		recorder:   newRecorder(cfg, logger),
		normalizer: newSQLNormalizer(cfg.MaxStatementLength),
	}
}

func (rt *Runtime) Enabled() bool { return rt != nil }

func (rt *Runtime) Config() Config {
	if rt == nil {
		return Config{}
	}
	return rt.cfg
}

// Start liga o gravador e a manutenção (retenção) usando o pool da aplicação.
func (rt *Runtime) Start(pool *pgxpool.Pool) {
	if rt == nil || pool == nil {
		return
	}
	rt.recorder.pool = pool
	ctx, cancel := context.WithCancel(context.Background())
	rt.cancel = cancel
	rt.done = make(chan struct{})
	go func() {
		defer close(rt.done)
		rt.recorder.run(ctx)
	}()
	go runMaintenance(ctx, pool, rt.cfg, rt.logger)
}

// Stop grava o que estiver pendente (até o timeout do contexto).
func (rt *Runtime) Stop(ctx context.Context) {
	if rt == nil || rt.cancel == nil {
		return
	}
	rt.cancel()
	select {
	case <-rt.done:
	case <-ctx.Done():
	}
}

type Stats struct {
	Enabled       bool    `json:"enabled"`
	SampleRate    float64 `json:"sampleRate"`
	DroppedEvents int64   `json:"droppedEvents"`
	WrittenEvents int64   `json:"writtenEvents"`
	QueuedEvents  int     `json:"queuedEvents"`
}

func (rt *Runtime) Stats() Stats {
	if rt == nil {
		return Stats{}
	}
	return Stats{
		Enabled:       true,
		SampleRate:    rt.cfg.SampleRate,
		DroppedEvents: rt.recorder.dropped.Load(),
		WrittenEvents: rt.recorder.written.Load(),
		QueuedEvents:  len(rt.recorder.events),
	}
}

// QueryTracer devolve o tracer do pgx (nil quando desligado).
func (rt *Runtime) QueryTracer() pgx.QueryTracer {
	if rt == nil {
		return nil
	}
	return &queryTracer{rt: rt}
}

var (
	pathIDSegment = regexp.MustCompile(`/(?:\d+|[0-9a-fA-F-]{32,36}|[A-Za-z0-9_-]{24,})(?:/|$)`)
)

// NormalizePath troca ids numéricos/uuid/hashes por :id e remove a query string.
func NormalizePath(path string) string {
	if index := strings.IndexByte(path, '?'); index >= 0 {
		path = path[:index]
	}
	for i := 0; i < 8; i++ {
		next := pathIDSegment.ReplaceAllStringFunc(path, func(match string) string {
			if strings.HasSuffix(match, "/") {
				return "/:id/"
			}
			return "/:id"
		})
		if next == path {
			break
		}
		path = next
	}
	return truncate(path, 300)
}

// pathModule deduz o módulo pelo prefixo (/v1/<modulo>/...), ignorando segmentos
// genéricos. O painel refina com o inventário quando disponível.
func pathModule(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, part := range parts {
		switch part {
		case "", "v1", "v2", "admin", "public", "me":
			continue
		}
		return truncate(part, 60)
	}
	return ""
}

func (rt *Runtime) shouldSample(status int, duration time.Duration, source string) bool {
	if status >= 500 || duration >= rt.cfg.SlowRequest || source == "runner" {
		return true
	}
	return rt.cfg.SampleRate >= 1 || rand.Float64() < rt.cfg.SampleRate
}

// recordExternal é chamado pelo transporte HTTP instrumentado.
func (rt *Runtime) recordExternal(ctx context.Context, req *http.Request, status int, duration time.Duration, timeout bool, err error) {
	state := stateFrom(ctx)
	event := externalEvent{
		OccurredAt:  time.Now().UTC(),
		Route:       "background",
		Integration: truncate(req.URL.Hostname(), 200),
		Method:      req.Method,
		Path:        NormalizePath(SanitizeURLPath(req.URL.Path, 300)),
		Status:      status,
		DurationMS:  float64(duration.Microseconds()) / 1000,
		Timeout:     timeout,
	}
	if err != nil {
		event.Error = SanitizeText(err.Error(), 500)
	}
	if state != nil {
		state.addExternal()
		event.CorrelationID = state.correlationID
		event.Route = state.routeName()
	}
	rt.recorder.enqueue(event)
}
