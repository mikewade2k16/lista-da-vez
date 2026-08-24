package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type actionProposalInsert struct {
	AccountID            string
	ResourceAccountID    string
	AdAccount            AdAccount
	Action               ActionKind
	Source               ActionProposalSource
	SourceConversationID string
	SourceMessageID      string
	TargetCampaign       *Campaign
	Payload              json.RawMessage
	Summary              string
	RequestHash          string
	IdempotencyKey       string
	CreatedByUserID      string
}

type actionRepository interface {
	CreateActionProposal(context.Context, actionProposalInsert) (ActionProposal, bool, error)
	GetActionProposal(context.Context, string, string) (ActionProposal, error)
	ListActionProposals(context.Context, string, int) ([]ActionProposal, error)
	BindAssistantActionProposal(context.Context, string, string, string, string, string) (ActionProposal, error)
	CancelActionProposal(context.Context, string, string, string, string) (ActionProposal, bool, error)
	CancelAssistantConversationActions(context.Context, string, string, string) (int, error)
	ExpireActionProposal(context.Context, string, string) (ActionProposal, bool, error)
	BeginActionExecution(context.Context, string, string, string, string, bool) (ActionProposal, bool, error)
	CompleteActionExecution(context.Context, string, string, ActionExecutionOutcome) (ActionProposal, error)
	ReconcileActionExecution(context.Context, string, string, string, ActionExecutionOutcome) (ActionProposal, error)
	GetActionPolicy(context.Context, string, string) (ActionPolicy, error)
	UpsertActionPolicy(context.Context, string, AdAccount, string, ActionPolicyInput) (ActionPolicy, error)
}

const actionProposalColumns = `id::text, account_id::text, resource_account_id::text,
	ad_account_id::text, meta_ad_account_id, ad_account_name, currency, action, source,
	source_conversation_id::text, source_message_id::text, target_campaign_id::text,
	source_bound,
	target_meta_campaign_id, payload, summary, request_hash, idempotency_key,
	confirmation_idempotency_key, cancellation_idempotency_key,
	guard_snapshot_version, guard_snapshot_hash, connection_id_snapshot::text,
	connection_revision_snapshot::text, ad_account_client_account_id_snapshot::text,
	ad_account_updated_at_snapshot, ad_account_hash_snapshot,
	policy_configured_snapshot, policy_id_snapshot::text, policy_updated_at_snapshot,
	policy_hash_snapshot, policy_currency_snapshot, policy_max_daily_budget_snapshot,
	policy_max_lifetime_budget_snapshot, policy_allow_create_snapshot,
	policy_allow_duplicate_snapshot, policy_allow_resume_snapshot,
	campaign_synced_at_snapshot, campaign_hash_snapshot, campaign_name_snapshot,
	campaign_status_snapshot, campaign_daily_budget_snapshot,
	campaign_lifetime_budget_snapshot, claimed_connection_id::text,
	claimed_connection_revision::text, status, attempt_count, external_entity_id,
	result_snapshot, error_code, error_message, created_by_user_id::text,
	confirmed_by_user_id::text, confirmed_at, execution_started_at, completed_at,
	reconciled_at, created_at, expires_at, updated_at`

