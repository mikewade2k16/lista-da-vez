package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type channelBindingRowScanner interface {
	Scan(...any) error
}

func scanChannelClientBinding(row channelBindingRowScanner) (ChannelClientBindingView, error) {
	var out ChannelClientBindingView
	var resourceID, resourceType, resourceLabel string
	err := row.Scan(
		&out.ID,
		&out.ClientAccountID,
		&out.Channel,
		&resourceID,
		&resourceType,
		&resourceLabel,
		&out.EffectiveFrom,
		&out.EffectiveTo,
		&out.Source,
		&out.Reason,
		&out.Revision,
		&out.CreatedAt,
		&out.UpdatedAt,
		&out.ClientAccountName,
	)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	out.ChannelResource = ChannelClientResourceView{
		ID: resourceID, Type: resourceType, Label: resourceLabel,
	}
	return out, nil
}

const channelClientBindingSelect = `
	select
		b.id::text,
		b.client_account_id::text,
		b.channel,
		coalesce(b.whatsapp_instance_id, b.instagram_account_id)::text,
		case when b.channel = 'WHATSAPP' then 'whatsapp_instance' else 'instagram_account' end,
		case
			when b.channel = 'WHATSAPP' then coalesce(wi.display_name, wi.instance_name, '')
			else coalesce(ia.display_name, ia.username, ia.ig_user_id, '')
		end,
		b.effective_from,
		b.effective_to,
		b.source,
		b.reason,
		b.revision,
		b.created_at,
		b.updated_at,
		coalesce(client.name, '')
	from messaging.channel_client_bindings b
	join core.accounts client
	  on client.id = b.client_account_id
	left join messaging.whatsapp_instances wi
	  on wi.account_id = b.account_id and wi.id = b.whatsapp_instance_id
	left join messaging.instagram_accounts ia
	  on ia.account_id = b.account_id and ia.id = b.instagram_account_id
`

