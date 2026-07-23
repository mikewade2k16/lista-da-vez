package omnichannel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const automationProfileCols = `p.id::text, p.client_account_id::text,
	p.whatsapp_instance_id::text, p.ai_agent_id::text, p.enabled,
	p.auto_close_enabled, p.auto_close_min_confidence::float8,
	p.auto_close_require_all_fields, p.auto_close_block_human_request,
	p.auto_close_block_sensitive, p.revision, p.created_at, p.updated_at,
	wi.instance_name, wi.provider, wi.display_name, wi.phone_number, wi.is_active,
	aa.name, aa.enabled, aa.active_version_id::text,
	(aa.enabled and aa.active_version_id is not null
	 and aa.provider_key_ciphertext <> '' and av.provider <> '' and av.model <> '')`

const automationProfileJoins = `
	from messaging.automation_profiles p
	join messaging.whatsapp_instances wi
	  on wi.account_id = p.account_id and wi.id = p.whatsapp_instance_id
	join messaging.ai_agents aa
	  on aa.account_id = p.account_id and aa.id = p.ai_agent_id
	left join messaging.ai_agent_versions av
	  on av.account_id = aa.account_id and av.agent_id = aa.id and av.id = aa.active_version_id`

const automationAgentCols = `aa.id::text, aa.slug, aa.name, aa.enabled,
	aa.active_version_id::text, aa.provider_key_ciphertext, aa.provider_key_last4,
	aa.created_by, aa.created_at, aa.updated_at`

func scanAutomationProfile(row rowScanner) (automationProfileRow, error) {
	var p automationProfileRow
	err := row.Scan(&p.ID, &p.ClientAccountID, &p.WhatsAppInstanceID, &p.AIAgentID,
		&p.Enabled, &p.AutoCloseEnabled, &p.AutoCloseMinConfidence,
		&p.AutoCloseRequireAllFields, &p.AutoCloseBlockHumanRequest,
		&p.AutoCloseBlockSensitive, &p.Revision, &p.CreatedAt, &p.UpdatedAt,
		&p.InstanceName, &p.InstanceProvider, &p.InstanceDisplayName,
		&p.InstancePhoneNumber, &p.InstanceActive, &p.AgentName, &p.AgentEnabled,
		&p.AgentActiveVersionID, &p.AgentReady)
	return p, err
}