func scanActionProposal(row rowScanner) (ActionProposal, error) {
	var proposal ActionProposal
	err := row.Scan(
		&proposal.ID, &proposal.AccountID, &proposal.ResourceAccountID,
		&proposal.AdAccountID, &proposal.MetaAdAccountID, &proposal.AdAccountName,
		&proposal.Currency, &proposal.Action, &proposal.Source,
		&proposal.SourceConversationID, &proposal.SourceMessageID,
		&proposal.TargetCampaignID, &proposal.SourceBound, &proposal.TargetMetaCampaignID, &proposal.Payload,
		&proposal.Summary, &proposal.RequestHash, &proposal.IdempotencyKey,
		&proposal.ConfirmationIdempotencyKey, &proposal.CancellationIdempotencyKey,
		&proposal.GuardSnapshotVersion, &proposal.GuardSnapshotHash,
		&proposal.ConnectionIDSnapshot, &proposal.ConnectionRevisionSnapshot,
		&proposal.AdAccountClientIDSnapshot, &proposal.AdAccountUpdatedAtSnapshot,
		&proposal.AdAccountHashSnapshot, &proposal.PolicyConfiguredSnapshot,
		&proposal.PolicyIDSnapshot, &proposal.PolicyUpdatedAtSnapshot,
		&proposal.PolicyHashSnapshot, &proposal.PolicyCurrencySnapshot,
		&proposal.PolicyMaxDailySnapshot, &proposal.PolicyMaxLifetimeSnapshot,
		&proposal.PolicyAllowCreateSnapshot, &proposal.PolicyAllowDuplicateSnapshot,
		&proposal.PolicyAllowResumeSnapshot, &proposal.CampaignSyncedAtSnapshot,
		&proposal.CampaignHashSnapshot, &proposal.CampaignNameSnapshot,
		&proposal.CampaignStatusSnapshot, &proposal.CampaignDailySnapshot,
		&proposal.CampaignLifetimeSnapshot, &proposal.ClaimedConnectionID,
		&proposal.ClaimedConnectionRevision, &proposal.Status, &proposal.AttemptCount,
		&proposal.ExternalEntityID, &proposal.ResultSnapshot, &proposal.ErrorCode,
		&proposal.ErrorMessage, &proposal.CreatedByUserID, &proposal.ConfirmedByUserID,
		&proposal.ConfirmedAt, &proposal.ExecutionStartedAt, &proposal.CompletedAt,
		&proposal.ReconciledAt, &proposal.CreatedAt, &proposal.ExpiresAt, &proposal.UpdatedAt,
	)
	return proposal, err
}

