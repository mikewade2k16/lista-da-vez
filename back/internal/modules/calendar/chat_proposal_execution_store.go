package calendar

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"html"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tasks"
)

type chatAskClaim struct {
	Created  bool
	Status   string
	Response ChatAskResult
}

type chatAskIdempotencyStore interface {
	ClaimChatAsk(ctx context.Context, accountID, actorUserID, key string, requestHash []byte, target chatTarget) (chatAskClaim, error)
	BindChatAskConversation(ctx context.Context, accountID, actorUserID, key, conversationID string) error
	CompleteChatAsk(ctx context.Context, accountID, actorUserID, key string, requestHash []byte, response ChatAskResult) error
}

func hashJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return sum[:], nil
}

func seedChatProposalExecutions(
	ctx context.Context,
	tx pgx.Tx,
	accountID, conversationID string,
	message ChatMessage,
) error {
	if message.Role != chatRoleAssistant || len(message.Proposals) == 0 {
		return nil
	}
	topology, err := resolveCalendarStorageAccountTx(ctx, tx, accountID)
	if err != nil {
		return err
	}
	items := make(map[string]AIContextEvent, len(message.CalendarItems))
	for _, item := range message.CalendarItems {
		if id := strings.TrimSpace(item.ID); id != "" {
			items[id] = item
		}
	}
	const q = `insert into calendar.chat_proposal_executions (
		account_id, conversation_id, message_id, proposal_id, kind, action,
		proposal_snapshot, proposal_hash, target_id, expected_version,
		before_snapshot, before_hash, storage_account_id
	) values (
		$1::uuid, $2::uuid, $3::uuid, $4, $5, $6,
		$7::jsonb, $8, $9::uuid, $10, $11::jsonb, $12, $13::uuid
	) on conflict (account_id, message_id, proposal_id) do nothing`
	for _, proposal := range message.Proposals {
		if proposal.Kind == "metaAction" {
			continue
		}
		proposalRaw, err := json.Marshal(proposal)
		if err != nil {
			return err
		}
		proposalHash := sha256.Sum256(proposalRaw)
		beforeRaw := []byte(`{}`)
		var beforeHash []byte
		var expectedVersion *int
		targetID := ""
		if proposal.Action != "create" {
			targetID = normalizeUUID(proposal.Fields.TargetID)
			if item, ok := items[targetID]; ok {
				beforeRaw, err = json.Marshal(item)
				if err != nil {
					return err
				}
				sum := sha256.Sum256(beforeRaw)
				beforeHash = sum[:]
				if item.Version > 0 {
					version := item.Version
					expectedVersion = &version
				}
			}
		}
		if proposal.Kind == "taskItem" || (proposal.Kind == "task" && proposal.Action != "create") {
			targetID = normalizeUUID(proposal.Fields.TargetID)
			if targetID == "" {
				return ErrProposalSnapshotMissing
			}
			task, taskErr := tasks.LoadAssistantTaskSnapshotTx(ctx, tx, accountID, targetID)
			if taskErr != nil {
				if errors.Is(taskErr, pgx.ErrNoRows) || errors.Is(taskErr, tasks.ErrTaskNotFound) {
					return ErrProposalSnapshotMissing
				}
				return taskErr
			}
			beforeRaw, err = json.Marshal(task)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(beforeRaw)
			beforeHash = sum[:]
			version := task.Version
			expectedVersion = &version
		}
		// Note/profile create podem encontrar uma linha ja existente e, portanto,
		// tambem precisam de snapshot. Ausencia permanece representada por `{}` +
		// hash nulo; o executor exige que ela continue ausente ate confirmar.
		switch proposal.Kind {
		case "note":
			if proposal.Fields.Note != nil {
				var note NoteView
				err = tx.QueryRow(ctx, `select month_key, content, updated_by, updated_at
					from calendar.notes where account_id = $1::uuid and month_key = $2`,
					topology.StorageAccountID, strings.TrimSpace(proposal.Fields.Note.Month)).Scan(
					&note.Month, &note.Content, &note.UpdatedBy, &note.UpdatedAt,
				)
				if err == nil {
					beforeRaw, err = json.Marshal(note)
					if err == nil {
						sum := sha256.Sum256(beforeRaw)
						beforeHash = sum[:]
					}
				} else if errors.Is(err, pgx.ErrNoRows) {
					err = nil
				}
			}
		case "clientProfile":
			clientID := normalizeUUID(proposal.Fields.ClientID)
			if clientID != "" {
				profile, profileErr := scanProfile(tx.QueryRow(ctx, `select client_id::text,
					segment, positioning, description, history, site_url, instagram, address,
					objectives, brand_voice, extra, updated_at from calendar.client_profiles
					where account_id = $1::uuid and client_id = $2::uuid`,
					topology.StorageAccountID, clientID))
				if profileErr == nil {
					beforeRaw, err = json.Marshal(profile.view())
					if err == nil {
						sum := sha256.Sum256(beforeRaw)
						beforeHash = sum[:]
					}
				} else if !errors.Is(profileErr, pgx.ErrNoRows) {
					err = profileErr
				}
			}
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, q, accountID, conversationID, message.ID,
			proposal.ID, proposal.Kind, proposal.Action, proposalRaw, proposalHash[:],
			nullUUID(targetID), expectedVersion, beforeRaw, beforeHash,
			topology.StorageAccountID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ClaimChatAsk(
	ctx context.Context,
	accountID, actorUserID, key string,
	requestHash []byte,
	target chatTarget,
) (chatAskClaim, error) {
	const insert = `insert into calendar.chat_ask_requests (
		account_id, actor_user_id, idempotency_key, request_hash,
		requested_conversation_id, entry_surface, scope_mode, scope_client_id
	) values ($1::uuid, $2::uuid, $3, $4, $5::uuid, $6, $7, $8::uuid)
	 on conflict (account_id, actor_user_id, idempotency_key) do nothing`
	requestedConversationID := ""
	if target.existing {
		requestedConversationID = target.conv.ID
	}
	tag, err := s.pool.Exec(ctx, insert, accountID, actorUserID, key, requestHash,
		nullUUID(requestedConversationID), target.surface, target.mode, nullUUID(target.clientID))
	if err != nil {
		return chatAskClaim{}, err
	}
	if tag.RowsAffected() == 1 {
		return chatAskClaim{Created: true, Status: "executing"}, nil
	}
	const expireUncertain = `update calendar.chat_ask_requests
		set status = 'unknown', updated_at = now(), error_code = 'execution_outcome_unknown'
		where account_id = $1::uuid and actor_user_id = $2::uuid and idempotency_key = $3
		  and status = 'executing' and updated_at < now() - interval '5 minutes'`
	if _, err := s.pool.Exec(ctx, expireUncertain, accountID, actorUserID, key); err != nil {
		return chatAskClaim{}, err
	}

	const get = `select request_hash, status, response_snapshot
		from calendar.chat_ask_requests
		where account_id = $1::uuid and actor_user_id = $2::uuid and idempotency_key = $3`
	var storedHash []byte
	var status string
	var responseRaw json.RawMessage
	if err := s.pool.QueryRow(ctx, get, accountID, actorUserID, key).Scan(
		&storedHash, &status, &responseRaw,
	); err != nil {
		return chatAskClaim{}, err
	}
	if !equalHash(storedHash, requestHash) {
		return chatAskClaim{}, ErrIdempotencyConflict
	}
	claim := chatAskClaim{Status: status}
	if status == "succeeded" {
		if err := json.Unmarshal(responseRaw, &claim.Response); err != nil {
			return chatAskClaim{}, err
		}
		return claim, nil
	}
	return chatAskClaim{}, ErrIdempotencyInProgress
}

func (s *Store) BindChatAskConversation(
	ctx context.Context,
	accountID, actorUserID, key, conversationID string,
) error {
	const q = `update calendar.chat_ask_requests
		set conversation_id = $4::uuid, updated_at = now()
		where account_id = $1::uuid and actor_user_id = $2::uuid
		  and idempotency_key = $3 and status = 'executing'
		  and (conversation_id is null or conversation_id = $4::uuid)`
	tag, err := s.pool.Exec(ctx, q, accountID, actorUserID, key, conversationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrIdempotencyConflict
	}
	return nil
}

func (s *Store) CompleteChatAsk(
	ctx context.Context,
	accountID, actorUserID, key string,
	requestHash []byte,
	response ChatAskResult,
) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	const q = `update calendar.chat_ask_requests
		set status = 'succeeded', response_snapshot = $5::jsonb,
			conversation_id = $6::uuid, updated_at = now(), completed_at = now()
		where account_id = $1::uuid and actor_user_id = $2::uuid
		  and idempotency_key = $3 and request_hash = $4 and status = 'executing'`
	tag, err := s.pool.Exec(ctx, q, accountID, actorUserID, key, requestHash, raw, response.ConversationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrIdempotencyConflict
	}
	return nil
}

func equalHash(left, right []byte) bool {
	if len(left) != sha256.Size || len(right) != sha256.Size {
		return false
	}
	return subtle.ConstantTimeCompare(left, right) == 1
}

func validateProposalRowSnapshot(
	action string,
	inserted bool,
	seededBefore json.RawMessage,
	seededBeforeHash, currentHash []byte,
) (json.RawMessage, []byte, error) {
	seededAbsent := string(seededBefore) == "{}" && len(seededBeforeHash) == 0
	if action == "create" && seededAbsent {
		if !inserted {
			return nil, nil, ErrVersionConflict
		}
		return json.RawMessage(`{}`), nil, nil
	}
	if len(seededBeforeHash) != sha256.Size || string(seededBefore) == "{}" {
		return nil, nil, ErrProposalSnapshotMissing
	}
	if inserted || !equalHash(currentHash, seededBeforeHash) {
		return nil, nil, ErrVersionConflict
	}
	return append(json.RawMessage(nil), seededBefore...), append([]byte(nil), currentHash...), nil
}

func (s *Store) ExecuteChatProposal(
	ctx context.Context,
	command chatProposalExecutionCommand,
) (chatProposalExecutionResult, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return chatProposalExecutionResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const sourceQuery = `select e.status, e.proposal_snapshot, e.proposal_hash,
		e.confirmation_key, e.confirmation_request_hash, e.expected_version,
		e.before_snapshot, e.before_hash, e.result_snapshot, e.target_id::text,
		e.storage_account_id::text, c.created_by_user_id::text, c.entry_surface, c.scope_mode,
		c.scope_client_id::text, card.elem
		from calendar.chat_proposal_executions e
		join calendar.chat_conversations c
		  on c.account_id = e.account_id and c.id = e.conversation_id and c.deleted_at is null
		join calendar.chat_messages m
		  on m.account_id = e.account_id and m.conversation_id = e.conversation_id and m.id = e.message_id
		cross join lateral (
			select elem from jsonb_array_elements(m.proposals) elem
			where elem->>'id' = e.proposal_id limit 1
		) card
		where e.account_id = $1::uuid and e.conversation_id = $2::uuid
		  and e.message_id = $3::uuid and e.proposal_id = $4
		for update of e, c, m`
	var (
		status, storedConfirmationKey                   string
		proposalRaw, beforeRaw, resultRaw, currentRaw   json.RawMessage
		proposalHash, storedRequestHash, beforeHash     []byte
		expectedVersion                                 *int
		targetID, storedStorageAccountID, scopeClientID *string
		conversationOwnerID, entrySurface, scopeMode    string
	)
	var nullableConfirmationKey *string
	if err := tx.QueryRow(ctx, sourceQuery, command.AccountID, command.ConversationID,
		command.MessageID, command.ProposalID).Scan(
		&status, &proposalRaw, &proposalHash, &nullableConfirmationKey, &storedRequestHash,
		&expectedVersion, &beforeRaw, &beforeHash, &resultRaw, &targetID,
		&storedStorageAccountID, &conversationOwnerID, &entrySurface, &scopeMode, &scopeClientID, &currentRaw,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return chatProposalExecutionResult{}, ErrNotFound
		}
		return chatProposalExecutionResult{}, err
	}
	if nullableConfirmationKey != nil {
		storedConfirmationKey = *nullableConfirmationKey
	}
	storedStorage := ""
	if storedStorageAccountID != nil {
		storedStorage = *storedStorageAccountID
	}
	conversationClient := ""
	if scopeClientID != nil {
		conversationClient = *scopeClientID
	}
	executionAccess, err := validateChatProposalExecutionScopeTx(
		ctx, tx, command, storedStorage, conversationOwnerID, entrySurface, scopeMode, conversationClient,
	)
	if err != nil {
		return chatProposalExecutionResult{}, err
	}
	if status == "succeeded" {
		if storedConfirmationKey != command.ConfirmationKey || !equalHash(storedRequestHash, command.RequestHash) {
			return chatProposalExecutionResult{}, ErrIdempotencyConflict
		}
		message, err := chatMessageTx(ctx, tx, command.AccountID, command.ConversationID, command.MessageID)
		if err != nil {
			return chatProposalExecutionResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return chatProposalExecutionResult{}, ErrProposalExecutionUnknown
		}
		return chatProposalExecutionResult{Message: message, Resource: resultRaw, Replayed: true}, nil
	}
	if status != "pending" {
		return chatProposalExecutionResult{}, ErrProposalExecutionUnknown
	}
	var sourceProposal, currentProposal StoredProposal
	if json.Unmarshal(proposalRaw, &sourceProposal) != nil || json.Unmarshal(currentRaw, &currentProposal) != nil {
		return chatProposalExecutionResult{}, ErrProposalSnapshotMissing
	}
	computedProposalHash, err := hashJSON(sourceProposal)
	if err != nil || !equalHash(computedProposalHash, proposalHash) {
		return chatProposalExecutionResult{}, ErrProposalSnapshotMissing
	}
	currentProposal.Execution = nil
	computedCurrentHash, err := hashJSON(currentProposal)
	if err != nil || !equalHash(computedCurrentHash, proposalHash) || currentProposal.Status != "pending" {
		return chatProposalExecutionResult{}, ErrProposalSnapshotMissing
	}
	if sourceProposal.Action != command.Proposal.Action || sourceProposal.Kind != command.Proposal.Kind {
		return chatProposalExecutionResult{}, ErrProposalSnapshotMissing
	}
	if targetID != nil && normalizeUUID(*targetID) != normalizeUUID(command.Proposal.Fields.TargetID) {
		return chatProposalExecutionResult{}, ErrProposalSnapshotMissing
	}
	editableRaw, err := json.Marshal(command.Proposal.Fields)
	if err != nil {
		return chatProposalExecutionResult{}, err
	}
	const claim = `update calendar.chat_proposal_executions
		set status = 'executing', confirmation_key = $5,
			confirmation_request_hash = $6, editable_fields = $7::jsonb,
			storage_account_id = $8::uuid, actor_user_id = $9::uuid,
			attempted_at = now(), updated_at = now()
		where account_id = $1::uuid and conversation_id = $2::uuid
		  and message_id = $3::uuid and proposal_id = $4 and status = 'pending'`
	tag, err := tx.Exec(ctx, claim, command.AccountID, command.ConversationID,
		command.MessageID, command.ProposalID, command.ConfirmationKey, command.RequestHash,
		editableRaw, executionAccess.StorageAccountID, command.ActorUserID)
	if err != nil {
		return chatProposalExecutionResult{}, ErrIdempotencyConflict
	}
	if tag.RowsAffected() != 1 {
		return chatProposalExecutionResult{}, ErrProposalExecutionUnknown
	}

	var resource any
	var taskMutation *tasks.AssistantTaskMutationResult
	var eventMutation *chatEventPostCommit
	switch command.Proposal.Kind {
	case "event":
		var outcome eventProposalExecutionResult
		outcome, err = executeEventProposalTx(
			ctx, tx, command, executionAccess, expectedVersion, beforeRaw, beforeHash,
		)
		if err == nil {
			resource = outcome.Resource
			beforeRaw = outcome.Before
			beforeHash = outcome.BeforeHash
			eventMutation = outcome.PostCommit
		}
	case "note":
		resource, beforeRaw, beforeHash, err = executeNoteProposalTx(ctx, tx, command, beforeRaw, beforeHash)
	case "clientProfile":
		resource, beforeRaw, beforeHash, err = executeProfileProposalTx(ctx, tx, command, beforeRaw, beforeHash)
	case "task", "taskItem":
		var outcome tasks.AssistantTaskMutationResult
		outcome, err = executeTaskProposalTx(ctx, tx, command, expectedVersion, beforeRaw, beforeHash)
		if err == nil {
			resource = outcome.After
			taskMutation = &outcome
		}
	default:
		err = ErrProposalExecutionUnavailable
	}
	if err != nil {
		return chatProposalExecutionResult{}, err
	}
	resultRaw, err = json.Marshal(resource)
	if err != nil {
		return chatProposalExecutionResult{}, err
	}
	const acceptCard = `update calendar.chat_messages m
		set proposals = coalesce((
			select jsonb_agg(
				case when elem->>'id' = $4 then jsonb_set(elem, '{status}', '"accepted"'::jsonb) else elem end
				order by ord
			) from jsonb_array_elements(m.proposals) with ordinality as card(elem, ord)
		), '[]'::jsonb)
		where m.id = $1::uuid and m.account_id = $2::uuid and m.conversation_id = $3::uuid
		  and exists (select 1 from jsonb_array_elements(m.proposals) elem
		              where elem->>'id' = $4 and elem->>'status' = 'pending')`
	tag, err = tx.Exec(ctx, acceptCard, command.MessageID, command.AccountID,
		command.ConversationID, command.ProposalID)
	if err != nil {
		return chatProposalExecutionResult{}, err
	}
	if tag.RowsAffected() != 1 {
		return chatProposalExecutionResult{}, ErrProposalSnapshotMissing
	}
	const complete = `update calendar.chat_proposal_executions
		set status = 'succeeded', before_snapshot = $5::jsonb, before_hash = $6,
			result_snapshot = $7::jsonb, error_code = '', error_message = '',
			completed_at = now(), updated_at = now()
		where account_id = $1::uuid and conversation_id = $2::uuid
		  and message_id = $3::uuid and proposal_id = $4 and status = 'executing'`
	if _, err := tx.Exec(ctx, complete, command.AccountID, command.ConversationID,
		command.MessageID, command.ProposalID, beforeRaw, beforeHash, resultRaw); err != nil {
		return chatProposalExecutionResult{}, err
	}
	message, err := chatMessageTx(ctx, tx, command.AccountID, command.ConversationID, command.MessageID)
	if err != nil {
		return chatProposalExecutionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return chatProposalExecutionResult{}, ErrProposalExecutionUnknown
	}
	return chatProposalExecutionResult{
		Message: message, Resource: resultRaw, TaskMutation: taskMutation, EventMutation: eventMutation,
	}, nil
}

func chatMessageTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID, conversationID, messageID string,
) (ChatMessage, error) {
	const q = `select ` + chatMessageCols + ` from calendar.chat_messages
		where id = $1::uuid and account_id = $2::uuid and conversation_id = $3::uuid`
	return scanChatMessage(tx.QueryRow(ctx, q, messageID, accountID, conversationID))
}