func (s *Store) ListChannelClientBindings(ctx context.Context, accountID string, filter ChannelClientBindingFilter) ([]ChannelClientBindingView, error) {
	rows, err := s.pool.Query(ctx, channelClientBindingSelect+`
		where b.account_id = $1::uuid
		  and ($2 = '' or b.client_account_id = $2::uuid)
		  and ($3 = '' or b.channel = $3)
		  and (
		    $4 = ''
		    or ($4 = 'active' and b.effective_to is null)
		    or ($4 = 'ended' and b.effective_to is not null)
		  )
		  and (
		    $5 = ''
		    or (b.updated_at, b.id) < (
		        select cursor_row.updated_at, cursor_row.id
		        from messaging.channel_client_bindings cursor_row
		        where cursor_row.account_id = $1::uuid and cursor_row.id = $5::uuid
		    )
		  )
		order by b.updated_at desc, b.id desc
		limit $6`,
		accountID,
		strings.TrimSpace(filter.ClientAccountID),
		strings.ToUpper(strings.TrimSpace(filter.Channel)),
		strings.ToLower(strings.TrimSpace(filter.State)),
		strings.TrimSpace(filter.Cursor),
		filter.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ChannelClientBindingView, 0)
	for rows.Next() {
		item, scanErr := scanChannelClientBinding(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetChannelClientBinding(ctx context.Context, accountID, bindingID string) (ChannelClientBindingView, error) {
	out, err := scanChannelClientBinding(s.pool.QueryRow(
		ctx,
		channelClientBindingSelect+` where b.account_id = $1::uuid and b.id = $2::uuid`,
		accountID,
		bindingID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return ChannelClientBindingView{}, ErrNotFound
	}
	return out, err
}

// ChannelBindingClientEligible repete no banco a fronteira owner/client. Uma
// conta standalone pode apontar para si; a conta-agencia somente para cliente
// ativo da mesma organization. O catalogo permission-scoped ainda e validado
// pelo service antes desta defesa em profundidade.
func (s *Store) ChannelBindingClientEligible(ctx context.Context, accountID, clientAccountID string) (bool, error) {
	var eligible bool
	err := s.pool.QueryRow(ctx, `
		select exists (
			select 1
			from core.accounts owner
			join core.accounts client on client.id = $2::uuid
			where owner.id = $1::uuid
			  and owner.is_active = true
			  and client.is_active = true
			  and (
			    owner.id = client.id
			    or (
			      owner.is_agency = true
			      and client.is_agency = false
			      and owner.organization_id is not null
			      and client.organization_id = owner.organization_id
			    )
			  )
		)`, accountID, clientAccountID).Scan(&eligible)
	return eligible, err
}

func (s *Store) ChannelBindingResourceExists(ctx context.Context, accountID, channel, resourceID string) (bool, bool, error) {
	var active bool
	var err error
	switch strings.ToUpper(strings.TrimSpace(channel)) {
	case "WHATSAPP":
		err = s.pool.QueryRow(ctx, `
			select is_active
			from messaging.whatsapp_instances
			where account_id = $1::uuid and id = $2::uuid`,
			accountID, resourceID).Scan(&active)
	case "INSTAGRAM":
		err = s.pool.QueryRow(ctx, `
			select is_active
			from messaging.instagram_accounts
			where account_id = $1::uuid and id = $2::uuid`,
			accountID, resourceID).Scan(&active)
	default:
		return false, false, ErrValidation
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return true, active, nil
}

func existingChannelBindingEvent(ctx context.Context, tx pgx.Tx, accountID, idempotencyKey, requestHash string) (string, bool, error) {
	var bindingID, storedHash string
	err := tx.QueryRow(ctx, `
		select coalesce(successor_binding_id, binding_id)::text, request_hash
		from messaging.channel_client_binding_events
		where account_id = $1::uuid and idempotency_key = $2`,
		accountID, idempotencyKey).Scan(&bindingID, &storedHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if storedHash != requestHash {
		return "", false, ErrConflict
	}
	return bindingID, true, nil
}

func (s *Store) CreateChannelClientBinding(ctx context.Context, accountID string, in channelClientBindingWrite) (string, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	bindingID, err := createChannelClientBindingTx(ctx, tx, accountID, in)
	if err != nil {
		return "", err
	}
	return bindingID, tx.Commit(ctx)
}

// createChannelClientBindingTx e compartilhado pela API de bindings e pelo
// provisionamento de uma instancia. Isso permite que a instancia, o primeiro
// grant manage e o self-binding standalone_default nascam atomicamente.
func createChannelClientBindingTx(ctx context.Context, tx pgx.Tx, accountID string, in channelClientBindingWrite) (string, error) {
	var err error
	if existingID, ok, lookupErr := existingChannelBindingEvent(
		ctx, tx, accountID, in.IdempotencyKey, in.RequestHash,
	); lookupErr != nil {
		return "", lookupErr
	} else if ok {
		return existingID, nil
	}

	var whatsappID, instagramID any
	if in.Channel == "WHATSAPP" {
		whatsappID = in.ResourceID
	}
	if in.Channel == "INSTAGRAM" {
		instagramID = in.ResourceID
	}

	var bindingID string
	err = tx.QueryRow(ctx, `
		insert into messaging.channel_client_bindings (
			account_id, client_account_id, channel, whatsapp_instance_id,
			instagram_account_id, effective_from, source, reason, created_by_user_id
		)
		values (
			$1::uuid, $2::uuid, $3, $4::uuid, $5::uuid, $6, $7, $8,
			nullif($9, '')::uuid
		)
		returning id::text`,
		accountID,
		in.ClientAccountID,
		in.Channel,
		whatsappID,
		instagramID,
		in.EffectiveFrom,
		in.Source,
		in.Reason,
		in.ActorUserID,
	).Scan(&bindingID)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrConflict
		}
		return "", err
	}

	snapshot, _ := json.Marshal(map[string]any{
		"bindingId":       bindingID,
		"clientAccountId": in.ClientAccountID,
		"channel":         in.Channel,
		"resourceId":      in.ResourceID,
		"effectiveFrom":   in.EffectiveFrom,
		"revision":        1,
	})
	_, err = tx.Exec(ctx, `
		insert into messaging.channel_client_binding_events (
			account_id, binding_id, event_type, reason, idempotency_key,
			request_hash, actor_user_id, snapshot
		)
		values (
			$1::uuid, $2::uuid, 'created', $3, $4, $5,
			nullif($6, '')::uuid, $7::jsonb
		)`,
		accountID,
		bindingID,
		in.Reason,
		in.IdempotencyKey,
		in.RequestHash,
		in.ActorUserID,
		snapshot,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrConflict
		}
		return "", err
	}
	return bindingID, nil
}

func (s *Store) ReassignChannelClientBinding(
	ctx context.Context,
	accountID, bindingID, requestHash string,
	in ReassignChannelClientBindingInput,
	actorUserID string,
) (string, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if existingID, ok, lookupErr := existingChannelBindingEvent(
		ctx, tx, accountID, in.IdempotencyKey, requestHash,
	); lookupErr != nil {
		return "", lookupErr
	} else if ok {
		return existingID, tx.Commit(ctx)
	}

	var channel, oldClientID, resourceID string
	var effectiveFrom time.Time
	var effectiveTo *time.Time
	var revision int64
	err = tx.QueryRow(ctx, `
		select
			channel,
			client_account_id::text,
			coalesce(whatsapp_instance_id, instagram_account_id)::text,
			effective_from,
			effective_to,
			revision
		from messaging.channel_client_bindings
		where account_id = $1::uuid and id = $2::uuid
		for update`,
		accountID, bindingID,
	).Scan(&channel, &oldClientID, &resourceID, &effectiveFrom, &effectiveTo, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if effectiveTo != nil || revision != in.ExpectedRevision || !in.EffectiveAt.After(effectiveFrom) {
		return "", ErrConflict
	}

	tag, err := tx.Exec(ctx, `
		update messaging.channel_client_bindings
		set effective_to = $3,
		    ended_by_user_id = nullif($4, '')::uuid,
		    revision = revision + 1,
		    updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
		  and revision = $5 and effective_to is null`,
		accountID, bindingID, in.EffectiveAt, actorUserID, in.ExpectedRevision,
	)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() != 1 {
		return "", ErrConflict
	}

	var whatsappID, instagramID any
	if channel == "WHATSAPP" {
		whatsappID = resourceID
	} else {
		instagramID = resourceID
	}
	var successorID string
	err = tx.QueryRow(ctx, `
		insert into messaging.channel_client_bindings (
			account_id, client_account_id, channel, whatsapp_instance_id,
			instagram_account_id, effective_from, source, reason, created_by_user_id
		)
		values (
			$1::uuid, $2::uuid, $3, $4::uuid, $5::uuid, $6, 'manual', $7,
			nullif($8, '')::uuid
		)
		returning id::text`,
		accountID,
		in.TargetClientAccountID,
		channel,
		whatsappID,
		instagramID,
		in.EffectiveAt,
		strings.TrimSpace(in.Reason),
		actorUserID,
	).Scan(&successorID)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrConflict
		}
		return "", err
	}

	snapshot, _ := json.Marshal(map[string]any{
		"oldBindingId":       bindingID,
		"successorBindingId": successorID,
		"oldClientAccountId": oldClientID,
		"clientAccountId":    in.TargetClientAccountID,
		"channel":            channel,
		"resourceId":         resourceID,
		"effectiveAt":        in.EffectiveAt,
		"expectedRevision":   in.ExpectedRevision,
	})
	_, err = tx.Exec(ctx, `
		insert into messaging.channel_client_binding_events (
			account_id, binding_id, successor_binding_id, event_type, reason,
			idempotency_key, request_hash, actor_user_id, snapshot
		)
		values (
			$1::uuid, $2::uuid, $3::uuid, 'reassigned', $4, $5, $6,
			nullif($7, '')::uuid, $8::jsonb
		)`,
		accountID,
		bindingID,
		successorID,
		strings.TrimSpace(in.Reason),
		in.IdempotencyKey,
		requestHash,
		actorUserID,
		snapshot,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrConflict
		}
		return "", err
	}
	return successorID, tx.Commit(ctx)
}