func (s *Store) CreateActionProposal(ctx context.Context, input actionProposalInsert) (ActionProposal, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ActionProposal{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, existingErr := scanActionProposal(tx.QueryRow(ctx, `select `+actionProposalColumns+`
		from meta_ads.action_proposals
		where account_id = $1::uuid and idempotency_key = $2
		for share`, input.AccountID, input.IdempotencyKey))
	if existingErr == nil {
		if existing.RequestHash != input.RequestHash {
			return ActionProposal{}, false, ErrActionIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return ActionProposal{}, false, err
		}
		return existing, false, nil
	}
	if !errors.Is(existingErr, pgx.ErrNoRows) {
		return ActionProposal{}, false, existingErr
	}

	var targetCampaignID string
	if input.TargetCampaign != nil {
		targetCampaignID = input.TargetCampaign.ID
	}
	guard, err := captureActionGuardSnapshot(
		ctx, tx, input.AccountID, input.ResourceAccountID, input.AdAccount.ID,
		targetCampaignID, input.RequestHash,
	)
	if err != nil {
		return ActionProposal{}, false, err
	}
	if guard.AdAccount.AccountID != input.ResourceAccountID ||
		guard.AdAccount.ID != input.AdAccount.ID ||
		guard.AdAccount.MetaAdAccountID != input.AdAccount.MetaAdAccountID {
		return ActionProposal{}, false, pgx.ErrNoRows
	}
	targetMetaCampaignID := ""
	if input.TargetCampaign != nil {
		if guard.Campaign == nil || guard.Campaign.ID != input.TargetCampaign.ID ||
			guard.Campaign.MetaCampaignID != input.TargetCampaign.MetaCampaignID {
			return ActionProposal{}, false, pgx.ErrNoRows
		}
		targetMetaCampaignID = guard.Campaign.MetaCampaignID
	}

	var policyID, policyCurrency string
	var policyUpdatedAt *time.Time
	var policyMaxDaily, policyMaxLifetime *float64
	var policyAllowCreate, policyAllowDuplicate, policyAllowResume bool
	policyConfigured := guard.Policy != nil
	if guard.Policy != nil {
		policyID = guard.Policy.ID
		policyCurrency = strings.ToUpper(strings.TrimSpace(guard.Policy.Currency))
		policyUpdatedAt = &guard.Policy.UpdatedAt
		policyMaxDaily = guard.Policy.MaxDailyBudget
		policyMaxLifetime = guard.Policy.MaxLifetimeBudget
		policyAllowCreate = guard.Policy.AllowCreate
		policyAllowDuplicate = guard.Policy.AllowDuplicate
		policyAllowResume = guard.Policy.AllowResume
	}
	var campaignSyncedAt *time.Time
	var campaignName, campaignStatus string
	var campaignDaily, campaignLifetime *float64
	if guard.Campaign != nil {
		campaignSyncedAt = &guard.Campaign.SyncedAt
		campaignName = guard.Campaign.Name
		campaignStatus = guard.Campaign.Status
		campaignDaily = guard.Campaign.DailyBudget
		campaignLifetime = guard.Campaign.LifetimeBudget
	}
	query := `insert into meta_ads.action_proposals
		(account_id, resource_account_id, ad_account_id, meta_ad_account_id,
		 ad_account_name, currency, action, source, source_conversation_id,
		 source_message_id, source_bound, target_campaign_id, target_meta_campaign_id, payload,
		 summary, request_hash, idempotency_key, created_by_user_id,
		 guard_snapshot_version, guard_snapshot_hash, connection_id_snapshot,
		 connection_revision_snapshot, ad_account_client_account_id_snapshot,
		 ad_account_updated_at_snapshot, ad_account_hash_snapshot,
		 policy_configured_snapshot, policy_id_snapshot, policy_updated_at_snapshot,
		 policy_hash_snapshot, policy_currency_snapshot,
		 policy_max_daily_budget_snapshot, policy_max_lifetime_budget_snapshot,
		 policy_allow_create_snapshot, policy_allow_duplicate_snapshot,
		 policy_allow_resume_snapshot, campaign_synced_at_snapshot,
		 campaign_hash_snapshot, campaign_name_snapshot, campaign_status_snapshot,
		 campaign_daily_budget_snapshot, campaign_lifetime_budget_snapshot)
		values ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8,
			nullif($9, '')::uuid, nullif($10, '')::uuid, $11, nullif($12, '')::uuid,
			$13, $14::jsonb, $15, $16, $17, nullif($18, '')::uuid,
			$19, $20, $21::uuid, $22::uuid, nullif($23, '')::uuid,
			$24, $25, $26, nullif($27, '')::uuid, $28, $29, $30,
			$31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41)
		on conflict (account_id, idempotency_key) do nothing
		returning ` + actionProposalColumns
	proposal, err := scanActionProposal(tx.QueryRow(ctx, query,
		input.AccountID, input.ResourceAccountID, input.AdAccount.ID,
		guard.AdAccount.MetaAdAccountID, guard.AdAccount.Name,
		strings.ToUpper(strings.TrimSpace(guard.AdAccount.Currency)),
		input.Action, input.Source, input.SourceConversationID, input.SourceMessageID,
		input.Source == ActionSourceManual, targetCampaignID, targetMetaCampaignID, input.Payload, input.Summary,
		input.RequestHash, input.IdempotencyKey, input.CreatedByUserID,
		actionGuardSnapshotVersion, guard.Hash, guard.ConnectionID, guard.ConnectionRevision,
		optionalActionString(guard.AdAccount.ClientAccountID), guard.AdAccount.UpdatedAt,
		guard.AdAccountHash, policyConfigured, policyID, policyUpdatedAt,
		guard.PolicyHash, policyCurrency, policyMaxDaily, policyMaxLifetime,
		policyAllowCreate, policyAllowDuplicate, policyAllowResume, campaignSyncedAt,
		guard.CampaignHash, campaignName, campaignStatus, campaignDaily, campaignLifetime,
	))
	created := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		proposal, err = scanActionProposal(tx.QueryRow(ctx, `select `+actionProposalColumns+`
			from meta_ads.action_proposals
			where account_id = $1::uuid and idempotency_key = $2`, input.AccountID, input.IdempotencyKey))
	}
	if err != nil {
		return ActionProposal{}, false, err
	}
	if proposal.RequestHash != input.RequestHash {
		return ActionProposal{}, false, ErrActionIdempotencyConflict
	}
	if created {
		if _, err := tx.Exec(ctx, `insert into meta_ads.action_proposal_events
			(account_id, proposal_id, event_type, actor_user_id, detail)
			values ($1::uuid, $2::uuid, 'proposed', nullif($3, '')::uuid,
				jsonb_build_object(
					'action', $4::text, 'source', $5::text,
					'guardSnapshotVersion', $6::smallint,
					'guardSnapshotHash', $7::text
				))`, input.AccountID, proposal.ID, input.CreatedByUserID,
			input.Action, input.Source, actionGuardSnapshotVersion, guard.Hash); err != nil {
			return ActionProposal{}, false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ActionProposal{}, false, err
	}
	return proposal, created, nil
}

func (s *Store) GetActionProposal(ctx context.Context, accountID, proposalID string) (ActionProposal, error) {
	return scanActionProposal(s.pool.QueryRow(ctx, `select `+actionProposalColumns+`
		from meta_ads.action_proposals
		where account_id = $1::uuid and id = $2::uuid`, accountID, proposalID))
}

func (s *Store) ListActionProposals(ctx context.Context, accountID string, limit int) ([]ActionProposal, error) {
	rows, err := s.pool.Query(ctx, `select `+actionProposalColumns+`
		from meta_ads.action_proposals
		where account_id = $1::uuid
		order by created_at desc, id desc
		limit $2`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ActionProposal, 0, limit)
	for rows.Next() {
		proposal, scanErr := scanActionProposal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, proposal)
	}
	return out, rows.Err()
}

