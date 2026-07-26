package omnichannel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func isWhatsAppGroupExternalID(externalID string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(externalID)), "@g.us")
}

// aiOutboundAllowedTx is the final fail-closed gate for every AI-originated message.
// It runs while the conversation is locked. A profile change invalidates queued work in the
// same transaction, and this check guarantees that no work survives after that commit.
func aiOutboundAllowedTx(ctx context.Context, tx pgx.Tx, accountID, conversationID string) (bool, error) {
	var externalID string
	var instanceID *string
	var bindingState, bindingMode string
	if err := tx.QueryRow(ctx, `
		select
			c.external_id,
			c.instance_id::text,
			c.client_binding_state,
			coalesce(ac.channel_binding_mode, 'shadow')
		from messaging.conversations c
		left join messaging.account_config ac on ac.account_id = c.account_id
		where c.account_id=$1::uuid and c.id=$2::uuid`,
		accountID, conversationID,
	).Scan(&externalID, &instanceID, &bindingState, &bindingMode); err != nil {
		return false, err
	}
	if instanceID == nil || strings.TrimSpace(*instanceID) == "" || isWhatsAppGroupExternalID(externalID) {
		return false, nil
	}
	if bindingMode == "enforced" && bindingState != "resolved" {
		return false, nil
	}

	var allowed bool
	err := tx.QueryRow(ctx, `select true
		from messaging.automation_profiles p
		join messaging.whatsapp_instances wi
		  on wi.account_id=p.account_id and wi.id=p.whatsapp_instance_id
		join messaging.ai_agents aa
		  on aa.account_id=p.account_id and aa.id=p.ai_agent_id
		where p.account_id=$1::uuid and p.whatsapp_instance_id=$2::uuid
		  and p.enabled and wi.is_active and aa.enabled and aa.active_version_id is not null`,
		accountID, *instanceID).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return allowed, err
}

func invalidateAutomationInstanceTx(ctx context.Context, tx pgx.Tx, accountID, instanceID, reason string, dispatchV2 bool) error {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return nil
	}
	if reason == "" {
		reason = "automation_disabled"
	}

	// ai_generation is the lease for every AI action. Moving ai_active to routing also
	// prevents the UI/runtime from continuing to present an automation that is now off.
	if _, err := tx.Exec(ctx, `update messaging.conversations
		set state=case when state='ai_active' then 'routing' else state end,
		    ai_generation=ai_generation+1, updated_at=now()
		where account_id=$1::uuid and instance_id=$2::uuid`, accountID, instanceID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update messaging.outbox o
		set status='dead', last_error=$3, locked_at=null, locked_by='', updated_at=now()
		from messaging.messages m
		where o.account_id=$1::uuid and o.kind=$4 and o.status in ('pending','processing')
		  and m.account_id=$1::uuid and m.instance_id=$2::uuid and m.origin='ai'
		  and m.status='PENDING' and o.payload->>'messageId'=m.id::text`,
		accountID, instanceID, reason, OutboundJobKind); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update messaging.messages
		set status='FAILED', provider_error_code=$3, updated_at=now()
		where account_id=$1::uuid and instance_id=$2::uuid
		  and origin='ai' and status='PENDING'`, accountID, instanceID, reason); err != nil {
		return err
	}
	if dispatchV2 {
		_, err := tx.Exec(ctx, `update messaging.ai_dispatches d
			set status='cancelled', last_error=$3, locked_at=null, updated_at=now()
			from messaging.conversations c
			where d.account_id=$1::uuid and d.status in ('buffering','queued','processing')
			  and c.account_id=d.account_id and c.id=d.conversation_id
			  and c.instance_id=$2::uuid`, accountID, instanceID, reason)
		return err
	}
	return nil
}
