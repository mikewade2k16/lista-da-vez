package omnichannel

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAIInboundIntentTransaction uses a disposable PostgreSQL database to prove
// that the inbound marker and AI intent share one commit/rollback boundary.
func TestAIInboundIntentTransaction(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OMNI_AI_INBOUND_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("OMNI_AI_INBOUND_TEST_DATABASE_URL nao definido")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	var databaseName string
	if err := pool.QueryRow(ctx, `select current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("current_database: %v", err)
	}
	if !strings.HasPrefix(databaseName, "omni_ai_inbound_test_") {
		t.Fatalf("banco de teste recusado: %q", databaseName)
	}

	if _, err := pool.Exec(ctx, `
		drop schema if exists messaging cascade;
		create schema messaging;
		create table messaging.atomic_probe (
			probe_key text primary key
		);
		create table messaging.conversations (
			id uuid primary key,
			account_id uuid not null,
			state text not null,
			queue_id uuid,
			department_id uuid,
			assigned_user_id uuid,
			last_message_at timestamptz not null default now(),
			ai_generation bigint not null default 0,
			updated_at timestamptz not null default now()
		);
		create table messaging.outbox (
			id bigserial primary key,
			account_id uuid not null,
			ordering_key text not null,
			idempotency_key text not null,
			kind text not null,
			payload jsonb not null,
			status text not null default 'pending',
			attempts integer not null default 0,
			max_attempts integer not null default 3,
			run_after timestamptz not null default now(),
			locked_at timestamptz,
			locked_by text not null default '',
			last_error text not null default '',
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			unique (account_id, idempotency_key)
		);
	`); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	const (
		accountID      = "11111111-1111-4111-8111-111111111111"
		conversationID = "22222222-2222-4222-8222-222222222222"
		messageID      = "33333333-3333-4333-8333-333333333333"
	)
	if _, err := pool.Exec(ctx, `insert into messaging.conversations
		(id, account_id, state) values ($1::uuid, $2::uuid, 'new')`,
		conversationID, accountID); err != nil {
		t.Fatal(err)
	}

	t.Run("commit e idempotencia", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if _, err := tx.Exec(ctx, `insert into messaging.atomic_probe(probe_key)
			values ('message-committed')`); err != nil {
			t.Fatal(err)
		}
		if err := applyStateUpdateTx(
			ctx,
			tx,
			accountID,
			conversationID,
			stateUpdate{
				State:               StateAIActive,
				BumpLastMessage:     true,
				AdvanceAIGeneration: true,
			},
			false,
		); err != nil {
			t.Fatal(err)
		}
		if err := enqueueAIInboundJobTx(
			ctx, tx, accountID, conversationID, messageID,
		); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		var probeCount, intentCount int
		var state string
		var generation int64
		var kind, payloadConversationID, payloadMessageID string
		if err := pool.QueryRow(ctx, `select count(*) from messaging.atomic_probe
			where probe_key='message-committed'`).Scan(&probeCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*), min(kind),
				min(payload->>'conversationId'), min(payload->>'messageId')
			from messaging.outbox
			where account_id=$1::uuid and idempotency_key=$2`,
			accountID, "ai-inbound:"+messageID).
			Scan(&intentCount, &kind, &payloadConversationID, &payloadMessageID); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select state, ai_generation
			from messaging.conversations
			where account_id=$1::uuid and id=$2::uuid`,
			accountID, conversationID).Scan(&state, &generation); err != nil {
			t.Fatal(err)
		}
		if probeCount != 1 || intentCount != 1 ||
			state != string(StateAIActive) || generation != 1 ||
			kind != AIInboundJobKind ||
			payloadConversationID != conversationID ||
			payloadMessageID != messageID {
			t.Fatalf(
				"commit mismatch: probe=%d intent=%d state=%q generation=%d kind=%q conversation=%q message=%q",
				probeCount, intentCount, state, generation, kind,
				payloadConversationID, payloadMessageID,
			)
		}

		replay, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = replay.Rollback(ctx) }()
		if err := enqueueAIInboundJobTx(
			ctx, replay, accountID, conversationID, messageID,
		); err != nil {
			t.Fatal(err)
		}
		if err := replay.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*) from messaging.outbox
			where account_id=$1::uuid and idempotency_key=$2`,
			accountID, "ai-inbound:"+messageID).Scan(&intentCount); err != nil {
			t.Fatal(err)
		}
		if intentCount != 1 {
			t.Fatalf("intent count after replay = %d, want 1", intentCount)
		}
	})

	t.Run("falha do intento reverte a escrita anterior", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `alter table messaging.outbox
			add constraint reject_ai_inbound_intent
			check (kind <> 'omnichannel.ai.inbound') not valid`); err != nil {
			t.Fatal(err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `insert into messaging.atomic_probe(probe_key)
			values ('message-rolled-back')`); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err := applyStateUpdateTx(
			ctx,
			tx,
			accountID,
			conversationID,
			stateUpdate{
				State:               StateAIActive,
				BumpLastMessage:     true,
				AdvanceAIGeneration: true,
			},
			false,
		); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		failedMessageID := "44444444-4444-4444-8444-444444444444"
		if err := enqueueAIInboundJobTx(
			ctx, tx, accountID, conversationID, failedMessageID,
		); err == nil {
			_ = tx.Rollback(ctx)
			t.Fatal("enqueueAIInboundJobTx should fail")
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}

		var probeCount, intentCount int
		var generation int64
		if err := pool.QueryRow(ctx, `select count(*) from messaging.atomic_probe
			where probe_key='message-rolled-back'`).Scan(&probeCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select count(*) from messaging.outbox
			where account_id=$1::uuid and idempotency_key=$2`,
			accountID, "ai-inbound:"+failedMessageID).Scan(&intentCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `select ai_generation from messaging.conversations
			where account_id=$1::uuid and id=$2::uuid`,
			accountID, conversationID).Scan(&generation); err != nil {
			t.Fatal(err)
		}
		if probeCount != 0 || intentCount != 0 || generation != 1 {
			t.Fatalf(
				"partial transaction: probe=%d intent=%d generation=%d",
				probeCount, intentCount, generation,
			)
		}
	})
}
