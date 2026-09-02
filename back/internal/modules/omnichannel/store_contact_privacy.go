package omnichannel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func scanHiddenContact(row rowScanner) (contactPrivacyRow, error) {
	var out contactPrivacyRow
	err := row.Scan(
		&out.ContactID,
		&out.ConversationID,
		&out.ContactName,
		&out.ContactPhone,
		&out.HiddenAt,
		&out.HistoryClearedAt,
		&out.HiddenByUserID,
		&out.HistoryClearedByUser,
	)
	return out, err
}

func (s *Store) HideContactByConversation(ctx context.Context, accountID, conversationID, userID string, clearHistory bool) (contactPrivacyRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contactPrivacyRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var contactID string
	err = tx.QueryRow(ctx, `select contact_id::text
		from messaging.conversations
		where account_id=$1::uuid and id=$2::uuid and contact_id is not null
		for update`, accountID, conversationID).Scan(&contactID)
	if err != nil {
		return contactPrivacyRow{}, err
	}

	_, err = tx.Exec(ctx, `insert into messaging.contact_suppressions (
			account_id,contact_id,is_hidden,hidden_at,hidden_by_user_id,
			history_cleared_at,history_cleared_by_user_id,restored_at,restored_by_user_id,updated_at
		) values ($1::uuid,$2::uuid,true,now(),$3::uuid,
			case when $4 then now() else null end,
			case when $4 then $3::uuid else null end,null,null,now())
		on conflict (account_id,contact_id) do update set
			is_hidden=true,
			hidden_at=now(),
			hidden_by_user_id=excluded.hidden_by_user_id,
			history_cleared_at=case when $4 then now()
				else messaging.contact_suppressions.history_cleared_at end,
			history_cleared_by_user_id=case when $4 then $3::uuid
				else messaging.contact_suppressions.history_cleared_by_user_id end,
			restored_at=null,restored_by_user_id=null,updated_at=now()`,
		accountID, contactID, userID, clearHistory)
	if err != nil {
		return contactPrivacyRow{}, err
	}
	if _, err := tx.Exec(ctx, `update messaging.ai_reply_drafts draft
		set status='expired',decision_reason='contact_hidden',decided_at=now(),updated_at=now()
		from messaging.conversations conversation
		where draft.account_id=$1::uuid and draft.account_id=conversation.account_id
		  and draft.conversation_id=conversation.id and draft.status='pending'
		  and conversation.contact_id=$2::uuid`, accountID, contactID); err != nil {
		return contactPrivacyRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contactPrivacyRow{}, err
	}
	return s.GetHiddenContact(ctx, accountID, contactID)
}

