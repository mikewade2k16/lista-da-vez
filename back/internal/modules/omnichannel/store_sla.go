package omnichannel

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

const queueSLAPolicyColumns = `id::text, queue_id::text, first_response_seconds,
	resolution_seconds, business_hours_only, is_active, created_at, updated_at`

func scanQueueSLAPolicy(row rowScanner) (QueueSLAPolicyView, error) {
	var out QueueSLAPolicyView
	err := row.Scan(&out.ID, &out.QueueID, &out.FirstResponseSeconds, &out.ResolutionSeconds,
		&out.BusinessHoursOnly, &out.IsActive, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (s *Store) GetQueueSLAPolicy(ctx context.Context, accountID, queueID string) (QueueSLAPolicyView, error) {
	row, err := scanQueueSLAPolicy(s.pool.QueryRow(ctx, `select `+queueSLAPolicyColumns+`
		from messaging.queue_sla_policies where account_id=$1::uuid and queue_id=$2::uuid`, accountID, queueID))
	if errors.Is(err, pgx.ErrNoRows) {
		return QueueSLAPolicyView{}, ErrNotFound
	}
	return row, err
}

func (s *Store) UpsertQueueSLAPolicy(ctx context.Context, accountID, queueID string, in QueueSLAPolicyInput) (QueueSLAPolicyView, error) {
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	return scanQueueSLAPolicy(s.pool.QueryRow(ctx, `insert into messaging.queue_sla_policies
		(account_id,queue_id,first_response_seconds,resolution_seconds,business_hours_only,is_active)
		select $1::uuid,q.id,$3,$4,$5,$6 from messaging.queues q
		where q.account_id=$1::uuid and q.id=$2::uuid
		on conflict (account_id,queue_id) do update set first_response_seconds=excluded.first_response_seconds,
		resolution_seconds=excluded.resolution_seconds,business_hours_only=excluded.business_hours_only,
		is_active=excluded.is_active,updated_at=now()
		returning `+queueSLAPolicyColumns, accountID, queueID, in.FirstResponseSeconds, in.ResolutionSeconds, in.BusinessHoursOnly, active))
}

func (s *Store) insertSLAEvent(ctx context.Context, accountID, conversationID, handoffID, eventType, key string, metadata []byte) error {
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	_, err := s.pool.Exec(ctx, `insert into messaging.sla_events
		(account_id,conversation_id,handoff_id,event_type,idempotency_key,metadata)
		values ($1::uuid,$2::uuid,nullif($3,'')::uuid,$4,$5,$6::jsonb)
		on conflict (account_id,idempotency_key) do nothing`, accountID, conversationID, handoffID, eventType, key, metadata)
	return err
}

func insertSLAEventTx(ctx context.Context, tx pgx.Tx, accountID, conversationID, handoffID, eventType, key string, metadata []byte) error {
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	_, err := tx.Exec(ctx, `insert into messaging.sla_events
		(account_id,conversation_id,handoff_id,event_type,idempotency_key,metadata)
		values ($1::uuid,$2::uuid,nullif($3,'')::uuid,$4,$5,$6::jsonb)
		on conflict (account_id,idempotency_key) do nothing`, accountID, conversationID, handoffID, eventType, key, metadata)
	return err
}

type slaDueRow struct {
	AccountID      string
	ConversationID string
	HandoffID      string
	QueueID        string
	RequestedAt    int64
	FirstResponse  int
}

// EvaluateSLAs materializa apenas eventos idempotentes. Horário comercial é
// deliberadamente tratado como wall-clock nesta primeira fatia; o scheduler não
// inventa feriados/calendários e a policy deixa a flag visível para a próxima
// integração com o módulo Calendário.
func (s *Store) EvaluateSLAs(ctx context.Context, nowUnix int64) (int, error) {
	rows, err := s.pool.Query(ctx, `select h.account_id::text,h.conversation_id::text,h.id::text,
		h.target_queue_id::text,extract(epoch from h.requested_at)::bigint,p.first_response_seconds
		from messaging.handoffs h join messaging.queue_sla_policies p
		  on p.account_id=h.account_id and p.queue_id=h.target_queue_id and p.is_active
		where h.status in ('queued','accepted') and h.target_queue_id is not null`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	created := 0
	for rows.Next() {
		var item slaDueRow
		if err := rows.Scan(&item.AccountID, &item.ConversationID, &item.HandoffID, &item.QueueID, &item.RequestedAt, &item.FirstResponse); err != nil {
			return created, err
		}
		for _, eventType := range dueSLAEvents(nowUnix, item.RequestedAt, item.FirstResponse) {
			if err := s.insertSLAEvent(ctx, item.AccountID, item.ConversationID, item.HandoffID, eventType, "sla:"+item.HandoffID+":"+eventType, nil); err != nil {
				return created, err
			}
			created++
		}
	}
	return created, rows.Err()
}

func dueSLAEvents(nowUnix, requestedAt int64, firstResponseSeconds int) []string {
	if firstResponseSeconds < 1 || nowUnix < requestedAt {
		return nil
	}
	elapsed := nowUnix - requestedAt
	if elapsed >= int64(firstResponseSeconds) {
		return []string{"warning", "breached"}
	}
	if elapsed >= int64(firstResponseSeconds*8/10) {
		return []string{"warning"}
	}
	return nil
}
