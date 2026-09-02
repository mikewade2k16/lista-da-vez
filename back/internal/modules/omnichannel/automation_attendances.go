package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const (
	automationAttendanceAIActive  = "ai_active"
	automationAttendanceAIStopped = "ai_stopped"
	automationAttendanceHuman     = "human_active"
)

// ListAutomationAttendances projects the operational cards from authoritative
// conversation, dispatch, handoff and message state. It deliberately returns
// only the pending message preview; full history remains on the inbox API.
func (s *Store) ListAutomationAttendances(ctx context.Context, accountID, clientID string, limit int) ([]automationAttendanceRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `select c.id::text,p.client_account_id::text,
		coalesce(c.contact_name,''),coalesce(c.contact_phone,''),p.whatsapp_instance_id::text,
		wi.instance_name,c.state,coalesce(dispatch.status,''),handoff.id::text,
		coalesce(handoff.reason_code,''),coalesce(handoff.summary,''),
		coalesce(ai_run.status,''),coalesce(ai_run.error,''),ai_run.confidence,
		ai_run.min_confidence,ai_run.max_ai_turns,
		coalesce(pending.unanswered_count,0),coalesce(pending.preview,''),pending.pending_since,
		case when c.state='ai_active' then c.updated_at else coalesce(handoff.requested_at,c.updated_at) end
		from messaging.conversations c
		join messaging.automation_profiles p
		  on p.account_id=c.account_id and p.whatsapp_instance_id=c.instance_id
		 and p.client_account_id=c.client_account_id
		join messaging.channel_client_bindings binding
		  on binding.account_id=p.account_id
		 and binding.client_account_id=p.client_account_id
		 and binding.whatsapp_instance_id=p.whatsapp_instance_id
		 and binding.channel='WHATSAPP'
		 and binding.effective_from <= now()
		 and (binding.effective_to is null or binding.effective_to > now())
		join messaging.whatsapp_instances wi
		  on wi.account_id=p.account_id and wi.id=p.whatsapp_instance_id
		left join lateral (
			select h.id,h.reason_code,h.summary,h.requested_at
			from messaging.handoffs h
			where h.account_id=c.account_id and h.conversation_id=c.id
			  and h.status in ('requested','queued')
			  and h.requested_at > `+s.effectiveHistoryCutoffExpression("c")+`
			order by h.requested_at desc,h.id desc limit 1
		) handoff on true
		left join lateral (
			select d.status
			from messaging.ai_dispatches d
			where d.account_id=c.account_id and d.conversation_id=c.id
			  and d.generation=c.ai_generation
			  and not exists (select 1 from unnest(d.message_ids) captured(message_id)
				where not exists (select 1 from messaging.messages dispatch_message
					where dispatch_message.account_id=d.account_id
					  and dispatch_message.conversation_id=d.conversation_id
					  and dispatch_message.id=captured.message_id`+
		s.historyVisibleMessagePredicate("dispatch_message", "c")+`))
			order by d.created_at desc,d.id desc limit 1
		) dispatch on true
		left join lateral (
			select r.status,coalesce(r.error,'') as error,
			       nullif(coalesce(r.output->>'Confidence',r.output->>'confidence'),'')::float8 as confidence,
			       av.min_confidence::float8 as min_confidence,av.max_ai_turns
			from messaging.ai_runs r
			join messaging.ai_agent_versions av
			  on av.account_id=r.account_id and av.id=r.agent_version_id
			where r.account_id=c.account_id and r.conversation_id=c.id
			  and exists (select 1 from messaging.messages run_message
				where run_message.account_id=r.account_id and run_message.conversation_id=c.id
				  and run_message.id=r.message_id`+s.historyVisibleMessagePredicate("run_message", "c")+`)
			order by r.created_at desc,r.id desc limit 1
		) ai_run on true
		left join lateral (
			select count(*)::bigint as unanswered_count,
			       (array_agg(left(m.content,500) order by m.created_at desc,m.id desc))[1] as preview,
			       min(m.created_at) as pending_since
			from messaging.messages m
			where m.account_id=c.account_id and m.conversation_id=c.id and m.direction='INBOUND'
			`+s.historyVisibleMessagePredicate("m", "c")+`
			  and m.created_at > coalesce((
				select max(answer.created_at) from messaging.messages answer
				where answer.account_id=c.account_id and answer.conversation_id=c.id
				  and answer.direction='OUTBOUND' and answer.status <> 'FAILED'`+
		s.historyVisibleMessagePredicate("answer", "c")+`
			  ),'-infinity'::timestamptz)
		) pending on true
		where c.account_id=$1::uuid`+s.historyVisibleConversationPredicate("c")+`
		  and not exists (select 1 from messaging.contact_suppressions suppression
		      where suppression.account_id=c.account_id and suppression.contact_id=c.contact_id
		        and suppression.is_hidden=true)
		  and (c.state in ('ai_active','human_active') or handoff.id is not null)
		  and ($2='' or p.client_account_id=nullif($2,'')::uuid)
		order by (case when c.state='ai_active' then c.updated_at else coalesce(handoff.requested_at,c.updated_at) end) desc,c.id desc
		limit $3`, accountID, strings.TrimSpace(clientID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]automationAttendanceRow, 0)
	for rows.Next() {
		var item automationAttendanceRow
		if err := rows.Scan(&item.ConversationID, &item.ClientAccountID, &item.ContactName,
			&item.ContactPhone, &item.WhatsAppInstanceID, &item.InstanceName,
			&item.ConversationState, &item.DispatchStatus, &item.HandoffID, &item.ReasonCode,
			&item.Summary, &item.AIRunStatus, &item.AIRunError, &item.AIConfidence,
			&item.MinimumConfidence, &item.MaxAITurns, &item.UnansweredCount, &item.PendingMessagePreview,
			&item.PendingSince, &item.ActivitySince); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) AutomationConversationScope(ctx context.Context, accountID, conversationID string) (automationConversationScope, error) {
	var out automationConversationScope
	err := s.pool.QueryRow(ctx, `select p.client_account_id::text,c.state
		from messaging.conversations c
		join messaging.automation_profiles p
		  on p.account_id=c.account_id and p.whatsapp_instance_id=c.instance_id
		 and p.client_account_id=c.client_account_id
		join messaging.channel_client_bindings binding
		  on binding.account_id=p.account_id
		 and binding.client_account_id=p.client_account_id
		 and binding.whatsapp_instance_id=p.whatsapp_instance_id
		 and binding.channel='WHATSAPP'
		 and binding.effective_from <= now()
		 and (binding.effective_to is null or binding.effective_to > now())
		where c.account_id=$1::uuid and c.id=$2::uuid`+s.historyVisibleConversationPredicate("c")+`
		  and not exists (select 1 from messaging.contact_suppressions suppression
		      where suppression.account_id=c.account_id and suppression.contact_id=c.contact_id
		        and suppression.is_hidden=true)`, accountID, conversationID).
		Scan(&out.ClientAccountID, &out.ConversationState)
	return out, translate(err)
}