func (s *Store) BindAssistantActionProposal(
	ctx context.Context,
	accountID, proposalID, conversationID, messageID, actorUserID string,
) (ActionProposal, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ActionProposal{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	proposal, err := scanActionProposal(tx.QueryRow(ctx, `select `+actionProposalColumns+`
		from meta_ads.action_proposals
		where account_id = $1::uuid and id = $2::uuid
		for update`, accountID, proposalID))
	if err != nil {
		return ActionProposal{}, err
	}
	if proposal.Source != ActionSourceAssistant || proposal.SourceConversationID == nil ||
		proposal.SourceMessageID == nil || *proposal.SourceConversationID != conversationID ||
		*proposal.SourceMessageID != messageID {
		return ActionProposal{}, ErrActionValidation
	}
	if proposal.SourceBound {
		if err := tx.Commit(ctx); err != nil {
			return ActionProposal{}, err
		}
		return proposal, nil
	}
	if proposal.Status != ActionPending {
		return ActionProposal{}, ErrActionSourceUnbound
	}
	proposal, err = scanActionProposal(tx.QueryRow(ctx, `update meta_ads.action_proposals
		set source_bound = true, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid and status = 'pending' and not source_bound
		returning `+actionProposalColumns, accountID, proposalID))
	if err != nil {
		return ActionProposal{}, err
	}
	if _, err := tx.Exec(ctx, `insert into meta_ads.action_proposal_events
		(account_id, proposal_id, event_type, actor_user_id, detail)
		values ($1::uuid, $2::uuid, 'bound', nullif($3, '')::uuid,
			jsonb_build_object('conversationId', $4::text, 'messageId', $5::text))`,
		accountID, proposalID, actorUserID, conversationID, messageID); err != nil {
		return ActionProposal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ActionProposal{}, err
	}
	return proposal, nil
}

func (s *Store) CancelActionProposal(
	ctx context.Context,
	accountID, proposalID, actorUserID, cancellationKey string,
) (ActionProposal, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ActionProposal{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	proposal, err := scanActionProposal(tx.QueryRow(ctx, `select `+actionProposalColumns+`
		from meta_ads.action_proposals
		where account_id = $1::uuid and id = $2::uuid
		for update`, accountID, proposalID))
	if err != nil {
		return ActionProposal{}, false, err
	}
	if proposal.Status == ActionCancelled {
		if proposal.CancellationIdempotencyKey == nil || *proposal.CancellationIdempotencyKey != cancellationKey {
			return ActionProposal{}, false, ErrActionIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return ActionProposal{}, false, err
		}
		return proposal, false, nil
	}
	if proposal.Status != ActionPending {
		return ActionProposal{}, false, ErrActionNotCancellable
	}
	proposal, err = scanActionProposal(tx.QueryRow(ctx, `update meta_ads.action_proposals
		set status = 'cancelled', cancellation_idempotency_key = $3,
			completed_at = now(), updated_at = now()
		where account_id = $1::uuid and id = $2::uuid and status = 'pending'
		returning `+actionProposalColumns, accountID, proposalID, cancellationKey))
	if err != nil {
		if isActionUniqueViolation(err) {
			return ActionProposal{}, false, ErrActionIdempotencyConflict
		}
		return ActionProposal{}, false, err
	}
	if _, err := tx.Exec(ctx, `insert into meta_ads.action_proposal_events
		(account_id, proposal_id, event_type, actor_user_id, detail)
		values ($1::uuid, $2::uuid, 'cancelled', nullif($3, '')::uuid,
			jsonb_build_object('cancellationIdempotencyKey', $4::text))`,
		accountID, proposalID, actorUserID, cancellationKey); err != nil {
		return ActionProposal{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ActionProposal{}, false, err
	}
	return proposal, true, nil
}

func (s *Store) CancelAssistantConversationActions(
	ctx context.Context,
	accountID, conversationID, actorUserID string,
) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	locked, err := tx.Query(ctx, `select id::text, status
		from meta_ads.action_proposals
		where account_id = $1::uuid and source = 'assistant'
			and source_conversation_id = $2::uuid
		for update`, accountID, conversationID)
	if err != nil {
		return 0, err
	}
	for locked.Next() {
		var proposalID string
		var status ActionStatus
		if err := locked.Scan(&proposalID, &status); err != nil {
			locked.Close()
			return 0, err
		}
		if status == ActionExecuting || status == ActionUnknown {
			locked.Close()
			return 0, ErrActionNotCancellable
		}
	}
	if err := locked.Err(); err != nil {
		locked.Close()
		return 0, err
	}
	locked.Close()
	rows, err := tx.Query(ctx, `update meta_ads.action_proposals
		set status = 'cancelled',
			cancellation_idempotency_key = 'assistant-conversation-delete:' || source_conversation_id::text || ':' || id::text,
			completed_at = now(), updated_at = now()
		where account_id = $1::uuid and source = 'assistant'
			and source_conversation_id = $2::uuid and status = 'pending'
		returning id::text`, accountID, conversationID)
	if err != nil {
		return 0, err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, proposalID := range ids {
		if _, err := tx.Exec(ctx, `insert into meta_ads.action_proposal_events
			(account_id, proposal_id, event_type, actor_user_id, detail)
			values ($1::uuid, $2::uuid, 'cancelled', nullif($3, '')::uuid,
				jsonb_build_object('reason', 'conversation_deleted'))`,
			accountID, proposalID, actorUserID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (s *Store) ExpireActionProposal(
	ctx context.Context,
	accountID, proposalID string,
) (ActionProposal, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ActionProposal{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	proposal, err := scanActionProposal(tx.QueryRow(ctx, `select `+actionProposalColumns+`
		from meta_ads.action_proposals
		where account_id = $1::uuid and id = $2::uuid
		for update`, accountID, proposalID))
	if err != nil {
		return ActionProposal{}, false, err
	}
	var expired bool
	if err := tx.QueryRow(ctx, `select expires_at <= now()
		from meta_ads.action_proposals
		where account_id = $1::uuid and id = $2::uuid`, accountID, proposalID).Scan(&expired); err != nil {
		return ActionProposal{}, false, err
	}
	if proposal.Status != ActionPending || !expired {
		if err := tx.Commit(ctx); err != nil {
			return ActionProposal{}, false, err
		}
		return proposal, false, nil
	}
	proposal, err = scanActionProposal(tx.QueryRow(ctx, `update meta_ads.action_proposals
		set status = 'expired', completed_at = now(), updated_at = now()
		where account_id = $1::uuid and id = $2::uuid and status = 'pending'
		returning `+actionProposalColumns, accountID, proposalID))
	if err != nil {
		return ActionProposal{}, false, err
	}
	if _, err := tx.Exec(ctx, `insert into meta_ads.action_proposal_events
		(account_id, proposal_id, event_type, detail)
		values ($1::uuid, $2::uuid, 'expired', jsonb_build_object('expiresAt', $3::timestamptz))`,
		accountID, proposalID, proposal.ExpiresAt); err != nil {
		return ActionProposal{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ActionProposal{}, false, err
	}
	return proposal, true, nil
}

// BeginActionExecution serializa confirmacoes concorrentes. Somente pending
// ganha a unica tentativa; replays devolvem a linha corrente sem nova chamada.
func (s *Store) BeginActionExecution(
	ctx context.Context,
	accountID, proposalID, actorUserID, confirmationKey string,
	acknowledgeSpend bool,
) (ActionProposal, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ActionProposal{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	proposal, err := scanActionProposal(tx.QueryRow(ctx, `select `+actionProposalColumns+`
		from meta_ads.action_proposals
		where account_id = $1::uuid and id = $2::uuid
		for update`, accountID, proposalID))
	if err != nil {
		return ActionProposal{}, false, err
	}
	if proposal.Status != ActionPending {
		if err := tx.Commit(ctx); err != nil {
			return ActionProposal{}, false, err
		}
		return proposal, false, nil
	}
	if proposal.Source == ActionSourceAssistant && !proposal.SourceBound {
		return ActionProposal{}, false, ErrActionSourceUnbound
	}
	var expired bool
	if err := tx.QueryRow(ctx, `select expires_at <= now()
		from meta_ads.action_proposals
		where account_id = $1::uuid and id = $2::uuid`, accountID, proposalID).Scan(&expired); err != nil {
		return ActionProposal{}, false, err
	}
	if expired {
		proposal, err = scanActionProposal(tx.QueryRow(ctx, `update meta_ads.action_proposals
			set status = 'expired', completed_at = now(), updated_at = now()
			where account_id = $1::uuid and id = $2::uuid and status = 'pending'
			returning `+actionProposalColumns, accountID, proposalID))
		if err != nil {
			return ActionProposal{}, false, err
		}
		if _, err := tx.Exec(ctx, `insert into meta_ads.action_proposal_events
			(account_id, proposal_id, event_type, detail)
			values ($1::uuid, $2::uuid, 'expired', jsonb_build_object('expiresAt', $3::timestamptz))`,
			accountID, proposalID, proposal.ExpiresAt); err != nil {
			return ActionProposal{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ActionProposal{}, false, err
		}
		return proposal, false, nil
	}

	var guard actionGuardSnapshot
	normalizedPayload, normalizeErr := normalizeActionPayload(proposal.Action, proposal.Payload)
	guardStale := proposal.GuardSnapshotVersion != actionGuardSnapshotVersion ||
		normalizeErr != nil ||
		actionRequestHash(proposal.Action, proposal.AdAccountID, normalizedPayload.Raw) != proposal.RequestHash
	if !guardStale {
		targetCampaignID := ""
		if proposal.TargetCampaignID != nil {
			targetCampaignID = *proposal.TargetCampaignID
		}
		guard, err = captureActionGuardSnapshot(
			ctx, tx, proposal.AccountID, proposal.ResourceAccountID,
			proposal.AdAccountID, targetCampaignID, proposal.RequestHash,
		)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			guardStale = true
		case err != nil:
			return ActionProposal{}, false, err
		case !actionGuardMatchesProposal(guard, proposal):
			guardStale = true
		}
	}
	if guardStale {
		proposal, err = scanActionProposal(tx.QueryRow(ctx, `update meta_ads.action_proposals
			set status = 'failed', error_code = 'proposal_stale',
				error_message = 'A conexao, o vinculo, a politica ou a campanha mudou.',
				completed_at = now(), updated_at = now()
			where account_id = $1::uuid and id = $2::uuid and status = 'pending'
			returning `+actionProposalColumns, accountID, proposalID))
		if err != nil {
			return ActionProposal{}, false, err
		}
		if _, err := tx.Exec(ctx, `insert into meta_ads.action_proposal_events
			(account_id, proposal_id, event_type, actor_user_id, detail)
			values ($1::uuid, $2::uuid, 'failed', nullif($3, '')::uuid,
				jsonb_build_object('status', 'failed', 'reason', 'proposal_stale'))`,
			accountID, proposalID, actorUserID); err != nil {
			return ActionProposal{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ActionProposal{}, false, err
		}
		return proposal, false, ErrActionProposalStale
	}

	proposal, err = scanActionProposal(tx.QueryRow(ctx, `update meta_ads.action_proposals
		set status = 'executing', confirmation_idempotency_key = $3,
			confirmed_by_user_id = nullif($4, '')::uuid, confirmed_at = now(),
			execution_started_at = now(), attempt_count = 1,
			claimed_connection_id = $5::uuid,
			claimed_connection_revision = $6::uuid, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid and status = 'pending'
		returning `+actionProposalColumns, accountID, proposalID, confirmationKey,
		actorUserID, guard.ConnectionID, guard.ConnectionRevision))
	if err != nil {
		if isActionUniqueViolation(err) {
			return ActionProposal{}, false, ErrActionIdempotencyConflict
		}
		return ActionProposal{}, false, err
	}
	if _, err := tx.Exec(ctx, `insert into meta_ads.action_proposal_events
		(account_id, proposal_id, event_type, actor_user_id, detail)
		values ($1::uuid, $2::uuid, 'confirmed', nullif($3, '')::uuid,
			jsonb_build_object(
				'confirmationIdempotencyKey', $4::text,
				'reinforced', $5::boolean,
				'connectionId', $6::text,
				'connectionRevision', $7::text,
				'guardSnapshotHash', $8::text
			))`,
		accountID, proposalID, actorUserID, confirmationKey,
		acknowledgeSpend, guard.ConnectionID, guard.ConnectionRevision,
		guard.Hash); err != nil {
		return ActionProposal{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ActionProposal{}, false, err
	}
	return proposal, true, nil
}

func (s *Store) CompleteActionExecution(
	ctx context.Context,
	accountID, proposalID string,
	outcome ActionExecutionOutcome,
) (ActionProposal, error) {
	return s.finishActionExecution(ctx, accountID, proposalID, "", outcome, false)
}

func (s *Store) ReconcileActionExecution(
	ctx context.Context,
	accountID, proposalID, actorUserID string,
	outcome ActionExecutionOutcome,
) (ActionProposal, error) {
	return s.finishActionExecution(ctx, accountID, proposalID, actorUserID, outcome, true)
}

func (s *Store) finishActionExecution(
	ctx context.Context,
	accountID, proposalID, actorUserID string,
	outcome ActionExecutionOutcome,
	reconciled bool,
) (ActionProposal, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ActionProposal{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	proposal, err := scanActionProposal(tx.QueryRow(ctx, `select `+actionProposalColumns+`
		from meta_ads.action_proposals
		where account_id = $1::uuid and id = $2::uuid
		for update`, accountID, proposalID))
	if err != nil {
		return ActionProposal{}, err
	}
	allowed := proposal.Status == ActionExecuting
	if reconciled {
		allowed = proposal.Status == ActionExecuting || proposal.Status == ActionUnknown
	}
	if !allowed {
		if err := tx.Commit(ctx); err != nil {
			return ActionProposal{}, err
		}
		return proposal, nil
	}

	reconciledAt := "reconciled_at"
	if reconciled {
		reconciledAt = "now()"
	}
	query := `update meta_ads.action_proposals
		set status = $3, external_entity_id = $4, result_snapshot = $5::jsonb,
			error_code = $6, error_message = $7, completed_at = now(),
			reconciled_at = ` + reconciledAt + `, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
		returning ` + actionProposalColumns
	proposal, err = scanActionProposal(tx.QueryRow(ctx, query, accountID, proposalID,
		outcome.Status, outcome.ExternalEntityID, outcome.Result, outcome.ErrorCode, outcome.ErrorMessage))
	if err != nil {
		return ActionProposal{}, err
	}
	eventType := string(outcome.Status)
	detail := json.RawMessage(`{"status":"` + eventType + `"}`)
	if reconciled {
		eventType = "reconciled"
		detail, err = json.Marshal(map[string]string{"status": string(outcome.Status)})
		if err != nil {
			return ActionProposal{}, err
		}
	}
	if _, err := tx.Exec(ctx, `insert into meta_ads.action_proposal_events
		(account_id, proposal_id, event_type, actor_user_id, detail)
		values ($1::uuid, $2::uuid, $3, nullif($4, '')::uuid, $5::jsonb)`,
		accountID, proposalID, eventType, actorUserID, detail); err != nil {
		return ActionProposal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ActionProposal{}, err
	}
	return proposal, nil
}

func (s *Store) GetActionPolicy(ctx context.Context, accountID, adAccountID string) (ActionPolicy, error) {
	return scanActionPolicy(s.pool.QueryRow(ctx, `select id::text, account_id::text,
		ad_account_id::text, currency, max_daily_budget, max_lifetime_budget,
		allow_create, allow_duplicate, allow_resume, updated_by_user_id::text,
		created_at, updated_at
		from meta_ads.action_policies
		where account_id = $1::uuid and ad_account_id = $2::uuid`, accountID, adAccountID))
}

func (s *Store) UpsertActionPolicy(
	ctx context.Context,
	accountID string,
	adAccount AdAccount,
	actorUserID string,
	input ActionPolicyInput,
) (ActionPolicy, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ActionPolicy{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var lockedAdAccountID string
	if err := tx.QueryRow(ctx, `select id::text
		from meta_ads.ad_accounts
		where account_id = $1::uuid and id = $2::uuid and is_current
		for update`, accountID, adAccount.ID).Scan(&lockedAdAccountID); err != nil {
		return ActionPolicy{}, err
	}
	policy, err := scanActionPolicy(tx.QueryRow(ctx, `insert into meta_ads.action_policies
		(account_id, ad_account_id, currency, max_daily_budget, max_lifetime_budget,
		 allow_create, allow_duplicate, allow_resume, updated_by_user_id, updated_at)
		values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, nullif($9, '')::uuid, now())
		on conflict (account_id, ad_account_id) do update
		set currency = excluded.currency,
			max_daily_budget = excluded.max_daily_budget,
			max_lifetime_budget = excluded.max_lifetime_budget,
			allow_create = excluded.allow_create,
			allow_duplicate = excluded.allow_duplicate,
			allow_resume = excluded.allow_resume,
			updated_by_user_id = excluded.updated_by_user_id,
			updated_at = now()
		returning id::text, account_id::text, ad_account_id::text, currency,
			max_daily_budget, max_lifetime_budget, allow_create, allow_duplicate,
		allow_resume, updated_by_user_id::text, created_at, updated_at`,
		accountID, adAccount.ID, strings.ToUpper(adAccount.Currency), input.MaxDailyBudget,
		input.MaxLifetimeBudget, input.AllowCreate, input.AllowDuplicate,
		input.AllowResume, actorUserID))
	if err != nil {
		return ActionPolicy{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ActionPolicy{}, err
	}
	return policy, nil
}

func scanActionPolicy(row rowScanner) (ActionPolicy, error) {
	var policy ActionPolicy
	err := row.Scan(&policy.ID, &policy.AccountID, &policy.AdAccountID, &policy.Currency,
		&policy.MaxDailyBudget, &policy.MaxLifetimeBudget, &policy.AllowCreate,
		&policy.AllowDuplicate, &policy.AllowResume, &policy.UpdatedByUserID,
		&policy.CreatedAt, &policy.UpdatedAt)
	return policy, err
}

func (s *Store) GetCampaignForAction(
	ctx context.Context,
	accountID, adAccountID, campaignID string,
) (Campaign, error) {
	const query = `select campaign.id, campaign.account_id, campaign.ad_account_id,
		campaign.meta_campaign_id, campaign.name, campaign.objective,
		campaign.status, campaign.daily_budget, campaign.lifetime_budget,
		campaign.is_current, campaign.synced_at
		from meta_ads.campaigns campaign
		join meta_ads.ad_accounts aa
		  on aa.id = campaign.ad_account_id and aa.account_id = campaign.account_id
		join meta_ads.connections connection
		  on connection.id = aa.connection_id and connection.account_id = aa.account_id
		where campaign.account_id = $1::uuid and campaign.ad_account_id = $2::uuid
		  and campaign.id = $3::uuid and campaign.is_current and aa.is_current
		  and connection.status = 'active'`
	return scanCampaign(s.pool.QueryRow(ctx, query, accountID, adAccountID, campaignID))
}

func isActionUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
