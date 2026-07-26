package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (s *Store) ApplyChannelClientBindingRepair(
	ctx context.Context,
	accountID string,
	p auth.Principal,
	in ChannelClientBindingRepairApplyInput,
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

	var (
		bindingID       string
		previewChecksum string
		previewStatus   string
		watermark       time.Time
		filtersRaw      []byte
		scanned         int64
		eligible        int64
	)
	err = tx.QueryRow(ctx, `
		select
			binding_id::text,
			preview_checksum,
			status,
			watermark,
			filters,
			scanned_count,
			eligible_count
		from messaging.channel_client_binding_repair_jobs
		where account_id = $1::uuid and id = $2::uuid and mode = 'preview'
		for update`,
		accountID, in.PreviewID,
	).Scan(
		&bindingID,
		&previewChecksum,
		&previewStatus,
		&watermark,
		&filtersRaw,
		&scanned,
		&eligible,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChannelClientBindingRepairJobView{}, ErrNotFound
	}
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	if previewStatus != "completed" || previewChecksum != in.PreviewChecksum {
		return ChannelClientBindingRepairJobView{}, ErrConflict
	}
	binding, err := loadRepairBinding(ctx, tx, accountID, bindingID)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	var filters struct {
		IncludeClosed bool `json:"includeClosed"`
	}
	if err = json.Unmarshal(filtersRaw, &filters); err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}

	var jobID string
	err = tx.QueryRow(ctx, `
		insert into messaging.channel_client_binding_repair_jobs (
			account_id, channel, whatsapp_instance_id, instagram_account_id,
			client_account_id, binding_id, mode, status, filters, watermark,
			preview_job_id, preview_checksum, idempotency_key, request_hash,
			scanned_count, eligible_count, actor_user_id, reason, started_at
		)
		values (
			$1::uuid,
			$2,
			case when $2 = 'WHATSAPP' then $3::uuid else null end,
			case when $2 = 'INSTAGRAM' then $3::uuid else null end,
			$4::uuid,
			$5::uuid,
			'apply',
			'processing',
			$6::jsonb,
			$7,
			$8::uuid,
			$9,
			$10,
			$11,
			$12,
			$13,
			nullif($14, '')::uuid,
			$15,
			now()
		)
		returning id::text`,
		accountID,
		binding.Channel,
		binding.ResourceID,
		binding.ClientAccountID,
		bindingID,
		filtersRaw,
		watermark,
		in.PreviewID,
		in.PreviewChecksum,
		in.IdempotencyKey,
		requestHash,
		scanned,
		eligible,
		p.UserID,
		in.Reason,
	).Scan(&jobID)
	if err != nil {
		if isUniqueViolation(err) {
			return ChannelClientBindingRepairJobView{}, ErrConflict
		}
		return ChannelClientBindingRepairJobView{}, err
	}

	updateQuery := fmt.Sprintf(`
		with eligible_rows as (
			select c.id
			from messaging.conversations c
			where %s
			  and not exists (
			    select 1
			    from messaging.messages m
			    where m.account_id = c.account_id
			      and m.conversation_id = c.id
			      and m.direction = 'OUTBOUND'
			      and m.origin <> 'ai'
			  )
			for update skip locked
		)
		update messaging.conversations c
		set client_account_id = $9::uuid,
		    channel_client_binding_id = $2::uuid,
		    client_binding_state = 'resolved',
		    client_bound_at = now(),
		    updated_at = now()
		from eligible_rows e
		where c.account_id = $1::uuid and c.id = e.id`, repairEligibilityPredicate())
	tag, err := tx.Exec(ctx, updateQuery,
		accountID,
		bindingID,
		binding.Channel,
		binding.ResourceID,
		binding.EffectiveFrom,
		binding.EffectiveTo,
		watermark,
		filters.IncludeClosed,
		binding.ClientAccountID,
	)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	repaired := tag.RowsAffected()

	_, err = tx.Exec(ctx, `
		update messaging.contact_touchpoints t
		set client_account_id = c.client_account_id,
		    channel_client_binding_id = c.channel_client_binding_id,
		    client_binding_state = c.client_binding_state
		from messaging.conversations c
		where t.account_id = $1::uuid
		  and c.account_id = t.account_id
		  and c.id = t.conversation_id
		  and c.channel_client_binding_id = $2::uuid
		  and t.client_binding_state in ('unresolved', 'quarantined')`,
		accountID, bindingID,
	)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}

	status := "completed"
	if repaired < eligible {
		status = "partial"
	}
	_, err = tx.Exec(ctx, `
		update messaging.channel_client_binding_repair_jobs
		set status = $3,
		    repaired_count = $4,
		    skipped_count = greatest(scanned_count - $4, 0),
		    completed_at = now(),
		    updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`,
		accountID, jobID, status, repaired,
	)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	return s.GetChannelClientBindingRepairJob(ctx, accountID, jobID)
}

type repairJobRowScanner interface {
	Scan(...any) error
}

func scanRepairJob(row repairJobRowScanner) (ChannelClientBindingRepairJobView, error) {
	var out ChannelClientBindingRepairJobView
	err := row.Scan(
		&out.ID,
		&out.Channel,
		&out.ChannelResourceID,
		&out.ClientAccountID,
		&out.BindingID,
		&out.Mode,
		&out.Status,
		&out.Watermark,
		&out.PreviewJobID,
		&out.PreviewChecksum,
		&out.ScannedCount,
		&out.EligibleCount,
		&out.RepairedCount,
		&out.QuarantinedCount,
		&out.SkippedCount,
		&out.LastErrorCode,
		&out.CreatedAt,
		&out.StartedAt,
		&out.CompletedAt,
		&out.UpdatedAt,
	)
	return out, err
}

func getRepairJobWithQuerier(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, accountID, jobID string) (ChannelClientBindingRepairJobView, error) {
	out, err := scanRepairJob(querier.QueryRow(ctx, `
		select
			id::text,
			channel,
			coalesce(whatsapp_instance_id, instagram_account_id)::text,
			client_account_id::text,
			binding_id::text,
			mode,
			status,
			watermark,
			preview_job_id::text,
			preview_checksum,
			scanned_count,
			eligible_count,
			repaired_count,
			quarantined_count,
			skipped_count,
			last_error_code,
			created_at,
			started_at,
			completed_at,
			updated_at
		from messaging.channel_client_binding_repair_jobs
		where account_id = $1::uuid and id = $2::uuid`,
		accountID, jobID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return ChannelClientBindingRepairJobView{}, ErrNotFound
	}
	return out, err
}

func (s *Store) GetChannelClientBindingRepairJob(ctx context.Context, accountID, jobID string) (ChannelClientBindingRepairJobView, error) {
	return getRepairJobWithQuerier(ctx, s.pool, accountID, jobID)
}