func (s *Store) ListAutomationProfiles(ctx context.Context, accountID string) ([]automationProfileRow, error) {
	rows, err := s.pool.Query(ctx, `select `+automationProfileCols+automationProfileJoins+`
		where p.account_id = $1::uuid order by p.updated_at desc, p.id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]automationProfileRow, 0)
	for rows.Next() {
		profile, err := scanAutomationProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, profile)
	}
	return out, rows.Err()
}

func (s *Store) GetAutomationProfile(ctx context.Context, accountID, clientID string) (automationProfileRow, error) {
	return scanAutomationProfile(s.pool.QueryRow(ctx, `select `+automationProfileCols+automationProfileJoins+`
		where p.account_id = $1::uuid and p.client_account_id = $2::uuid`, accountID, clientID))
}

func (s *Store) AutomationBindingReadiness(ctx context.Context, accountID, instanceID, agentID string) (automationBindingReadiness, error) {
	var out automationBindingReadiness
	err := s.pool.QueryRow(ctx, `select
		exists(select 1 from messaging.whatsapp_instances where account_id = $1::uuid and id = $2::uuid),
		exists(select 1 from messaging.ai_agents where account_id = $1::uuid and id = $3::uuid),
		exists(select 1 from messaging.whatsapp_instances where account_id = $1::uuid and id = $2::uuid and is_active),
		exists(select 1 from messaging.ai_agents aa
			join messaging.ai_agent_versions av on av.account_id = aa.account_id
			 and av.agent_id = aa.id and av.id = aa.active_version_id
			where aa.account_id = $1::uuid and aa.id = $3::uuid and aa.enabled
			 and aa.provider_key_ciphertext <> '' and av.provider <> '' and av.model <> '')`,
		accountID, instanceID, agentID).Scan(&out.InstanceFound, &out.AgentFound,
		&out.InstanceReady, &out.AgentReady)
	return out, err
}

func (s *Store) UpsertAutomationProfile(ctx context.Context, accountID, clientID, userID string, in automationProfileWrite) (automationProfileRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return automationProfileRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var previousInstanceID, previousAgentID string
	var previousEnabled bool
	previousFound := true
	err = tx.QueryRow(ctx, `select whatsapp_instance_id::text, ai_agent_id::text, enabled
		from messaging.automation_profiles
		where account_id=$1::uuid and client_account_id=$2::uuid for update`, accountID, clientID).
		Scan(&previousInstanceID, &previousAgentID, &previousEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		previousFound = false
	} else if err != nil {
		return automationProfileRow{}, err
	}

	cmd, err := tx.Exec(ctx, `insert into messaging.automation_profiles
		(account_id, client_account_id, whatsapp_instance_id, ai_agent_id, enabled,
		 auto_close_enabled, auto_close_min_confidence, auto_close_require_all_fields,
		 auto_close_block_human_request, auto_close_block_sensitive,
		 created_by_user_id, updated_by_user_id)
		select $1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8, $9, $10,
		       nullif($11, '')::uuid, nullif($11, '')::uuid
		where exists (select 1 from messaging.whatsapp_instances
		              where account_id = $1::uuid and id = $3::uuid)
		  and exists (select 1 from messaging.ai_agents
		              where account_id = $1::uuid and id = $4::uuid)
		on conflict (account_id, client_account_id) do update set
			whatsapp_instance_id = excluded.whatsapp_instance_id,
			ai_agent_id = excluded.ai_agent_id,
			enabled = excluded.enabled,
			auto_close_enabled = excluded.auto_close_enabled,
			auto_close_min_confidence = excluded.auto_close_min_confidence,
			auto_close_require_all_fields = excluded.auto_close_require_all_fields,
			auto_close_block_human_request = excluded.auto_close_block_human_request,
			auto_close_block_sensitive = excluded.auto_close_block_sensitive,
			updated_by_user_id = excluded.updated_by_user_id,
			revision = messaging.automation_profiles.revision + 1,
			updated_at = now()`, accountID, clientID, in.WhatsAppInstanceID, in.AIAgentID,
		in.Enabled, in.AutoCloseEnabled, in.AutoCloseMinConfidence,
		in.AutoCloseRequireAllFields, in.AutoCloseBlockHumanRequest,
		in.AutoCloseBlockSensitive, userID)
	if err != nil {
		return automationProfileRow{}, err
	}
	if cmd.RowsAffected() == 0 {
		return automationProfileRow{}, pgx.ErrNoRows
	}

	bindingChanged := previousFound && (previousInstanceID != in.WhatsAppInstanceID || previousAgentID != in.AIAgentID)
	if !in.Enabled || bindingChanged {
		if err := invalidateAutomationInstanceTx(ctx, tx, accountID, in.WhatsAppInstanceID, "automation_disabled", s.AIDispatchV2Enabled()); err != nil {
			return automationProfileRow{}, err
		}
	}
	if bindingChanged && previousInstanceID != in.WhatsAppInstanceID {
		if err := invalidateAutomationInstanceTx(ctx, tx, accountID, previousInstanceID, "automation_binding_changed", s.AIDispatchV2Enabled()); err != nil {
			return automationProfileRow{}, err
		}
	}
	if previousFound && previousEnabled && !in.Enabled && previousInstanceID != in.WhatsAppInstanceID {
		if err := invalidateAutomationInstanceTx(ctx, tx, accountID, previousInstanceID, "automation_disabled", s.AIDispatchV2Enabled()); err != nil {
			return automationProfileRow{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return automationProfileRow{}, err
	}
	return s.GetAutomationProfile(ctx, accountID, clientID)
}

// ActiveAgentForInstance aplica o vinculo cliente-numero-agente. O perfil habilitado e a
// instancia ativa sao obrigatorios: nao existe fallback global. O switch da automacao e o
// master kill switch e qualquer ausencia/desativacao falha fechada.
func (s *Store) ActiveAgentForInstance(ctx context.Context, accountID, instanceID string) (agentRow, bool, error) {
	if strings.TrimSpace(instanceID) == "" {
		return agentRow{}, false, nil
	}
	var profileEnabled bool
	agent, err := scanAgentWithPrefix(s.pool.QueryRow(ctx, `select p.enabled, `+automationAgentCols+`
		from messaging.automation_profiles p
		join messaging.whatsapp_instances wi on wi.account_id=p.account_id and wi.id=p.whatsapp_instance_id
		join messaging.ai_agents aa on aa.account_id = p.account_id and aa.id = p.ai_agent_id
		where p.account_id = $1::uuid and p.whatsapp_instance_id = $2::uuid and wi.is_active`,
		accountID, instanceID), &profileEnabled)
	if err == nil {
		if !profileEnabled || !agent.Enabled || agent.ActiveVersionID == nil {
			return agentRow{}, false, nil
		}
		return agent, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return agentRow{}, false, err
	}

	return agentRow{}, false, nil
}

func (s *Store) AutomationClientForInstance(ctx context.Context, accountID, instanceID string) (string, bool, error) {
	var clientID string
	err := s.pool.QueryRow(ctx, `select client_account_id::text
		from messaging.automation_profiles
		where account_id=$1::uuid and whatsapp_instance_id=nullif($2,'')::uuid and enabled`,
		accountID, strings.TrimSpace(instanceID)).Scan(&clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return clientID, true, nil
}

func scanAgentWithPrefix(row rowScanner, enabled *bool) (agentRow, error) {
	var a agentRow
	err := row.Scan(enabled, &a.ID, &a.Slug, &a.Name, &a.Enabled, &a.ActiveVersionID,
		&a.ProviderKeyCipher, &a.ProviderKeyLast4, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}
