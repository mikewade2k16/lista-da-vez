package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (repository *PostgresRepository) ListAudit(ctx context.Context, accountID, taskID string) ([]AuditEntry, error) {
	sql, args := repository.scopedQuery(accountID, `
		select id, account_id::text, user_id::text, action, resource_type, resource_id,
		       coalesce(before, '{}'::jsonb), coalesce(after, '{}'::jsonb), at
		from tasks.audit_log
		where account_id = $1::uuid and resource_type = 'task' and resource_id = $2
		order by at desc
		limit 200
	`, taskID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]AuditEntry, 0)
	for rows.Next() {
		entry, err := scanAuditEntry(rows.Scan)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (repository *PostgresRepository) InsertAuditEntry(ctx context.Context, entry AuditEntry) error {
	beforeJSON, _ := json.Marshal(entry.Before)
	afterJSON, _ := json.Marshal(entry.After)
	_, err := repository.pool.Exec(ctx, `
		insert into tasks.audit_log (
			account_id, user_id, action, resource_type, resource_id, before, after
		) values ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb, $7::jsonb)
	`, entry.AccountID, entry.UserID, entry.Action, entry.ResourceType, entry.ResourceID, beforeJSON, afterJSON)
	return err
}

func (repository *PostgresRepository) ListActiveTimeEntries(ctx context.Context, access AccessContext) ([]TimeEntry, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select id::text, task_id::text, user_id::text, account_id::text, started_at,
		       paused_at, resumed_at, stopped_at, duration_ms, notes, version,
		       created_at, updated_at
		from tasks.task_time_entries
		where account_id = $1::uuid and stopped_at is null
		  and ($2::boolean = true or user_id = $3::uuid)
		order by started_at desc
	`, access.Has(PermTrackingViewAll), access.UserID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]TimeEntry, 0)
	for rows.Next() {
		entry, err := scanTimeEntry(rows.Scan)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (repository *PostgresRepository) StartTracking(ctx context.Context, accountID, taskID, userID string) (TimeEntry, error) {
	sql, args := repository.scopedQuery(accountID, `
		insert into tasks.task_time_entries (task_id, user_id, account_id, started_at)
		select t.id, $3::uuid, t.account_id, now()
		from tasks.tasks t
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		returning id::text, task_id::text, user_id::text, account_id::text, started_at,
		          paused_at, resumed_at, stopped_at, duration_ms, notes, version,
		          created_at, updated_at
	`, taskID, userID)
	entry, err := scanTimeEntry(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return TimeEntry{}, ErrTaskNotFound
	}
	return entry, err
}

func (repository *PostgresRepository) PauseTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error) {
	return repository.updateTracking(ctx, accountID, taskID, userID, expectedVersion, "pause")
}

func (repository *PostgresRepository) ResumeTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error) {
	return repository.updateTracking(ctx, accountID, taskID, userID, expectedVersion, "resume")
}

func (repository *PostgresRepository) StopTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error) {
	return repository.updateTracking(ctx, accountID, taskID, userID, expectedVersion, "stop")
}

func (repository *PostgresRepository) TrackingMetrics(ctx context.Context, accountID string, input TrackingMetricsInput) (TrackingMetrics, error) {
	var metrics TrackingMetrics
	metrics.ByClient = []TrackingMetricsBucket{}
	metrics.ByUser = []TrackingMetricsBucket{}
	metrics.ByType = []TrackingMetricsBucket{}

	// Total escalar respeita todos os filtros (userId, clientAccountId, from, to).
	totalWhere, totalArgs := trackingMetricsFilter(accountID, input, true)
	totalSQL := `select coalesce(sum(e.duration_ms), 0)::bigint, count(*)::bigint
		from tasks.task_time_entries e
		join tasks.tasks t on t.id = e.task_id
		where ` + totalWhere
	if err := repository.pool.QueryRow(ctx, totalSQL, totalArgs...).Scan(&metrics.TotalDurationMs, &metrics.EntryCount); err != nil {
		return TrackingMetrics{}, err
	}

	// Breakdowns respeitam periodo (+userId), mas NAO o clientAccountId, para listar todos os
	// clientes/usuarios/tipos do periodo num unico round-trip (elimina o N+1 do front).
	where, args := trackingMetricsFilter(accountID, input, false)

	byClient, err := repository.trackingMetricsBuckets(ctx, `
		select coalesce(t.client_account_id::text, ''),
		       coalesce(nullif(trim(a.name), ''), 'Sem cliente'),
		       coalesce(sum(e.duration_ms), 0)::bigint, count(*)::bigint
		from tasks.task_time_entries e
		join tasks.tasks t on t.id = e.task_id
		left join core.accounts a on a.id = t.client_account_id
		where `+where+`
		group by t.client_account_id, a.name
		order by 3 desc`, args)
	if err != nil {
		return TrackingMetrics{}, err
	}
	metrics.ByClient = byClient

	byUser, err := repository.trackingMetricsBuckets(ctx, `
		select e.user_id::text,
		       coalesce(nullif(trim(u.nick), ''), nullif(trim(u.display_name), ''), u.email, e.user_id::text),
		       coalesce(sum(e.duration_ms), 0)::bigint, count(*)::bigint
		from tasks.task_time_entries e
		join tasks.tasks t on t.id = e.task_id
		left join core.users u on u.id = e.user_id
		where `+where+`
		group by e.user_id, u.nick, u.display_name, u.email
		order by 3 desc`, args)
	if err != nil {
		return TrackingMetrics{}, err
	}
	metrics.ByUser = byUser

	byType, err := repository.trackingMetricsBuckets(ctx, `
		select coalesce(nullif(trim(t.ui_metadata->>'type'), ''), 'Sem tipo'),
		       coalesce(nullif(trim(t.ui_metadata->>'type'), ''), 'Sem tipo'),
		       coalesce(sum(e.duration_ms), 0)::bigint, count(*)::bigint
		from tasks.task_time_entries e
		join tasks.tasks t on t.id = e.task_id
		where `+where+`
		group by coalesce(nullif(trim(t.ui_metadata->>'type'), ''), 'Sem tipo')
		order by 3 desc`, args)
	if err != nil {
		return TrackingMetrics{}, err
	}
	metrics.ByType = byType

	return metrics, nil
}

// trackingMetricsFilter monta o WHERE parametrizado compartilhado pelas queries de tracking.
// includeClient controla se o filtro de clientAccountId entra (so no total escalar; nos breakdowns
// ele e omitido para nao colapsar o agrupamento a um unico cliente).
func trackingMetricsFilter(accountID string, input TrackingMetricsInput, includeClient bool) (string, []any) {
	clause := strings.Builder{}
	clause.WriteString("e.account_id = $1::uuid")
	args := []any{accountID}
	position := 2
	if input.UserID != "" {
		fmt.Fprintf(&clause, " and e.user_id = $%d::uuid", position)
		args = append(args, input.UserID)
		position++
	}
	if includeClient && input.ClientAccountID != "" {
		fmt.Fprintf(&clause, " and t.client_account_id = $%d::uuid", position)
		args = append(args, input.ClientAccountID)
		position++
	}
	if input.From != nil {
		fmt.Fprintf(&clause, " and e.started_at >= $%d", position)
		args = append(args, *input.From)
		position++
	}
	if input.To != nil {
		fmt.Fprintf(&clause, " and e.started_at <= $%d", position)
		args = append(args, *input.To)
	}
	return clause.String(), args
}

func (repository *PostgresRepository) trackingMetricsBuckets(ctx context.Context, sql string, args []any) ([]TrackingMetricsBucket, error) {
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buckets := make([]TrackingMetricsBucket, 0)
	for rows.Next() {
		var bucket TrackingMetricsBucket
		if err := rows.Scan(&bucket.Key, &bucket.Label, &bucket.TotalDurationMs, &bucket.EntryCount); err != nil {
			return nil, err
		}
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}

func (repository *PostgresRepository) updateTracking(
	ctx context.Context,
	accountID string,
	taskID string,
	userID string,
	expectedVersion *int,
	action string,
) (TimeEntry, error) {
	var query string
	switch action {
	case "pause":
		query = `
			update tasks.task_time_entries
			   set paused_at = now(),
			       duration_ms = duration_ms + floor(extract(epoch from (now() - coalesce(resumed_at, started_at))) * 1000)::bigint,
			       version = version + 1,
			       updated_at = now()
			 where account_id = $1::uuid and task_id = $2::uuid and user_id = $3::uuid
			   and stopped_at is null and paused_at is null
			   and ($4::integer is null or version = $4)
			returning id::text, task_id::text, user_id::text, account_id::text, started_at,
			          paused_at, resumed_at, stopped_at, duration_ms, notes, version,
			          created_at, updated_at
		`
	case "resume":
		query = `
			update tasks.task_time_entries
			   set resumed_at = now(),
			       paused_at = null,
			       version = version + 1,
			       updated_at = now()
			 where account_id = $1::uuid and task_id = $2::uuid and user_id = $3::uuid
			   and stopped_at is null and paused_at is not null
			   and ($4::integer is null or version = $4)
			returning id::text, task_id::text, user_id::text, account_id::text, started_at,
			          paused_at, resumed_at, stopped_at, duration_ms, notes, version,
			          created_at, updated_at
		`
	case "stop":
		query = `
			update tasks.task_time_entries
			   set stopped_at = now(),
			       duration_ms = case
			           when paused_at is null then duration_ms + floor(extract(epoch from (now() - coalesce(resumed_at, started_at))) * 1000)::bigint
			           else duration_ms
			       end,
			       version = version + 1,
			       updated_at = now()
			 where account_id = $1::uuid and task_id = $2::uuid and user_id = $3::uuid
			   and stopped_at is null
			   and ($4::integer is null or version = $4)
			returning id::text, task_id::text, user_id::text, account_id::text, started_at,
			          paused_at, resumed_at, stopped_at, duration_ms, notes, version,
			          created_at, updated_at
		`
	default:
		return TimeEntry{}, ErrValidation
	}

	entry, err := scanTimeEntry(repository.pool.QueryRow(ctx, query, accountID, taskID, userID, expectedVersion).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return TimeEntry{}, ErrTimeEntryNotFound
	}
	return entry, err
}

func scanTimeEntry(scan func(...any) error) (TimeEntry, error) {
	var entry TimeEntry
	err := scan(&entry.ID, &entry.TaskID, &entry.UserID, &entry.AccountID, &entry.StartedAt, &entry.PausedAt, &entry.ResumedAt, &entry.StoppedAt, &entry.DurationMs, &entry.Notes, &entry.Version, &entry.CreatedAt, &entry.UpdatedAt)
	return entry, err
}

func scanAuditEntry(scan func(...any) error) (AuditEntry, error) {
	var entry AuditEntry
	var beforeRaw, afterRaw []byte
	err := scan(&entry.ID, &entry.AccountID, &entry.UserID, &entry.Action, &entry.ResourceType, &entry.ResourceID, &beforeRaw, &afterRaw, &entry.At)
	if err != nil {
		return AuditEntry{}, err
	}
	entry.Before = decodeMap(beforeRaw)
	entry.After = decodeMap(afterRaw)
	return entry, nil
}
