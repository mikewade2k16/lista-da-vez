package customerintelligence

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

func TestContextSnapshotRetentionRepositoryLifecycle(t *testing.T) {
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
		values ($1, 'CI context retention owner')
		returning id`,
		"ci-context-retention-owner-"+suffix,
	).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		insert into core.accounts (slug, name)
		values ($1, 'CI context retention client')
		returning id`,
		"ci-context-retention-client-"+suffix,
	).Scan(&clientAccountID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		insert into core.users (email, display_name)
		values ($1, 'CI context retention actor')
		returning id`,
		"ci-context-retention-"+suffix+"@example.invalid",
	).Scan(&actorID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(
			cleanupCtx,
			`delete from core.accounts where id = $1`,
			accountID,
		)
		_, _ = pool.Exec(
			cleanupCtx,
			`delete from core.accounts where id = $1`,
			clientAccountID,
		)
		_, _ = pool.Exec(
			cleanupCtx,
			`delete from core.users where id = $1`,
			actorID,
		)
	})

	repository := NewPostgresRepository(pool)
	now := time.Now().UTC()
	raw := []byte(`{"schemaVersion":"context.envelope.v1","sensitive":"value"}`)
	snapshotID, err := repository.SaveContextSnapshot(
		ctx,
		ContextEnvelope{
			AccountID:       accountID,
			ClientAccountID: clientAccountID,
			SubjectID:       "44444444-4444-4444-8444-444444444444",
			RelationshipID:  "33333333-3333-4333-8333-333333333333",
			ProcessKeys:     []string{"conversation.reply"},
			Purpose:         "customer_service",
			AsOf:            now.Add(-time.Hour),
			ExpiresAt:       now.Add(-time.Minute),
			Budget: ContextBudget{
				IncludedItems:   3,
				EstimatedTokens: 120,
			},
			Warnings: []string{"source_stale"},
		},
		"v1:encrypted-context-payload",
		hashBytes(raw),
	)
	if err != nil {
		t.Fatal(err)
	}

	scopes, err := repository.ListExpiredRetentionScopes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !containsRetentionScope(scopes, accountID, clientAccountID) {
		t.Fatalf("expired context scope missing: %#v", scopes)
	}
	applied, err := repository.ApplyExpiredContextSnapshotRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: accountID},
		"context-retention-cross-client",
		10,
	)
	if err != nil || applied != 0 {
		t.Fatalf("cross-client applied=%d err=%v", applied, err)
	}

	var legalHoldID string
	if err := pool.QueryRow(ctx, `
		insert into intelligence.context_snapshot_legal_holds (
		    account_id,
		    client_account_id,
		    context_snapshot_id,
		    reason_code,
		    hold_reference,
		    created_by_user_id
		)
		values ($1, $2, $3, 'litigation_preservation', 'CONTEXT-HOLD-1', $4)
		returning id`,
		accountID,
		clientAccountID,
		snapshotID,
		actorID,
	).Scan(&legalHoldID); err != nil {
		t.Fatal(err)
	}
	scopes, err = repository.ListExpiredRetentionScopes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if containsRetentionScope(scopes, accountID, clientAccountID) {
		t.Fatalf("held context was scheduled for retention: %#v", scopes)
	}
	applied, err = repository.ApplyExpiredContextSnapshotRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: clientAccountID},
		"context-retention-held",
		10,
	)
	if err != nil || applied != 0 {
		t.Fatalf("held context applied=%d err=%v", applied, err)
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
		snapshotID,
	); err == nil {
		t.Fatal("database allowed context retention under active legal hold")
	}
	var heldState, heldCiphertext string
	if err := pool.QueryRow(ctx, `
		select retention_state, coalesce(payload_ciphertext, '')
		from intelligence.context_snapshots
		where account_id = $1 and client_account_id = $2 and id = $3`,
		accountID,
		clientAccountID,
		snapshotID,
	).Scan(&heldState, &heldCiphertext); err != nil {
		t.Fatal(err)
	}
	if heldState != retentionStateActive || heldCiphertext == "" {
		t.Fatalf(
			"held context state=%q ciphertextPresent=%v",
			heldState,
			heldCiphertext != "",
		)
	}

	if _, err := pool.Exec(ctx, `
		update intelligence.context_snapshot_legal_holds
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
	applied, err = repository.ApplyExpiredContextSnapshotRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: clientAccountID},
		"context-retention-apply",
		10,
	)
	if err != nil || applied != 1 {
		t.Fatalf("context retention applied=%d err=%v", applied, err)
	}
	applied, err = repository.ApplyExpiredContextSnapshotRetention(
		ctx,
		Scope{AccountID: accountID, ClientAccountID: clientAccountID},
		"context-retention-replay",
		10,
	)
	if err != nil || applied != 0 {
		t.Fatalf("context retention replay applied=%d err=%v", applied, err)
	}

	var payloadNull, tombstoned bool
	var cipherVersion, payloadHash, state, reasonCode string
	if err := pool.QueryRow(ctx, `
		select
		    payload_ciphertext is null,
		    cipher_key_version,
		    payload_hash,
		    retention_state,
		    tombstoned_at is not null,
		    retention_reason_code
		from intelligence.context_snapshots
		where account_id = $1 and client_account_id = $2 and id = $3`,
		accountID,
		clientAccountID,
		snapshotID,
	).Scan(
		&payloadNull,
		&cipherVersion,
		&payloadHash,
		&state,
		&tombstoned,
		&reasonCode,
	); err != nil {
		t.Fatal(err)
	}
	if !payloadNull ||
		cipherVersion != "" ||
		payloadHash != "" ||
		state != contextSnapshotRetentionStateCryptoShredded ||
		!tombstoned ||
		reasonCode != "context_snapshot_expired" {
		t.Fatalf(
			"retained null=%v key=%q hash=%q state=%q tombstoned=%v reason=%q",
			payloadNull,
			cipherVersion,
			payloadHash,
			state,
			tombstoned,
			reasonCode,
		)
	}

	var retentionAuditCount, holdAuditCount int
	var metadataBytes []byte
	if err := pool.QueryRow(ctx, `
		select count(*)::integer, min(metadata::text)
		from intelligence.audit_events
		where account_id = $1
		  and client_account_id = $2
		  and aggregate_type = 'context_snapshot'
		  and aggregate_id = $3
		  and event_type = 'context.snapshot_retention_applied'`,
		accountID,
		clientAccountID,
		snapshotID,
	).Scan(&retentionAuditCount, &metadataBytes); err != nil {
		t.Fatal(err)
	}
	if retentionAuditCount != 1 {
		t.Fatalf("context retention audit count = %d", retentionAuditCount)
	}
	if err := pool.QueryRow(ctx, `
		select count(*)::integer
		from intelligence.audit_events
		where account_id = $1
		  and client_account_id = $2
		  and aggregate_type = 'context_snapshot_legal_hold'
		  and aggregate_id = $3
		  and event_type in (
		      'context.snapshot_legal_hold_created',
		      'context.snapshot_legal_hold_released'
		  )`,
		accountID,
		clientAccountID,
		legalHoldID,
	).Scan(&holdAuditCount); err != nil {
		t.Fatal(err)
	}
	if holdAuditCount != 2 {
		t.Fatalf("context legal hold audit count = %d", holdAuditCount)
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"payload",
		"payloadCiphertext",
		"payloadHash",
		"cipherKeyVersion",
		"subjectId",
		"relationshipId",
	} {
		if _, ok := metadata[forbidden]; ok {
			t.Fatalf("context audit leaked %q: %#v", forbidden, metadata)
		}
	}
}

func containsRetentionScope(
	scopes []Scope,
	accountID, clientAccountID string,
) bool {
	for _, scope := range scopes {
		if scope.AccountID == accountID &&
			scope.ClientAccountID == clientAccountID {
			return true
		}
	}
	return false
}
