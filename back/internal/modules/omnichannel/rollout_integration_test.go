package omnichannel

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	platformdb "github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

func TestRolloutKillSwitchIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OMNI_E1_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("OMNI_E1_TEST_DATABASE_URL nao definido")
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool, err := newHistoryFixturePool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); pool.Close() })
	var databaseName string
	if err := pool.QueryRow(ctx, `select current_database()`).Scan(&databaseName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(databaseName, "omni_e1_test_") {
		t.Fatalf("banco de teste recusado: %q", databaseName)
	}
	if err := platformdb.ApplyMigrationsWithOptions(ctx, pool, platformdb.MigrationOptions{SkipDataSeeds: true}); err != nil {
		t.Fatalf("apply real migrations: %v", err)
	}

	const (
		account  = "81111111-1111-4111-8111-111111111111"
		actor    = "82222222-2222-4222-8222-222222222222"
		org      = "82333333-3333-4333-8333-333333333333"
		agency   = "82444444-4444-4444-8444-444444444444"
		role     = "83333333-3333-4333-8333-333333333333"
		instance = "84444444-4444-4444-8444-444444444444"
		agent    = "85555555-5555-4555-8555-555555555555"
		version  = "86666666-6666-4666-8666-666666666666"
		conv     = "87777777-7777-4777-8777-777777777777"
		message  = "88888888-8888-4888-8888-888888888888"
		dispatch = "89999999-9999-4999-8999-999999999999"
		draftMsg = "8aaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		draftJob = "8bbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `delete from messaging.channel_client_bindings where account_id=$1::uuid;
			delete from core.organization_users where organization_id=$3::uuid;
			delete from core.accounts where id=$1::uuid;
			delete from core.organizations where id=$3::uuid;
			delete from core.users where id in ($2::uuid,$4::uuid)`, account, actor, org, agency)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `insert into core.modules(id,schema_name,label)
		values ('omnichannel','messaging','Omnichannel')
		on conflict(id) do update set schema_name=excluded.schema_name,label=excluded.label;
		insert into core.permissions(key,module_id,label,scope) values
		('omnichannel.settings.manage','omnichannel','Manage settings','account'),
		('omnichannel.audit.view','omnichannel','View audit','account')
		on conflict(key) do update set module_id=excluded.module_id,label=excluded.label,scope=excluded.scope;
		insert into core.organizations(id,slug,name) values ($10::uuid,'e10-org','E10 Org');
		insert into core.accounts(id,organization_id,slug,name) values ($1::uuid,$10::uuid,'e10-rollout','E10 Rollout');
		insert into core.account_modules(account_id,module_id,enabled) values ($1::uuid,'omnichannel',true);
		insert into core.users(id,email,display_name) values ($2::uuid,'e10@example.invalid','E10 Actor');
		insert into core.users(id,email,display_name) values ($11::uuid,'e10-agency@example.invalid','E10 Agency');
		insert into core.organization_users(organization_id,user_id,org_role)
		values ($10::uuid,$11::uuid,'agency_owner');
		insert into core.account_users(account_id,user_id) values ($1::uuid,$2::uuid);
		insert into core.roles(id,account_id,code,label) values ($3::uuid,$1::uuid,'e10-manager','E10 Manager');
		insert into core.user_role_assignments(account_id,user_id,role_id) values ($1::uuid,$2::uuid,$3::uuid);
		insert into core.role_permissions(role_id,permission_key) values
		($3::uuid,'omnichannel.settings.manage'),($3::uuid,'omnichannel.audit.view');
		insert into messaging.whatsapp_instances
		(id,account_id,instance_name,provider,is_active,credentials_ciphertext)
		values ($4::uuid,$1::uuid,'E10','mock',true,'cipher');
		insert into messaging.ai_agents(id,account_id,slug,name,enabled)
		values ($5::uuid,$1::uuid,'e10','E10',true);
		insert into messaging.ai_agent_versions(id,account_id,agent_id,version,status,provider,model)
		values ($6::uuid,$1::uuid,$5::uuid,1,'published','mock','mock');
		update messaging.ai_agents set active_version_id=$6::uuid
		where account_id=$1::uuid and id=$5::uuid;
		insert into messaging.channel_client_bindings
		(account_id,client_account_id,channel,whatsapp_instance_id,source,reason)
		values ($1::uuid,$1::uuid,'WHATSAPP',$4::uuid,'manual','e10 fixture');
		insert into messaging.automation_profiles
		(account_id,client_account_id,whatsapp_instance_id,ai_agent_id,enabled)
		values ($1::uuid,$1::uuid,$4::uuid,$5::uuid,true);
		insert into messaging.conversations
		(id,account_id,client_account_id,instance_id,instance_scope_key,channel,external_id,state,ai_generation)
		values ($7::uuid,$1::uuid,$1::uuid,$4::uuid,'E10','WHATSAPP','e10-contact','ai_active',0);
		insert into messaging.messages
		(id,account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,content,status,origin)
		values ($8::uuid,$1::uuid,$7::uuid,$4::uuid,'E10','INBOUND','TEXT','fixture','SENT','contact');
		insert into messaging.ai_dispatches
		(id,account_id,conversation_id,agent_version_id,generation,status,message_ids,run_after,idempotency_key)
		values ($9::uuid,$1::uuid,$7::uuid,$6::uuid,0,'queued',array[$8::uuid],now(),'e10-dispatch')`,
		account, actor, role, instance, agent, version, conv, message, dispatch, org, agency); err != nil {
		t.Fatal(err)
	}
	accountChecker := auth.NewPostgresAccountMemberChecker(pool)
	if allowed, checkErr := accountChecker.IsMember(ctx, account, agency); checkErr != nil || allowed {
		t.Fatalf("agency owner sem membership allowed=%v err=%v", allowed, checkErr)
	}
	if _, err := pool.Exec(ctx, `insert into core.account_users(account_id,user_id) values ($1::uuid,$2::uuid)`, account, agency); err != nil {
		t.Fatal(err)
	}
	if allowed, checkErr := accountChecker.IsMember(ctx, account, agency); checkErr != nil || !allowed {
		t.Fatalf("agency owner com membership allowed=%v err=%v", allowed, checkErr)
	}
	if _, err := pool.Exec(ctx, `delete from core.account_users where account_id=$1::uuid and user_id=$2::uuid`, account, agency); err != nil {
		t.Fatal(err)
	}

	store := NewStore(pool)
	service := NewRolloutService(store)
	current, err := service.Get(ctx, account, actor)
	if err != nil || current.Mode != RolloutModeActive || !current.LegacyDefault || current.Revision != 0 {
		t.Fatalf("legacy default=%+v err=%v", current, err)
	}
	configured, err := service.Update(ctx, account, actor, RolloutConfigInput{
		Mode: RolloutModeAutoPilot, AllowedInstanceIDs: []string{instance},
		AutoReplyPercent: 100, AllowedHours: RolloutHours{Timezone: "America/Sao_Paulo"},
		ExpectedRevision: 0, Reason: "habilitar canario interno",
	})
	if err != nil || configured.Revision != 1 || configured.LegacyDefault {
		t.Fatalf("configured=%+v err=%v", configured, err)
	}
	if _, err := service.Update(ctx, account, actor, RolloutConfigInput{
		Mode: RolloutModeActive, AutoReplyPercent: 100,
		AllowedHours:     RolloutHours{Timezone: "America/Sao_Paulo"},
		ExpectedRevision: 0, Reason: "stale update",
	}); !errors.Is(err, ErrRolloutRevisionConflict) {
		t.Fatalf("stale update=%v", err)
	}
	pauseReason := "incidente critico simulado"
	paused, err := service.Update(ctx, account, actor, RolloutConfigInput{
		Mode: RolloutModePaused, AutoReplyPercent: 0,
		AllowedHours:     RolloutHours{Timezone: "America/Sao_Paulo"},
		KillSwitchReason: &pauseReason, ExpectedRevision: 1, Reason: "ensaio do kill switch",
	})
	if err != nil || paused.Revision != 2 || paused.Mode != RolloutModePaused {
		t.Fatalf("paused=%+v err=%v", paused, err)
	}
	var state, dispatchStatus, lastError string
	var generation int64
	if err := pool.QueryRow(ctx, `select conversation.state,conversation.ai_generation,
		dispatch.status,dispatch.last_error
		from messaging.conversations conversation
		join messaging.ai_dispatches dispatch on dispatch.account_id=conversation.account_id
		 and dispatch.conversation_id=conversation.id
		where conversation.account_id=$1::uuid and conversation.id=$2::uuid`, account, conv).
		Scan(&state, &generation, &dispatchStatus, &lastError); err != nil {
		t.Fatal(err)
	}
	if state != "routing" || generation != 1 || dispatchStatus != "cancelled" || lastError != "rollout_paused" {
		t.Fatalf("state=%s generation=%d dispatch=%s/%s", state, generation, dispatchStatus, lastError)
	}
	var changes int
	if err := pool.QueryRow(ctx, `select count(*) from messaging.rollout_changes where account_id=$1::uuid`, account).Scan(&changes); err != nil || changes != 2 {
		t.Fatalf("changes=%d err=%v", changes, err)
	}
	health, err := NewOperationalService(store, false).Health(ctx, account, actor)
	if err != nil || health.Bindings.Mismatches != 0 || health.Database.Status != "ok" {
		t.Fatalf("health=%+v err=%v", health, err)
	}
	qrA, qrB := newSharedQRCache(pool), newSharedQRCache(pool)
	qrA.set(account, "E10", "data:image/png;base64,shared")
	if got := qrB.get(account, "E10"); got != "data:image/png;base64,shared" {
		t.Fatalf("shared qr=%q", got)
	}
	qrB.set(account, "E10", "")
	if got := qrA.get(account, "E10"); got != "" {
		t.Fatalf("shared qr clear=%q", got)
	}
	limiterA, limiterB := newSharedRateLimiter(pool), newSharedRateLimiter(pool)
	scopeSuffix := time.Now().UTC().Format("20060102T150405.000000000")
	if !limiterA.allow("e10-test-"+scopeSuffix, "192.0.2.1", 1, time.Minute) ||
		limiterB.allow("e10-test-"+scopeSuffix, "192.0.2.1", 1, time.Minute) {
		t.Fatal("rate limit compartilhado entre replicas nao foi respeitado")
	}

	const (
		attempts = 32
		limit    = 8
	)
	limiters := []*rateLimiter{
		newSharedRateLimiter(pool),
		newSharedRateLimiter(pool),
		newSharedRateLimiter(pool),
		newSharedRateLimiter(pool),
	}
	var allowed atomic.Int64
	var wg sync.WaitGroup
	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func(index int) {
			defer wg.Done()
			if limiters[index%len(limiters)].allow("e10-concurrent-"+scopeSuffix, "192.0.2.2", limit, time.Minute) {
				allowed.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := allowed.Load(); got != limit {
		t.Fatalf("rate limit concorrente permitiu %d, esperado %d", got, limit)
	}

	closedPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	closedPool.Close()
	if newSharedRateLimiter(closedPool).allow("e10-fail-closed", "192.0.2.3", 1, time.Minute) {
		t.Fatal("rate limit deve negar quando o armazenamento compartilhado esta indisponivel")
	}

	assist, err := service.Update(ctx, account, actor, RolloutConfigInput{
		Mode: RolloutModeAssist, AutoReplyPercent: 0,
		AllowedHours:     RolloutHours{Timezone: "America/Sao_Paulo"},
		ExpectedRevision: 2, Reason: "validar draft assist",
	})
	if err != nil || assist.Revision != 3 || assist.Mode != RolloutModeAssist {
		t.Fatalf("assist=%+v err=%v", assist, err)
	}
	if _, err := pool.Exec(ctx, `update messaging.conversations
		set state='ai_active',ai_generation=2 where account_id=$1::uuid and id=$2::uuid;
		insert into messaging.messages
		(id,account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,content,status,origin)
		values ($3::uuid,$1::uuid,$2::uuid,$4::uuid,'E10','INBOUND','TEXT','preciso de ajuda','SENT','contact');
		insert into messaging.ai_dispatches
		(id,account_id,conversation_id,agent_version_id,generation,status,message_ids,run_after,idempotency_key)
		values ($5::uuid,$1::uuid,$2::uuid,$6::uuid,2,'processing',array[$3::uuid],now(),'e10-assist-dispatch')`,
		account, conv, draftMsg, instance, draftJob, version); err != nil {
		t.Fatal(err)
	}
	draft, saved, err := store.CompleteAIDispatchWithReplyDraft(
		ctx, account, draftJob, 2, "", "Resposta sugerida pela IA",
	)
	if err != nil || !saved || draft.Status != "pending" || draft.Content != "Resposta sugerida pela IA" {
		t.Fatalf("draft=%+v saved=%v err=%v", draft, saved, err)
	}
	if pending, found, err := store.GetPendingAIReplyDraft(ctx, account, conv); err != nil || !found || pending.ID != draft.ID {
		t.Fatalf("pending=%+v found=%v err=%v", pending, found, err)
	}
	domain := NewService(store)
	instanceID := instance
	human, created, err := store.CreateHumanOutboundMessage(ctx, outboundMessageInsert{
		AccountID: account, ConversationID: conv, InstanceID: &instanceID,
		InstanceScopeKey: "E10", SenderUserID: actor, MessageType: "TEXT",
		Content: "Resposta sugerida pela IA, revisada.", MetadataJSON: []byte(`{}`),
		Origin: "human", AIReplyDraftID: draft.ID,
	}, "e10-assist-human", func(snapshot convSnapshot) (stateUpdate, *decisionRecord, error) {
		return domain.decideTransition(ctx, account, EventMsgOutboundHuman,
			TransitionPayload{ActorUserID: actor}, snapshot)
	}, nil)
	if err != nil || !created || human.ID == "" {
		t.Fatalf("human=%+v created=%v err=%v", human, created, err)
	}
	var draftStatus, draftReason string
	var draftEdited bool
	if err := pool.QueryRow(ctx, `select status,edited,decision_reason
		from messaging.ai_reply_drafts where account_id=$1::uuid and id=$2::uuid`, account, draft.ID).
		Scan(&draftStatus, &draftEdited, &draftReason); err != nil ||
		draftStatus != "used" || !draftEdited || draftReason != "operator_used" {
		t.Fatalf("used draft status=%s edited=%v reason=%s err=%v", draftStatus, draftEdited, draftReason, err)
	}

	if _, err := pool.Exec(ctx, `update messaging.ai_reply_drafts
		set status='pending',used_message_id=null,decided_by_user_id=null,decision_reason='',
		    edited=false,decided_at=null,updated_at=now()
		where account_id=$1::uuid and id=$2::uuid`, account, draft.ID); err != nil {
		t.Fatal(err)
	}
	if dismissed, err := store.DismissAIReplyDraft(ctx, account, conv, draft.ID, actor, "nao adequada"); err != nil || !dismissed {
		t.Fatalf("dismissed=%v err=%v", dismissed, err)
	}
	if err := pool.QueryRow(ctx, `select status,decision_reason from messaging.ai_reply_drafts
		where account_id=$1::uuid and id=$2::uuid`, account, draft.ID).Scan(&draftStatus, &draftReason); err != nil ||
		draftStatus != "dismissed" || draftReason != "nao adequada" {
		t.Fatalf("dismissed draft status=%s reason=%s err=%v", draftStatus, draftReason, err)
	}
	if _, err := pool.Exec(ctx, `update messaging.ai_reply_drafts
		set status='pending',decided_by_user_id=null,decision_reason='',decided_at=null,updated_at=now()
		where account_id=$1::uuid and id=$2::uuid`, account, draft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResetInstanceHistory(ctx, historyResetWrite{
		AccountID: account, InstanceID: instance, ActorUserID: actor,
		Confirmation: "E10", ExpectedRevision: 0, Reason: "validar expiracao do draft",
	}); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `select status,decision_reason from messaging.ai_reply_drafts
		where account_id=$1::uuid and id=$2::uuid`, account, draft.ID).Scan(&draftStatus, &draftReason); err != nil ||
		draftStatus != "expired" || draftReason != "history_reset" {
		t.Fatalf("reset draft status=%s reason=%s err=%v", draftStatus, draftReason, err)
	}
	if _, err := pool.Exec(ctx, `update messaging.ai_reply_drafts
		set status='pending',decided_by_user_id=null,decision_reason='',decided_at=null,updated_at=now()
		where account_id=$1::uuid and id=$2::uuid`, account, draft.ID); err != nil {
		t.Fatal(err)
	}
	active, err := service.Update(ctx, account, actor, RolloutConfigInput{
		Mode: RolloutModeActive, AutoReplyPercent: 100,
		AllowedHours:     RolloutHours{Timezone: "America/Sao_Paulo"},
		ExpectedRevision: 3, Reason: "encerrar validacao assist",
	})
	if err != nil || active.Revision != 4 {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	if err := pool.QueryRow(ctx, `select status,decision_reason from messaging.ai_reply_drafts
		where account_id=$1::uuid and id=$2::uuid`, account, draft.ID).Scan(&draftStatus, &draftReason); err != nil ||
		draftStatus != "expired" || draftReason != "rollout_active" {
		t.Fatalf("expired draft status=%s reason=%s err=%v", draftStatus, draftReason, err)
	}
}
