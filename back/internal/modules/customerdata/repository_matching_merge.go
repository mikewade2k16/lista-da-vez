package customerdata

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func candidateColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "client_account_id::text",
		alias + "incoming_source_key", alias + "incoming_source_type",
		alias + "incoming_source_id", alias + "candidate_subject_id::text",
		alias + "candidate_relationship_id::text", alias + "match_method",
		alias + "match_confidence", alias + "evidence_refs", alias + "risk_flags",
		alias + "status", alias + "decision_reason", alias + "revision",
		alias + "created_at", alias + "updated_at",
	}, ", ")
}

func scanCandidate(row rowScanner) (MatchCandidate, error) {
	var item MatchCandidate
	var evidence, flags []byte
	err := row.Scan(
		&item.ID, &item.ClientAccountID, &item.IncomingSourceKey,
		&item.IncomingSourceType, &item.IncomingSourceID, &item.CandidateSubjectID,
		&item.CandidateRelationshipID, &item.MatchMethod, &item.MatchConfidence,
		&evidence, &flags, &item.Status, &item.DecisionReason, &item.Revision,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return MatchCandidate{}, mapDBError(err)
	}
	item.EvidenceRefs = evidence
	item.RiskFlags = flags
	return item, nil
}

func (r *PostgresRepository) ListMatchCandidates(
	ctx context.Context,
	scope Scope,
	filter MatchCandidateFilter,
) ([]MatchCandidate, string, error) {
	cursor, err := decodeStableCursor(filter.Cursor)
	if err != nil {
		return nil, "", err
	}
	args := []any{scope.AccountID, scope.ClientAccountID}
	var query strings.Builder
	query.WriteString(`
		select ` + candidateColumns("") + `
		from customer_data.match_candidates
		where account_id = $1::uuid and client_account_id = $2::uuid
	`)
	if filter.Status != "" {
		appendClause(&query, &args, " and status = $%d", filter.Status)
	}
	if !cursor.At.IsZero() {
		args = append(args, cursor.At, cursor.ID)
		_, _ = fmt.Fprintf(
			&query,
			" and (created_at, id) < ($%d, $%d::uuid)",
			len(args)-1,
			len(args),
		)
	}
	args = append(args, filter.Limit+1)
	_, _ = fmt.Fprintf(&query, " order by created_at desc, id desc limit $%d", len(args))
	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := make([]MatchCandidate, 0, filter.Limit)
	var next string
	for rows.Next() {
		item, err := scanCandidate(rows)
		if err != nil {
			return nil, "", err
		}
		if len(items) == filter.Limit {
			next = encodeStableCursor(items[len(items)-1].CreatedAt, items[len(items)-1].ID)
			break
		}
		items = append(items, item)
	}
	return items, next, rows.Err()
}

