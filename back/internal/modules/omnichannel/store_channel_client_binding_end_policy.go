package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) EndChannelClientBinding(
	ctx context.Context,
	accountID, bindingID, requestHash string,
	in EndChannelClientBindingInput,
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

	var channel, clientAccountID, resourceID string
	var effectiveFrom time.Time
	var effectiveTo *time.Time
	var revision int64
	var resourceActive bool
	err = tx.QueryRow(ctx, `
		select
			b.channel,
			b.client_account_id::text,
			coalesce(b.whatsapp_instance_id, b.instagram_account_id)::text,
			b.effective_from,
			b.effective_to,
			b.revision,
			case
			  when b.channel = 'WHATSAPP' then coalesce(wi.is_active, false)
			  else coalesce(ia.is_active, false)
			end
		from messaging.channel_client_bindings b
		left join messaging.whatsapp_instances wi
		  on wi.account_id = b.account_id and wi.id = b.whatsapp_instance_id
		left join messaging.instagram_accounts ia
		  on ia.account_id = b.account_id and ia.id = b.instagram_account_id
		where b.account_id = $1::uuid and b.id = $2::uuid
		for update of b`,
		accountID, bindingID,
	).Scan(&channel, &clientAccountID, &resourceID, &effectiveFrom, &effectiveTo, &revision, &resourceActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if resourceActive || effectiveTo != nil || revision != in.ExpectedRevision || !in.EffectiveAt.After(effectiveFrom) {
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

	snapshot, _ := json.Marshal(map[string]any{
		"bindingId":        bindingID,
		"clientAccountId":  clientAccountID,
		"channel":          channel,
		"resourceId":       resourceID,
		"effectiveAt":      in.EffectiveAt,
		"expectedRevision": in.ExpectedRevision,
	})
	_, err = tx.Exec(ctx, `
		insert into messaging.channel_client_binding_events (
			account_id, binding_id, event_type, reason, idempotency_key,
			request_hash, actor_user_id, snapshot
		)
		values (
			$1::uuid, $2::uuid, 'ended', $3, $4, $5,
			nullif($6, '')::uuid, $7::jsonb
		)`,
		accountID,
		bindingID,
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
	return bindingID, tx.Commit(ctx)
}

func (s *Store) GetChannelClientBindingPolicy(ctx context.Context, accountID string) (ChannelClientBindingPolicyView, error) {
	var out ChannelClientBindingPolicyView
	err := s.pool.QueryRow(ctx, `
		select
			coalesce(channel_binding_mode, 'shadow'),
			coalesce(customer_intelligence_mode, 'off'),
			coalesce(customer_intelligence_failure_policy, 'retry_then_handoff'),
			coalesce(integration_policy_revision, 1),
			updated_at
		from messaging.account_config
		where account_id = $1::uuid`,
		accountID,
	).Scan(
		&out.ChannelBindingMode,
		&out.CustomerIntelligenceMode,
		&out.CustomerIntelligenceFailurePolicy,
		&out.Revision,
		&out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChannelClientBindingPolicyView{}, ErrNotFound
	}
	return out, err
}

func (s *Store) UpdateChannelClientBindingPolicy(
	ctx context.Context,
	accountID string,
	in ChannelClientBindingPolicyInput,
) (ChannelClientBindingPolicyView, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ChannelClientBindingPolicyView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if in.CustomerIntelligenceMode != "off" {
		var dependenciesEnabled bool
		err = tx.QueryRow(ctx, `
			select coalesce(bool_and(coalesce(am.enabled, false)), false)
			from (values ('customer_data'), ('customer_intelligence')) required(module_id)
			left join core.account_modules am
			  on am.account_id = $1::uuid and am.module_id = required.module_id`,
			accountID,
		).Scan(&dependenciesEnabled)
		if err != nil {
			return ChannelClientBindingPolicyView{}, err
		}
		if !dependenciesEnabled {
			return ChannelClientBindingPolicyView{}, ErrConflict
		}
	}

	if in.ChannelBindingMode == "enforced" {
		var unresolvedResourceExists bool
		err = tx.QueryRow(ctx, `
			select exists (
				select 1
				from messaging.whatsapp_instances wi
				where wi.account_id = $1::uuid and wi.is_active = true
				  and not exists (
				    select 1 from messaging.channel_client_bindings b
				    where b.account_id = wi.account_id
				      and b.channel = 'WHATSAPP'
				      and b.whatsapp_instance_id = wi.id
				      and b.effective_to is null
				  )
				union all
				select 1
				from messaging.instagram_accounts ia
				where ia.account_id = $1::uuid and ia.is_active = true
				  and not exists (
				    select 1 from messaging.channel_client_bindings b
				    where b.account_id = ia.account_id
				      and b.channel = 'INSTAGRAM'
				      and b.instagram_account_id = ia.id
				      and b.effective_to is null
				  )
			)`,
			accountID,
		).Scan(&unresolvedResourceExists)
		if err != nil {
			return ChannelClientBindingPolicyView{}, err
		}
		if unresolvedResourceExists {
			return ChannelClientBindingPolicyView{}, ErrConflict
		}
	}

	tag, err := tx.Exec(ctx, `
		update messaging.account_config
		set channel_binding_mode = $2,
		    customer_intelligence_mode = $3,
		    customer_intelligence_failure_policy = $4,
		    integration_policy_revision = integration_policy_revision + 1,
		    updated_at = now()
		where account_id = $1::uuid
		  and integration_policy_revision = $5`,
		accountID,
		in.ChannelBindingMode,
		in.CustomerIntelligenceMode,
		in.CustomerIntelligenceFailurePolicy,
		in.ExpectedRevision,
	)
	if err != nil {
		return ChannelClientBindingPolicyView{}, err
	}
	if tag.RowsAffected() != 1 {
		return ChannelClientBindingPolicyView{}, ErrConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return ChannelClientBindingPolicyView{}, err
	}
	return s.GetChannelClientBindingPolicy(ctx, accountID)
}