type eventProposalExecutionResult struct {
	Resource   EventView
	Before     json.RawMessage
	BeforeHash []byte
	PostCommit *chatEventPostCommit
}

func executeEventProposalTx(
	ctx context.Context,
	tx pgx.Tx,
	command chatProposalExecutionCommand,
	executionAccess chatProposalExecutionAccess,
	expectedVersion *int,
	seededBefore json.RawMessage,
	seededBeforeHash []byte,
) (eventProposalExecutionResult, error) {
	fields := command.Proposal.Fields
	if command.Proposal.Action == "create" {
		input, err := validateEvent(command.StorageAccountID, EventInput{
			Date: fields.Date, Time: fields.Time, ClientID: fields.ClientID,
			Type: fields.Type, Title: fields.Title, Status: fields.Status,
			Priority: fields.Priority, ResponsibleID: fields.ResponsibleID,
			InvolvedIDs: append([]string(nil), fields.InvolvedIDs...),
			Description: firstNonEmpty(fields.Description, fields.ContentHTML), Source: "ai",
		})
		if err != nil {
			return eventProposalExecutionResult{}, err
		}
		if err := validateExecutionClientTx(ctx, tx, command, executionAccess, input.ClientID); err != nil {
			return eventProposalExecutionResult{}, err
		}
		if err := validateEventPeopleTx(ctx, tx, command.StorageAccountID, input.ResponsibleID, input.InvolvedIDs); err != nil {
			return eventProposalExecutionResult{}, err
		}
		const q = `insert into calendar.events (
			account_id, client_id, event_date, event_time, type, title, status, priority,
			responsible_id, involved_ids, media, description, source
		) values ($1::uuid, $2::uuid, $3::date, $4, $5, $6, $7, $8, $9, $10::jsonb, '[]'::jsonb, $11, 'ai')
		returning ` + eventCols
		event, err := scanEvent(tx.QueryRow(ctx, q, command.StorageAccountID, nullUUID(input.ClientID),
			input.Date, input.Time, input.Type, input.Title, input.Status, input.Priority,
			input.ResponsibleID, jsonArray(input.InvolvedIDs), input.Description))
		if err != nil {
			return eventProposalExecutionResult{}, err
		}
		return eventProposalExecutionResult{Resource: event.view(), Before: json.RawMessage(`{}`)}, nil
	}
	if expectedVersion == nil || *expectedVersion <= 0 || len(seededBeforeHash) != sha256.Size || string(seededBefore) == "{}" {
		return eventProposalExecutionResult{}, ErrProposalSnapshotMissing
	}
	targetID := normalizeUUID(fields.TargetID)
	query := `select ` + eventCols + eventTaskIDCol + ` from calendar.events e
		where e.id = $1::uuid and e.account_id = $2::uuid`
	args := []any{targetID, command.StorageAccountID}
	if command.LockedClientID != "" {
		query += " and e.client_id = $3::uuid"
		args = append(args, command.LockedClientID)
	}
	query += " for update of e"
	current, err := scanEventWithTask(tx.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return eventProposalExecutionResult{}, ErrNotFound
	}
	if err != nil {
		return eventProposalExecutionResult{}, err
	}
	if err := validateExecutionClientTx(
		ctx, tx, command, executionAccess, eventClientID(current),
	); err != nil {
		return eventProposalExecutionResult{}, err
	}
	if current.Version != *expectedVersion {
		return eventProposalExecutionResult{}, ErrVersionConflict
	}
	linkedTaskID := strings.TrimSpace(ptrToStr(current.TaskID))
	beforeRaw, err := json.Marshal(current.view())
	if err != nil {
		return eventProposalExecutionResult{}, err
	}
	beforeSum := sha256.Sum256(beforeRaw)
	if command.Proposal.Action == "delete" {
		tag, err := tx.Exec(ctx, `delete from calendar.events
			where id = $1::uuid and account_id = $2::uuid and version = $3`,
			current.ID, command.StorageAccountID, current.Version)
		if err != nil {
			return eventProposalExecutionResult{}, err
		}
		if tag.RowsAffected() != 1 {
			return eventProposalExecutionResult{}, ErrVersionConflict
		}
		return eventProposalExecutionResult{
			Resource: current.view(), Before: beforeRaw, BeforeHash: beforeSum[:],
			PostCommit: &chatEventPostCommit{Event: current, LinkedTaskID: linkedTaskID, Deleted: true},
		}, nil
	}
	input := eventInputFromProposal(current, fields)
	input, err = validateEvent(command.StorageAccountID, input)
	if err != nil {
		return eventProposalExecutionResult{}, err
	}
	if err := validateExecutionClientTx(ctx, tx, command, executionAccess, input.ClientID); err != nil {
		return eventProposalExecutionResult{}, err
	}
	if err := validateEventPeopleTx(ctx, tx, command.StorageAccountID, input.ResponsibleID, input.InvolvedIDs); err != nil {
		return eventProposalExecutionResult{}, err
	}
	const update = `update calendar.events set
		client_id = $3::uuid, event_date = $4::date, event_time = $5, type = $6,
		title = $7, status = $8, priority = $9, responsible_id = $10,
		involved_ids = $11::jsonb, description = $12, version = version + 1, updated_at = now()
		where id = $1::uuid and account_id = $2::uuid and version = $13
		returning ` + eventCols
	updated, err := scanEvent(tx.QueryRow(ctx, update, current.ID, command.StorageAccountID,
		nullUUID(input.ClientID), input.Date, input.Time, input.Type, input.Title, input.Status,
		input.Priority, input.ResponsibleID, jsonArray(input.InvolvedIDs), input.Description,
		current.Version))
	if errors.Is(err, pgx.ErrNoRows) {
		return eventProposalExecutionResult{}, ErrVersionConflict
	}
	if err != nil {
		return eventProposalExecutionResult{}, err
	}
	return eventProposalExecutionResult{
		Resource: updated.view(), Before: beforeRaw, BeforeHash: beforeSum[:],
		PostCommit: &chatEventPostCommit{
			Event: updated, LinkedTaskID: linkedTaskID, PreviousDescription: current.Description,
		},
	}, nil
}