func (s *AutomationService) ListAttendances(ctx context.Context, accountID string, p auth.Principal, clientID string, limit int) ([]AutomationAttendanceView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return nil, err
	}
	clients, err := s.accessibleClients(ctx, p)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]AutomationClientRef, len(clients))
	for _, client := range clients {
		byID[client.ID] = client
	}
	clientID = strings.TrimSpace(clientID)
	if clientID != "" {
		if !omnichannelUUIDPattern.MatchString(clientID) {
			return nil, ErrNotFound
		}
		if _, allowed := byID[clientID]; !allowed {
			return nil, ErrNotFound
		}
	}
	rows, err := s.store.ListAutomationAttendances(ctx, accountID, clientID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]AutomationAttendanceView, 0, len(rows))
	for _, row := range rows {
		client, allowed := byID[row.ClientAccountID]
		if !allowed {
			continue
		}
		if err := s.permissions.assertConversationAccess(ctx, accountID, p.UserID, row.ConversationID,
			"omnichannel.agents.manage", InstanceGrantManage); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}
		mode := automationAttendanceAIStopped
		if row.ConversationState == string(StateAIActive) {
			mode = automationAttendanceAIActive
		} else if row.ConversationState == string(StateHumanActive) {
			mode = automationAttendanceHuman
		}
		reasonCode, summary := normalizedAutomationAttendanceReason(row)
		out = append(out, AutomationAttendanceView{
			ID: row.ConversationID, Mode: mode, Client: client,
			ConversationID: row.ConversationID, ContactName: row.ContactName,
			ContactPhone: row.ContactPhone, WhatsAppInstanceID: row.WhatsAppInstanceID,
			InstanceName: row.InstanceName, ConversationState: row.ConversationState,
			DispatchStatus: row.DispatchStatus, HandoffID: row.HandoffID,
			ReasonCode: reasonCode, Summary: summary, AIConfidence: row.AIConfidence,
			MinimumConfidence: row.MinimumConfidence, MaxAITurns: row.MaxAITurns,
			UnansweredCount: row.UnansweredCount, PendingMessagePreview: row.PendingMessagePreview,
			PendingSince: row.PendingSince, ActivitySince: row.ActivitySince,
		})
	}
	return out, nil
}

