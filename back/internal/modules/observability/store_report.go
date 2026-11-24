package observability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	platformobs "github.com/mikewade2k16/lista-da-vez/back/internal/platform/observability"
)

// trafficWeek resume uma semana de tráfego.
type trafficWeek struct {
	Calls     int64   // chamadas em obs.http_route_hourly (inclui o tráfego do runner)
	P95Ms     float64 // p95 estimado pelo histograma do mesmo agregado
	LiveTotal int64   // requisições reais (obs.http_requests source = 'live')
	Live5xx   int64
}

type routeWeek struct {
	Calls   int64
	Buckets []int64
}

// routeDelta compara o p95 de uma rota entre duas semanas.
type routeDelta struct {
	Route   string
	PrevP95 float64
	CurP95  float64
	Calls   int64
}

type namedCount struct {
	Name  string
	Count int64
}

type deployWeek struct {
	Count    int
	Failed   int
	MedianMs float64
}

type moduleScore struct {
	Module string
	Score  float64
}

type runNotes struct {
	ID     string
	At     time.Time
	Scores []moduleScore // todos os módulos com nota, da menor para a maior
}

// weeklyData é tudo que o relatório semanal consome (fronteira entre SQL e texto).
type weeklyData struct {
	From, To time.Time // semana atual [From, To); a anterior é [From-7d, From)
	Cur      trafficWeek
	Prev     trafficWeek
	Worse    []routeDelta
	Better   []routeDelta

	AlertsOpened    map[string]int64 // severidade -> abertos na semana
	AlertsOpenNow   int64
	AlertsResolved  int64
	AlertsTopRules  []namedCount
	FindingsOpen    map[string]int64 // severidade -> abertos
	FindingsOverdue int64
	Deploys         deployWeek
	Run             *runNotes
}

const (
	routeMinCalls   = 50   // chamadas mínimas nas duas semanas para comparar uma rota
	routeMinDeltaMs = 50.0 // variação mínima de p95 (ms)
	routeMinChange  = 0.2  // variação mínima relativa
)

// WeeklyReportData lê os números da semana [to-7d, to). today é a data local (AAAA-MM-DD)
// usada para o vencimento dos achados.
func (s *Store) WeeklyReportData(ctx context.Context, to time.Time, today string) (weeklyData, error) {
	from := to.AddDate(0, 0, -7)
	prevFrom := from.AddDate(0, 0, -7)
	data := weeklyData{From: from, To: to, AlertsOpened: map[string]int64{}, FindingsOpen: map[string]int64{}}
	var err error
	if data.Cur, err = s.trafficWeek(ctx, from, to); err != nil {
		return data, err
	}
	if data.Prev, err = s.trafficWeek(ctx, prevFrom, from); err != nil {
		return data, err
	}
	cur, prev, err := s.routeWeeks(ctx, prevFrom, from, to)
	if err != nil {
		return data, err
	}
	data.Worse, data.Better = compareRoutes(cur, prev)
	if err = s.alertsWeek(ctx, &data, from); err != nil {
		return data, err
	}
	if err = s.findingsWeek(ctx, &data, today); err != nil {
		return data, err
	}
	if data.Deploys, err = s.deploysWeek(ctx, from, to); err != nil {
		return data, err
	}
	if data.Run, err = s.lastFullRun(ctx); err != nil {
		return data, err
	}
	return data, nil
}

func (s *Store) trafficWeek(ctx context.Context, from, to time.Time) (trafficWeek, error) {
	var week trafficWeek
	var buckets []int64
	err := s.pool.QueryRow(quiet(ctx), `
		select coalesce(sum(calls), 0)::bigint,
		       coalesce((select array_agg(t.s order by t.o) from (
		          select u.o, sum(u.v)::bigint as s
		          from obs.http_route_hourly h2, unnest(h2.buckets) with ordinality as u(v, o)
		          where h2.bucket >= $1 and h2.bucket < $2 group by u.o) t), '{}'::bigint[])
		from obs.http_route_hourly where bucket >= $1 and bucket < $2`, from, to).Scan(&week.Calls, &buckets)
	if err != nil {
		return week, fmt.Errorf("weekly traffic: %w", err)
	}
	week.P95Ms = platformobs.PercentileFromBuckets(buckets, 0.95)
	err = s.pool.QueryRow(quiet(ctx), `
		select count(*), count(*) filter (where status >= 500)
		from obs.http_requests
		where source = 'live' and occurred_at >= $1 and occurred_at < $2`, from, to).Scan(&week.LiveTotal, &week.Live5xx)
	if err != nil {
		return week, fmt.Errorf("weekly live traffic: %w", err)
	}
	return week, nil
}

