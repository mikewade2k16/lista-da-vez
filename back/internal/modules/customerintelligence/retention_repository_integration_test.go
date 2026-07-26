package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

func TestObservationRetentionRepositoryLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL nao definido")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.ApplyMigrationsWithOptions(
		ctx,
		pool,
		database.MigrationOptions{SkipDataSeeds: true},
	); err != nil {
		t.Fatal(err)
	}

	suffix := time.Now().UTC().Format("20060102150405.000000000")
	var accountID, clientAccountID, actorID string
	if err := pool.QueryRow(ctx, `
		insert into core.accounts (slug, name)
		values ($1, 'CI retention owner')
		returning id`,
		"ci-retention-owner-"+suffix,
	).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		insert into core.accounts (slug, name)
		values ($1, 'CI retention client')
		returning id`,
		"ci-retention-client-"+suffix,
	).Scan(&clientAccountID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		insert into core.users (email, display_name)
		values ($1, 'CI retention actor')
		returning id`,
		"ci-retention-"+suffix+"@example.invalid",
	).Scan(&actorID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `delete from core.accounts where id = $1`, accountID)
		_, _ = pool.Exec(cleanupCtx, `delete from core.accounts where id = $1`, clientAccountID)
		_, _ = pool.Exec(cleanupCtx, `delete from core.users where id = $1`, actorID)
	})

	repository := NewPostgresRepository(pool)
	sourceInput := SourceConfigInput{
		ClientAccountID:    clientAccountID,
		SourceKey:          "erp",
		ConnectionKey:      "retention",
		Status:             "enabled",
		Mode:               "scheduled",
		PurposeKey:         "customer_profile",
		FieldAllowlist:     []string{"total_amount_cents"},
		FreshnessSeconds:   3600,
		RetentionPolicyKey: "customer_profile.short",
		SnapshotTTLSeconds: minSnapshotTTLSeconds,
		OnExpiry:           retentionActionTombstone,
		Config: json.RawMessage(
			`{"connectionId":"erp-retention","entityTypes":["order"]}`,
		),
	}
	if _, err := repository.UpsertSourceConfig(
		ctx,
		accountID,
		actorID,
		sourceInput,
	); !errors.Is(err, ErrRetentionPolicyApprovalRequired) {
		t.Fatalf("source without approved retention policy error = %v", err)
	}
	var implicitPolicyCount int
	if err := pool.QueryRow(ctx, `
		select count(*)::integer
		from intelligence.retention_policy_versions
		where account_id = $1 and policy_key = $2`,
		accountID,
		sourceInput.RetentionPolicyKey,
	).Scan(&implicitPolicyCount); err != nil {
		t.Fatal(err)
	}
	if implicitPolicyCount != 0 {
		t.Fatalf("source config implicitly created %d policies", implicitPolicyCount)
	}

	if _, err := pool.Exec(ctx, `
		insert into intelligence.retention_policy_versions (
		    account_id, policy_key, version, status, category_rules,
		    snapshot_ttl_seconds, on_expiry, legal_hold_behavior,
		    block_reingestion, created_by_user_id, published_by_user_id,
		    published_at, publication_reason_code, approval_reference
		)
		values (
		    $1, 'customer_profile.illegal_direct_publish', 1, 'published',
		    '{}'::jsonb, $2, 'tombstone', 'preserve', true,
		    $3, $3, now(), 'legal_review_approved', 'LEGAL-DIRECT-1'
		)`,
		accountID,
		minSnapshotTTLSeconds,
		actorID,
	); err == nil {
		t.Fatal("direct published policy insert unexpectedly succeeded")
	}

	firstDraft, err := repository.CreateRetentionPolicyDraft(
		ctx,
		accountID,
		actorID,
		sourceInput.RetentionPolicyKey,
		RetentionPolicyDraftInput{
			SnapshotTTLSeconds: sourceInput.SnapshotTTLSeconds,
			OnExpiry:           sourceInput.OnExpiry,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if firstDraft.Status != "draft" ||
		firstDraft.Revision != 1 ||
		firstDraft.PublishedAt != nil {
		t.Fatalf("first draft = %#v", firstDraft)
	}
	if _, err := repository.PublishRetentionPolicyVersion(
		ctx,
		accountID,
		actorID,
		firstDraft.ID,
		PublishRetentionPolicyInput{
			ExpectedRevision:  firstDraft.Revision + 1,
			ReasonCode:        "legal_review_approved",
			ApprovalReference: "LEGAL-RETENTION-1",
		},
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("publish with stale revision error = %v", err)
	}
	firstPolicy, err := repository.PublishRetentionPolicyVersion(
		ctx,
		accountID,
		actorID,
		firstDraft.ID,
		PublishRetentionPolicyInput{
			ExpectedRevision:  firstDraft.Revision,
			ReasonCode:        "legal_review_approved",
			ApprovalReference: "LEGAL-RETENTION-1",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if firstPolicy.Status != "published" ||
		firstPolicy.Revision != firstDraft.Revision+1 ||
		firstPolicy.PublishedByUserID != actorID ||
		firstPolicy.ApprovalReference != "LEGAL-RETENTION-1" {
		t.Fatalf("first published policy = %#v", firstPolicy)
	}

	source, err := repository.UpsertSourceConfig(
		ctx,
		accountID,
		actorID,
		sourceInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	firstPolicyID := source.RetentionPolicyVersionID
	firstPolicyVersion := source.RetentionPolicyVersion
	if firstPolicyID != firstPolicy.ID ||
		firstPolicyVersion != firstPolicy.Version {
		t.Fatalf("source policy = %#v, published = %#v", source, firstPolicy)
	}

	secondDraft, err := repository.CreateRetentionPolicyDraft(
		ctx,
		accountID,
		actorID,
		sourceInput.RetentionPolicyKey,
		RetentionPolicyDraftInput{
			SnapshotTTLSeconds: sourceInput.SnapshotTTLSeconds,
			OnExpiry:           retentionActionCryptoShred,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	secondPolicy, err := repository.PublishRetentionPolicyVersion(
		ctx,
		accountID,
		actorID,
		secondDraft.ID,
		PublishRetentionPolicyInput{
			ExpectedRevision:  secondDraft.Revision,
			ReasonCode:        "legal_review_approved",
			ApprovalReference: "LEGAL-RETENTION-2",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	sourceInput.ExpectedRevision = source.Revision
	sourceInput.OnExpiry = retentionActionCryptoShred
	source, err = repository.UpsertSourceConfig(
		ctx,
		accountID,
		actorID,
		sourceInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	if source.RetentionPolicyVersionID == "" ||
		source.RetentionPolicyVersionID == firstPolicyID ||
		source.RetentionPolicyVersion != firstPolicyVersion+1 ||
		source.RetentionPolicyVersionID != secondPolicy.ID ||
		source.SnapshotTTLSeconds != minSnapshotTTLSeconds ||
		source.OnExpiry != retentionActionCryptoShred {
		t.Fatalf("source retention = %#v", source)
	}
	var oldPolicyStatus string
	if err := pool.QueryRow(ctx, `
		select status
		from intelligence.retention_policy_versions
		where account_id = $1 and id = $2`,
		accountID,
		firstPolicyID,
	).Scan(&oldPolicyStatus); err != nil {
		t.Fatal(err)
	}
	if oldPolicyStatus != "published" {
		t.Fatalf("old immutable policy status = %q", oldPolicyStatus)
	}
	var publicationAuditCount int
	if err := pool.QueryRow(ctx, `
		select count(*)::integer
		from intelligence.audit_events
		where account_id = $1
		  and event_type = 'retention_policy.published'
		  and aggregate_type = 'retention_policy_version'
		  and aggregate_id in ($2, $3)
		  and metadata ->> 'approvalReference' in (
		      'LEGAL-RETENTION-1',
		      'LEGAL-RETENTION-2'
		  )`,
		accountID,
		firstPolicy.ID,
		secondPolicy.ID,
	).Scan(&publicationAuditCount); err != nil {
		t.Fatal(err)
	}
	if publicationAuditCount != 2 {
		t.Fatalf("publication audit count = %d", publicationAuditCount)
	}

	run, created, err := repository.CreateSourceRun(ctx, SourceSyncRequest{
		AccountID:       accountID,
		ClientAccountID: clientAccountID,
		SourceConfigID:  source.ID,
		IdempotencyKey:  "retention-integration-run",
		Trigger:         "manual",
	})
	if err != nil || !created ||
		run.RetentionPolicyVersionID != source.RetentionPolicyVersionID {
		t.Fatalf("run=%#v created=%v err=%v", run, created, err)
	}

	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt(`{"total_amount_cents":300}`)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := repository.InsertObservations(ctx, run, []Observation{{
		IdempotencyKey:     "retention-observation-1",
		EntityType:         "order",
		EntityID:           "order-retention-1",
		Version:            "1",
		SubjectID:          "44444444-4444-4444-8444-444444444444",
		RelationshipID:     "33333333-3333-4333-8333-333333333333",
		Snapshot:           json.RawMessage(`{"total_amount_cents":300}`),
		SnapshotCiphertext: ciphertext,
		Sensitivity:        "personal",
		PurposeKey:         "customer_profile",
	}})
	if err != nil || accepted != 1 {
		t.Fatalf("insert accepted=%d err=%v", accepted, err)
	}

	var observationID, policyID, state string
	var observedAt, expiresAt time.Time
	if err := pool.QueryRow(ctx, `
		select id, retention_policy_version_id, retention_state,
		       observed_at, expires_at
		from intelligence.source_observations
		where account_id = $1 and client_account_id = $2
		  and source_config_id = $3 and idempotency_key = $4`,
		accountID,
		clientAccountID,
		source.ID,
		"retention-observation-1",
	).Scan(
		&observationID,
		&policyID,
		&state,
		&observedAt,
		&expiresAt,
	); err != nil {
		t.Fatal(err)
	}
	if policyID != source.RetentionPolicyVersionID || state != retentionStateActive {
		t.Fatalf("policy=%s state=%s", policyID, state)
	}
	if delta := expiresAt.Sub(observedAt); delta != 24*time.Hour {
		t.Fatalf("expiry delta = %s", delta)
	}

	if _, err := pool.Exec(ctx, `
		update intelligence.source_observations
		set expires_at = now() - interval '1 second'
		where account_id = $1 and client_account_id = $2 and id = $3`,
		accountID,
		clientAccountID,
		observationID,
	); err != nil {
		t.Fatal(err)
	}

	applied, err := repository.ApplyExpiredObservationRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: accountID},
		"retention-cross-client-probe",
		10,
	)
	if err != nil || applied != 0 {
		t.Fatalf("cross-client applied=%d err=%v", applied, err)
	}

	var legalHoldID string
	if err := pool.QueryRow(ctx, `
		insert into intelligence.observation_legal_holds (
		    account_id,
		    client_account_id,
		    observation_id,
		    reason_code,
		    hold_reference,
		    created_by_user_id
		)
		values ($1, $2, $3, 'litigation_preservation', 'LEGAL-HOLD-1', $4)
		returning id`,
		accountID,
		clientAccountID,
		observationID,
		actorID,
	).Scan(&legalHoldID); err != nil {
		t.Fatal(err)
	}
	heldContextSnapshotID, err := repository.SaveContextSnapshot(
		ctx,
		ContextEnvelope{
			AccountID:       accountID,
			ClientAccountID: clientAccountID,
			SubjectID:       "44444444-4444-4444-8444-444444444444",
			RelationshipID:  "33333333-3333-4333-8333-333333333333",
			ProcessKeys:     []string{"conversation.reply"},
			Purpose:         "customer_service",
			AsOf:            time.Now().UTC().Add(-time.Hour),
			ExpiresAt:       time.Now().UTC().Add(-time.Minute),
			Budget: ContextBudget{
				IncludedItems:   1,
				EstimatedTokens: 32,
			},
			Warnings: []string{},
		},
		"v1:observation-hold-protected-context",
		hashBytes([]byte(`{"heldBy":"observation"}`)),
	)
	if err != nil {
		t.Fatal(err)
	}
	scopes, err := repository.ListExpiredRetentionScopes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range scopes {
		if scope.AccountID == accountID &&
			scope.ClientAccountID == clientAccountID {
			t.Fatalf("held observation was scheduled for retention: %#v", scope)
		}
	}
	applied, err = repository.ApplyExpiredObservationRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: clientAccountID},
		"retention-held-observation",
		10,
	)
	if err != nil || applied != 0 {
		t.Fatalf("held retention applied=%d err=%v", applied, err)
	}
	contextApplied, err := repository.ApplyExpiredContextSnapshotRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: clientAccountID},
		"retention-context-held-by-observation",
		10,
	)
	if err != nil || contextApplied != 0 {
		t.Fatalf(
			"observation-held context retention applied=%d err=%v",
			contextApplied,
			err,
		)
	}
	if _, err := pool.Exec(ctx, `
		update intelligence.source_observations
		set
		    snapshot_json = null,
		    snapshot_ciphertext = null,
		    cipher_key_version = '',
		    retention_state = 'crypto_shredded',
		    retention_applied_at = now(),
		    retention_reason_code = 'retention_policy_expired'
		where account_id = $1 and client_account_id = $2 and id = $3`,
		accountID,
		clientAccountID,
		observationID,
	); err == nil {
		t.Fatal("database allowed retention while legal hold was active")
	}
	var heldState string
	var heldCiphertextPresent bool
	if err := pool.QueryRow(ctx, `
		select retention_state, snapshot_ciphertext is not null
		from intelligence.source_observations
		where account_id = $1 and client_account_id = $2 and id = $3`,
		accountID,
		clientAccountID,
		observationID,
	).Scan(&heldState, &heldCiphertextPresent); err != nil {
		t.Fatal(err)
	}
	if heldState != retentionStateActive || !heldCiphertextPresent {
		t.Fatalf(
			"held observation state=%q ciphertextPresent=%v",
			heldState,
			heldCiphertextPresent,
		)
	}
	if _, err := pool.Exec(ctx, `
		update intelligence.context_snapshots
		set
		    payload_ciphertext = null,
		    cipher_key_version = '',
		    payload_hash = '',
		    retention_state = 'crypto_shredded',
		    tombstoned_at = now(),
		    retention_reason_code = 'context_snapshot_expired'
		where account_id = $1 and client_account_id = $2 and id = $3`,
		accountID,
		clientAccountID,
		heldContextSnapshotID,
	); err == nil {
		t.Fatal("database allowed context retention while related observation hold was active")
	}
	if _, err := pool.Exec(ctx, `
		update intelligence.observation_legal_holds
		set
		    status = 'released',
		    released_by_user_id = $4,
		    released_at = now()
		where account_id = $1
		  and client_account_id = $2
		  and id = $3`,
		accountID,
		clientAccountID,
		legalHoldID,
		actorID,
	); err != nil {
		t.Fatal(err)
	}
	var legalHoldAuditCount int
	if err := pool.QueryRow(ctx, `
		select count(*)::integer
		from intelligence.audit_events
		where account_id = $1
		  and client_account_id = $2
		  and aggregate_type = 'observation_legal_hold'
		  and aggregate_id = $3
		  and event_type in (
		      'source.observation_legal_hold_created',
		      'source.observation_legal_hold_released'
		  )`,
		accountID,
		clientAccountID,
		legalHoldID,
	).Scan(&legalHoldAuditCount); err != nil {
		t.Fatal(err)
	}
	if legalHoldAuditCount != 2 {
		t.Fatalf("legal hold audit count = %d", legalHoldAuditCount)
	}

	contextApplied, err = repository.ApplyExpiredContextSnapshotRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: clientAccountID},
		"retention-context-after-observation-hold",
		10,
	)
	if err != nil || contextApplied != 1 {
		t.Fatalf(
			"released observation hold context retention applied=%d err=%v",
			contextApplied,
			err,
		)
	}
	contextApplied, err = repository.ApplyExpiredContextSnapshotRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: clientAccountID},
		"retention-context-after-observation-hold-replay",
		10,
	)
	if err != nil || contextApplied != 0 {
		t.Fatalf(
			"released observation hold context retention replay=%d err=%v",
			contextApplied,
			err,
		)
	}
	var contextState string
	if err := pool.QueryRow(ctx, `
		select retention_state
		from intelligence.context_snapshots
		where account_id = $1 and client_account_id = $2 and id = $3`,
		accountID,
		clientAccountID,
		heldContextSnapshotID,
	).Scan(&contextState); err != nil {
		t.Fatal(err)
	}
	if contextState != contextSnapshotRetentionStateCryptoShredded {
		t.Fatalf("released observation hold context state=%q", contextState)
	}

	applied, err = repository.ApplyExpiredObservationRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: clientAccountID},
		"retention-job-integration",
		10,
	)
	if err != nil || applied != 1 {
		t.Fatalf("retention applied=%d err=%v", applied, err)
	}
	applied, err = repository.ApplyExpiredObservationRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: clientAccountID},
		"retention-job-integration-replay",
		10,
	)
	if err != nil || applied != 0 {
		t.Fatalf("retention replay applied=%d err=%v", applied, err)
	}

	var snapshotJSONNull, ciphertextNull, appliedAtSet bool
	var reasonCode string
	if err := pool.QueryRow(ctx, `
		select snapshot_json is null, snapshot_ciphertext is null,
		       retention_state, retention_applied_at is not null,
		       retention_reason_code
		from intelligence.source_observations
		where account_id = $1 and client_account_id = $2 and id = $3`,
		accountID,
		clientAccountID,
		observationID,
	).Scan(
		&snapshotJSONNull,
		&ciphertextNull,
		&state,
		&appliedAtSet,
		&reasonCode,
	); err != nil {
		t.Fatal(err)
	}
	if !snapshotJSONNull || !ciphertextNull || !appliedAtSet ||
		state != retentionStateCryptoShredded ||
		reasonCode != "retention_policy_expired" {
		t.Fatalf(
			"retained jsonNull=%v cipherNull=%v state=%s applied=%v reason=%s",
			snapshotJSONNull,
			ciphertextNull,
			state,
			appliedAtSet,
			reasonCode,
		)
	}

	var auditCount int
	var metadataBytes []byte
	if err := pool.QueryRow(ctx, `
		select count(*)::integer, min(metadata::text)
		from intelligence.audit_events
		where account_id = $1
		  and client_account_id = $2
		  and aggregate_type = 'source_observation'
		  and aggregate_id = $3
		  and event_type = 'source.observation_retention_applied'`,
		accountID,
		clientAccountID,
		observationID,
	).Scan(&auditCount, &metadataBytes); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("retention audit count = %d", auditCount)
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"snapshot",
		"snapshotCiphertext",
		"sourceEntityId",
		"idempotencyKey",
		"payload",
	} {
		if _, ok := metadata[forbidden]; ok {
			t.Fatalf("audit leaked %q: %#v", forbidden, metadata)
		}
	}
}
