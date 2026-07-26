package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

type candidateClaimScanner interface {
	Scan(dest ...any) error
}

func (s *PostgresRepository) GetRuntimeClaimSource(
	ctx context.Context,
	scope Scope,
	subjectID, relationshipID, runID string,
) (runtimeClaimSource, error) {
	var source runtimeClaimSource
	err := s.pool.QueryRow(ctx, `
		select run.id, run.process_key, run.status, run.execution_mode,
		       run.process_definition_id, run.process_config_version_id,
		       run.prompt_binding_id, binding.platform_prompt_version_id,
		       coalesce(binding.agency_prompt_version_id::text, ''),
		       coalesce(binding.client_prompt_version_id::text, ''),
		       binding.process_prompt_version_id, run.agent_version_id,
		       run.model_id, coalesce(run.context_snapshot_id::text, ''),
		       run.output_schema_version, run.subject_id, run.relationship_id,
		       run.output_ciphertext
		from intelligence.runtime_runs run
		join intelligence.prompt_bindings binding
		  on binding.account_id = run.account_id
		 and binding.id = run.prompt_binding_id
		where run.account_id = $1::uuid
		  and run.client_account_id = $2::uuid
		  and run.subject_id = $3::uuid
		  and run.relationship_id = $4::uuid
		  and run.id = $5::uuid
		  and run.status = 'succeeded'
		  and run.execution_mode = 'active'
		  and run.output_ciphertext <> ''`,
		scope.AccountID, scope.ClientAccountID, subjectID, relationshipID, runID,
	).Scan(
		&source.RunRef.RunID, &source.RunRef.ProcessKey,
		&source.RunRef.Status, &source.RunRef.ExecutionMode,
		&source.RunRef.ProcessDefinitionID,
		&source.RunRef.ProcessConfigVersionID,
		&source.RunRef.PromptBindingID,
		&source.RunRef.PlatformPromptVersionID,
		&source.RunRef.AgencyPromptVersionID,
		&source.RunRef.ClientPromptVersionID,
		&source.RunRef.ProcessPromptVersionID,
		&source.RunRef.AgentVersionID,
		&source.RunRef.ModelID,
		&source.RunRef.ContextSnapshotID,
		&source.RunRef.OutputSchemaVersion,
		&source.SubjectID,
		&source.RelationshipID,
		&source.OutputCiphertext,
	)
	return source, repositoryError(err)
}

