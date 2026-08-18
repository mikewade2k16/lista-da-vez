package contentoperations

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Load(ctx context.Context, scope Scope, from, to time.Time) ([]TaskSnapshot, []EventSnapshot, error)
}

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Load(ctx context.Context, scope Scope, from, to time.Time) ([]TaskSnapshot, []EventSnapshot, error) {
	clientIDs := make([]string, 0, len(scope.Clients))
	for _, client := range scope.Clients {
		if id := strings.TrimSpace(client.ID); id != "" {
			clientIDs = append(clientIDs, id)
		}
	}
	if len(clientIDs) == 0 {
		return []TaskSnapshot{}, []EventSnapshot{}, nil
	}

	rows, err := r.pool.Query(ctx, `
		select t.id::text, coalesce(t.client_account_id::text, nullif(t.ui_metadata->>'clientId',''), ''),
		       t.title, t.updated_at, coalesce(t.ui_metadata->'checklist', '[]'::jsonb)
		from tasks.tasks t
		where t.account_id = $1::uuid and t.archived = false
		  and coalesce(t.client_account_id::text, nullif(t.ui_metadata->>'clientId',''), '') = any($2::text[])
		order by t.updated_at desc`, scope.StorageAccountID, clientIDs)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	tasks := make([]TaskSnapshot, 0)
	for rows.Next() {
		var task TaskSnapshot
		var raw []byte
		if err := rows.Scan(&task.ID, &task.ClientID, &task.Title, &task.Updated, &raw); err != nil {
			return nil, nil, err
		}
		_ = json.Unmarshal(raw, &task.Items)
		if task.Items == nil {
			task.Items = []ChecklistItem{}
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	eventRows, err := r.pool.Query(ctx, `
		select e.id::text, e.client_id::text, e.event_date, e.type, e.title, e.status,
		       coalesce((select tr.task_id::text from tasks.task_relations tr join tasks.tasks t on t.id=tr.task_id
		         where tr.module='calendar' and tr.resource_type='event' and tr.resource_id=e.id::text
		           and t.account_id=e.account_id order by tr.refreshed_at desc limit 1), '')
		from calendar.events e
		where e.account_id = $1::uuid and e.client_id::text = any($2::text[])
		  and e.event_date between $3::date and $4::date
		order by e.event_date desc`, scope.StorageAccountID, clientIDs, from, to)
	if err != nil {
		return nil, nil, err
	}
	defer eventRows.Close()
	events := make([]EventSnapshot, 0)
	for eventRows.Next() {
		var event EventSnapshot
		if err := eventRows.Scan(&event.ID, &event.ClientID, &event.Date, &event.Type, &event.Title, &event.Status, &event.TaskID); err != nil {
			return nil, nil, err
		}
		events = append(events, event)
	}
	return tasks, events, eventRows.Err()
}
