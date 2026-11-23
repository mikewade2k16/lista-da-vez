// Package observability expõe o painel de observabilidade (somente
// platform_admin) e o coletor de eventos do front. A instrumentação em si vive em
// internal/platform/observability.
package observability

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	platformobs "github.com/mikewade2k16/lista-da-vez/back/internal/platform/observability"
)

var (
	ErrRunnerDisabled = errors.New("runner disabled in production")
	ErrQueueBusy      = errors.New("run queue busy")
)

type Service struct {
	store   *Store
	runtime *platformobs.Runtime
	env     string
	alerts  *Alerts
}

func NewService(store *Store, runtime *platformobs.Runtime, env string) *Service {
	return &Service{store: store, runtime: runtime, env: env}
}

// WithAlerts liga o canal de alertas (e-mail) usado por /health e /alerts/test.
func (s *Service) WithAlerts(alerts *Alerts) *Service {
	s.alerts = alerts
	return s
}

func (s *Service) isProduction() bool {
	return strings.EqualFold(s.env, "production") || strings.EqualFold(s.env, "prod")
}

type EndpointStat struct {
	Route      string  `json:"route"`
	Method     string  `json:"method"`
	Module     string  `json:"module"`
	Calls      int64   `json:"calls"`
	Errors4xx  int64   `json:"errors4xx"`
	Errors5xx  int64   `json:"errors5xx"`
	ErrorRate  float64 `json:"errorRate"`
	AvgMs      float64 `json:"avgMs"`
	P50Ms      float64 `json:"p50Ms"`
	P95Ms      float64 `json:"p95Ms"`
	P99Ms      float64 `json:"p99Ms"`
	MaxMs      float64 `json:"maxMs"`
	AvgQueries float64 `json:"avgQueries"`
	MaxQueries int     `json:"maxQueries"`
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }

func (s *Service) Endpoints(ctx context.Context, days int, module, sortBy string) ([]EndpointStat, error) {
	rows, err := s.store.RouteStats(ctx, days, "")
	if err != nil {
		return nil, err
	}
	items := make([]EndpointStat, 0, len(rows))
	for _, row := range rows {
		stat := toEndpointStat(row)
		if module != "" && stat.Module != module {
			continue
		}
		items = append(items, stat)
	}
	sortEndpoints(items, sortBy)
	return items, nil
}

func toEndpointStat(row routeRow) EndpointStat {
	method, _, _ := strings.Cut(row.Route, " ")
	module := row.Module
	if module == "" {
		module = row.PathModule
	}
	stat := EndpointStat{
		Route: row.Route, Method: method, Module: module, Calls: row.Calls,
		Errors4xx: row.Errors4xx, Errors5xx: row.Errors5xx, MaxMs: round1(row.MaxMS), MaxQueries: row.QueriesMax,
		P50Ms: platformobs.PercentileFromBuckets(row.Buckets, 0.50),
		P95Ms: platformobs.PercentileFromBuckets(row.Buckets, 0.95),
		P99Ms: platformobs.PercentileFromBuckets(row.Buckets, 0.99),
	}
	if row.Calls > 0 {
		stat.ErrorRate = math.Round(float64(row.Errors5xx)/float64(row.Calls)*10000) / 10000
		stat.AvgMs = round1(row.TotalMS / float64(row.Calls))
		stat.AvgQueries = round1(float64(row.QueriesTotal) / float64(row.Calls))
	}
	return stat
}

func sortEndpoints(items []EndpointStat, sortBy string) {
	less := func(a, b EndpointStat) bool { return a.P95Ms > b.P95Ms }
	switch sortBy {
	case "errors":
		less = func(a, b EndpointStat) bool { return a.ErrorRate > b.ErrorRate }
	case "calls":
		less = func(a, b EndpointStat) bool { return a.Calls > b.Calls }
	case "queries":
		less = func(a, b EndpointStat) bool { return a.AvgQueries > b.AvgQueries }
	}
	sort.SliceStable(items, func(i, j int) bool { return less(items[i], items[j]) })
}

