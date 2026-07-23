package omnichannel

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (s *Store) ListAutomationInterventions(ctx context.Context, accountID, clientID string, limit int) ([]automationInterventionRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `select h.id::text,p.client_account_id::text,
		h.conversation_id::text,coalesce(c.contact_name,''),coalesce(c.contact_phone,''),
		p.whatsapp_instance_id::text,wi.instance_name,h.reason_code,h.summary,h.collected_fields,
		h.status,c.state,h.target_queue_id::text,h.requested_at
		from messaging.handoffs h
		join messaging.conversations c
		  on c.account_id=h.account_id and c.id=h.conversation_id
		join messaging.automation_profiles p
		  on p.account_id=c.account_id and p.whatsapp_instance_id=c.instance_id
		join messaging.whatsapp_instances wi
		  on wi.account_id=p.account_id and wi.id=p.whatsapp_instance_id
		where h.account_id=$1::uuid and h.status in ('requested','queued')
		  and not exists (select 1 from messaging.contact_suppressions suppression
		      where suppression.account_id=c.account_id and suppression.contact_id=c.contact_id
		        and suppression.is_hidden=true)
		  and ($2='' or p.client_account_id=nullif($2,'')::uuid)
		order by h.requested_at desc,h.id desc limit $3`, accountID, strings.TrimSpace(clientID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]automationInterventionRow, 0)
	for rows.Next() {
		var item automationInterventionRow
		if err := rows.Scan(&item.ID, &item.ClientAccountID, &item.ConversationID,
			&item.ContactName, &item.ContactPhone, &item.WhatsAppInstanceID, &item.InstanceName,
			&item.ReasonCode, &item.Summary, &item.CollectedFields, &item.Status,
			&item.ConversationState, &item.TargetQueueID, &item.WaitingSince); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *AutomationService) ListInterventions(ctx context.Context, accountID string, p auth.Principal, clientID string, limit int) ([]AutomationInterventionView, error) {
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
	rows, err := s.store.ListAutomationInterventions(ctx, accountID, clientID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]AutomationInterventionView, 0, len(rows))
	for _, row := range rows {
		client, allowed := byID[row.ClientAccountID]
		if !allowed {
			continue
		}
		out = append(out, AutomationInterventionView{
			ID: row.ID, Client: client, ConversationID: row.ConversationID,
			ContactName: row.ContactName, ContactPhone: row.ContactPhone,
			WhatsAppInstanceID: row.WhatsAppInstanceID, InstanceName: row.InstanceName,
			ReasonCode: row.ReasonCode, Summary: row.Summary,
			CollectedFieldKeys: collectedFieldKeys(row.CollectedFields), Status: row.Status,
			ConversationState: row.ConversationState, TargetQueueID: row.TargetQueueID,
			WaitingSince: row.WaitingSince,
		})
	}
	return out, nil
}

func collectedFieldKeys(raw json.RawMessage) []string {
	values := map[string]any{}
	if len(raw) == 0 || json.Unmarshal(raw, &values) != nil {
		return []string{}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if key = strings.TrimSpace(key); key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