func (r *PostgresRepository) GetMatchCandidate(
	ctx context.Context,
	scope Scope,
	candidateID string,
) (MatchCandidate, error) {
	return scanCandidate(r.pool.QueryRow(ctx, `
		select `+candidateColumns("")+`
		from customer_data.match_candidates
		where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, candidateID))
}

func (r *PostgresRepository) DecideMatchCandidate(
	ctx context.Context,
	scope Scope,
	candidateID string,
	input MatchDecisionInput,
) (MatchCandidate, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return MatchCandidate{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var replay bool
	err = tx.QueryRow(ctx, `
		select exists (
			select 1 from customer_data.outbox_events
			where account_id = $1::uuid
			  and client_account_id = $2::uuid
			  and idempotency_key = $3
			)
	`, scope.AccountID, scope.ClientAccountID, "candidate.decision:"+input.IdempotencyKey).Scan(&replay)
	if err != nil {
		return MatchCandidate{}, false, err
	}
	if replay {
		item, err := scanCandidate(tx.QueryRow(ctx, `
			select `+candidateColumns("")+` from customer_data.match_candidates
			where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
		`, scope.AccountID, scope.ClientAccountID, candidateID))
		return item, true, err
	}
	current, err := scanCandidate(tx.QueryRow(ctx, `
		select `+candidateColumns("")+`
		from customer_data.match_candidates
		where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
		for update
	`, scope.AccountID, scope.ClientAccountID, candidateID))
	if err != nil {
		return MatchCandidate{}, false, err
	}
	if current.Status != "pending" || current.Revision != input.ExpectedRevision {
		return MatchCandidate{}, false, ErrConflict
	}
	var relationshipID *string
	if input.Decision == "accept" && input.CreateRelationship {
		targetSubjectID := current.CandidateSubjectID
		if input.TargetSubjectID != nil {
			targetSubjectID = input.TargetSubjectID
		}
		if targetSubjectID == nil || *targetSubjectID == "" {
			return MatchCandidate{}, false, invalid("targetSubjectId", "required")
		}
		var id string
		err = tx.QueryRow(ctx, `
			insert into customer_data.relationships (
				account_id, client_account_id, subject_id, display_name,
				lifecycle_status, classification_source, tags, custom_fields,
				created_by_user_id, updated_by_user_id
			) values (
				$1::uuid, $2::uuid, $3::uuid, 'Contato', 'lead', 'manual',
				'[]'::jsonb, '{}'::jsonb, nullif($4, '')::uuid, nullif($4, '')::uuid
			)
			on conflict (account_id, client_account_id, subject_id) do nothing
			returning id::text
		`, scope.AccountID, scope.ClientAccountID, *targetSubjectID, scope.ActorUserID).Scan(&id)
		if err == pgx.ErrNoRows {
			err = tx.QueryRow(ctx, `
				select id::text from customer_data.relationships
				where account_id = $1::uuid and client_account_id = $2::uuid and subject_id = $3::uuid
			`, scope.AccountID, scope.ClientAccountID, *targetSubjectID).Scan(&id)
		}
		if err != nil {
			return MatchCandidate{}, false, mapDBError(err)
		}
		relationshipID = &id
	}
	status := "rejected"
	if input.Decision == "accept" {
		status = "accepted"
	}
	item, err := scanCandidate(tx.QueryRow(ctx, `
		update customer_data.match_candidates
		set status = $4,
		    candidate_subject_id = coalesce(nullif($5, '')::uuid, candidate_subject_id),
		    candidate_relationship_id = coalesce(nullif($6, '')::uuid, candidate_relationship_id),
		    decision_reason = $7,
		    reviewed_by_user_id = nullif($8, '')::uuid,
		    reviewed_at = now(),
		    revision = revision + 1,
		    updated_at = now()
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and id = $3::uuid
		  and status = 'pending'
		  and revision = $9
		returning `+candidateColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, candidateID, status,
		pointerString(input.TargetSubjectID), pointerString(relationshipID),
		strings.TrimSpace(input.Reason), scope.ActorUserID, input.ExpectedRevision))
	if err != nil {
		return MatchCandidate{}, false, mapMutationError(err)
	}
	if err := insertOutbox(ctx, tx, scope, "match_candidate", item.ID, "customer_data.identity.changed", "candidate.decision:"+input.IdempotencyKey); err != nil {
		return MatchCandidate{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MatchCandidate{}, false, err
	}
	return item, false, nil
}

type mergeRelationshipSnapshot struct {
	ID            string `json:"id"`
	RevisionAfter int64  `json:"revisionAfter"`
}

type mergeSnapshot struct {
	SourceRevisionAfter int64                       `json:"sourceRevisionAfter"`
	TargetRevisionAfter int64                       `json:"targetRevisionAfter"`
	Relationships       []mergeRelationshipSnapshot `json:"relationships"`
}

func mergeEventColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "client_account_id::text",
		alias + "source_subject_id::text", alias + "target_subject_id::text",
		alias + "affected_relationship_ids", alias + "reason", alias + "event_kind",
		alias + "reverses_event_id::text", alias + "snapshot", alias + "created_at",
	}, ", ")
}

func scanMergeEvent(row rowScanner) (MergeEvent, error) {
	var item MergeEvent
	var relationshipsRaw, snapshot []byte
	err := row.Scan(
		&item.ID, &item.ClientAccountID, &item.SourceSubjectID, &item.TargetSubjectID,
		&relationshipsRaw, &item.Reason, &item.EventKind, &item.ReversesEventID,
		&snapshot, &item.CreatedAt,
	)
	if err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	_ = json.Unmarshal(relationshipsRaw, &item.AffectedRelationshipIDs)
	item.Snapshot = snapshot
	return item, nil
}

func (r *PostgresRepository) MergeSubjects(
	ctx context.Context,
	scope Scope,
	sourceSubjectID string,
	input MergeInput,
) (MergeEvent, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return MergeEvent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	existing, err := scanMergeEvent(tx.QueryRow(ctx, `
		select `+mergeEventColumns("")+`
		from customer_data.merge_events
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and idempotency_key = $3
	`, scope.AccountID, scope.ClientAccountID, input.IdempotencyKey))
	if err == nil {
		existing.Replayed = true
		return existing, nil
	}
	if err != ErrNotFound {
		return MergeEvent{}, err
	}
	type subjectState struct {
		id, status string
		revision   int64
		mergedInto *string
	}
	rows, err := tx.Query(ctx, `
		select id::text, status, merged_into_subject_id::text, revision
		from customer_data.subjects
		where account_id = $1::uuid and id = any($2::uuid[])
		order by id
		for update
	`, scope.AccountID, []string{sourceSubjectID, input.TargetSubjectID})
	if err != nil {
		return MergeEvent{}, err
	}
	states := map[string]subjectState{}
	for rows.Next() {
		var state subjectState
		if err := rows.Scan(&state.id, &state.status, &state.mergedInto, &state.revision); err != nil {
			rows.Close()
			return MergeEvent{}, err
		}
		states[state.id] = state
	}
	rows.Close()
	source, sourceOK := states[sourceSubjectID]
	target, targetOK := states[input.TargetSubjectID]
	if !sourceOK || !targetOK {
		return MergeEvent{}, ErrNotFound
	}
	if source.status != "active" || target.status != "active" ||
		source.revision != input.ExpectedSourceRevision || target.revision != input.ExpectedTargetRevision {
		return MergeEvent{}, ErrConflict
	}
	var outsideScope bool
	err = tx.QueryRow(ctx, `
		select exists (
			select 1 from customer_data.relationships
			where account_id = $1::uuid
			  and subject_id = any($2::uuid[])
			  and client_account_id <> $3::uuid
		)
	`, scope.AccountID, []string{sourceSubjectID, input.TargetSubjectID}, scope.ClientAccountID).Scan(&outsideScope)
	if err != nil {
		return MergeEvent{}, err
	}
	if outsideScope {
		return MergeEvent{}, ErrConflict
	}
	var overlap bool
	err = tx.QueryRow(ctx, `
		select exists (
			select 1
			from customer_data.relationships source
			join customer_data.relationships target
			  on target.account_id = source.account_id
			 and target.client_account_id = source.client_account_id
			where source.account_id = $1::uuid
			  and source.subject_id = $2::uuid
			  and target.subject_id = $3::uuid
		)
	`, scope.AccountID, sourceSubjectID, input.TargetSubjectID).Scan(&overlap)
	if err != nil {
		return MergeEvent{}, err
	}
	if overlap {
		return MergeEvent{}, ErrConflict
	}
	relationshipRows, err := tx.Query(ctx, `
		select id::text, revision
		from customer_data.relationships
		where account_id = $1::uuid and client_account_id = $2::uuid and subject_id = $3::uuid
		order by id
		for update
	`, scope.AccountID, scope.ClientAccountID, sourceSubjectID)
	if err != nil {
		return MergeEvent{}, err
	}
	relationships := make([]mergeRelationshipSnapshot, 0)
	relationshipIDs := make([]string, 0)
	for relationshipRows.Next() {
		var id string
		var revision int64
		if err := relationshipRows.Scan(&id, &revision); err != nil {
			relationshipRows.Close()
			return MergeEvent{}, err
		}
		relationshipIDs = append(relationshipIDs, id)
		relationships = append(relationships, mergeRelationshipSnapshot{ID: id, RevisionAfter: revision + 1})
	}
	relationshipRows.Close()
	if len(relationshipIDs) == 0 {
		return MergeEvent{}, ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.relationships
		set subject_id = $4::uuid, revision = revision + 1, updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and subject_id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, sourceSubjectID, input.TargetSubjectID); err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.subject_identities
		set subject_id = $4::uuid, revision = revision + 1, updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid and subject_id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, sourceSubjectID, input.TargetSubjectID); err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.subject_source_links
		set subject_id = $4::uuid, updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid and subject_id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, sourceSubjectID, input.TargetSubjectID); err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.segment_memberships
		set subject_id = $4::uuid
		where account_id = $1::uuid and client_account_id = $2::uuid and subject_id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, sourceSubjectID, input.TargetSubjectID); err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.subjects
		set status = 'merged', merged_into_subject_id = $3::uuid,
		    revision = revision + 1, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
	`, scope.AccountID, sourceSubjectID, input.TargetSubjectID); err != nil {
		return MergeEvent{}, err
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.subjects
		set revision = revision + 1, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
	`, scope.AccountID, input.TargetSubjectID); err != nil {
		return MergeEvent{}, err
	}
	snapshot := mergeSnapshot{
		SourceRevisionAfter: source.revision + 1,
		TargetRevisionAfter: target.revision + 1,
		Relationships:       relationships,
	}
	snapshotRaw, _ := json.Marshal(snapshot)
	relationshipsRaw, _ := json.Marshal(relationshipIDs)
	event, err := scanMergeEvent(tx.QueryRow(ctx, `
		insert into customer_data.merge_events (
			account_id, client_account_id, source_subject_id, target_subject_id,
			affected_relationship_ids, reason, actor_user_id, idempotency_key,
			event_kind, snapshot
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::jsonb, $6,
			nullif($7, '')::uuid, $8, 'merge', $9::jsonb
		)
		returning `+mergeEventColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, sourceSubjectID, input.TargetSubjectID,
		relationshipsRaw, strings.TrimSpace(input.Reason), scope.ActorUserID,
		input.IdempotencyKey, snapshotRaw))
	if err != nil {
		return MergeEvent{}, err
	}
	if err := insertOutbox(ctx, tx, scope, "merge", event.ID, "customer_data.merge.completed", "merge:"+input.IdempotencyKey); err != nil {
		return MergeEvent{}, err
	}
	if err := insertAudit(ctx, tx, scope, sourceSubjectID, "", "merge", "merge", event.ID, input.Reason); err != nil {
		return MergeEvent{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MergeEvent{}, err
	}
	return event, nil
}

func (r *PostgresRepository) UndoMerge(
	ctx context.Context,
	scope Scope,
	mergeEventID string,
	input UndoMergeInput,
) (MergeEvent, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return MergeEvent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	existing, err := scanMergeEvent(tx.QueryRow(ctx, `
		select `+mergeEventColumns("")+`
		from customer_data.merge_events
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and idempotency_key = $3
	`, scope.AccountID, scope.ClientAccountID, input.IdempotencyKey))
	if err == nil {
		existing.Replayed = true
		return existing, nil
	}
	if err != ErrNotFound {
		return MergeEvent{}, err
	}
	mergeEvent, err := scanMergeEvent(tx.QueryRow(ctx, `
		select `+mergeEventColumns("")+`
		from customer_data.merge_events
		where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
		for update
	`, scope.AccountID, scope.ClientAccountID, mergeEventID))
	if err != nil {
		return MergeEvent{}, err
	}
	if mergeEvent.EventKind != "merge" {
		return MergeEvent{}, ErrConflict
	}
	var alreadyUndone bool
	err = tx.QueryRow(ctx, `
		select exists (
			select 1 from customer_data.merge_events
			where account_id = $1::uuid and client_account_id = $2::uuid
			  and event_kind = 'undo' and reverses_event_id = $3::uuid
		)
	`, scope.AccountID, scope.ClientAccountID, mergeEventID).Scan(&alreadyUndone)
	if err != nil {
		return MergeEvent{}, err
	}
	if alreadyUndone {
		return MergeEvent{}, ErrConflict
	}
	var snapshot mergeSnapshot
	if json.Unmarshal(mergeEvent.Snapshot, &snapshot) != nil {
		return MergeEvent{}, ErrConflict
	}
	var sourceStatus string
	var sourceTarget *string
	var sourceRevision, targetRevision int64
	err = tx.QueryRow(ctx, `
		select status, merged_into_subject_id::text, revision
		from customer_data.subjects
		where account_id = $1::uuid and id = $2::uuid
		for update
	`, scope.AccountID, mergeEvent.SourceSubjectID).Scan(&sourceStatus, &sourceTarget, &sourceRevision)
	if err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	err = tx.QueryRow(ctx, `
		select revision from customer_data.subjects
		where account_id = $1::uuid and id = $2::uuid
		for update
	`, scope.AccountID, mergeEvent.TargetSubjectID).Scan(&targetRevision)
	if err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	if sourceStatus != "merged" || sourceTarget == nil || *sourceTarget != mergeEvent.TargetSubjectID ||
		sourceRevision != snapshot.SourceRevisionAfter || targetRevision != snapshot.TargetRevisionAfter {
		return MergeEvent{}, ErrConflict
	}
	for _, relationship := range snapshot.Relationships {
		var subjectID string
		var revision int64
		err := tx.QueryRow(ctx, `
			select subject_id::text, revision
			from customer_data.relationships
			where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
			for update
		`, scope.AccountID, scope.ClientAccountID, relationship.ID).Scan(&subjectID, &revision)
		if err != nil || subjectID != mergeEvent.TargetSubjectID || revision != relationship.RevisionAfter {
			return MergeEvent{}, ErrConflict
		}
	}
	ids := make([]string, 0, len(snapshot.Relationships))
	for _, relationship := range snapshot.Relationships {
		ids = append(ids, relationship.ID)
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.relationships
		set subject_id = $4::uuid, revision = revision + 1, updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid and id = any($3::uuid[])
	`, scope.AccountID, scope.ClientAccountID, ids, mergeEvent.SourceSubjectID); err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.subject_identities
		set subject_id = $4::uuid, revision = revision + 1, updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and relationship_id = any($3::uuid[])
	`, scope.AccountID, scope.ClientAccountID, ids, mergeEvent.SourceSubjectID); err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.subject_source_links
		set subject_id = $4::uuid, updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and relationship_id = any($3::uuid[])
	`, scope.AccountID, scope.ClientAccountID, ids, mergeEvent.SourceSubjectID); err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.segment_memberships
		set subject_id = $4::uuid
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and relationship_id = any($3::uuid[])
	`, scope.AccountID, scope.ClientAccountID, ids, mergeEvent.SourceSubjectID); err != nil {
		return MergeEvent{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.subjects
		set status = 'active', merged_into_subject_id = null,
		    revision = revision + 1, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
	`, scope.AccountID, mergeEvent.SourceSubjectID); err != nil {
		return MergeEvent{}, err
	}
	if _, err := tx.Exec(ctx, `
		update customer_data.subjects
		set revision = revision + 1, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
	`, scope.AccountID, mergeEvent.TargetSubjectID); err != nil {
		return MergeEvent{}, err
	}
	relationshipsRaw, _ := json.Marshal(ids)
	snapshotRaw, _ := json.Marshal(map[string]any{"restoredRelationshipIds": ids})
	event, err := scanMergeEvent(tx.QueryRow(ctx, `
		insert into customer_data.merge_events (
			account_id, client_account_id, source_subject_id, target_subject_id,
			affected_relationship_ids, reason, actor_user_id, idempotency_key,
			event_kind, reverses_event_id, snapshot
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::jsonb, $6,
			nullif($7, '')::uuid, $8, 'undo', $9::uuid, $10::jsonb
		)
		returning `+mergeEventColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, mergeEvent.SourceSubjectID,
		mergeEvent.TargetSubjectID, relationshipsRaw, strings.TrimSpace(input.Reason),
		scope.ActorUserID, input.IdempotencyKey, mergeEventID, snapshotRaw))
	if err != nil {
		return MergeEvent{}, err
	}
	if err := insertOutbox(ctx, tx, scope, "merge", event.ID, "customer_data.merge.undone", "merge.undo:"+input.IdempotencyKey); err != nil {
		return MergeEvent{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MergeEvent{}, err
	}
	return event, nil
}
