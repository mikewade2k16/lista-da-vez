package site

import (
	"context"
	"fmt"
	"strings"
)

// Analytics agrega site.tracking_events para o dashboard do site. Tudo e
// escopado por account_id e por uma janela de `days` (default 14). As queries
// usam os indices (account_id, received_at), (account_id, event_*) e os de
// session/visitor, entao nao ha varredura completa.
func (r *PostgresTrackingRepository) Analytics(ctx context.Context, filter TrackingAnalyticsFilter) (TrackingAnalyticsView, error) {
	days := filter.Days
	if days < 1 {
		days = 14
	}
	if days > 365 {
		days = 365
	}

	// Base: account + (opcional) source. Windowed adiciona o filtro de janela.
	args := []any{filter.AccountID}
	base := "te.account_id = $1::uuid"
	if source := strings.TrimSpace(filter.Source); source != "" {
		args = append(args, source)
		base += " and (te.source = $2 or te.source_label = $2)"
	}
	args = append(args, days)
	daysIdx := len(args)
	windowed := fmt.Sprintf("%s and te.received_at >= now() - make_interval(days => $%d)", base, daysIdx)

	view := TrackingAnalyticsView{RangeDays: days}

	// 1. Totais (janela) + sub-janelas absolutas (hoje, 7 dias).
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		select
			count(*),
			count(distinct nullif(te.session_id, '')),
			count(distinct nullif(te.visitor_id, '')),
			count(*) filter (where te.event_type = 'page_view' or te.event_name = 'page_view'),
			count(*) filter (where te.received_at::date = current_date),
			count(*) filter (where te.received_at >= now() - interval '7 days')
		from site.tracking_events te
		where %s
	`, windowed), args...).Scan(
		&view.Totals.TotalEvents, &view.Totals.TotalSessions, &view.Totals.TotalVisitors,
		&view.Totals.PageViews, &view.Totals.Today, &view.Totals.Last7Days,
	); err != nil {
		return TrackingAnalyticsView{}, err
	}

	// 2. Dispositivos.
	devices, err := r.aggCount(ctx, fmt.Sprintf(`
		select coalesce(nullif(te.device_type, ''), 'desconhecido'), count(*)
		from site.tracking_events te
		where %s
		group by 1 order by 2 desc, 1
	`, windowed), args)
	if err != nil {
		return TrackingAnalyticsView{}, err
	}
	view.Devices = devices

	// 3. Eventos por tipo (event_name com fallback event_type).
	eventsByType, err := r.aggCount(ctx, fmt.Sprintf(`
		select coalesce(nullif(te.event_name, ''), nullif(te.event_type, ''), 'desconhecido'), count(*)
		from site.tracking_events te
		where %s
		group by 1 order by 2 desc, 1
		limit 20
	`, windowed), args)
	if err != nil {
		return TrackingAnalyticsView{}, err
	}
	view.EventsByType = eventsByType

	// 4. Conversoes: eventos de interacao (fora os passivos), por visitantes unicos.
	conversions, err := r.aggCount(ctx, fmt.Sprintf(`
		select te.event_name, count(distinct nullif(te.visitor_id, ''))
		from site.tracking_events te
		where %s and te.event_name <> ''
		  and lower(te.event_name) not in ('page_view', 'active_time', 'engagement', 'scroll', 'heartbeat')
		group by 1 order by 2 desc, 1
		limit 12
	`, windowed), args)
	if err != nil {
		return TrackingAnalyticsView{}, err
	}
	view.Conversions = make([]TrackingConversion, 0, len(conversions))
	for _, item := range conversions {
		view.Conversions = append(view.Conversions, TrackingConversion{
			Key:   item.Label,
			Label: item.Label,
			Count: item.Count,
		})
	}

	// 5. Acessos por dia (serie continua incluindo dias sem evento).
	view.AccessByDay, err = r.accessByDay(ctx, base, daysIdx, args)
	if err != nil {
		return TrackingAnalyticsView{}, err
	}

	// 6. Origem do trafego (top 10 referrers).
	referrers, err := r.aggCount(ctx, fmt.Sprintf(`
		select coalesce(nullif(te.referrer, ''), '(direto)'), count(*)
		from site.tracking_events te
		where %s
		group by 1 order by 2 desc, 1
		limit 10
	`, windowed), args)
	if err != nil {
		return TrackingAnalyticsView{}, err
	}
	view.TopReferrers = referrers

	// 7. Ultimas visitas.
	view.RecentVisits, err = r.recentVisits(ctx, windowed, args)
	if err != nil {
		return TrackingAnalyticsView{}, err
	}

	return view, nil
}

func (r *PostgresTrackingRepository) aggCount(ctx context.Context, sql string, args []any) ([]TrackingCountItem, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TrackingCountItem, 0)
	for rows.Next() {
		var item TrackingCountItem
		if err := rows.Scan(&item.Label, &item.Count); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresTrackingRepository) accessByDay(ctx context.Context, base string, daysIdx int, args []any) ([]TrackingDailyCount, error) {
	sql := fmt.Sprintf(`
		select to_char(d.day, 'YYYY-MM-DD'), coalesce(c.cnt, 0)
		from generate_series(
			(current_date - make_interval(days => $%d::int - 1))::date,
			current_date,
			interval '1 day'
		) as d(day)
		left join (
			select te.received_at::date as day, count(*) as cnt
			from site.tracking_events te
			where %s and te.received_at >= now() - make_interval(days => $%d)
			group by 1
		) c on c.day = d.day::date
		order by d.day
	`, daysIdx, base, daysIdx)

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TrackingDailyCount, 0)
	for rows.Next() {
		var item TrackingDailyCount
		if err := rows.Scan(&item.Date, &item.Count); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresTrackingRepository) recentVisits(ctx context.Context, windowed string, args []any) ([]TrackingRecentVisit, error) {
	sql := fmt.Sprintf(`
		select te.received_at,
		       coalesce(nullif(te.device_type, ''), '-'),
		       coalesce(nullif(te.ip, ''), '-'),
		       coalesce(nullif(te.referrer, ''), ''),
		       coalesce(nullif(te.page_path, ''), '')
		from site.tracking_events te
		where %s
		order by te.received_at desc
		limit 50
	`, windowed)

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TrackingRecentVisit, 0)
	for rows.Next() {
		var item TrackingRecentVisit
		if err := rows.Scan(&item.ReceivedAt, &item.DeviceType, &item.IP, &item.Referrer, &item.PagePath); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