func eventInputFromProposal(current CalendarEvent, fields ChatProposalFields) EventInput {
	view := current.view()
	input := EventInput{Date: view.Date, Time: view.Time, ClientID: view.ClientID,
		Type: view.Type, Title: view.Title, Status: view.Status, Priority: view.Priority,
		ResponsibleID: view.ResponsibleID, Description: view.Description}
	_ = json.Unmarshal(view.InvolvedIDs, &input.InvolvedIDs)
	if fields.Date != "" {
		input.Date = fields.Date
	}
	if fields.Time != "" {
		input.Time = fields.Time
	}
	if fields.ClientID != "" {
		input.ClientID = fields.ClientID
	}
	if fields.Type != "" {
		input.Type = fields.Type
	}
	if fields.Title != "" {
		input.Title = fields.Title
	}
	if fields.Status != "" {
		input.Status = fields.Status
	}
	if fields.Priority != "" {
		input.Priority = fields.Priority
	}
	if fields.ResponsibleID != "" {
		input.ResponsibleID = fields.ResponsibleID
	}
	if fields.InvolvedIDs != nil {
		input.InvolvedIDs = append([]string(nil), fields.InvolvedIDs...)
	}
	if body := firstNonEmpty(fields.Description, fields.ContentHTML); body != "" {
		input.Description = body
	}
	return input
}

func validateEventPeopleTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID, responsibleID string,
	involvedIDs []string,
) error {
	ids := append([]string(nil), involvedIDs...)
	if strings.TrimSpace(responsibleID) != "" {
		ids = append(ids, responsibleID)
	}
	for _, raw := range ids {
		id := normalizeUUID(raw)
		if id == "" {
			return ErrForbidden
		}
		var exists bool
		if err := tx.QueryRow(ctx, `select exists (
			select 1 from core.account_users
			where account_id = $1::uuid and user_id = $2::uuid and is_active = true
		)`, accountID, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrForbidden
		}
	}
	return nil
}

func executeNoteProposalTx(
	ctx context.Context,
	tx pgx.Tx,
	command chatProposalExecutionCommand,
	seededBefore json.RawMessage,
	seededBeforeHash []byte,
) (NoteView, json.RawMessage, []byte, error) {
	note := command.Proposal.Fields.Note
	if note == nil || !monthRe.MatchString(strings.TrimSpace(note.Month)) {
		return NoteView{}, nil, nil, ErrInvalidDate
	}
	month := strings.TrimSpace(note.Month)
	insertTag, err := tx.Exec(ctx, `insert into calendar.notes (account_id, month_key)
		values ($1::uuid, $2) on conflict (account_id, month_key) do nothing`,
		command.StorageAccountID, month)
	if err != nil {
		return NoteView{}, nil, nil, err
	}
	var current NoteView
	if err := tx.QueryRow(ctx, `select month_key, content, updated_by, updated_at
		from calendar.notes where account_id = $1::uuid and month_key = $2 for update`,
		command.StorageAccountID, month).Scan(
		&current.Month, &current.Content, &current.UpdatedBy, &current.UpdatedAt,
	); err != nil {
		return NoteView{}, nil, nil, err
	}
	beforeRaw, err := json.Marshal(current)
	if err != nil {
		return NoteView{}, nil, nil, err
	}
	beforeSum := sha256.Sum256(beforeRaw)
	receiptBefore, receiptBeforeHash, err := validateProposalRowSnapshot(
		command.Proposal.Action, insertTag.RowsAffected() == 1,
		seededBefore, seededBeforeHash, beforeSum[:],
	)
	if err != nil {
		return NoteView{}, nil, nil, err
	}
	next := ""
	if command.Proposal.Action != "delete" {
		chunk := wrapProposalNote(note.Content)
		if chunk == "" {
			return NoteView{}, nil, nil, ErrInvalidProposalStatus
		}
		if note.Mode == "replace" || strings.TrimSpace(current.Content) == "" {
			next = chunk
		} else {
			next = strings.TrimSpace(current.Content) + chunk
		}
	}
	var saved NoteView
	if err := tx.QueryRow(ctx, `update calendar.notes
		set content = $3, updated_by = $4, updated_at = now()
		where account_id = $1::uuid and month_key = $2
		returning month_key, content, updated_by, updated_at`,
		command.StorageAccountID, month, next, command.ActorLabel).Scan(
		&saved.Month, &saved.Content, &saved.UpdatedBy, &saved.UpdatedAt,
	); err != nil {
		return NoteView{}, nil, nil, err
	}
	return saved, receiptBefore, receiptBeforeHash, nil
}

