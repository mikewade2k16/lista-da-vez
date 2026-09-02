package omnichannel

import "context"

func (s *Store) ListCustomerMessageEvidence(
	ctx context.Context,
	request CustomerEvidenceRequest,
	contactIDs []string,
) ([]CustomerMessageEvidence, error) {
	rows, err := s.pool.Query(ctx, `
		select m.id::text,
		       c.id::text,
		       c.contact_id::text,
		       c.channel,
		       m.direction,
		       m.message_type,
		       m.content,
		       m.status,
		       coalesce(m.media_mime_type, ''),
		       coalesce(m.media_file_name, ''),
		       coalesce(m.media_caption, ''),
		       m.media_duration_seconds,
		       m.created_at,
		       m.updated_at
		from messaging.messages m
		join messaging.conversations c
		  on c.account_id = m.account_id
		 and c.id = m.conversation_id
		where m.account_id = $1::uuid
		  and c.client_account_id = $2::uuid
		  and c.contact_id::text = any($3::text[])
		  and m.created_at >= now() - ($4::int * interval '1 day')
		`+s.historyVisibleMessagePredicate("m", "c")+`
		order by m.created_at desc, m.id desc
		limit $5
	`,
		request.AccountID,
		request.ClientAccountID,
		contactIDs,
		request.LookbackDays,
		request.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]CustomerMessageEvidence, 0)
	for rows.Next() {
		var item CustomerMessageEvidence
		if err := rows.Scan(
			&item.MessageID,
			&item.ConversationID,
			&item.ContactSourceID,
			&item.Channel,
			&item.Direction,
			&item.MessageType,
			&item.Content,
			&item.Status,
			&item.MediaMimeType,
			&item.MediaFileName,
			&item.MediaCaption,
			&item.MediaDurationSecs,
			&item.OccurredAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
