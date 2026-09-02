package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	platformdb "github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

// TestHistoryResetRoutingFenceIntegration prova as duas ordens da corrida reset x roteamento
// contra migrations reais. O banco precisa ser descartavel e manter o prefixo de seguranca E1.
func TestHistoryResetRoutingFenceIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OMNI_E1_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("OMNI_E1_TEST_DATABASE_URL nao definido")
	}
	ctx, cancel := context.WithCancel(context.Background())
	fixturePool, err := newHistoryFixturePool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
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
		accountID    = "91919191-9191-4191-8191-919191919191"
		actorID      = "92929292-9292-4292-8292-929292929292"
		departmentID = "93939393-9393-4393-8393-939393939393"
		queueID      = "94949494-9494-4494-8494-949494949494"
		ruleID       = "95959595-9595-4595-8595-959595959595"
	)
	cleanup := func() {
		_, _ = fixturePool.Exec(context.Background(), `delete from core.accounts where id=$1::uuid;
			delete from core.users where id=$2::uuid`, accountID, actorID)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := fixturePool.Exec(ctx, `insert into core.accounts(id,slug,name)
		values ($1::uuid,'p0-routing-fence','P0 routing fence');
		insert into core.users(id,email,display_name,is_platform_admin)
		values ($2::uuid,'p0-routing-fence@example.invalid','P0 routing actor',true);
		insert into messaging.departments(id,account_id,slug,name)
		values ($3::uuid,$1::uuid,'p0-routing-department','P0 routing department');
		insert into messaging.queues(id,account_id,department_id,slug,name)
		values ($4::uuid,$1::uuid,$3::uuid,'p0-routing-queue','P0 routing queue');
		insert into messaging.routing_rules(id,account_id,name,priority,conditions,target_queue_id)
		values ($5::uuid,$1::uuid,'P0 always match',1,'[]'::jsonb,$4::uuid)`,
		accountID, actorID, departmentID, queueID, ruleID); err != nil {
		t.Fatal(err)
	}

	oldAt := time.Now().UTC().Add(-time.Hour)
	seedScope := func(instanceID, instanceName, conversationID, sentinel string) {
		t.Helper()
		if _, err := fixturePool.Exec(ctx, `insert into messaging.whatsapp_instances
			(id,account_id,instance_name,provider,display_name,is_active)
			values ($2::uuid,$1::uuid,$3,'evolution',$3,true);
			insert into messaging.conversations
			(id,account_id,instance_id,instance_scope_key,channel,external_id,contact_name,
			 contact_phone,state,extracted_fields,last_message_at,created_at)
			values ($4::uuid,$1::uuid,$2::uuid,$3,'WHATSAPP',$5,$5,'5511999999999',
			 'routing',jsonb_build_object('lead',$6),$7,$7);
			insert into messaging.messages
			(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
			 content,status,origin,created_at)
			values ($1::uuid,$4::uuid,$2::uuid,$3,'INBOUND','TEXT',$6,'SENT','contact',$7)`,
			accountID, instanceID, instanceName, conversationID, instanceName+"@s.whatsapp.net",
			sentinel, oldAt); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("reset-first nao materializa decisao com historico antigo", func(t *testing.T) {
		const (
			instanceID     = "96969696-9696-4696-8696-969696969696"
			instanceName   = "P0 Routing Reset First"
			conversationID = "97979797-9797-4797-8797-979797979797"
		)
		seedScope(instanceID, instanceName, conversationID, "reset-first-secret")

		blocker, err := fixturePool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })
		var lockedInstanceID string
		if err := blocker.QueryRow(ctx, `select id::text from messaging.whatsapp_instances
			where account_id=$1::uuid and id=$2::uuid for update`, accountID, instanceID).
			Scan(&lockedInstanceID); err != nil {
			t.Fatal(err)
		}

		resetPool := newHistoryNamedPool(t, ctx, dsn, "p0-routing-reset-first-reset")
		routePool := newHistoryRoutingNamedPool(t, ctx, dsn, "p0-routing-reset-first-route")
		t.Cleanup(func() {
			routePool.Close()
			resetPool.Close()
		})
		type resetResult struct {
			value historyResetResult
			err   error
		}
		resetCh := make(chan resetResult, 1)
		go func() {
			value, resetErr := NewStore(resetPool).ResetInstanceHistory(ctx, historyResetWrite{
				AccountID: accountID, InstanceID: instanceID, ActorUserID: actorID,
				Confirmation: instanceName, ExpectedRevision: 0,
			})
			resetCh <- resetResult{value: value, err: resetErr}
		}()
		waitForHistoryLock(t, ctx, fixturePool, "p0-routing-reset-first-reset")

		type routeResult struct {
			state State
			err   error
		}
		routeCh := make(chan routeResult, 1)
		go func() {
			state, routeErr := NewService(NewStore(routePool)).SystemRoute(ctx, accountID, conversationID)
			routeCh <- routeResult{state: state, err: routeErr}
		}()
		waitForHistoryLock(t, ctx, fixturePool, "p0-routing-reset-first-route")
		if err := blocker.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		reset := receiveHistoryResult(t, resetCh)
		if reset.err != nil || reset.value.Revision != 1 {
			t.Fatalf("reset-first reset=%+v err=%v", reset.value, reset.err)
		}
		route := receiveHistoryResult(t, routeCh)
		if !errors.Is(route.err, ErrNotFound) {
			t.Fatalf("reset-first route state=%q err=%v, want not found", route.state, route.err)
		}
		var decisions int
		if err := fixturePool.QueryRow(ctx, `select count(*) from messaging.routing_decisions
			where account_id=$1::uuid and conversation_id=$2::uuid`, accountID, conversationID).
			Scan(&decisions); err != nil || decisions != 0 {
			t.Fatalf("reset-first physical decisions=%d err=%v", decisions, err)
		}
	})

	t.Run("route-first segura reset e decisao fica anterior ao cutoff", func(t *testing.T) {
		const (
			instanceID     = "98989898-9898-4898-8898-989898989898"
			instanceName   = "P0 Routing Route First"
			conversationID = "99999999-9999-4999-8999-999999999999"
			sentinel       = "route-first-secret"
		)
		seedScope(instanceID, instanceName, conversationID, sentinel)

		conversationBlocker, err := fixturePool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conversationBlocker.Rollback(context.Background()) })
		var lockedConversationID string
		if err := conversationBlocker.QueryRow(ctx, `select id::text from messaging.conversations
			where account_id=$1::uuid and id=$2::uuid for update`, accountID, conversationID).
			Scan(&lockedConversationID); err != nil {
			t.Fatal(err)
		}
		routePool := newHistoryRoutingNamedPool(t, ctx, dsn, "p0-routing-route-first-route")
		resetPool := newHistoryNamedPool(t, ctx, dsn, "p0-routing-route-first-reset")
		t.Cleanup(func() {
			resetPool.Close()
			routePool.Close()
		})
		type routeResult struct {
			state State
			err   error
		}
		routeCh := make(chan routeResult, 1)
		go func() {
			state, routeErr := NewService(NewStore(routePool)).SystemRoute(ctx, accountID, conversationID)
			routeCh <- routeResult{state: state, err: routeErr}
		}()
		waitForHistoryLock(t, ctx, fixturePool, "p0-routing-route-first-route")

		type resetResult struct {
			value historyResetResult
			err   error
		}
		resetCh := make(chan resetResult, 1)
		go func() {
			value, resetErr := NewStore(resetPool).ResetInstanceHistory(ctx, historyResetWrite{
				AccountID: accountID, InstanceID: instanceID, ActorUserID: actorID,
				Confirmation: instanceName, ExpectedRevision: 0,
			})
			resetCh <- resetResult{value: value, err: resetErr}
		}()
		waitForHistoryLock(t, ctx, fixturePool, "p0-routing-route-first-reset")
		select {
		case result := <-resetCh:
			t.Fatalf("reset atravessou route-first: %+v", result)
		case <-time.After(100 * time.Millisecond):
		}
		if err := conversationBlocker.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		route := receiveHistoryResult(t, routeCh)
		if route.err != nil || route.state != StateQueued {
			t.Fatalf("route-first route state=%q err=%v", route.state, route.err)
		}
		reset := receiveHistoryResult(t, resetCh)
		if reset.err != nil || reset.value.Revision != 1 {
			t.Fatalf("route-first reset=%+v err=%v", reset.value, reset.err)
		}

		var input json.RawMessage
		var decidedAt time.Time
		if err := fixturePool.QueryRow(ctx, `select input,decided_at from messaging.routing_decisions
			where account_id=$1::uuid and conversation_id=$2::uuid`, accountID, conversationID).
			Scan(&input, &decidedAt); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(input), sentinel) || !decidedAt.Before(reset.value.Cutoff) {
			t.Fatalf("route-first input=%s decidedAt=%s cutoff=%s", input,
				decidedAt.UTC().Format(time.RFC3339Nano), reset.value.Cutoff.Format(time.RFC3339Nano))
		}
		visible, err := NewStore(routePool).ListRoutingDecisions(ctx, accountID, conversationID)
		if err != nil || len(visible) != 0 {
			t.Fatalf("route-first visible decisions=%+v err=%v", visible, err)
		}
	})
}

func newHistoryRoutingNamedPool(t *testing.T, ctx context.Context, dsn, applicationName string) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 2
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	config.ConnConfig.RuntimeParams["application_name"] = applicationName
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}
