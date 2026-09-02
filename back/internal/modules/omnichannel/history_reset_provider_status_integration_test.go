package omnichannel

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	platformdb "github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

// TestHistoryResetLateProviderStatusIntegration proves that a late provider ACK
// is audited without reviving an AI message terminalized by history reset.
func TestHistoryResetLateProviderStatusIntegration(t *testing.T) {
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
		accountID      = "89898989-8989-4989-8989-898989898989"
		actorID        = "90909090-9090-4090-8090-909090909090"
		instanceID     = "91919191-9191-4191-8191-919191919191"
		conversationID = "92929292-9292-4292-8292-929292929292"
		messageID      = "93939393-9393-4393-8393-939393939393"
		instanceName   = "Late ACK Fence"
		externalID     = "provider-late-ack-message"
	)
	_, _ = fixturePool.Exec(ctx, `delete from core.accounts where id=$1::uuid;
		delete from core.users where id=$2::uuid`, accountID, actorID)
	t.Cleanup(func() {
		_, _ = fixturePool.Exec(context.Background(), `delete from core.accounts where id=$1::uuid;
			delete from core.users where id=$2::uuid`, accountID, actorID)
	})
	futureAt := time.Now().UTC().Add(time.Hour)
	if _, err := fixturePool.Exec(ctx, `insert into core.accounts(id,slug,name)
		values ($1::uuid,'p0-late-ack-fence','P0 late ACK fence');
		insert into core.users(id,email,display_name)
		values ($2::uuid,'p0-late-ack@example.invalid','P0 late ACK actor');
		insert into core.account_users(account_id,user_id) values ($1::uuid,$2::uuid);
		insert into messaging.whatsapp_instances
		(id,account_id,instance_name,provider,display_name,is_active)
		values ($3::uuid,$1::uuid,$4,'evolution',$4,true);
		insert into messaging.conversations
		(id,account_id,instance_id,instance_scope_key,channel,external_id,contact_phone,
		 contact_name,state,ai_generation,last_message_at,created_at)
		values ($5::uuid,$1::uuid,$3::uuid,$4,'WHATSAPP','late-ack-contact',
		 '5511999999999','Late ACK contact','ai_active',1,$7,now());
		insert into messaging.messages
		(id,account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
		 content,status,origin,external_message_id,metadata_json,created_at)
		values ($6::uuid,$1::uuid,$5::uuid,$3::uuid,$4,'OUTBOUND','TEXT',
		 'future stale AI reply','PENDING','ai',$8,'{"aiGeneration":0}'::jsonb,$7)`,
		accountID, actorID, instanceID, instanceName, conversationID, messageID, futureAt, externalID); err != nil {
		t.Fatal(err)
	}
	store := NewStore(appPool)
	reset, err := store.ResetInstanceHistory(ctx, historyResetWrite{
		AccountID: accountID, InstanceID: instanceID, ActorUserID: actorID,
		Confirmation: instanceName, ExpectedRevision: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !futureAt.After(reset.Cutoff) {
		t.Fatalf("fixture must exercise timestamp escape: future=%s cutoff=%s", futureAt, reset.Cutoff)
	}
	assertTerminal := func(stage string) {
		t.Helper()
		var status, errorCode string
		if err := fixturePool.QueryRow(ctx, `select status,coalesce(provider_error_code,'')
			from messaging.messages where account_id=$1::uuid and id=$2::uuid`,
			accountID, messageID).Scan(&status, &errorCode); err != nil {
			t.Fatal(err)
		}
		if status != "FAILED" || errorCode != "history_reset" {
			t.Fatalf("%s status=%s error=%s", stage, status, errorCode)
		}
	}
	assertTerminal("after reset")
	if visible, err := store.ListMessages(ctx, accountID, actorID, conversationID,
		MessagePageFilter{Limit: 100}); err != nil || len(visible) != 0 {
		t.Fatalf("reset message visible=%d err=%v", len(visible), err)
	}

	ackAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Microsecond)
	result, err := store.PersistInbound(ctx, inboundWrite{
		AccountID: accountID, Provider: "evolution", ExternalEventID: "p0-late-ack-event",
		EventKind: "message_status", InstanceName: instanceName, InstanceID: instanceID,
		PayloadMasked: json.RawMessage(`{"status":"SENT"}`),
		Status: &inboundStatusWrite{
			ExternalMessageID: externalID, Status: "SENT", OccurredAt: ackAt,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusChanged {
		t.Fatalf("late ACK changed terminal projection: %+v", result)
	}
	assertTerminal("after late ACK")
	var providerStatus string
	var providerStatusAt time.Time
	if err := fixturePool.QueryRow(ctx, `select provider_status,provider_status_at
		from messaging.webhook_events where account_id=$1::uuid
		  and external_event_id='p0-late-ack-event'`, accountID).
		Scan(&providerStatus, &providerStatusAt); err != nil {
		t.Fatal(err)
	}
	if providerStatus != "SENT" || !providerStatusAt.Equal(ackAt) {
		t.Fatalf("late ACK evidence status=%s at=%s", providerStatus, providerStatusAt)
	}
	if visible, err := store.ListMessages(ctx, accountID, actorID, conversationID,
		MessagePageFilter{Limit: 100}); err != nil || len(visible) != 0 {
		t.Fatalf("late ACK revived message=%d err=%v", len(visible), err)
	}

	store.SetHistoryCutoffEnforced(false)
	rollback, err := store.ListMessages(ctx, accountID, actorID, conversationID,
		MessagePageFilter{Limit: 100})
	if err != nil || len(rollback) != 1 || rollback[0].ID != messageID {
		t.Fatalf("rollback did not reveal terminal evidence: messages=%v err=%v",
			messageIDs(rollback), err)
	}
	store.SetHistoryCutoffEnforced(true)
	if visible, err := store.ListMessages(ctx, accountID, actorID, conversationID,
		MessagePageFilter{Limit: 100}); err != nil || len(visible) != 0 {
		t.Fatalf("reenabled cutoff did not hide terminal evidence=%d err=%v", len(visible), err)
	}
}
