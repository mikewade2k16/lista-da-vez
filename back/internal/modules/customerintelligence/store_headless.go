package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresRepository) PersistHeadlessRefresh(
	ctx context.Context,
	input HeadlessRefreshPersistence,
) (int, error) {
	if err := validateHeadlessPersistenceInput(input); err != nil {
		return 0, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := verifyHeadlessPersistenceScope(ctx, tx, input); err != nil {
		return 0, repositoryError(err)
	}

	persisted := 0
	for _, execution := range input.Executions {
		var rows int
		switch execution.RunRef.ProcessKey {
		case "profile.summary":
			rows, err = persistHeadlessSummary(ctx, tx, input, execution)
		case "recommendation.follow_up",
			"recommendation.offer",
			"recommendation.important_dates":
			rows, err = persistHeadlessRecommendation(ctx, tx, input, execution)
		case "source.suggest":
			rows, err = persistHeadlessSourceSuggestions(ctx, tx, input, execution)
		default:
			err = ErrInvalidInput
		}
		if err != nil {
			return 0, repositoryError(err)
		}
		persisted += rows
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, repositoryError(err)
	}
	return persisted, nil
}

func validateHeadlessPersistenceInput(input HeadlessRefreshPersistence) error {
	if !validUUID(input.Scope.AccountID) ||
		!validUUID(input.Scope.ClientAccountID) ||
		!validUUID(input.SubjectID) ||
		!validUUID(input.RelationshipID) ||
		!validUUID(input.ContextID) ||
		input.AsOf.IsZero() ||
		len(input.Executions) == 0 ||
		len(input.Executions) > maxHeadlessRelationshipRuns ||
		input.Context.SnapshotID != input.ContextID ||
		input.Context.AccountID != input.Scope.AccountID ||
		input.Context.ClientAccountID != input.Scope.ClientAccountID ||
		input.Context.SubjectID != input.SubjectID ||
		input.Context.RelationshipID != input.RelationshipID {
		return ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(input.Executions))
	for _, execution := range input.Executions {
		ref := execution.RunRef
		if !relationshipHeadlessProcessCatalog[ref.ProcessKey] ||
			!validUUID(ref.RunID) ||
			!validUUID(ref.PromptBindingID) ||
			!validUUID(ref.ContextSnapshotID) ||
			ref.ContextSnapshotID != input.ContextID ||
			ref.Status != "succeeded" ||
			ref.ExecutionMode != "active" ||
			len(execution.Output) == 0 ||
			execution.OutputCiphertext == "" ||
			execution.OutputHash == "" ||
			execution.OutputHash != hashBytes(execution.Output) {
			return ErrInvalidInput
		}
		if _, duplicate := seen[ref.ProcessKey]; duplicate {
			return ErrInvalidInput
		}
		seen[ref.ProcessKey] = struct{}{}
		if err := validateTypedProcessOutput(ref.ProcessKey, execution.Output); err != nil {
			return ErrInvalidInput
		}
		if err := validateRelationshipProcessProvenance(
			ref.ProcessKey,
			execution.Output,
			input.Context,
		); err != nil {
			return ErrInvalidInput
		}
	}
	return nil
}

func verifyHeadlessPersistenceScope(
	ctx context.Context,
	tx pgx.Tx,
	input HeadlessRefreshPersistence,
) error {
	var authorized bool
	if err := tx.QueryRow(ctx, `
		select exists (
		    select 1
		      from customer_data.relationships
		     where account_id = $1
		       and client_account_id = $2
		       and subject_id = $3
		       and id = $4
		       and archived_at is null
		)`,
		input.Scope.AccountID,
		input.Scope.ClientAccountID,
		input.SubjectID,
		input.RelationshipID,
	).Scan(&authorized); err != nil {
		return err
	}
	if !authorized {
		return ErrForbidden
	}
	if err := tx.QueryRow(ctx, `
		select exists (
		    select 1
		      from intelligence.context_snapshots
		     where account_id = $1
		       and client_account_id = $2
		       and subject_id = $3
		       and relationship_id = $4
		       and id = $5
		)`,
		input.Scope.AccountID,
		input.Scope.ClientAccountID,
		input.SubjectID,
		input.RelationshipID,
		input.ContextID,
	).Scan(&authorized); err != nil {
		return err
	}
	if !authorized {
		return ErrForbidden
	}
	for _, execution := range input.Executions {
		if err := tx.QueryRow(ctx, `
			select exists (
			    select 1
			      from intelligence.runtime_runs
			     where account_id = $1
			       and client_account_id = $2
			       and subject_id = $3
			       and relationship_id = $4
			       and context_snapshot_id = $5
			       and id = $6
			       and process_key = $7
			       and status = 'succeeded'
			)`,
			input.Scope.AccountID,
			input.Scope.ClientAccountID,
			input.SubjectID,
			input.RelationshipID,
			input.ContextID,
			execution.RunRef.RunID,
			execution.RunRef.ProcessKey,
		).Scan(&authorized); err != nil {
			return err
		}
		if !authorized {
			return ErrForbidden
		}
	}
	if err := verifyHeadlessContextReferences(ctx, tx, input); err != nil {
		return err
	}
	return nil
}

func verifyHeadlessContextReferences(
	ctx context.Context,
	tx pgx.Tx,
	input HeadlessRefreshPersistence,
) error {
	observationSet := make(map[string]processEvidenceRef)
	for _, observation := range input.Context.Observations {
		ref := processEvidenceRef{
			ObservationID: observation.ID,
			SourceKey:     observation.SourceKey,
		}
		observationSet[observationRefKey(ref.ObservationID, ref.SourceKey)] = ref
	}
	factSet := make(map[string]processFactRef)
	for _, fact := range input.Context.Facts {
		ref := processFactRef{FactID: fact.ID, FactKey: fact.Key, Version: fact.Version}
		factSet[factRefKey(ref.FactID, ref.FactKey, ref.Version)] = ref
		for _, evidence := range fact.Evidence {
			evidenceRef := processEvidenceRef{
				ObservationID: evidence.ObservationID,
				SourceKey:     evidence.SourceKey,
			}
			key := observationRefKey(evidenceRef.ObservationID, evidenceRef.SourceKey)
			observationSet[key] = evidenceRef
		}
	}
	observations := make([]processEvidenceRef, 0, len(observationSet))
	for _, ref := range observationSet {
		observations = append(observations, ref)
	}
	facts := make([]processFactRef, 0, len(factSet))
	for _, ref := range factSet {
		facts = append(facts, ref)
	}
	if len(observations) > 0 {
		raw, err := json.Marshal(observations)
		if err != nil {
			return err
		}
		var count int
		if err := tx.QueryRow(ctx, `
			select count(*)
			  from jsonb_array_elements($5::jsonb) expected
			  join intelligence.source_observations observation
			    on observation.id = (expected ->> 'observationId')::uuid
			   and observation.source_key = expected ->> 'sourceKey'
			   and observation.account_id = $1
			   and observation.client_account_id = $2
			   and observation.subject_id = $3
			   and observation.relationship_id = $4`,
			input.Scope.AccountID,
			input.Scope.ClientAccountID,
			input.SubjectID,
			input.RelationshipID,
			raw,
		).Scan(&count); err != nil {
			return err
		}
		if count != len(observations) {
			return ErrForbidden
		}
	}
	if len(facts) > 0 {
		raw, err := json.Marshal(facts)
		if err != nil {
			return err
		}
		var count int
		if err := tx.QueryRow(ctx, `
			select count(*)
			  from jsonb_array_elements($5::jsonb) expected
			  join intelligence.facts fact
			    on fact.id = (expected ->> 'factId')::uuid
			   and fact.fact_key = expected ->> 'factKey'
			   and fact.version = (expected ->> 'version')::integer
			   and fact.account_id = $1
			   and fact.client_account_id = $2
			   and fact.subject_id = $3
			   and fact.relationship_id = $4
			   and fact.resolution_state in ('resolved', 'verified', 'contested')`,
			input.Scope.AccountID,
			input.Scope.ClientAccountID,
			input.SubjectID,
			input.RelationshipID,
			raw,
		).Scan(&count); err != nil {
			return err
		}
		if count != len(facts) {
			return ErrForbidden
		}
	}
	return nil
}

func persistHeadlessSummary(
	ctx context.Context,
	tx pgx.Tx,
	input HeadlessRefreshPersistence,
	execution HeadlessPersistedExecution,
) (int, error) {
	var output profileSummaryResult
	if err := decodeStrictProcessOutput(execution.Output, &output); err != nil {
		return 0, ErrInvalidInput
	}
	var exists bool
	if err := tx.QueryRow(ctx, `
		select exists (
		    select 1
		      from intelligence.summary_versions
		     where account_id = $1 and runtime_run_id = $2
		)`,
		input.Scope.AccountID,
		execution.RunRef.RunID,
	).Scan(&exists); err != nil || exists {
		return 0, err
	}
	lockKey := strings.Join([]string{
		input.Scope.AccountID,
		input.RelationshipID,
		"relationship_profile",
	}, ":")
	if _, err := tx.Exec(
		ctx,
		`select pg_advisory_xact_lock(hashtextextended($1, 0))`,
		lockKey,
	); err != nil {
		return 0, err
	}
	var currentID string
	currentVersion := 0
	err := tx.QueryRow(ctx, `
		select id, version
		  from intelligence.summary_versions
		 where account_id = $1
		   and client_account_id = $2
		   and relationship_id = $3
		   and summary_type = 'relationship_profile'
		   and status = 'published'
		 order by version desc
		 limit 1
		 for update`,
		input.Scope.AccountID,
		input.Scope.ClientAccountID,
		input.RelationshipID,
	).Scan(&currentID, &currentVersion)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	var summaryID string
	inputFingerprint := hashBytes([]byte(input.ContextID + ":" + execution.OutputHash))
	if err := tx.QueryRow(ctx, `
		insert into intelligence.summary_versions (
		    account_id, client_account_id, subject_id, relationship_id,
		    summary_type, version, status, content_ciphertext,
		    sections_ciphertext, content_hash, cipher_key_version,
		    input_fingerprint, as_of, prompt_binding_id, runtime_run_id,
		    confidence
		)
		values (
		    $1, $2, $3, $4, 'relationship_profile', $5, 'draft', $6,
		    '', $7, 'v1', $8, $9, $10, $11, $12
		)
		returning id`,
		input.Scope.AccountID,
		input.Scope.ClientAccountID,
		input.SubjectID,
		input.RelationshipID,
		currentVersion+1,
		execution.OutputCiphertext,
		execution.OutputHash,
		inputFingerprint,
		input.AsOf,
		execution.RunRef.PromptBindingID,
		execution.RunRef.RunID,
		output.Confidence,
	).Scan(&summaryID); err != nil {
		return 0, err
	}
	if currentID != "" {
		if _, err := tx.Exec(ctx, `
			update intelligence.summary_versions
			   set status = 'superseded',
			       superseded_by_summary_id = $4
			 where account_id = $1
			   and client_account_id = $2
			   and id = $3
			   and status = 'published'`,
			input.Scope.AccountID,
			input.Scope.ClientAccountID,
			currentID,
			summaryID,
		); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(ctx, `
		update intelligence.summary_versions
		   set status = 'published', published_at = now()
		 where account_id = $1 and id = $2 and status = 'draft'`,
		input.Scope.AccountID,
		summaryID,
	); err != nil {
		return 0, err
	}
	factRefs := append([]processFactRef(nil), output.FactRefs...)
	for _, section := range output.Sections {
		factRefs = append(factRefs, section.FactRefs...)
	}
	seenFacts := make(map[string]struct{}, len(factRefs))
	for _, ref := range factRefs {
		if _, exists := seenFacts[ref.FactID]; exists {
			continue
		}
		seenFacts[ref.FactID] = struct{}{}
		if _, err := tx.Exec(ctx, `
			insert into intelligence.summary_evidence (
			    account_id, summary_id, fact_id
			)
			values ($1, $2, $3)
			on conflict do nothing`,
			input.Scope.AccountID,
			summaryID,
			ref.FactID,
		); err != nil {
			return 0, err
		}
	}
	if err := insertHeadlessAudit(
		ctx,
		tx,
		input,
		"headless.summary.published",
		"summary",
		summaryID,
		execution,
		map[string]any{"version": currentVersion + 1},
	); err != nil {
		return 0, err
	}
	return 1, nil
}

func persistHeadlessRecommendation(
	ctx context.Context,
	tx pgx.Tx,
	input HeadlessRefreshPersistence,
	execution HeadlessPersistedExecution,
) (int, error) {
	recommendationType, confidence, reasonCode, validFrom, expiresAt, err :=
		headlessRecommendationDetails(execution.RunRef.ProcessKey, execution.Output)
	if err != nil {
		return 0, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `
		select exists (
		    select 1
		      from intelligence.recommendations
		     where account_id = $1
		       and runtime_run_id = $2
		       and recommendation_type = $3
		)`,
		input.Scope.AccountID,
		execution.RunRef.RunID,
		recommendationType,
	).Scan(&exists); err != nil || exists {
		return 0, err
	}
	lockKey := strings.Join([]string{
		input.Scope.AccountID,
		input.RelationshipID,
		"recommendation",
		recommendationType,
	}, ":")
	if _, err := tx.Exec(
		ctx,
		`select pg_advisory_xact_lock(hashtextextended($1, 0))`,
		lockKey,
	); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		update intelligence.recommendations
		   set status = 'superseded', updated_at = now()
		 where account_id = $1
		   and client_account_id = $2
		   and relationship_id = $3
		   and recommendation_type = $4
		   and status = 'proposed'`,
		input.Scope.AccountID,
		input.Scope.ClientAccountID,
		input.RelationshipID,
		recommendationType,
	); err != nil {
		return 0, err
	}
	var recommendationID string
	if err := tx.QueryRow(ctx, `
		insert into intelligence.recommendations (
		    account_id, client_account_id, subject_id, relationship_id,
		    recommendation_type, status, payload_ciphertext,
		    cipher_key_version, rationale_code, confidence, valid_from,
		    expires_at, prompt_binding_id, runtime_run_id,
		    context_snapshot_id
		)
		values (
		    $1, $2, $3, $4, $5, 'proposed', $6, 'v1', $7, $8, $9,
		    $10, $11, $12, $13
		)
		returning id`,
		input.Scope.AccountID,
		input.Scope.ClientAccountID,
		input.SubjectID,
		input.RelationshipID,
		recommendationType,
		execution.OutputCiphertext,
		reasonCode,
		confidence,
		validFrom,
		expiresAt,
		execution.RunRef.PromptBindingID,
		execution.RunRef.RunID,
		input.ContextID,
	).Scan(&recommendationID); err != nil {
		return 0, err
	}
	if err := insertHeadlessAudit(
		ctx,
		tx,
		input,
		"headless.recommendation.proposed",
		"recommendation",
		recommendationID,
		execution,
		map[string]any{"recommendationType": recommendationType},
	); err != nil {
		return 0, err
	}
	return 1, nil
}

func headlessRecommendationDetails(
	processKey string,
	raw json.RawMessage,
) (string, float64, string, *time.Time, time.Time, error) {
	switch processKey {
	case "recommendation.follow_up":
		var output followUpRecommendationResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return "", 0, "", nil, time.Time{}, ErrInvalidInput
		}
		validFrom, err := time.Parse(time.RFC3339, output.RecommendedAt)
		if err != nil {
			return "", 0, "", nil, time.Time{}, ErrInvalidInput
		}
		expiresAt, err := time.Parse(time.RFC3339, output.ExpiresAt)
		if err != nil {
			return "", 0, "", nil, time.Time{}, ErrInvalidInput
		}
		return "follow_up", output.Confidence, firstReason(output.ReasonCodes),
			&validFrom, expiresAt, nil
	case "recommendation.offer":
		var output offerRecommendationResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return "", 0, "", nil, time.Time{}, ErrInvalidInput
		}
		validFrom, err := time.Parse(time.RFC3339, output.ValidityCheckedAt)
		if err != nil {
			return "", 0, "", nil, time.Time{}, ErrInvalidInput
		}
		expiresAt, err := time.Parse(time.RFC3339, output.ExpiresAt)
		if err != nil {
			return "", 0, "", nil, time.Time{}, ErrInvalidInput
		}
		return "offer", output.Confidence, firstReason(output.FitReasonCodes),
			&validFrom, expiresAt, nil
	case "recommendation.important_dates":
		var output importantDateRecommendationResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return "", 0, "", nil, time.Time{}, ErrInvalidInput
		}
		expiresAt, err := time.Parse(time.RFC3339, output.ExpiresAt)
		if err != nil {
			return "", 0, "", nil, time.Time{}, ErrInvalidInput
		}
		return "important_date", output.Confidence, firstReason(output.ReasonCodes),
			nil, expiresAt, nil
	default:
		return "", 0, "", nil, time.Time{}, ErrInvalidInput
	}
}

func persistHeadlessSourceSuggestions(
	ctx context.Context,
	tx pgx.Tx,
	input HeadlessRefreshPersistence,
	execution HeadlessPersistedExecution,
) (int, error) {
	var output sourceSuggestionResult
	if err := decodeStrictProcessOutput(execution.Output, &output); err != nil {
		return 0, ErrInvalidInput
	}
	persisted := 0
	for _, suggestion := range output.Suggestions {
		if !validSourceKey(suggestion.SourceKey) {
			return 0, ErrInvalidInput
		}
		expiresAt, err := time.Parse(time.RFC3339, suggestion.ExpiresAt)
		if err != nil {
			return 0, ErrInvalidInput
		}
		var exists bool
		if err := tx.QueryRow(ctx, `
			select exists (
			    select 1
			      from intelligence.source_suggestions
			     where account_id = $1
			       and runtime_run_id = $2
			       and source_key = $3
			)`,
			input.Scope.AccountID,
			execution.RunRef.RunID,
			suggestion.SourceKey,
		).Scan(&exists); err != nil {
			return 0, err
		}
		if exists {
			continue
		}
		lockKey := strings.Join([]string{
			input.Scope.AccountID,
			input.RelationshipID,
			"source",
			suggestion.SourceKey,
		}, ":")
		if _, err := tx.Exec(
			ctx,
			`select pg_advisory_xact_lock(hashtextextended($1, 0))`,
			lockKey,
		); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
			update intelligence.source_suggestions
			   set status = 'expired'
			 where account_id = $1
			   and client_account_id = $2
			   and relationship_id = $3
			   and source_key = $4
			   and status = 'proposed'`,
			input.Scope.AccountID,
			input.Scope.ClientAccountID,
			input.RelationshipID,
			suggestion.SourceKey,
		); err != nil {
			return 0, err
		}
		gapCodes, err := json.Marshal(uniqueSorted(suggestion.GapCodes))
		if err != nil {
			return 0, err
		}
		var suggestionID string
		if err := tx.QueryRow(ctx, `
			insert into intelligence.source_suggestions (
			    account_id, client_account_id, subject_id, relationship_id,
			    source_key, gap_codes, rationale_code, confidence, status,
			    runtime_run_id, rationale_ciphertext, cipher_key_version,
			    expires_at, prompt_binding_id, context_snapshot_id, output_hash
			)
			values (
			    $1, $2, $3, $4, $5, $6, $7, $8, 'proposed', $9, $10,
			    'v1', $11, $12, $13, $14
			)
			returning id`,
			input.Scope.AccountID,
			input.Scope.ClientAccountID,
			input.SubjectID,
			input.RelationshipID,
			suggestion.SourceKey,
			gapCodes,
			suggestion.RationaleCode,
			suggestion.Confidence,
			execution.RunRef.RunID,
			execution.OutputCiphertext,
			expiresAt,
			execution.RunRef.PromptBindingID,
			input.ContextID,
			execution.OutputHash,
		).Scan(&suggestionID); err != nil {
			return 0, err
		}
		if err := insertHeadlessAudit(
			ctx,
			tx,
			input,
			"headless.source_suggestion.proposed",
			"source_suggestion",
			suggestionID,
			execution,
			map[string]any{"sourceKey": suggestion.SourceKey},
		); err != nil {
			return 0, err
		}
		persisted++
	}
	return persisted, nil
}

func insertHeadlessAudit(
	ctx context.Context,
	tx pgx.Tx,
	input HeadlessRefreshPersistence,
	eventType, aggregateType, aggregateID string,
	execution HeadlessPersistedExecution,
	extra map[string]any,
) error {
	metadata := map[string]any{
		"processKey":   execution.RunRef.ProcessKey,
		"runtimeRunId": execution.RunRef.RunID,
		"contextId":    input.ContextID,
	}
	for key, value := range extra {
		metadata[key] = value
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		insert into intelligence.audit_events (
		    account_id, client_account_id, event_type, aggregate_type,
		    aggregate_id, correlation_id, reason_code, metadata
		)
		values ($1, $2, $3, $4, $5, $6, 'headless_refresh', $7)`,
		input.Scope.AccountID,
		input.Scope.ClientAccountID,
		eventType,
		aggregateType,
		aggregateID,
		execution.RunRef.RunID,
		raw,
	)
	return err
}

func firstReason(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return "model_recommendation"
}

var _ headlessResultRepository = (*PostgresRepository)(nil)
