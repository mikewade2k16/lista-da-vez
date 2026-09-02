package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	platformdb "github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

// TestHistoryAutoCloseFenceIntegration proves that an AI auto-close cannot
// materialize a reply derived from hidden history after a connection reset.
func TestHistoryAutoCloseFenceIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OMNI_E1_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("OMNI_E1_TEST_DATABASE_URL nao definido")
	}
	ctx, cancel := context.WithCancel(context.Background())
	fixturePool, err := newHistoryFixturePool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	appPool, err := newHistoryPool(ctx, dsn, "")
	if err != nil {
		fixturePool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		appPool.Close()
		fixturePool.Close()
	})
	var databaseName string
	if err := fixturePool.QueryRow(ctx, `select current_database()`).Scan(&databaseName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(databaseName, "omni_e1_test_") {
		t.Fatalf("banco de teste recusado: %q", databaseName)
	}
	if err := platformdb.ApplyMigrationsWithOptions(ctx, fixturePool,
		platformdb.MigrationOptions{SkipDataSeeds: true}); err != nil {
		t.Fatalf("apply real migrations: %v", err)
	}

	const (
		accountID = "81818181-8181-4181-8181-818181818181"
		actorID   = "82828282-8282-4282-8282-828282828282"
	)
	_, _ = fixturePool.Exec(ctx, `delete from core.accounts where id=$1::uuid;
		delete from core.users where id=$2::uuid`, accountID, actorID)
	t.Cleanup(func() {
		_, _ = fixturePool.Exec(context.Background(), `delete from core.accounts where id=$1::uuid;
			delete from core.users where id=$2::uuid`, accountID, actorID)
	})
	if _, err := fixturePool.Exec(ctx, `insert into core.accounts(id,slug,name)
		values ($1::uuid,'p0-auto-close-fence','P0 auto-close fence');
		insert into core.users(id,email,display_name)
		values ($2::uuid,'p0-auto-close@example.invalid','P0 auto-close actor');
		insert into core.account_users(account_id,user_id) values ($1::uuid,$2::uuid)`,
		accountID, actorID); err != nil {
		t.Fatal(err)
	}

	type scope struct {
		instanceID, instanceName, conversationID, inboundID string
	}
	seedScope := func(instanceID, instanceName, conversationID, inboundID string) scope {
		t.Helper()
		oldAt := time.Now().UTC().Add(-time.Hour)
		if _, err := fixturePool.Exec(ctx, `insert into messaging.whatsapp_instances
			(id,account_id,instance_name,provider,display_name,is_active)
			values ($2::uuid,$1::uuid,$3,'evolution',$3,true);
			insert into messaging.conversations
			(id,account_id,instance_id,instance_scope_key,channel,external_id,contact_phone,
			 contact_name,state,ai_generation,extracted_fields,last_message_at,created_at)
			values ($4::uuid,$1::uuid,$2::uuid,$3,'WHATSAPP',$5,$5,$5,'ai_active',1,
			 '{"oldSecret":"must-not-cross-reset"}'::jsonb,$6,$6);
			insert into messaging.messages
			(id,account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
			 content,status,origin,created_at)
			values ($7::uuid,$1::uuid,$4::uuid,$2::uuid,$3,'INBOUND','TEXT',
			 'old auto-close input','SENT','contact',$6)`,
			accountID, instanceID, instanceName, conversationID,
			instanceName+"@s.whatsapp.net", oldAt, inboundID); err != nil {
			t.Fatal(err)
		}
		return scope{instanceID: instanceID, instanceName: instanceName,
			conversationID: conversationID, inboundID: inboundID}
	}
	accepted := func(locked autoCloseLockedContext) (autoClosePersistence, error) {
		return autoClosePersistence{
			Evaluation: AutoCloseEvaluation{
				Accepted: true, ReasonCodes: []string{}, MissingFields: []string{},
				CapturedGeneration: 1, CurrentGeneration: locked.Snapshot.AIGeneration,
			},
			Update:     stateUpdate{State: StateClosed, InvalidateAI: true},
			PolicyJSON: json.RawMessage(`{"test":"history-fence"}`),
		}, nil
	}
	request := func(key, reply string) AutoCloseRequest {
		return AutoCloseRequest{
			Proposal:       AutoCloseProposal{Requested: true, Confidence: 1},
			IdempotencyKey: key, CapturedGeneration: 1,
			FinalReply: reply, ReplyIdempotencyKey: key + ":reply",
		}
	}

	resetFirst := seedScope(
		"83838383-8383-4383-8383-838383838383", "Auto Close Reset First",
		"84848484-8484-4484-8484-848484848484", "85858585-8585-4585-8585-858585858585",
	)
	if _, err := NewStore(appPool).ResetInstanceHistory(ctx, historyResetWrite{
		AccountID: accountID, InstanceID: resetFirst.instanceID, ActorUserID: actorID,
		Confirmation: resetFirst.instanceName, ExpectedRevision: 0,
	}); err != nil {
		t.Fatal(err)
	}
	var resetFirstCallbacks atomic.Int32
	_, err = NewStore(appPool).ApplyAIAutoClose(ctx, accountID, resetFirst.conversationID,
		request("p0-auto-close-reset-first", "must not exist"),
		func(locked autoCloseLockedContext) (autoClosePersistence, error) {
			resetFirstCallbacks.Add(1)
			return accepted(locked)
		})
	if !errors.Is(err, ErrNotFound) || resetFirstCallbacks.Load() != 0 {
		t.Fatalf("reset-first err=%v callbacks=%d", err, resetFirstCallbacks.Load())
	}
	var resetFirstReplies, resetFirstEvaluations int
	if err := fixturePool.QueryRow(ctx, `select
		(select count(*) from messaging.messages where account_id=$1::uuid
		 and conversation_id=$2::uuid and content='must not exist'),
		(select count(*) from messaging.ai_close_evaluations where account_id=$1::uuid
		 and idempotency_key='p0-auto-close-reset-first')`, accountID, resetFirst.conversationID).
		Scan(&resetFirstReplies, &resetFirstEvaluations); err != nil {
		t.Fatal(err)
	}
	if resetFirstReplies != 0 || resetFirstEvaluations != 0 {
		t.Fatalf("reset-first materialized replies=%d evaluations=%d",
			resetFirstReplies, resetFirstEvaluations)
	}

	effectFirst := seedScope(
		"86868686-8686-4686-8686-868686868686", "Auto Close Effect First",
		"87878787-8787-4787-8787-878787878787", "88888888-8888-4888-8888-888888888888",
	)
	effectPool := newHistoryNamedPool(t, ctx, dsn, "p0-auto-close-effect-first")
	resetPool := newHistoryNamedPool(t, ctx, dsn, "p0-auto-close-reset-wait")
	t.Cleanup(func() {
		effectPool.Close()
		resetPool.Close()
	})
	type closeResult struct {
		view AutoCloseDecisionView
		err  error
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	closeCh := make(chan closeResult, 1)
	go func() {
		view, closeErr := NewStore(effectPool).ApplyAIAutoClose(
			ctx, accountID, effectFirst.conversationID,
			request("p0-auto-close-effect-first", "final reply before reset"),
			func(locked autoCloseLockedContext) (autoClosePersistence, error) {
				close(entered)
				<-release
				return accepted(locked)
			},
		)
		closeCh <- closeResult{view: view, err: closeErr}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("auto-close did not acquire history lease")
	}
	type resetResult struct {
		view historyResetResult
		err  error
	}
	resetCh := make(chan resetResult, 1)
	go func() {
		view, resetErr := NewStore(resetPool).ResetInstanceHistory(ctx, historyResetWrite{
			AccountID: accountID, InstanceID: effectFirst.instanceID, ActorUserID: actorID,
			Confirmation: effectFirst.instanceName, ExpectedRevision: 0,
		})
		resetCh <- resetResult{view: view, err: resetErr}
	}()
	waitForHistoryLock(t, ctx, fixturePool, "p0-auto-close-reset-wait")
	select {
	case result := <-resetCh:
		t.Fatalf("reset crossed active auto-close lease: %+v", result)
	default:
	}
	close(release)
	closed := receiveHistoryResult(t, closeCh)
	if closed.err != nil || !closed.view.Accepted || closed.view.FinalMessageID == nil {
		t.Fatalf("effect-first close=%+v err=%v", closed.view, closed.err)
	}
	reset := receiveHistoryResult(t, resetCh)
	if reset.err != nil {
		t.Fatal(reset.err)
	}
	var replyCreatedAt, evaluationCreatedAt time.Time
	var replyStatus, replyError, outboxStatus, outboxError string
	if err := fixturePool.QueryRow(ctx, `select message.created_at,message.status,
		coalesce(message.provider_error_code,''),evaluation.created_at,outbox.status,
		coalesce(outbox.last_error,'')
		from messaging.messages message
		join messaging.ai_close_evaluations evaluation
		  on evaluation.account_id=message.account_id
		 and evaluation.conversation_id=message.conversation_id
		 and evaluation.idempotency_key='p0-auto-close-effect-first'
		join messaging.outbox outbox
		  on outbox.account_id=message.account_id
		 and outbox.payload->>'messageId'=message.id::text
		where message.account_id=$1::uuid and message.id=$2::uuid`,
		accountID, *closed.view.FinalMessageID).
		Scan(&replyCreatedAt, &replyStatus, &replyError, &evaluationCreatedAt,
			&outboxStatus, &outboxError); err != nil {
		t.Fatal(err)
	}
	if replyCreatedAt.After(reset.view.Cutoff) || evaluationCreatedAt.After(reset.view.Cutoff) ||
		replyStatus != "FAILED" || replyError != "history_reset" ||
		outboxStatus != "dead" || outboxError != "history_reset" {
		t.Fatalf("effect-first cutoff=%s reply=%s status=%s/%s evaluation=%s outbox=%s/%s",
			reset.view.Cutoff, replyCreatedAt, replyStatus, replyError,
			evaluationCreatedAt, outboxStatus, outboxError)
	}
	visible, err := NewStore(appPool).ListMessages(ctx, accountID, actorID,
		effectFirst.conversationID, MessagePageFilter{Limit: 100})
	if err != nil || len(visible) != 0 {
		t.Fatalf("effect-first reply remained visible: messages=%d err=%v", len(visible), err)
	}
}