func wrapProposalNote(value string) string {
	value = htmlToPlainText(value)
	if value == "" {
		return ""
	}
	escaped := html.EscapeString(value)
	return "<p>" + strings.ReplaceAll(escaped, "\n", "<br>") + "</p>"
}

func executeProfileProposalTx(
	ctx context.Context,
	tx pgx.Tx,
	command chatProposalExecutionCommand,
	seededBefore json.RawMessage,
	seededBeforeHash []byte,
) (ProfileView, json.RawMessage, []byte, error) {
	clientID := normalizeUUID(command.Proposal.Fields.ClientID)
	profile := command.Proposal.Fields.Profile
	if clientID == "" || profile == nil {
		return ProfileView{}, nil, nil, ErrInvalidClient
	}
	insertTag, err := tx.Exec(ctx, `insert into calendar.client_profiles (account_id, client_id)
		values ($1::uuid, $2::uuid) on conflict (account_id, client_id) do nothing`,
		command.StorageAccountID, clientID)
	if err != nil {
		return ProfileView{}, nil, nil, err
	}
	const get = `select client_id::text, segment, positioning, description, history,
		site_url, instagram, address, objectives, brand_voice, extra, updated_at
		from calendar.client_profiles
		where account_id = $1::uuid and client_id = $2::uuid for update`
	current, err := scanProfile(tx.QueryRow(ctx, get, command.StorageAccountID, clientID))
	if err != nil {
		return ProfileView{}, nil, nil, err
	}
	beforeRaw, err := json.Marshal(current.view())
	if err != nil {
		return ProfileView{}, nil, nil, err
	}
	beforeSum := sha256.Sum256(beforeRaw)
	receiptBefore, receiptBeforeHash, err := validateProposalRowSnapshot(
		command.Proposal.Action, insertTag.RowsAffected() == 1,
		seededBefore, seededBeforeHash, beforeSum[:],
	)
	if err != nil {
		return ProfileView{}, nil, nil, err
	}
	next := mergeProposalProfile(current, command.Proposal.Action, profile)
	extraRaw, err := json.Marshal(trimExtra(next.Extra))
	if err != nil {
		return ProfileView{}, nil, nil, err
	}
	const update = `update calendar.client_profiles set
		segment = $3, positioning = $4, description = $5, history = $6,
		site_url = $7, instagram = $8, address = $9, objectives = $10,
		brand_voice = $11, extra = $12::jsonb, updated_by = $13, updated_at = now()
		where account_id = $1::uuid and client_id = $2::uuid
		returning client_id::text, segment, positioning, description, history,
		          site_url, instagram, address, objectives, brand_voice, extra, updated_at`
	saved, err := scanProfile(tx.QueryRow(ctx, update, command.StorageAccountID, clientID,
		next.Segment, next.Positioning, next.Description, next.History, next.SiteURL,
		next.Instagram, next.Address, next.Objectives, next.BrandVoice, extraRaw, command.ActorLabel))
	if err != nil {
		return ProfileView{}, nil, nil, err
	}
	return saved.view(), receiptBefore, receiptBeforeHash, nil
}