func (s *PostgresRepository) RecordOutcomeWithClaims(
	ctx context.Context,
	outcome AcceptedOutcome,
	claims []preparedCandidateClaim,
) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	reasonCode := "accepted"
	var legacyDetails struct {
		ReasonCode string `json:"reasonCode"`
	}
	_ = json.Unmarshal(normalizedJSON(outcome.Payload, `{}`), &legacyDetails)
	if legacyDetails.ReasonCode != "" {
		reasonCode = legacyDetails.ReasonCode
	}
	processRuns, err := json.Marshal(outcome.ProcessRuns)
	if err != nil {
		return false, err
	}
	var acceptedOutcomeID string
	err = tx.QueryRow(ctx, `
		insert into intelligence.accepted_outcomes (
		    event_id, account_id, client_account_id, interaction_id, decision_id,
		    conversation_id, subject_id, relationship_id, outcome, reason_code,
		    process_run_refs, occurred_at
		)
		values (
		    $1::uuid, $2::uuid, $3::uuid, $4, $5,
		    nullif($6, '')::uuid, $7::uuid, $8::uuid, $9, $10, $11::jsonb, $12
		)
		on conflict do nothing
		returning id`,
		outcome.EventID, outcome.AccountID, outcome.ClientAccountID,
		outcome.InteractionID, outcome.DecisionID, outcome.ConversationID,
		outcome.SubjectID, outcome.RelationshipID, outcome.OutcomeType,
		reasonCode, processRuns, outcome.OccurredAt,
	).Scan(&acceptedOutcomeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, repositoryError(err)
	}

	for _, claim := range claims {
		_, insertErr := insertCandidateClaimTx(ctx, tx, outcome, claim)
		if insertErr != nil {
			return false, insertErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func insertCandidateClaimTx(
	ctx context.Context,
	tx pgx.Tx,
	outcome AcceptedOutcome,
	claim preparedCandidateClaim,
) (bool, error) {
	var definitionID, definitionVersionID, sensitivity string
	err := tx.QueryRow(ctx, `
		select definition.id, version.id, version.sensitivity
		from intelligence.fact_definitions definition
		join intelligence.fact_definition_versions version
		  on version.account_id = definition.account_id
		 and version.id = definition.active_version_id
		 and version.fact_definition_id = definition.id
		 and version.status = 'published'
		where definition.account_id = $1::uuid
		  and definition.fact_key = $2
		  and definition.catalog_status = 'registered'
		  and version.value_type = $3`,
		outcome.AccountID,
		claim.Reference.FactKey,
		claim.Reference.ValueType,
	).Scan(&definitionID, &definitionVersionID, &sensitivity)
	if errors.Is(err, pgx.ErrNoRows) {
		_, auditErr := tx.Exec(ctx, `
			insert into intelligence.audit_events (
			    account_id, client_account_id, event_type, aggregate_type,
			    aggregate_id, correlation_id, reason_code, metadata
			)
			values (
			    $1::uuid, $2::uuid, 'claim.extraction_skipped',
			    'accepted_outcome', $3, $4, 'fact_definition_unavailable',
			    jsonb_build_object(
			        'factKey', $5::text,
			        'valueType', $6::text,
			        'ordinal', $7::integer
			    )
			)`,
			outcome.AccountID, outcome.ClientAccountID, outcome.EventID,
			outcome.InteractionID, claim.Reference.FactKey,
			claim.Reference.ValueType, claim.Reference.Ordinal,
		)
		return false, repositoryError(auditErr)
	}
	if err != nil {
		return false, repositoryError(err)
	}

	var claimID string
	err = tx.QueryRow(ctx, `
		insert into intelligence.claims (
		    account_id, client_account_id, subject_id, relationship_id,
		    fact_definition_id, fact_definition_version_id, fact_key, value_type,
		    value_normalized, value_ciphertext, cipher_key_version,
		    value_fingerprint, extraction_method, extractor_key,
		    extractor_version, prompt_binding_id, runtime_run_id, confidence,
		    verification_state, valid_from, valid_until, sensitivity, status,
		    source_outcome_event_id, source_claim_ordinal
		)
		values (
		    $1::uuid, $2::uuid, $3::uuid, $4::uuid,
		    $5::uuid, $6::uuid, $7, $8, null, $9, 'v1', $10,
		    'llm', $11, $12, $13::uuid, $14::uuid, $15,
		    'unverified', $16, $17, $18, 'candidate', $19::uuid, $20
		)
		on conflict (
		    account_id, client_account_id,
		    source_outcome_event_id, source_claim_ordinal
		) where source_outcome_event_id is not null
		do nothing
		returning id`,
		outcome.AccountID, outcome.ClientAccountID,
		outcome.SubjectID, outcome.RelationshipID,
		definitionID, definitionVersionID,
		claim.Reference.FactKey, claim.Reference.ValueType,
		claim.ValueCiphertext, claim.ValueCiphertextFingerprint,
		claim.Reference.ProcessKey, claim.Reference.OutputSchemaVersion,
		claim.Reference.PromptBindingID, claim.Reference.RuntimeRunID,
		claim.Reference.Confidence, claim.ValidFrom, claim.ValidUntil,
		sensitivity, outcome.EventID, claim.Reference.Ordinal,
	).Scan(&claimID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, repositoryError(err)
	}

	if len(claim.Reference.EvidenceObservationIDs) > 0 {
		if _, err = tx.Exec(ctx, `
			insert into intelligence.claim_evidence (
			    account_id, claim_id, observation_id, role
			)
			select $1::uuid, $2::uuid, observation.id, 'supports'
			from intelligence.source_observations observation
			where observation.account_id = $1::uuid
			  and observation.client_account_id = $3::uuid
			  and observation.subject_id = $4::uuid
			  and observation.relationship_id = $5::uuid
			  and observation.id = any($6::uuid[])
			on conflict (account_id, claim_id, observation_id) do nothing`,
			outcome.AccountID, claimID, outcome.ClientAccountID,
			outcome.SubjectID, outcome.RelationshipID,
			claim.Reference.EvidenceObservationIDs,
		); err != nil {
			return false, repositoryError(err)
		}
	}
	_, err = tx.Exec(ctx, `
		insert into intelligence.audit_events (
		    account_id, client_account_id, event_type, aggregate_type,
		    aggregate_id, correlation_id, reason_code, metadata
		)
		values (
		    $1::uuid, $2::uuid, 'claim.created', 'claim', $3,
		    $4, 'llm_candidate_extracted',
		    jsonb_build_object(
		        'factKey', $5::text,
		        'status', 'candidate',
		        'verificationState', 'unverified',
		        'runtimeRunId', $6::text,
		        'promptBindingId', $7::text,
		        'ordinal', $8::integer
		    )
		)`,
		outcome.AccountID, outcome.ClientAccountID, claimID,
		outcome.InteractionID, claim.Reference.FactKey,
		claim.Reference.RuntimeRunID, claim.Reference.PromptBindingID,
		claim.Reference.Ordinal,
	)
	return true, repositoryError(err)
}

func (s *PostgresRepository) ListCandidateClaims(
	ctx context.Context,
	scope Scope,
	relationshipID, status string,
	limit int,
) ([]CandidateClaim, error) {
	rows, err := s.pool.Query(ctx, candidateClaimSelect+`
		where claim.account_id = $1::uuid
		  and claim.client_account_id = $2::uuid
		  and claim.relationship_id = $3::uuid
		  and claim.status = $4
		order by claim.created_at desc, claim.id desc
		limit $5`,
		scope.AccountID, scope.ClientAccountID, relationshipID, status, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CandidateClaim, 0)
	for rows.Next() {
		item, err := scanCandidateClaim(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) GetCandidateClaim(
	ctx context.Context,
	scope Scope,
	claimID string,
) (CandidateClaim, error) {
	item, err := scanCandidateClaim(s.pool.QueryRow(ctx, candidateClaimSelect+`
		where claim.account_id = $1::uuid
		  and claim.client_account_id = $2::uuid
		  and claim.id = $3::uuid`,
		scope.AccountID, scope.ClientAccountID, claimID,
	))
	return item, repositoryError(err)
}

func (s *PostgresRepository) ReviewCandidateClaim(
	ctx context.Context,
	scope Scope,
	actorID, claimID string,
	input ClaimReviewInput,
) (CandidateClaim, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CandidateClaim{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var updatedID string
	err = tx.QueryRow(ctx, `
		update intelligence.claims
		set status = $5,
		    verification_state = case
		        when $5 = 'rejected' then 'rejected'
		        else verification_state
		    end,
		    reviewed_by_user_id = nullif($6, '')::uuid,
		    reviewed_at = now(),
		    review_reason_code = $7,
		    revision = revision + 1,
		    updated_at = now()
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and id = $3::uuid
		  and status = 'candidate'
		  and revision = $4
		returning id`,
		scope.AccountID, scope.ClientAccountID, claimID,
		input.ExpectedRevision, input.Status, actorID, input.ReasonCode,
	).Scan(&updatedID)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if checkErr := tx.QueryRow(ctx, `
			select exists (
			    select 1
			    from intelligence.claims
			    where account_id = $1::uuid
			      and client_account_id = $2::uuid
			      and id = $3::uuid
			)`,
			scope.AccountID, scope.ClientAccountID, claimID,
		).Scan(&exists); checkErr != nil {
			return CandidateClaim{}, repositoryError(checkErr)
		}
		if !exists {
			return CandidateClaim{}, ErrNotFound
		}
		return CandidateClaim{}, ErrConflict
	}
	if err != nil {
		return CandidateClaim{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `
		insert into intelligence.audit_events (
		    account_id, client_account_id, actor_user_id, event_type,
		    aggregate_type, aggregate_id, reason_code, metadata
		)
		values (
		    $1::uuid, $2::uuid, nullif($3, '')::uuid, 'claim.reviewed',
		    'claim', $4, $5,
		    jsonb_build_object('status', $6::text)
		)`,
		scope.AccountID, scope.ClientAccountID, actorID,
		updatedID, input.ReasonCode, input.Status,
	); err != nil {
		return CandidateClaim{}, repositoryError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return CandidateClaim{}, err
	}
	return s.GetCandidateClaim(ctx, scope, claimID)
}

const candidateClaimSelect = `
	select claim.id, claim.account_id, claim.client_account_id,
	       claim.subject_id, claim.relationship_id,
	       claim.fact_definition_id, claim.fact_definition_version_id,
	       claim.fact_key, claim.value_type,
	       coalesce(claim.value_normalized, 'null'::jsonb),
	       coalesce(claim.value_ciphertext, ''),
	       claim.extraction_method, claim.extractor_key, claim.extractor_version,
	       coalesce(claim.prompt_binding_id::text, ''),
	       coalesce(claim.runtime_run_id::text, ''),
	       claim.confidence, claim.verification_state,
	       claim.valid_from, claim.valid_until, claim.sensitivity, claim.status,
	       coalesce(claim.source_outcome_event_id::text, ''),
	       claim.source_claim_ordinal, claim.revision,
	       coalesce(claim.reviewed_by_user_id::text, ''),
	       claim.reviewed_at, claim.review_reason_code,
	       claim.created_at, claim.updated_at,
	       coalesce((
	           select jsonb_agg(
	               jsonb_build_object(
	                   'observationId', evidence.observation_id,
	                   'sourceKey', observation.source_key,
	                   'locator', observation.source_entity_type
	               )
	               order by evidence.created_at, evidence.observation_id
	           )
	           from intelligence.claim_evidence evidence
	           join intelligence.source_observations observation
	             on observation.account_id = evidence.account_id
	            and observation.id = evidence.observation_id
	           where evidence.account_id = claim.account_id
	             and evidence.claim_id = claim.id
	             and observation.client_account_id = claim.client_account_id
	             and observation.subject_id = claim.subject_id
	             and observation.relationship_id = claim.relationship_id
	       ), '[]'::jsonb)
	from intelligence.claims claim`

func scanCandidateClaim(scanner candidateClaimScanner) (CandidateClaim, error) {
	var item CandidateClaim
	var value, evidence []byte
	err := scanner.Scan(
		&item.ID, &item.AccountID, &item.ClientAccountID,
		&item.SubjectID, &item.RelationshipID,
		&item.FactDefinitionID, &item.FactDefinitionVersionID,
		&item.FactKey, &item.ValueType, &value, &item.valueCiphertext,
		&item.ExtractionMethod, &item.ExtractorKey, &item.ExtractorVersion,
		&item.PromptBindingID, &item.RuntimeRunID,
		&item.Confidence, &item.VerificationState,
		&item.ValidFrom, &item.ValidUntil, &item.Sensitivity, &item.Status,
		&item.SourceOutcomeEventID, &item.SourceClaimOrdinal, &item.Revision,
		&item.ReviewedByUserID, &item.ReviewedAt, &item.ReviewReasonCode,
		&item.CreatedAt, &item.UpdatedAt, &evidence,
	)
	if err != nil {
		return CandidateClaim{}, err
	}
	item.Value = normalizedJSON(value, `null`)
	_ = json.Unmarshal(evidence, &item.Evidence)
	if item.Evidence == nil {
		item.Evidence = []EvidenceRef{}
	}
	return item, nil
}

var _ candidateClaimRepository = (*PostgresRepository)(nil)
