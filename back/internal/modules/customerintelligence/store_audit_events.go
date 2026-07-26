package customerintelligence

import (
	"context"
	"time"
)

func (s *PostgresRepository) ListAuditEventPage(
	ctx context.Context,
	scope Scope,
	query auditEventRepositoryQuery,
) ([]AuditEvent, error) {
	occurredFrom := nullableAuditEventTime(query.OccurredFrom)
	occurredTo := nullableAuditEventTime(query.OccurredTo)
	hasCursor := query.Cursor != nil
	var cursorOccurredAt, cursorID any
	if hasCursor {
		cursorOccurredAt = query.Cursor.OccurredAt
		cursorID = query.Cursor.ID
	}
	rows, err := s.pool.Query(ctx, `
		select id, coalesce(client_account_id::text, ''),
		       coalesce(actor_user_id::text, ''), event_type, aggregate_type,
		       aggregate_id, correlation_id, reason_code, metadata, occurred_at
		from intelligence.audit_events
		where account_id = $1::uuid
		  and (client_account_id is null or client_account_id = $2::uuid)
		  and ($3::text = '' or event_type = $3::text)
		  and ($4::text = '' or aggregate_type = $4::text)
		  and ($5::timestamptz is null or occurred_at >= $5::timestamptz)
		  and ($6::timestamptz is null or occurred_at <= $6::timestamptz)
		  and (
		      $7::boolean = false
		      or (occurred_at, id) < ($8::timestamptz, $9::uuid)
		  )
		order by occurred_at desc, id desc
		limit $10`,
		scope.AccountID,
		scope.ClientAccountID,
		query.Action,
		query.EntityType,
		occurredFrom,
		occurredTo,
		hasCursor,
		cursorOccurredAt,
		cursorID,
		query.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AuditEvent, 0, query.Limit)
	for rows.Next() {
		var item AuditEvent
		var metadata []byte
		if err := rows.Scan(
			&item.ID,
			&item.ClientAccountID,
			&item.ActorUserID,
			&item.EventType,
			&item.AggregateType,
			&item.AggregateID,
			&item.CorrelationID,
			&item.ReasonCode,
			&metadata,
			&item.OccurredAt,
		); err != nil {
			return nil, err
		}
		item.Metadata = normalizedJSON(metadata, `{}`)
		items = append(items, item)
	}
	return items, rows.Err()
}

func nullableAuditEventTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}