func (s *Service) Overview(ctx context.Context, days int) (map[string]any, error) {
	totals, err := s.store.LiveTotals(ctx, days)
	if err != nil {
		return nil, err
	}
	runs, err := s.store.Runs(ctx, 30)
	if err != nil {
		return nil, err
	}
	fullRuns, err := s.store.FullRuns(ctx, 30)
	if err != nil {
		return nil, err
	}
	modules, err := s.store.LatestModuleScores(ctx)
	if err != nil {
		return nil, err
	}
	errorRate := 0.0
	if totals.Requests > 0 {
		errorRate = math.Round(float64(totals.Errors5xx)/float64(totals.Requests)*10000) / 10000
	}
	stats := s.runtime.Stats()
	trend := make([]map[string]any, 0, len(fullRuns))
	for index := len(fullRuns) - 1; index >= 0; index-- {
		run := fullRuns[index]
		trend = append(trend, map[string]any{"runId": run["id"], "startedAt": run["startedAt"], "scores": run["scores"], "coverage": run["coverage"]})
	}
	var latest map[string]any
	if len(runs) > 0 {
		latest = runs[0]
	}
	return map[string]any{
		"live": map[string]any{
			"enabled": stats.Enabled, "sampleRate": stats.SampleRate, "requests": totals.Requests,
			"errorRate": errorRate, "p95Ms": platformobs.PercentileFromBuckets(totals.Buckets, 0.95),
			"droppedEvents": stats.DroppedEvents, "queuedEvents": stats.QueuedEvents,
		},
		"latestRun": latest,
		"modules":   modules,
		"trend":     trend,
	}, nil
}

func (s *Service) Database(ctx context.Context, days int) (map[string]any, error) {
	slow, err := s.store.SlowQueries(ctx, days)
	if err != nil {
		return nil, err
	}
	nPlusOne, err := s.store.NPlusOne(ctx, days)
	if err != nil {
		return nil, err
	}
	top, err := s.store.TopQueries(ctx, days)
	if err != nil {
		return nil, err
	}
	checks, err := s.store.LatestRunResults(ctx, "db")
	if err != nil {
		return nil, err
	}
	return map[string]any{"slow": slow, "nPlusOne": nPlusOne, "top": top, "checks": checks}, nil
}

func (s *Service) Pages(ctx context.Context, days int) (map[string]any, error) {
	live, err := s.store.PageVitals(ctx, days)
	if err != nil {
		return nil, err
	}
	lastRun, err := s.store.LatestRunResults(ctx, "page")
	if err != nil {
		return nil, err
	}
	return map[string]any{"live": live, "lastRun": lastRun}, nil
}

func (s *Service) Errors(ctx context.Context, days int) ([]map[string]any, error) {
	return s.store.ErrorGroups(ctx, days)
}

func (s *Service) Trace(ctx context.Context, correlationID string) (map[string]any, error) {
	return s.store.Trace(ctx, correlationID)
}

func (s *Service) Runs(ctx context.Context, limit int) ([]map[string]any, error) {
	return s.store.Runs(ctx, limit)
}

func (s *Service) Run(ctx context.Context, id string) (map[string]any, error) {
	run, results, err := s.store.Run(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"run": run, "results": results}, nil
}

func (s *Service) RunRequests(ctx context.Context) (map[string]any, error) {
	items, err := s.store.RunRequests(ctx)
	if err != nil {
		return nil, err
	}
	online, err := s.store.WorkerOnline(ctx, 45*time.Second)
	if err != nil {
		return nil, err
	}
	return map[string]any{"canRun": !s.isProduction(), "workerOnline": online, "items": items}, nil
}

func (s *Service) RequestRun(ctx context.Context, requestedBy, module, only string) (int64, error) {
	if s.isProduction() {
		return 0, ErrRunnerDisabled
	}
	pending, err := s.store.PendingRequests(ctx)
	if err != nil {
		return 0, fmt.Errorf("count pending: %w", err)
	}
	if pending >= 5 {
		return 0, ErrQueueBusy
	}
	return s.store.EnqueueRunRequest(ctx, requestedBy, module, only)
}

func (s *Service) Collect(batch platformobs.FrontendBatch, userID, userAgent string) (int, error) {
	return s.runtime.RecordFrontend(batch, userID, userAgent)
}