// routeWeeks agrega por rota o histograma das duas semanas (14 dias, uma leitura).
func (s *Store) routeWeeks(ctx context.Context, prevFrom, from, to time.Time) (cur, prev map[string]routeWeek, err error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select route, bucket >= $2, calls, buckets
		from obs.http_route_hourly
		where bucket >= $1 and bucket < $3 and route <> 'UNMATCHED'`, prevFrom, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("weekly routes: %w", err)
	}
	defer rows.Close()
	cur, prev = map[string]routeWeek{}, map[string]routeWeek{}
	for rows.Next() {
		var route string
		var current bool
		var calls int64
		var buckets []int64
		if err := rows.Scan(&route, &current, &calls, &buckets); err != nil {
			return nil, nil, fmt.Errorf("weekly routes scan: %w", err)
		}
		target := prev
		if current {
			target = cur
		}
		target[route] = addRouteWeek(target[route], calls, buckets)
	}
	return cur, prev, rows.Err()
}

func addRouteWeek(week routeWeek, calls int64, buckets []int64) routeWeek {
	week.Calls += calls
	if len(buckets) > len(week.Buckets) {
		week.Buckets = append(week.Buckets, make([]int64, len(buckets)-len(week.Buckets))...)
	}
	for index, value := range buckets {
		week.Buckets[index] += value
	}
	return week
}

func (s *Store) alertsWeek(ctx context.Context, data *weeklyData, from time.Time) error {
	rows, err := s.pool.Query(quiet(ctx), `
		select severity, count(*) from obs.alerts where first_seen >= $1 group by severity`, from)
	if err != nil {
		return fmt.Errorf("weekly alerts: %w", err)
	}
	opened, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (namedCount, error) {
		var item namedCount
		return item, row.Scan(&item.Name, &item.Count)
	})
	if err != nil {
		return fmt.Errorf("weekly alerts rows: %w", err)
	}
	for _, item := range opened {
		data.AlertsOpened[item.Name] = item.Count
	}
	if err := s.pool.QueryRow(quiet(ctx), `
		select count(*) filter (where status = 'open'), count(*) filter (where resolved_at >= $1)
		from obs.alerts`, from).Scan(&data.AlertsOpenNow, &data.AlertsResolved); err != nil {
		return fmt.Errorf("weekly alerts totals: %w", err)
	}
	rows, err = s.pool.Query(quiet(ctx), `
		select rule, count(*) from obs.alerts where first_seen >= $1
		group by rule order by count(*) desc, rule limit 5`, from)
	if err != nil {
		return fmt.Errorf("weekly alerts top: %w", err)
	}
	data.AlertsTopRules, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (namedCount, error) {
		var item namedCount
		return item, row.Scan(&item.Name, &item.Count)
	})
	if err != nil {
		return fmt.Errorf("weekly alerts top rows: %w", err)
	}
	return nil
}

func (s *Store) findingsWeek(ctx context.Context, data *weeklyData, today string) error {
	rows, err := s.pool.Query(quiet(ctx), `
		select severity, count(*), count(*) filter (where due_date is not null and due_date < $1::date)
		from obs.findings where status = 'open' group by severity`, today)
	if err != nil {
		return fmt.Errorf("weekly findings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var severity string
		var total, overdue int64
		if err := rows.Scan(&severity, &total, &overdue); err != nil {
			return fmt.Errorf("weekly findings scan: %w", err)
		}
		data.FindingsOpen[severity] = total
		data.FindingsOverdue += overdue
	}
	return rows.Err()
}

func (s *Store) deploysWeek(ctx context.Context, from, to time.Time) (deployWeek, error) {
	rows, err := s.RecentDeploys(ctx, 200)
	if err != nil {
		return deployWeek{}, err
	}
	return summarizeDeploys(rows, from, to), nil
}

// summarizeDeploys conta os deploys do período; falha = qualquer status que não seja
// sucesso nem em andamento. Mediana sobre os sucessos com duração.
func summarizeDeploys(rows []deployRow, from, to time.Time) deployWeek {
	var week deployWeek
	var durations []float64
	for _, row := range rows {
		if row.StartedAt.Before(from) || !row.StartedAt.Before(to) {
			continue
		}
		week.Count++
		switch row.Status {
		case "success":
			if row.DurationMs != nil {
				durations = append(durations, *row.DurationMs)
			}
		case "running":
		default:
			week.Failed++
		}
	}
	week.MedianMs = percentile(durations, 0.5)
	return week
}

// lastFullRun devolve a nota de cada módulo na última execução completa do runner.
func (s *Store) lastFullRun(ctx context.Context) (*runNotes, error) {
	var notes runNotes
	err := s.pool.QueryRow(quiet(ctx), `
		select id, started_at from obs.runs where `+fullRunFilter+`
		order by started_at desc limit 1`).Scan(&notes.ID, &notes.At)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("weekly last run: %w", err)
	}
	modules, err := s.LatestModuleScores(ctx)
	if err != nil {
		return nil, err
	}
	for _, module := range modules {
		name, _ := module["module"].(string)
		if score, ok := module["score"].(float64); ok {
			notes.Scores = append(notes.Scores, moduleScore{Module: name, Score: score})
		}
	}
	return &notes, nil
}

// GetWeeklyState lê weeklyReport.state (vazio se nunca enviou).
func (s *Store) GetWeeklyState(ctx context.Context) (weeklyState, error) {
	var state weeklyState
	raw, found, err := s.GetSetting(ctx, settingsKeyWeeklyState)
	if err != nil || !found {
		return state, err
	}
	_ = json.Unmarshal(raw, &state) // JSON ilegível = nunca enviou
	return state, nil
}

func (s *Store) PutWeeklyState(ctx context.Context, state weeklyState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal weekly state: %w", err)
	}
	return s.PutSetting(ctx, settingsKeyWeeklyState, raw, "sistema")
}