func mergeProposalProfile(
	current ClientProfile,
	action string,
	proposal *ChatProposalProfile,
) ClientProfile {
	if action == "delete" && proposal.ClearAll {
		return ClientProfile{ClientID: current.ClientID}
	}
	if action == "delete" {
		for _, key := range proposal.ClearFields {
			clearClientProfileField(&current, key)
		}
		return current
	}
	if proposal.Segment != "" {
		current.Segment = proposal.Segment
	}
	if proposal.Positioning != "" {
		current.Positioning = proposal.Positioning
	}
	if proposal.Description != "" {
		current.Description = proposal.Description
	}
	if proposal.History != "" {
		current.History = proposal.History
	}
	if proposal.SiteURL != "" {
		current.SiteURL = proposal.SiteURL
	}
	if proposal.Instagram != "" {
		current.Instagram = proposal.Instagram
	}
	if proposal.Address != "" {
		current.Address = proposal.Address
	}
	if proposal.Objectives != "" {
		current.Objectives = proposal.Objectives
	}
	if proposal.BrandVoice != "" {
		current.BrandVoice = proposal.BrandVoice
	}
	if proposal.Extra != nil {
		if proposal.Extra.Audience != "" {
			current.Extra.Audience = proposal.Extra.Audience
		}
		if proposal.Extra.Offer != "" {
			current.Extra.Offer = proposal.Extra.Offer
		}
		if proposal.Extra.Pillars != "" {
			current.Extra.Pillars = proposal.Extra.Pillars
		}
		if proposal.Extra.Cadence != "" {
			current.Extra.Cadence = proposal.Extra.Cadence
		}
		if proposal.Extra.Restrictions != "" {
			current.Extra.Restrictions = proposal.Extra.Restrictions
		}
		if proposal.Extra.Performance != "" {
			current.Extra.Performance = proposal.Extra.Performance
		}
		if proposal.Extra.Assets != "" {
			current.Extra.Assets = proposal.Extra.Assets
		}
	}
	return current
}

func clearClientProfileField(profile *ClientProfile, key string) {
	switch strings.TrimSpace(key) {
	case "segment":
		profile.Segment = ""
	case "positioning":
		profile.Positioning = ""
	case "description":
		profile.Description = ""
	case "history":
		profile.History = ""
	case "siteUrl":
		profile.SiteURL = ""
	case "instagram":
		profile.Instagram = ""
	case "address":
		profile.Address = ""
	case "objectives":
		profile.Objectives = ""
	case "brandVoice":
		profile.BrandVoice = ""
	case "audience":
		profile.Extra.Audience = ""
	case "offer":
		profile.Extra.Offer = ""
	case "pillars":
		profile.Extra.Pillars = ""
	case "cadence":
		profile.Extra.Cadence = ""
	case "restrictions":
		profile.Extra.Restrictions = ""
	case "performance":
		profile.Extra.Performance = ""
	case "assets":
		profile.Extra.Assets = ""
	}
}