func (s *Store) RestoreHiddenContact(ctx context.Context, accountID, contactID, userID string) error {
	result, err := s.pool.Exec(ctx, `update messaging.contact_suppressions
		set is_hidden=false,restored_at=now(),restored_by_user_id=$3::uuid,updated_at=now()
		where account_id=$1::uuid and contact_id=$2::uuid and is_hidden=true`,
		accountID, contactID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) GetHiddenContact(ctx context.Context, accountID, contactID string) (contactPrivacyRow, error) {
	return scanHiddenContact(s.pool.QueryRow(ctx, hiddenContactsSelect+`
		where suppression.account_id=$1::uuid and suppression.contact_id=$2::uuid
		  and suppression.is_hidden=true`, accountID, contactID))
}

const hiddenContactsSelect = `select suppression.contact_id::text,
	coalesce(latest_conversation.id::text,''),coalesce(contact.name,''),coalesce(contact.phone,''),
	suppression.hidden_at,suppression.history_cleared_at,suppression.hidden_by_user_id::text,
	suppression.history_cleared_by_user_id::text
	from messaging.contact_suppressions suppression
	join messaging.contacts contact
	  on contact.account_id=suppression.account_id and contact.id=suppression.contact_id
	left join lateral (
		select conversation.id
		from messaging.conversations conversation
		where conversation.account_id=suppression.account_id
		  and conversation.contact_id=suppression.contact_id
		order by conversation.last_message_at desc,conversation.id desc limit 1
	) latest_conversation on true`

func (s *Store) ListHiddenContacts(ctx context.Context, accountID string) ([]contactPrivacyRow, error) {
	rows, err := s.pool.Query(ctx, hiddenContactsSelect+`
		where suppression.account_id=$1::uuid and suppression.is_hidden=true
		order by suppression.hidden_at desc,suppression.contact_id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]contactPrivacyRow, 0)
	for rows.Next() {
		item, err := scanHiddenContact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) IsConversationHidden(ctx context.Context, accountID, conversationID string) (bool, error) {
	var hidden bool
	err := s.pool.QueryRow(ctx, `select exists(
		select 1
		from messaging.conversations conversation
		join messaging.contact_suppressions suppression
		  on suppression.account_id=conversation.account_id
		 and suppression.contact_id=conversation.contact_id
		where conversation.account_id=$1::uuid and conversation.id=$2::uuid
		  and suppression.is_hidden=true)`, accountID, conversationID).Scan(&hidden)
	return hidden, err
}

func (s *Store) GetContactAIRestrictionByConversation(ctx context.Context, accountID, conversationID string) (ContactAIRestrictionView, error) {
	var out ContactAIRestrictionView
	var blockedUntil *time.Time
	var updatedAt *time.Time
	err := s.pool.QueryRow(ctx, `select conversation.contact_id::text,
		restriction.blocked_until, restriction.updated_at
		from messaging.conversations conversation
		left join messaging.contact_ai_restrictions restriction
		  on restriction.account_id=conversation.account_id and restriction.contact_id=conversation.contact_id
		where conversation.account_id=$1::uuid and conversation.id=$2::uuid
		  and conversation.contact_id is not null`, accountID, conversationID).Scan(&out.ContactID, &blockedUntil, &updatedAt)
	if err != nil {
		return ContactAIRestrictionView{}, err
	}
	out.BlockedUntil = blockedUntil
	out.UpdatedAt = updatedAt
	out.Mode = "allow"
	if updatedAt != nil && (blockedUntil == nil || blockedUntil.After(time.Now().UTC())) {
		out.Blocked = true
		out.Mode = "indefinite"
		if blockedUntil != nil {
			out.Mode = "until"
		}
	}
	return out, nil
}

func (s *Store) SetContactAIRestriction(ctx context.Context, accountID, conversationID, userID string, in ContactAIRestrictionInput) (ContactAIRestrictionView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ContactAIRestrictionView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var contactID string
	if err := tx.QueryRow(ctx, `select contact_id::text from messaging.conversations
		where account_id=$1::uuid and id=$2::uuid and contact_id is not null for update`, accountID, conversationID).Scan(&contactID); err != nil {
		return ContactAIRestrictionView{}, err
	}
	if in.Mode == "allow" {
		if _, err := tx.Exec(ctx, `delete from messaging.contact_ai_restrictions
			where account_id=$1::uuid and contact_id=$2::uuid`, accountID, contactID); err != nil {
			return ContactAIRestrictionView{}, err
		}
	} else {
		if _, err := tx.Exec(ctx, `insert into messaging.contact_ai_restrictions
			(account_id,contact_id,blocked_until,updated_by_user_id)
			values ($1::uuid,$2::uuid,$3,$4::uuid)
			on conflict (account_id,contact_id) do update set
			blocked_until=excluded.blocked_until,updated_by_user_id=excluded.updated_by_user_id,updated_at=now()`,
			accountID, contactID, in.BlockedUntil, userID); err != nil {
			return ContactAIRestrictionView{}, err
		}
		// O bloqueio manual precisa vencer qualquer trabalho AI que ja esteja em voo.
		// A geracao invalida leases; as tres atualizacoes cancelam outbox, mensagem e
		// dispatch durable no mesmo commit, sem falar diretamente com o provider.
		if _, err := tx.Exec(ctx, `update messaging.conversations
			set ai_generation=ai_generation+1,updated_at=now()
			where account_id=$1::uuid and contact_id=$2::uuid and state='ai_active'`, accountID, contactID); err != nil {
			return ContactAIRestrictionView{}, err
		}
		if _, err := tx.Exec(ctx, `update messaging.outbox o
			set status='dead',last_error='ai_contact_blocked',locked_at=null,locked_by='',updated_at=now()
			from messaging.messages m
			join messaging.conversations c
			  on c.account_id=m.account_id and c.id=m.conversation_id
			where o.account_id=$1::uuid and c.account_id=$1::uuid and c.contact_id=$2::uuid
			  and o.kind=$3 and o.status in ('pending','processing')
			  and m.origin='ai' and m.status='PENDING'
			  and o.payload->>'messageId'=m.id::text`, accountID, contactID, OutboundJobKind); err != nil {
			return ContactAIRestrictionView{}, err
		}
		if _, err := tx.Exec(ctx, `update messaging.messages m
			set status='FAILED',provider_error_code='ai_contact_blocked',updated_at=now()
			from messaging.conversations c
			where m.account_id=$1::uuid and c.account_id=$1::uuid and c.contact_id=$2::uuid
			  and m.conversation_id=c.id and m.origin='ai' and m.status='PENDING'`, accountID, contactID); err != nil {
			return ContactAIRestrictionView{}, err
		}
		if _, err := tx.Exec(ctx, `update messaging.ai_dispatches d
			set status='cancelled',last_error='ai_contact_blocked',locked_at=null,updated_at=now()
			from messaging.conversations c
			where d.account_id=$1::uuid and c.account_id=$1::uuid and c.contact_id=$2::uuid
			  and d.conversation_id=c.id and d.status in ('buffering','queued','processing')`, accountID, contactID); err != nil {
			return ContactAIRestrictionView{}, err
		}
		if _, err := tx.Exec(ctx, `update messaging.ai_reply_drafts draft
			set status='expired',decision_reason='ai_contact_blocked',decided_at=now(),updated_at=now()
			from messaging.conversations conversation
			where draft.account_id=$1::uuid and draft.account_id=conversation.account_id
			  and draft.conversation_id=conversation.id and draft.status='pending'
			  and conversation.contact_id=$2::uuid`, accountID, contactID); err != nil {
			return ContactAIRestrictionView{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ContactAIRestrictionView{}, err
	}
	return s.GetContactAIRestrictionByConversation(ctx, accountID, conversationID)
}

func (s *Store) IsConversationAIBlocked(ctx context.Context, accountID, conversationID string) (bool, error) {
	var blocked bool
	err := s.pool.QueryRow(ctx, `select exists(
		select 1 from messaging.conversations conversation
		join messaging.contact_ai_restrictions restriction
		  on restriction.account_id=conversation.account_id and restriction.contact_id=conversation.contact_id
		where conversation.account_id=$1::uuid and conversation.id=$2::uuid
		  and (restriction.blocked_until is null or restriction.blocked_until > now()))`, accountID, conversationID).Scan(&blocked)
	return blocked, err
}

func validPrivacyID(value string) bool {
	return omnichannelUUIDPattern.MatchString(strings.TrimSpace(value))
}

func privacyNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