func normalizedAutomationAttendanceReason(row automationAttendanceRow) (string, string) {
	reasonCode, summary := row.ReasonCode, row.Summary
	if row.ConversationState == string(StateHumanActive) && reasonCode == "" {
		return "human_active", "O atendimento está sob controle humano. Você pode transferi-lo novamente para a IA."
	}
	// Older dispatches collapsed every limit into low_confidence. Recover the
	// truthful reason from the authoritative run so existing cards are corrected.
	if row.AIRunStatus == runLimitExceeded && row.AIRunError == "max_ai_turns" {
		reasonCode = HandoffReasonMaxTurns
		summary = defaultAIHandoffSummary(reasonCode)
	}
	return reasonCode, summary
}

func (s *AutomationService) PauseAI(ctx context.Context, accountID string, p auth.Principal, conversationID string, in AutomationActionInput) (AutomationActionResult, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return AutomationActionResult{}, err
	}
	if err := normalizeAutomationAction(conversationID, &in); err != nil {
		return AutomationActionResult{}, err
	}
	scope, err := s.authorizeAutomationConversation(ctx, accountID, p, conversationID)
	if err != nil {
		return AutomationActionResult{}, err
	}
	if scope.ConversationState != string(StateAIActive) {
		return AutomationActionResult{}, ErrConflict
	}
	if s.domain == nil {
		return AutomationActionResult{}, ErrAutomationNotReady
	}
	_, err = s.domain.RequestAutomationHandoff(ctx, accountID, conversationID, p.UserID, HandoffRequest{
		ReasonCode:      HandoffReasonOperatorPause,
		Summary:         "Atendimento da IA pausado manualmente pelo operador.",
		CollectedFields: json.RawMessage(`{}`),
		IdempotencyKey:  "operator-pause:" + in.IdempotencyKey,
	})
	if err != nil {
		return AutomationActionResult{}, err
	}
	return AutomationActionResult{ConversationID: conversationID, State: string(StateQueued)}, nil
}

func (s *AutomationService) ReplyWithAI(ctx context.Context, accountID string, p auth.Principal, conversationID string, in AutomationActionInput) (AutomationActionResult, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return AutomationActionResult{}, err
	}
	if err := normalizeAutomationAction(conversationID, &in); err != nil {
		return AutomationActionResult{}, err
	}
	if _, err := s.authorizeAutomationConversation(ctx, accountID, p, conversationID); err != nil {
		return AutomationActionResult{}, err
	}
	dispatch, err := s.store.ReplayAutomationWithAI(ctx, accountID, conversationID, p.UserID, in.IdempotencyKey)
	if err != nil {
		return AutomationActionResult{}, err
	}
	return AutomationActionResult{ConversationID: conversationID, State: string(StateAIActive), DispatchID: dispatch.ID}, nil
}

func normalizeAutomationAction(conversationID string, in *AutomationActionInput) error {
	conversationID = strings.TrimSpace(conversationID)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if !omnichannelUUIDPattern.MatchString(conversationID) || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 80 {
		return ErrValidation
	}
	return nil
}

func (s *AutomationService) authorizeAutomationConversation(ctx context.Context, accountID string, p auth.Principal, conversationID string) (automationConversationScope, error) {
	if s.permissions == nil {
		return automationConversationScope{}, ErrForbidden
	}
	if err := s.permissions.assertConversationAccess(ctx, accountID, p.UserID, conversationID,
		"omnichannel.agents.manage", InstanceGrantManage); err != nil {
		return automationConversationScope{}, err
	}
	scope, err := s.store.AutomationConversationScope(ctx, accountID, conversationID)
	if err != nil {
		return automationConversationScope{}, err
	}
	clients, err := s.accessibleClients(ctx, p)
	if err != nil {
		return automationConversationScope{}, err
	}
	for _, client := range clients {
		if client.ID == scope.ClientAccountID {
			return scope, nil
		}
	}
	return automationConversationScope{}, ErrNotFound
}
