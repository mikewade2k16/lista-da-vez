package observability

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// recorder desacopla a requisição da gravação: eventos brutos entram num canal
// limitado (cheio = descarta e conta, nunca bloqueia a requisição) e agregados
// ficam em mapas em memória. Um goroutine grava tudo a cada FlushInterval.
type recorder struct {
	cfg     Config
	logger  *slog.Logger
	pool    *pgxpool.Pool
	events  chan any
	dropped atomic.Int64
	written atomic.Int64

	aggMu  sync.Mutex
	routes map[routeKey]*routeAggregate
	query  map[queryKey]*queryAggregate
}

func newRecorder(cfg Config, logger *slog.Logger) *recorder {
	return &recorder{
		cfg:    cfg,
		logger: logger,
		events: make(chan any, cfg.BufferSize),
		routes: make(map[routeKey]*routeAggregate),
		query:  make(map[queryKey]*queryAggregate),
	}
}

func (r *recorder) enqueue(event any) {
	select {
	case r.events <- event:
	default:
		r.dropped.Add(1)
	}
}

func hourBucket(at time.Time) time.Time { return at.UTC().Truncate(time.Hour) }

func (r *recorder) aggregateRequest(event httpEvent) {
	key := routeKey{Bucket: hourBucket(event.OccurredAt), Route: event.Route}
	r.aggMu.Lock()
	defer r.aggMu.Unlock()
	agg, ok := r.routes[key]
	if !ok {
		if len(r.routes) >= 20000 {
			r.dropped.Add(1)
			return
		}
		agg = &routeAggregate{PathModule: event.PathModule, Buckets: make([]int64, len(LatencyBucketsMS)+1)}
		r.routes[key] = agg
	}
	agg.Calls++
	switch {
	case event.Status >= 500:
		agg.Errors5xx++
	case event.Status >= 400:
		agg.Errors4xx++
	}
	agg.TotalMS += event.DurationMS
	agg.MaxMS = max(agg.MaxMS, event.DurationMS)
	agg.QueriesTotal += int64(event.QueryCount)
	agg.QueriesMax = max(agg.QueriesMax, event.QueryCount)
	agg.Buckets[bucketIndex(event.DurationMS)]++
}

func (r *recorder) aggregateQuery(at time.Time, route string, sql normalizedSQL, durationMS float64, failed bool) {
	key := queryKey{Bucket: hourBucket(at), Fingerprint: sql.Fingerprint, Route: route}
	r.aggMu.Lock()
	defer r.aggMu.Unlock()
	agg, ok := r.query[key]
	if !ok {
		if len(r.query) >= 50000 {
			r.dropped.Add(1)
			return
		}
		agg = &queryAggregate{Statement: sql.Statement, Verb: sql.Verb}
		r.query[key] = agg
	}
	agg.Calls++
	if failed {
		agg.Errors++
	}
	agg.TotalMS += durationMS
	agg.MaxMS = max(agg.MaxMS, durationMS)
}

func (r *recorder) takeAggregates() (map[routeKey]*routeAggregate, map[queryKey]*queryAggregate) {
	r.aggMu.Lock()
	defer r.aggMu.Unlock()
	routes, queries := r.routes, r.query
	r.routes = make(map[routeKey]*routeAggregate)
	r.query = make(map[queryKey]*queryAggregate)
	return routes, queries
}

// run grava em lote até ctx ser cancelado; no fim faz um último flush.
func (r *recorder) run(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.FlushInterval)
	defer ticker.Stop()
	batch := newEventBatch()
	for {
		select {
		case <-ctx.Done():
			r.drain(batch)
			r.flush(batch)
			return
		case event := <-r.events:
			batch.add(event)
			if batch.size() >= 1000 {
				r.flush(batch)
				batch = newEventBatch()
			}
		case <-ticker.C:
			r.flush(batch)
			batch = newEventBatch()
		}
	}
}

func (r *recorder) drain(batch *eventBatch) {
	for {
		select {
		case event := <-r.events:
			batch.add(event)
		default:
			return
		}
	}
}

func (r *recorder) flush(batch *eventBatch) {
	if r.pool == nil {
		return
	}
	routes, queries := r.takeAggregates()
	if batch.size() == 0 && len(routes) == 0 && len(queries) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(WithoutTracing(context.Background()), 20*time.Second)
	defer cancel()
	written, err := writeBatch(ctx, r.pool, batch, routes, queries)
	if err != nil {
		r.dropped.Add(int64(batch.size()))
		r.logger.Warn("observability_flush_failed", slog.String("error", SanitizeText(err.Error(), 500)))
		return
	}
	r.written.Add(int64(written))
}

type eventBatch struct {
	requests  []httpEvent
	slow      []slowQueryEvent
	nPlusOne  []nPlusOneEvent
	externals []externalEvent
	frontend  []frontendEvent
	presence  map[string]presenceEvent
}

func newEventBatch() *eventBatch { return &eventBatch{} }

func (b *eventBatch) add(event any) {
	switch typed := event.(type) {
	case httpEvent:
		b.requests = append(b.requests, typed)
	case slowQueryEvent:
		b.slow = append(b.slow, typed)
	case nPlusOneEvent:
		b.nPlusOne = append(b.nPlusOne, typed)
	case externalEvent:
		b.externals = append(b.externals, typed)
	case frontendEvent:
		b.frontend = append(b.frontend, typed)
	case presenceEvent:
		b.addPresence(typed)
	}
}

func (b *eventBatch) size() int {
	return len(b.requests) + len(b.slow) + len(b.nPlusOne) + len(b.externals) + len(b.frontend) + len(b.presence)
}
