package omnichannel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (s *Store) ListChannelClientBindingExceptions(ctx context.Context, accountID string) ([]ChannelClientBindingExceptionView, error) {
	rows, err := s.pool.Query(ctx, `
		with exceptions as (
			select
				c.id,
				c.channel,
				case
				  when c.channel = 'WHATSAPP' then c.instance_id::text
				  else ia.id::text
				end as resource_id,
				c.client_binding_state,
				c.last_message_at
			from messaging.conversations c
			left join messaging.instagram_accounts ia
			  on c.channel = 'INSTAGRAM'
			 and ia.account_id = c.account_id
			 and ia.ig_user_id = c.instance_scope_key
			where c.account_id = $1::uuid
			  and c.client_binding_state in ('unresolved', 'quarantined')
		)
		select
			e.channel,
			coalesce(e.resource_id, ''),
			e.client_binding_state,
			case
			  when e.resource_id is null then 'conversation_without_resource'
			  when e.client_binding_state = 'quarantined' then 'ambiguous_or_invalid_binding'
			  else 'no_effective_binding'
			end,
			count(distinct e.id),
			count(distinct t.id),
			max(e.last_message_at)
		from exceptions e
		left join messaging.contact_touchpoints t
		  on t.account_id = $1::uuid and t.conversation_id = e.id
		group by e.channel, e.resource_id, e.client_binding_state
		order by max(e.last_message_at) desc`,
		accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ChannelClientBindingExceptionView, 0)
	for rows.Next() {
		var item ChannelClientBindingExceptionView
		var latest *time.Time
		if err := rows.Scan(
			&item.Channel,
			&item.ChannelResourceID,
			&item.BindingState,
			&item.ReasonCode,
			&item.ConversationCount,
			&item.TouchpointCount,
			&latest,
		); err != nil {
			return nil, err
		}
		if latest != nil {
			item.LatestConversation = latest.UTC().Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

type repairBindingData struct {
	Channel         string
	ResourceID      string
	ClientAccountID string
	EffectiveFrom   time.Time
	EffectiveTo     *time.Time
}

func loadRepairBinding(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, accountID, bindingID string) (repairBindingData, error) {
	var out repairBindingData
	err := querier.QueryRow(ctx, `
		select
			channel,
			coalesce(whatsapp_instance_id, instagram_account_id)::text,
			client_account_id::text,
			effective_from,
			effective_to
		from messaging.channel_client_bindings
		where account_id = $1::uuid and id = $2::uuid`,
		accountID, bindingID,
	).Scan(
		&out.Channel,
		&out.ResourceID,
		&out.ClientAccountID,
		&out.EffectiveFrom,
		&out.EffectiveTo,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return repairBindingData{}, ErrNotFound
	}
	return out, err
}

func repairEligibilityPredicate() string {
	return `
		c.account_id = $1::uuid
		and $2::uuid is not null
		and (
		  ($3 = 'WHATSAPP' and c.channel = 'WHATSAPP' and c.instance_id = $4::uuid)
		  or
		  (
		    $3 = 'INSTAGRAM'
		    and c.channel = 'INSTAGRAM'
		    and exists (
		      select 1
		      from messaging.instagram_accounts ia
		      where ia.account_id = c.account_id
		        and ia.id = $4::uuid
		        and ia.ig_user_id = c.instance_scope_key
		    )
		  )
		)
		and c.client_binding_state in ('unresolved', 'quarantined')
		and c.last_message_at >= $5
		and c.last_message_at < coalesce($6, 'infinity'::timestamptz)
		and c.last_message_at <= $7
		and ($8::boolean or c.state <> 'closed')
	`
}

func existingRepairJob(
	ctx context.Context,
	tx pgx.Tx,
	accountID, idempotencyKey, requestHash string,
) (ChannelClientBindingRepairJobView, bool, error) {
	var id, storedHash string
	err := tx.QueryRow(ctx, `
		select id::text, request_hash
		from messaging.channel_client_binding_repair_jobs
		where account_id = $1::uuid and idempotency_key = $2`,
		accountID, idempotencyKey,
	).Scan(&id, &storedHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChannelClientBindingRepairJobView{}, false, nil
	}
	if err != nil {
		return ChannelClientBindingRepairJobView{}, false, err
	}
	if storedHash != requestHash {
		return ChannelClientBindingRepairJobView{}, false, ErrConflict
	}
	job, err := getRepairJobWithQuerier(ctx, tx, accountID, id)
	return job, true, err
}

func (s *Store) CreateChannelClientBindingRepairPreview(
	ctx context.Context,
	accountID string,
	p auth.Principal,
	in ChannelClientBindingRepairPreviewInput,
	requestHash string,
) (ChannelClientBindingRepairJobView, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if existing, ok, lookupErr := existingRepairJob(
		ctx, tx, accountID, in.IdempotencyKey, requestHash,
	); lookupErr != nil {
		return ChannelClientBindingRepairJobView{}, lookupErr
	} else if ok {
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return ChannelClientBindingRepairJobView{}, commitErr
		}
		return existing, nil
	}

	binding, err := loadRepairBinding(ctx, tx, accountID, in.BindingID)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	var scanned, eligible, quarantined int64
	countQuery := fmt.Sprintf(`
		select
			count(*),
			count(*) filter (
			  where not exists (
			    select 1
			    from messaging.messages m
			    where m.account_id = c.account_id
			      and m.conversation_id = c.id
			      and m.direction = 'OUTBOUND'
			      and m.origin <> 'ai'
			  )
			),
			count(*) filter (where c.client_binding_state = 'quarantined')
		from messaging.conversations c
		where %s`, repairEligibilityPredicate())
	err = tx.QueryRow(ctx, countQuery,
		accountID,
		in.BindingID,
		binding.Channel,
		binding.ResourceID,
		binding.EffectiveFrom,
		binding.EffectiveTo,
		in.Watermark,
		in.IncludeClosed,
	).Scan(&scanned, &eligible, &quarantined)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	checksumInput := fmt.Sprintf(
		"%s|%s|%d|%d|%d",
		requestHash, in.Watermark.UTC().Format(time.RFC3339Nano), scanned, eligible, quarantined,
	)
	checksumBytes := sha256.Sum256([]byte(checksumInput))
	checksum := hex.EncodeToString(checksumBytes[:])
	filters, _ := json.Marshal(map[string]any{
		"includeClosed":       in.IncludeClosed,
		"protectHumanReplies": true,
	})

	var jobID string
	err = tx.QueryRow(ctx, `
		insert into messaging.channel_client_binding_repair_jobs (
			account_id, channel, whatsapp_instance_id, instagram_account_id,
			client_account_id, binding_id, mode, status, filters, watermark,
			preview_checksum, idempotency_key, request_hash, scanned_count,
			eligible_count, quarantined_count, skipped_count, actor_user_id,
			reason, started_at, completed_at
		)
		values (
			$1::uuid,
			$2,
			case when $2 = 'WHATSAPP' then $3::uuid else null end,
			case when $2 = 'INSTAGRAM' then $3::uuid else null end,
			$4::uuid,
			$5::uuid,
			'preview',
			'completed',
			$6::jsonb,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13,
			$14,
			nullif($15, '')::uuid,
			$16,
			now(),
			now()
		)
		returning id::text`,
		accountID,
		binding.Channel,
		binding.ResourceID,
		binding.ClientAccountID,
		in.BindingID,
		filters,
		in.Watermark,
		checksum,
		in.IdempotencyKey,
		requestHash,
		scanned,
		eligible,
		quarantined,
		scanned-eligible,
		p.UserID,
		in.Reason,
	).Scan(&jobID)
	if err != nil {
		if isUniqueViolation(err) {
			return ChannelClientBindingRepairJobView{}, ErrConflict
		}
		return ChannelClientBindingRepairJobView{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	return s.GetChannelClientBindingRepairJob(ctx, accountID, jobID)
}
