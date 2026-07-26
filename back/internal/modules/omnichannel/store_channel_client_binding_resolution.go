package omnichannel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// resolveInboundChannelClientBindingTx resolves the binding valid at the
// provider timestamp. It never blocks persistence: absence becomes unresolved
// and ambiguity/invalid client becomes quarantined. The AI eligibility gate
// decides later whether the current account mode permits automation.
func (s *Store) resolveInboundChannelClientBindingTx(
	ctx context.Context,
	tx pgx.Tx,
	w inboundWrite,
) (channelBindingSnapshot, error) {
	var mode string
	err := tx.QueryRow(ctx, `
		select coalesce(channel_binding_mode, 'shadow')
		from messaging.account_config
		where account_id = $1::uuid`,
		w.AccountID,
	).Scan(&mode)
	if errors.Is(err, pgx.ErrNoRows) {
		mode = "shadow"
	} else if err != nil {
		return channelBindingSnapshot{}, err
	}
	if mode == "legacy" {
		return channelBindingSnapshot{State: "unresolved"}, nil
	}

	channel := strings.ToUpper(strings.TrimSpace(w.Message.Channel))
	resourceID := strings.TrimSpace(w.InstanceID)
	if channel == "INSTAGRAM" {
		err = tx.QueryRow(ctx, `
			select id::text
			from messaging.instagram_accounts
			where account_id = $1::uuid and ig_user_id = $2 and is_active = true`,
			w.AccountID, strings.TrimSpace(w.InstanceName),
		).Scan(&resourceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return channelBindingSnapshot{State: "unresolved"}, nil
		}
		if err != nil {
			return channelBindingSnapshot{}, err
		}
	}
	if resourceID == "" {
		return channelBindingSnapshot{State: "unresolved"}, nil
	}

	rows, err := tx.Query(ctx, `
		select
			b.client_account_id::text,
			b.id::text,
			(
			  client.is_active = true
			  and (
			    owner.id = client.id
			    or (
			      owner.is_agency = true
			      and client.is_agency = false
			      and owner.organization_id is not null
			      and client.organization_id = owner.organization_id
			    )
			  )
			) as client_valid
		from messaging.channel_client_bindings b
		join core.accounts owner on owner.id = b.account_id and owner.is_active = true
		join core.accounts client on client.id = b.client_account_id
		where b.account_id = $1::uuid
		  and b.channel = $2
		  and (
		    ($2 = 'WHATSAPP' and b.whatsapp_instance_id = $3::uuid)
		    or ($2 = 'INSTAGRAM' and b.instagram_account_id = $3::uuid)
		  )
		  and b.effective_from <= $4
		  and (b.effective_to is null or b.effective_to > $4)
		order by b.effective_from desc, b.id desc
		limit 2`,
		w.AccountID,
		channel,
		resourceID,
		w.Message.OccurredAt,
	)
	if err != nil {
		return channelBindingSnapshot{}, err
	}
	defer rows.Close()

	type candidate struct {
		clientID  string
		bindingID string
		valid     bool
	}
	candidates := make([]candidate, 0, 2)
	for rows.Next() {
		var item candidate
		if err = rows.Scan(&item.clientID, &item.bindingID, &item.valid); err != nil {
			return channelBindingSnapshot{}, err
		}
		candidates = append(candidates, item)
	}
	if err = rows.Err(); err != nil {
		return channelBindingSnapshot{}, err
	}
	if len(candidates) == 0 {
		return channelBindingSnapshot{State: "unresolved"}, nil
	}
	if len(candidates) > 1 {
		return channelBindingSnapshot{State: "quarantined"}, nil
	}
	boundAt := time.Now().UTC()
	if !candidates[0].valid {
		return channelBindingSnapshot{
			ClientAccountID: candidates[0].clientID,
			BindingID:       candidates[0].bindingID,
			State:           "quarantined",
			BoundAt:         &boundAt,
		}, nil
	}
	return channelBindingSnapshot{
		ClientAccountID: candidates[0].clientID,
		BindingID:       candidates[0].bindingID,
		State:           "resolved",
		BoundAt:         &boundAt,
	}, nil
}
