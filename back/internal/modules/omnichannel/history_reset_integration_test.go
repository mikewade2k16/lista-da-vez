package omnichannel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	platformdb "github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// TestHistoryResetP0Integration cobre a matriz de persistencia/isolamento do P0 em banco
// descartavel. O guard reutiliza o contrato E1: nunca executa contra banco sem prefixo de teste.
func TestHistoryResetP0Integration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OMNI_E1_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("OMNI_E1_TEST_DATABASE_URL nao definido")
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool, err := newHistoryFixturePool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		pool.Close()
	})
	appPool, err := newHistoryPool(ctx, dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		appPool.Close()
	})
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
	t.Log("fence: migrations ready")
	store := NewStore(appPool)

	const (
		accountA    = "11111111-1111-4111-8111-111111111111"
		accountB    = "22222222-2222-4222-8222-222222222222"
		instanceA   = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		instanceA2  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaab"
		instanceB   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
		actor       = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
		roleA       = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
		roleB       = "dddddddd-dddd-4ddd-8ddd-ddddddddddde"
		departmentA = "ffffffff-ffff-4fff-8fff-ffffffffffff"
		queueA      = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
		departmentB = "ffffffff-ffff-4fff-8fff-fffffffffff1"
		queueB      = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeee1"
		agentA      = "abababab-abab-4bab-8bab-abababababab"
		versionA    = "acacacac-acac-4cac-8cac-acacacacacac"
		dispatchA   = "12121212-1212-4121-8121-121212121212"
		bindingA    = "13131313-1313-4131-8131-131313131313"
	)
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `delete from messaging.customer_data_outbox where account_id=any($1::uuid[]);
			delete from messaging.intelligence_outbox where account_id=any($1::uuid[]);
			delete from messaging.channel_client_bindings where account_id=any($1::uuid[]);
			delete from core.accounts where id=any($1::uuid[]);
			delete from core.users where id=$2::uuid`, []string{accountA, accountB}, actor)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `insert into core.modules(id,schema_name,label)
		values ('omnichannel','messaging','Omnichannel')
		on conflict(id) do update set schema_name=excluded.schema_name,label=excluded.label;
		insert into core.permissions(key,module_id,label,scope) values
		('omnichannel.instances.manage','omnichannel','Manage instances','account'),
		('omnichannel.conversations.privacy.manage','omnichannel','Manage conversation privacy','account'),
		('omnichannel.conversations.view','omnichannel','View conversations','account'),
		('omnichannel.conversations.reply','omnichannel','Reply to conversations','account'),
		('omnichannel.contacts.manage','omnichannel','Manage contacts','account')
		on conflict(key) do update set module_id=excluded.module_id,label=excluded.label,scope=excluded.scope;
		insert into core.accounts(id,slug,name) values
		($1::uuid,'p0-history-account-a','P0 history account A'),
		($2::uuid,'p0-history-account-b','P0 history account B');
		insert into core.account_modules(account_id,module_id,enabled) values
		($1::uuid,'omnichannel',true),($2::uuid,'omnichannel',true);
		insert into core.users(id,email,display_name,is_platform_admin)
		values ($6::uuid,'p0-history-actor@example.invalid','P0 history actor',true);
		insert into core.account_users(account_id,user_id) values
		($1::uuid,$6::uuid),($2::uuid,$6::uuid);
		insert into core.roles(id,account_id,code,label) values
		($7::uuid,$1::uuid,'p0-history-a','P0 history A'),
		($8::uuid,$2::uuid,'p0-history-b','P0 history B');
		insert into core.user_role_assignments(account_id,user_id,role_id) values
		($1::uuid,$6::uuid,$7::uuid),($2::uuid,$6::uuid,$8::uuid);
		insert into core.role_permissions(role_id,permission_key) values
		($8::uuid,'omnichannel.instances.manage'),
		($8::uuid,'omnichannel.conversations.privacy.manage');
		insert into messaging.departments(id,account_id,slug,name) values
		($9::uuid,$1::uuid,'p0-department-a','P0 department A'),
		($10::uuid,$2::uuid,'p0-department-b','P0 department B');
		insert into messaging.queues(id,account_id,department_id,slug,name) values
		($11::uuid,$1::uuid,$9::uuid,'p0-queue-a','P0 queue A'),
		($12::uuid,$2::uuid,$10::uuid,'p0-queue-b','P0 queue B');
		insert into messaging.whatsapp_instances
		(id,account_id,instance_name,provider,display_name,credentials_ciphertext,is_active)
		values ($3::uuid,$1::uuid,'Crow Principal','evolution','Crow Principal','cipher-a',true),
		       ($4::uuid,$1::uuid,'Crow Secundaria','evolution','Crow Secundaria','cipher-a2',true),
		       ($5::uuid,$2::uuid,'Conta B','evolution','Conta B','cipher-b',true);
		insert into messaging.whatsapp_instance_user_grants
		(account_id,instance_id,user_id,access_level,is_active,granted_by_user_id,updated_by_user_id)
		values ($1::uuid,$3::uuid,$6::uuid,'manage',true,$6::uuid,$6::uuid),
		       ($1::uuid,$4::uuid,$6::uuid,'manage',true,$6::uuid,$6::uuid),
		       ($2::uuid,$5::uuid,$6::uuid,'manage',true,$6::uuid,$6::uuid);
		insert into messaging.ai_agents(id,account_id,slug,name,enabled)
		values ($13::uuid,$1::uuid,'p0-agent','P0 agent',true);
		insert into messaging.ai_agent_versions
		(id,account_id,agent_id,version,status,provider,model)
		values ($14::uuid,$1::uuid,$13::uuid,1,'published','openai','test');
		insert into messaging.automation_profiles
		(account_id,client_account_id,whatsapp_instance_id,ai_agent_id,enabled,
		 created_by_user_id,updated_by_user_id)
		values ($1::uuid,$1::uuid,$3::uuid,$13::uuid,true,$6::uuid,$6::uuid);
		insert into messaging.channel_client_bindings
		(id,account_id,client_account_id,channel,whatsapp_instance_id,source,reason,created_by_user_id)
		values ($15::uuid,$1::uuid,$1::uuid,'WHATSAPP',$3::uuid,'manual','p0 history reset',$6::uuid)`,
		accountA, accountB, instanceA, instanceA2, instanceB, actor, roleA, roleB,
		departmentA, departmentB, queueA, queueB, agentA, versionA, bindingA); err != nil {
		t.Fatal(err)
	}

	capabilities, err := loadInstanceCapabilities(ctx, store, accountA, Caller{UserID: actor, IsPlatformAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	if capabilities.View || capabilities.Reply || capabilities.Manage || capabilities.ResetHistory {
		t.Fatalf("platform role bypassed effective grants: %+v", capabilities)
	}
	svc := NewSessionService(store, nil, nil, nil, nil, nil)
	if _, err := pool.Exec(ctx, `insert into core.role_permissions(role_id,permission_key)
		values ($1::uuid,'omnichannel.instances.manage'),
		       ($1::uuid,'omnichannel.conversations.view'),
		       ($1::uuid,'omnichannel.conversations.reply'),
		       ($1::uuid,'omnichannel.contacts.manage')`, roleA); err != nil {
		t.Fatal(err)
	}
	capabilities, err = loadInstanceCapabilities(ctx, store, accountA, Caller{UserID: actor, IsPlatformAdmin: true})
	if err != nil || !capabilities.Manage || capabilities.ResetHistory {
		t.Fatalf("privacy permission must be independently required: capabilities=%+v err=%v", capabilities, err)
	}
	if _, err := svc.ResetInstanceHistory(ctx, accountA, instanceA, Caller{UserID: actor, IsPlatformAdmin: true},
		InstanceHistoryResetInput{Confirmation: "Crow Principal", ExpectedRevision: int64ptr(0)}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("missing privacy permission=%v, want forbidden", err)
	}
	if _, err := pool.Exec(ctx, `delete from core.role_permissions where role_id=$1::uuid;
		insert into core.role_permissions(role_id,permission_key)
		values ($1::uuid,'omnichannel.conversations.privacy.manage')`, roleA); err != nil {
		t.Fatal(err)
	}
	capabilities, err = loadInstanceCapabilities(ctx, store, accountA, Caller{UserID: actor, IsPlatformAdmin: true})
	if err != nil || capabilities.Manage || capabilities.ResetHistory {
		t.Fatalf("instance permission must be independently required: capabilities=%+v err=%v", capabilities, err)
	}
	if _, err := svc.ResetInstanceHistory(ctx, accountA, instanceA, Caller{UserID: actor, IsPlatformAdmin: true},
		InstanceHistoryResetInput{Confirmation: "Crow Principal", ExpectedRevision: int64ptr(0)}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("missing instance permission=%v, want forbidden", err)
	}
	if _, err := pool.Exec(ctx, `insert into core.role_permissions(role_id,permission_key)
		values ($1::uuid,'omnichannel.instances.manage'),
		       ($1::uuid,'omnichannel.conversations.view'),
		       ($1::uuid,'omnichannel.conversations.reply'),
		       ($1::uuid,'omnichannel.contacts.manage')`, roleA); err != nil {
		t.Fatal(err)
	}
	capabilities, err = loadInstanceCapabilities(ctx, store, accountA, Caller{UserID: actor, IsPlatformAdmin: true})
	if err != nil || !capabilities.Manage || !capabilities.ResetHistory {
		t.Fatalf("effective reset grants: capabilities=%+v err=%v", capabilities, err)
	}

	oldAt := time.Now().UTC().Add(-time.Hour)
	var conversationA, conversationA2, conversationB, conversationLegacy, conversationInstagram, conversationCompose string
	var oldInbound, oldOutbound, acceptedOutbound string
	seedConversation := func(accountID, instanceID, externalID, state string) string {
		t.Helper()
		queueID, departmentID := queueA, departmentA
		instanceScopeKey := "Crow Principal"
		if instanceID == instanceA2 {
			instanceScopeKey = "Crow Secundaria"
		}
		if accountID == accountB {
			queueID, departmentID = queueB, departmentB
			instanceScopeKey = "Conta B"
		}
		var id string
		err := pool.QueryRow(ctx, `insert into messaging.conversations
			(account_id,client_account_id,instance_id,instance_scope_key,channel,external_id,contact_name,state,
			 queue_id,department_id,assigned_user_id,extracted_fields,last_message_at,created_at)
			values ($1::uuid,$1::uuid,$2::uuid,$3,'WHATSAPP',$4,$4,$5,$8::uuid,$9::uuid,$6::uuid,
			 '{"lead":"preserve"}'::jsonb,$7,$7) returning id::text`,
			accountID, instanceID, instanceScopeKey, externalID, state, actor, oldAt, queueID, departmentID).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	conversationA = seedConversation(accountA, instanceA, "contact-a", "human_active")
	conversationA2 = seedConversation(accountA, instanceA2, "contact-a2", "queued")
	conversationB = seedConversation(accountB, instanceB, "contact-b", "queued")
	conversationCompose = seedConversation(accountA, instanceA, "5511888877777", "human_active")
	var contactCompose, contactLegacy string
	if err := pool.QueryRow(ctx, `insert into messaging.contacts
		(account_id,name,phone,source,first_seen_at,last_seen_at,first_channel,last_channel,relationship_status)
		values ($1::uuid,'Contato compose','5511888877777','MANUAL',$2,$2,'WHATSAPP','WHATSAPP','lead')
		returning id::text`, accountA, oldAt).Scan(&contactCompose); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `insert into messaging.contacts
		(account_id,name,phone,source,first_seen_at,last_seen_at,first_channel,last_channel,relationship_status)
		values ($1::uuid,'Contato legacy','5511888866666','MANUAL',$2,$2,'WHATSAPP','WHATSAPP','lead')
		returning id::text`, accountA, oldAt).Scan(&contactLegacy); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `update messaging.conversations
		set contact_id=$3::uuid,client_account_id=$1::uuid
		where account_id=$1::uuid and id=$2::uuid`,
		accountA, conversationCompose, contactCompose); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `insert into messaging.conversations
		(account_id,instance_id,instance_scope_key,channel,external_id,contact_name,state,
		 extracted_fields,last_message_at,created_at)
		values ($1::uuid,null,'Crow Principal','WHATSAPP','legacy-contact','legacy-contact','queued',
		 '{"legacy":"preserve"}'::jsonb,$2,$2) returning id::text`, accountA, oldAt).
		Scan(&conversationLegacy); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `update messaging.conversations
		set contact_id=$3::uuid,client_account_id=$1::uuid
		where account_id=$1::uuid and id=$2::uuid`,
		accountA, conversationLegacy, contactLegacy); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `update messaging.contacts
		set relationship_status='customer',tags='["vip-safe"]'::jsonb
		where account_id=$1::uuid and id=any($2::uuid[]);
		insert into messaging.contact_intelligence
		(account_id,contact_id,summary,facts,preferences,interaction_count,ai_reply_count,
		 handoff_count,last_intent,last_sentiment,last_confidence,last_outcome,
		 last_conversation_id,last_learned_at)
		values
		($1::uuid,$3::uuid,'ci-old-secret-compose','{"sentinel":"ci-old-secret-compose"}'::jsonb,
		 '{"preference":"ci-old-secret-compose"}'::jsonb,9,7,3,'ci-old-secret-compose',
		 'positive',0.900,'ci-old-secret-compose',$5::uuid,$6),
		($1::uuid,$4::uuid,'ci-old-secret-legacy','{"sentinel":"ci-old-secret-legacy"}'::jsonb,
		 '{"preference":"ci-old-secret-legacy"}'::jsonb,8,6,2,'ci-old-secret-legacy',
		 'negative',0.800,'ci-old-secret-legacy',$7::uuid,$6)`,
		accountA, []string{contactCompose, contactLegacy}, contactCompose, contactLegacy,
		conversationCompose, oldAt, conversationLegacy); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `insert into messaging.conversations
		(account_id,instance_id,instance_scope_key,channel,external_id,contact_name,state,
		 extracted_fields,last_message_at,created_at)
		values ($1::uuid,null,'Crow Principal','INSTAGRAM','instagram-contact','instagram-contact','queued',
		 '{"instagram":"preserve"}'::jsonb,$2,$2) returning id::text`, accountA, oldAt).
		Scan(&conversationInstagram); err != nil {
		t.Fatal(err)
	}
	seedMessage := func(accountID, conversationID, instanceID, content, direction, origin string, at time.Time) string {
		t.Helper()
		var id string
		err := pool.QueryRow(ctx, `insert into messaging.messages
			(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
			 content,status,origin,created_at) values
			($1::uuid,$2::uuid,$3::uuid,'scope',$4,'TEXT',$5,'PENDING',$6,$7) returning id::text`,
			accountID, conversationID, instanceID, direction, content, origin, at).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	oldInbound = seedMessage(accountA, conversationA, instanceA, "old-secret", "INBOUND", "contact", oldAt)
	oldOutbound = seedMessage(accountA, conversationA, instanceA, "old-ai", "OUTBOUND", "ai", oldAt.Add(time.Second))
	acceptedOutbound = seedMessage(accountA, conversationA, instanceA, "provider-accepted", "OUTBOUND", "human", oldAt.Add(2*time.Second))
	staleAIFuture := seedMessage(accountA, conversationA, instanceA, "stale-ai-after-cutoff", "OUTBOUND", "ai", time.Now().UTC().Add(time.Hour))
	if _, err := pool.Exec(ctx, `update messaging.messages set metadata_json='{"aiGeneration":0}'::jsonb
		where account_id=$1::uuid and id=$2::uuid`, accountA, staleAIFuture); err != nil {
		t.Fatal(err)
	}
	composeOldMessage := seedMessage(accountA, conversationCompose, instanceA, "compose-old-secret", "INBOUND", "contact", oldAt)
	_ = seedMessage(accountA, conversationA2, instanceA2, "other-instance", "INBOUND", "contact", oldAt)
	_ = seedMessage(accountB, conversationB, instanceB, "other-account", "INBOUND", "contact", oldAt)
	var legacyMessage, instagramMessage string
	if err := pool.QueryRow(ctx, `insert into messaging.messages
		(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
		 content,status,origin,created_at) values
		($1::uuid,$2::uuid,null,'Crow Principal','INBOUND','TEXT','legacy-secret','SENT','contact',$3)
		returning id::text`, accountA, conversationLegacy, oldAt).Scan(&legacyMessage); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `insert into messaging.messages
		(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
		 content,status,origin,created_at) values
		($1::uuid,$2::uuid,null,'Crow Principal','INBOUND','TEXT','instagram-visible','SENT','contact',$3)
		returning id::text`, accountA, conversationInstagram, oldAt).Scan(&instagramMessage); err != nil {
		t.Fatal(err)
	}
	var acceptedAuditID string
	if err := pool.QueryRow(ctx, `with accepted as (
		update messaging.messages set status='SENT',external_message_id='provider-accepted-id'
		where account_id=$1::uuid and id=$2::uuid returning id
	) insert into messaging.audit_events(account_id,conversation_id,message_id,event_type,payload_json)
	select $1::uuid,$3::uuid,id,'MESSAGE_OUTBOUND_SENT','{"accepted":true}'::jsonb from accepted
	returning id::text`, accountA, acceptedOutbound, conversationA).Scan(&acceptedAuditID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into messaging.handoffs
		(account_id,conversation_id,status,reason_code,summary,collected_fields,requested_at)
		values ($1::uuid,$2::uuid,'requested','policy','old-handoff-secret',
		        '{"old":"secret"}'::jsonb,$4);
		insert into messaging.sla_events
		(account_id,conversation_id,handoff_id,event_type,idempotency_key,occurred_at,metadata)
		select $1::uuid,$2::uuid,h.id,'started','p0-old-sla',$4,'{"old":"secret"}'::jsonb
		from messaging.handoffs h where h.account_id=$1::uuid and h.conversation_id=$2::uuid
		order by h.requested_at desc,h.id desc limit 1;
		insert into messaging.routing_decisions
		(account_id,conversation_id,outcome,reason,input,decided_at)
		values ($1::uuid,$2::uuid,'unrouted','old-routing-secret','{"old":"secret"}'::jsonb,$4);
		insert into messaging.ai_dispatches
		(id,account_id,conversation_id,agent_version_id,generation,status,message_ids,run_after,idempotency_key,created_at)
		values ($6::uuid,$1::uuid,$2::uuid,$7::uuid,0,'processing',array[$3::uuid],$4,'p0-dispatch',$4);
		insert into messaging.outbox(account_id,ordering_key,idempotency_key,kind,payload,status)
		values ($1::uuid,$2,'p0-pending','omnichannel.outbound',jsonb_build_object('messageId',$5::text),'pending'),
		       ($1::uuid,$2,'p0-processing','omnichannel.outbound',jsonb_build_object('messageId',$5::text),'processing'),
		       ($1::uuid,$2,'p0-stale-ai-future','omnichannel.outbound',jsonb_build_object('messageId',$8::text),'pending')`,
		accountA, conversationA, oldInbound, oldAt, oldOutbound, dispatchA, versionA, staleAIFuture); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into messaging.intelligence_outbox
		(event_id,account_id,client_account_id,ordering_key,idempotency_key,kind,
		 aggregate_id,payload,status,occurred_at)
		values ('16161616-1616-4161-8161-161616161616'::uuid,$1::uuid,$1::uuid,$2,
		 'p0-old-intelligence','omnichannel.intelligence_accepted',$2::uuid,
		 jsonb_build_object('dispatchId',$3::text),'pending',$4)`,
		accountA, conversationA, dispatchA, oldAt); err != nil {
		t.Fatal(err)
	}
	customerDataTx, err := appPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.insertCustomerDataInboundEventTx(ctx, customerDataTx, customerDataInboundSnapshot{
		AccountID: accountA, ClientAccountID: accountA, ContactID: contactCompose,
		ConversationID: conversationCompose, MessageID: composeOldMessage,
		ChannelClientBindingID: bindingA, Channel: "WHATSAPP", Provider: "evolution",
		OccurredAt: oldAt, BindingState: "resolved",
	}); err != nil {
		_ = customerDataTx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := customerDataTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if count, err := store.CountInstanceConversations(ctx, accountA, instanceA); err != nil || count != 3 {
		t.Fatalf("legacy delete guard count=%d err=%v", count, err)
	}
	if scoped, err := store.ListConversations(ctx, accountA, ConversationFilter{
		ConversationPageFilter: ConversationPageFilter{InstanceID: instanceA, Limit: 100},
	}); err != nil || len(scoped) != 3 {
		t.Fatalf("legacy instance filter=%v err=%v", conversationIDs(scoped), err)
	}
	var revisionBeforeLegacy, auditBeforeLegacy int
	if err := pool.QueryRow(ctx, `select history_reset_revision from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid`, accountA, instanceA).Scan(&revisionBeforeLegacy); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `select count(*) from messaging.audit_events where account_id=$1::uuid`, accountA).
		Scan(&auditBeforeLegacy); err != nil {
		t.Fatal(err)
	}
	legacyRecorder := httptest.NewRecorder()
	legacyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost,
		"/v1/omnichannel/tenant/whatsapp/conversations/clear", strings.NewReader(`{"broken":`))
	handleClearConversations(nil).ServeHTTP(legacyRecorder, legacyRequest)
	var revisionAfterLegacy, auditAfterLegacy int
	if err := pool.QueryRow(ctx, `select history_reset_revision from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid`, accountA, instanceA).Scan(&revisionAfterLegacy); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `select count(*) from messaging.audit_events where account_id=$1::uuid`, accountA).
		Scan(&auditAfterLegacy); err != nil {
		t.Fatal(err)
	}
	if legacyRecorder.Code != http.StatusConflict || revisionAfterLegacy != revisionBeforeLegacy ||
		auditAfterLegacy != auditBeforeLegacy {
		t.Fatalf("legacy endpoint mutated state: status=%d revision=%d/%d audit=%d/%d",
			legacyRecorder.Code, revisionBeforeLegacy, revisionAfterLegacy, auditBeforeLegacy, auditAfterLegacy)
	}

	if _, err := svc.ResetInstanceHistory(ctx, accountA, instanceA, Caller{UserID: "99999999-9999-4999-8999-999999999999"},
		InstanceHistoryResetInput{Confirmation: "Crow Principal", ExpectedRevision: int64ptr(0)}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("missing permissions=%v, want forbidden", err)
	}
	if _, err := svc.ResetInstanceHistory(ctx, accountA, instanceA, Caller{UserID: actor},
		InstanceHistoryResetInput{Confirmation: "wrong", ExpectedRevision: int64ptr(0)}); !errors.Is(err, ErrHistoryResetConfirmationMismatch) {
		t.Fatalf("confirmation=%v", err)
	}
	if _, err := svc.ResetInstanceHistory(ctx, accountA, instanceA, Caller{UserID: actor},
		InstanceHistoryResetInput{Confirmation: "Crow Principal", ExpectedRevision: int64ptr(1)}); !errors.Is(err, ErrHistoryResetRevisionConflict) {
		t.Fatalf("revision=%v", err)
	}
	if _, err := svc.ResetInstanceHistory(ctx, accountB, instanceA, Caller{UserID: actor},
		InstanceHistoryResetInput{Confirmation: "Crow Principal", ExpectedRevision: int64ptr(0)}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-account resource=%v, want 404 sentinel", err)
	}
	reset, err := svc.ResetInstanceHistory(ctx, accountA, instanceA, Caller{UserID: actor},
		InstanceHistoryResetInput{Confirmation: " Crow Principal ", Reason: "privacidade", ExpectedRevision: int64ptr(0)})
	if err != nil || reset.ResetRevision != 1 || reset.HiddenBefore.IsZero() {
		t.Fatalf("reset=%+v err=%v", reset, err)
	}
	if reset.HiddenBefore.Location() != time.UTC {
		t.Fatalf("hiddenBefore must be UTC: %s (%s)", reset.HiddenBefore, reset.HiddenBefore.Location())
	}
	if _, err := svc.ResetInstanceHistory(ctx, accountA, instanceA, Caller{UserID: actor},
		InstanceHistoryResetInput{Confirmation: "Crow Principal", ExpectedRevision: int64ptr(0)}); !errors.Is(err, ErrHistoryResetRevisionConflict) {
		t.Fatalf("stale revision after commit=%v", err)
	}

	var state, queueID, departmentID, assignedID, fields string
	var generation, messages, handoffs int
	if err := pool.QueryRow(ctx, `select state,queue_id::text,department_id::text,assigned_user_id::text,
		ai_generation,extracted_fields::text from messaging.conversations
		where account_id=$1::uuid and id=$2::uuid`, accountA, conversationA).
		Scan(&state, &queueID, &departmentID, &assignedID, &generation, &fields); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `select count(*) from messaging.messages where account_id=$1::uuid and conversation_id=$2::uuid`, accountA, conversationA).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `select count(*) from messaging.handoffs where account_id=$1::uuid and conversation_id=$2::uuid and status='requested'`, accountA, conversationA).Scan(&handoffs); err != nil {
		t.Fatal(err)
	}
	if state != "human_active" || queueID == "" || departmentID == "" || assignedID != actor ||
		generation != 1 || fields != "{}" || messages != 4 || handoffs != 1 {
		t.Fatalf("preservation state=%s queue=%s dept=%s assigned=%s gen=%d fields=%s messages=%d handoffs=%d",
			state, queueID, departmentID, assignedID, generation, fields, messages, handoffs)
	}
	var legacyInstanceID *string
	var legacyGeneration, instagramGeneration int
	var legacyFields, instagramFields string
	if err := pool.QueryRow(ctx, `select instance_id::text,ai_generation,extracted_fields::text
		from messaging.conversations where account_id=$1::uuid and id=$2::uuid`, accountA, conversationLegacy).
		Scan(&legacyInstanceID, &legacyGeneration, &legacyFields); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `select ai_generation,extracted_fields::text
		from messaging.conversations where account_id=$1::uuid and id=$2::uuid`, accountA, conversationInstagram).
		Scan(&instagramGeneration, &instagramFields); err != nil {
		t.Fatal(err)
	}
	if legacyInstanceID == nil || *legacyInstanceID != instanceA || legacyGeneration != 1 || legacyFields != "{}" {
		t.Fatalf("legacy link/reset instance=%v generation=%d fields=%s", legacyInstanceID, legacyGeneration, legacyFields)
	}
	if instagramGeneration != 0 || !strings.Contains(instagramFields, "instagram") {
		t.Fatalf("instagram changed generation=%d fields=%s", instagramGeneration, instagramFields)
	}
	var pendingStatus, processingStatus, staleAIFutureOutboxStatus string
	if err := pool.QueryRow(ctx, `select min(status) filter(where idempotency_key='p0-pending'),
		min(status) filter(where idempotency_key='p0-processing'),
		min(status) filter(where idempotency_key='p0-stale-ai-future') from messaging.outbox
		where account_id=$1::uuid`, accountA).Scan(&pendingStatus, &processingStatus, &staleAIFutureOutboxStatus); err != nil {
		t.Fatal(err)
	}
	if pendingStatus != "dead" || processingStatus != "processing" || staleAIFutureOutboxStatus != "dead" {
		t.Fatalf("outbox pending=%s processing=%s stale-ai=%s", pendingStatus, processingStatus, staleAIFutureOutboxStatus)
	}
	var intelligenceOutboxStatus, intelligenceOutboxError string
	if err := pool.QueryRow(ctx, `select status,last_error from messaging.intelligence_outbox
		where account_id=$1::uuid and idempotency_key='p0-old-intelligence'`, accountA).
		Scan(&intelligenceOutboxStatus, &intelligenceOutboxError); err != nil {
		t.Fatal(err)
	}
	var customerDataOutboxStatus, customerDataOutboxError string
	if err := pool.QueryRow(ctx, `select status,last_error from messaging.customer_data_outbox
		where account_id=$1::uuid and message_id=$2::uuid`, accountA, composeOldMessage).
		Scan(&customerDataOutboxStatus, &customerDataOutboxError); err != nil {
		t.Fatal(err)
	}
	if intelligenceOutboxStatus != "dead" || intelligenceOutboxError != "history_reset" ||
		customerDataOutboxStatus != "dead" || customerDataOutboxError != "history_reset" {
		t.Fatalf("integration outboxes intelligence=%s/%s customerData=%s/%s",
			intelligenceOutboxStatus, intelligenceOutboxError,
			customerDataOutboxStatus, customerDataOutboxError)
	}
	var staleAIFutureStatus, staleAIFutureCode string
	if err := pool.QueryRow(ctx, `select status,provider_error_code from messaging.messages
		where account_id=$1::uuid and id=$2::uuid`, accountA, staleAIFuture).
		Scan(&staleAIFutureStatus, &staleAIFutureCode); err != nil {
		t.Fatal(err)
	}
	if staleAIFutureStatus != "FAILED" || staleAIFutureCode != "history_reset" {
		t.Fatalf("stale future AI status=%s code=%s", staleAIFutureStatus, staleAIFutureCode)
	}
	var dispatchStatus, dispatchError string
	if err := pool.QueryRow(ctx, `select status,last_error from messaging.ai_dispatches
		where account_id=$1::uuid and id=$2::uuid`, accountA, dispatchA).Scan(&dispatchStatus, &dispatchError); err != nil {
		t.Fatal(err)
	}
	allowed, err := store.AIDispatchExternalEffectAllowed(ctx, accountA, dispatchA, 0)
	if err != nil || allowed || dispatchStatus != "cancelled" || dispatchError != "history_reset" {
		t.Fatalf("AI dispatch allowed=%v status=%s error=%s err=%v", allowed, dispatchStatus, dispatchError, err)
	}
	providerCalls := 0
	if _, err := store.DispatchOutbound(ctx, accountA, oldOutbound, func(outboundSendData) (string, error) {
		providerCalls++
		return "must-not-send", nil
	}); !errors.Is(err, ErrHistoryResetInvalidated) || providerCalls != 0 {
		t.Fatalf("claimed outbound err=%v providerCalls=%d", err, providerCalls)
	}
	var auditAccountID, auditActorID string
	var auditConversationID, auditMessageID *string
	var auditPayload []byte
	if err := pool.QueryRow(ctx, `select account_id::text,actor_user_id::text,
		conversation_id::text,message_id::text,payload_json
		from messaging.audit_events
		where account_id=$1::uuid and event_type='WHATSAPP_INSTANCE_HISTORY_RESET'`, accountA).
		Scan(&auditAccountID, &auditActorID, &auditConversationID, &auditMessageID, &auditPayload); err != nil {
		t.Fatal(err)
	}
	var auditFields map[string]any
	if err := json.Unmarshal(auditPayload, &auditFields); err != nil {
		t.Fatal(err)
	}
	newCutoff, cutoffOK := auditFields["newCutoff"].(string)
	parsedCutoff, cutoffErr := time.Parse(time.RFC3339Nano, newCutoff)
	if auditAccountID != accountA || auditActorID != actor || auditConversationID != nil ||
		auditMessageID != nil || auditFields["accountId"] != accountA ||
		auditFields["actorUserId"] != actor || auditFields["instanceId"] != instanceA ||
		auditFields["instanceName"] != "Crow Principal" || auditFields["previousCutoff"] != nil ||
		auditFields["previousRevision"] != float64(0) || auditFields["newRevision"] != float64(1) ||
		cutoffErr != nil || !cutoffOK || !parsedCutoff.Equal(reset.HiddenBefore) ||
		strings.Contains(string(auditPayload), "confirmation") || auditFields["reason"] != "privacidade" {
		t.Fatalf("audit payload=%s", auditPayload)
	}
	var targetProvider, targetCredential, targetProviderConfig, targetName string
	var targetActive bool
	if err := pool.QueryRow(ctx, `select provider,coalesce(credentials_ciphertext,''),
		provider_config::text,instance_name,is_active
		from messaging.whatsapp_instances where account_id=$1::uuid and id=$2::uuid`,
		accountA, instanceA).Scan(&targetProvider, &targetCredential, &targetProviderConfig,
		&targetName, &targetActive); err != nil {
		t.Fatal(err)
	}
	if targetProvider != "evolution" || targetCredential != "cipher-a" ||
		targetProviderConfig != "{}" || targetName != "Crow Principal" || !targetActive {
		t.Fatalf("target connection mutated provider=%s credential=%s config=%s name=%s active=%v",
			targetProvider, targetCredential, targetProviderConfig, targetName, targetActive)
	}
	var acceptedStatus, acceptedExternal string
	var acceptedAuditCount int
	if err := pool.QueryRow(ctx, `select status,coalesce(external_message_id,'') from messaging.messages
		where account_id=$1::uuid and id=$2::uuid`, accountA, acceptedOutbound).
		Scan(&acceptedStatus, &acceptedExternal); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `select count(*) from messaging.audit_events
		where account_id=$1::uuid and id=$2::uuid and event_type='MESSAGE_OUTBOUND_SENT'`, accountA, acceptedAuditID).
		Scan(&acceptedAuditCount); err != nil {
		t.Fatal(err)
	}
	if acceptedStatus != "SENT" || acceptedExternal != "provider-accepted-id" || acceptedAuditCount != 1 {
		t.Fatalf("accepted send changed status=%s external=%s audit=%d", acceptedStatus, acceptedExternal, acceptedAuditCount)
	}
	for _, untouched := range []struct {
		accountID, instanceID, credential string
	}{{accountA, instanceA2, "cipher-a2"}, {accountB, instanceB, "cipher-b"}} {
		var cutoff *time.Time
		var revision int64
		var credential, provider string
		var active bool
		if err := pool.QueryRow(ctx, `select history_visible_from,history_reset_revision,
			coalesce(credentials_ciphertext,''),provider,is_active from messaging.whatsapp_instances
			where account_id=$1::uuid and id=$2::uuid`, untouched.accountID, untouched.instanceID).
			Scan(&cutoff, &revision, &credential, &provider, &active); err != nil {
			t.Fatal(err)
		}
		if cutoff != nil || revision != 0 || credential != untouched.credential || provider != "evolution" || !active {
			t.Fatalf("untouched instance=%s cutoff=%v revision=%d credential=%s provider=%s active=%v",
				untouched.instanceID, cutoff, revision, credential, provider, active)
		}
	}

	conversations, err := store.ListConversations(ctx, accountA, ConversationFilter{ConversationPageFilter: ConversationPageFilter{Limit: 100}})
	if err != nil || len(conversations) != 2 || !containsConversation(conversations, conversationA2) ||
		!containsConversation(conversations, conversationInstagram) || containsConversation(conversations, conversationLegacy) {
		t.Fatalf("post-reset conversations=%v err=%v", conversationIDs(conversations), err)
	}
	if other, err := store.ListConversations(ctx, accountB, ConversationFilter{ConversationPageFilter: ConversationPageFilter{Limit: 100}}); err != nil || len(other) != 1 || other[0].ID != conversationB {
		t.Fatalf("account B conversations=%v err=%v", conversationIDs(other), err)
	}
	if old, err := store.ListMessages(ctx, accountA, actor, conversationA, MessagePageFilter{Limit: 100}); err != nil || len(old) != 0 {
		t.Fatalf("old messages=%d err=%v", len(old), err)
	}
	if legacy, err := store.ListMessages(ctx, accountA, actor, conversationLegacy, MessagePageFilter{Limit: 100}); err != nil || len(legacy) != 0 {
		t.Fatalf("legacy old messages=%v err=%v", messageIDs(legacy), err)
	}
	if _, err := store.GetMessage(ctx, accountA, actor, conversationLegacy, legacyMessage); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("legacy direct message=%v, want hidden", err)
	}
	if instagram, err := store.ListMessages(ctx, accountA, actor, conversationInstagram, MessagePageFilter{Limit: 100}); err != nil || len(instagram) != 1 || instagram[0].ID != instagramMessage {
		t.Fatalf("instagram messages=%v err=%v", messageIDs(instagram), err)
	}
	if _, err := store.GetMediaDescriptor(ctx, accountA, actor, conversationA, oldInbound); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("old media descriptor=%v", err)
	}
	store.SetHistoryCutoffEnforced(false)
	rollbackMessages, rollbackErr := store.ListMessages(
		ctx, accountA, actor, conversationA, MessagePageFilter{Limit: 100},
	)
	var persistedCutoff *time.Time
	if err := pool.QueryRow(ctx, `select history_visible_from from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid`, accountA, instanceA).Scan(&persistedCutoff); err != nil {
		t.Fatal(err)
	}
	if rollbackErr != nil || !containsMessage(rollbackMessages, oldInbound) ||
		!containsMessage(rollbackMessages, staleAIFuture) ||
		persistedCutoff == nil || !persistedCutoff.Equal(reset.HiddenBefore) {
		t.Fatalf("rollback flag did not reveal persisted history: messages=%v cutoff=%v err=%v",
			messageIDs(rollbackMessages), persistedCutoff, rollbackErr)
	}
	store.SetHistoryCutoffEnforced(true)
	if hiddenAgain, err := store.ListMessages(ctx, accountA, actor, conversationA,
		MessagePageFilter{Limit: 100}); err != nil || containsMessage(hiddenAgain, oldInbound) ||
		containsMessage(hiddenAgain, staleAIFuture) {
		t.Fatalf("reenabled cutoff did not hide old history: %v err=%v", messageIDs(hiddenAgain), err)
	}

	// O GET publico permanece 404, mas o compose autorizado pode reutilizar a linha canonica
	// sem projetar preview antigo. Uma citacao antiga continua proibida; um outbound novo faz a
	// conversa reaparecer normalmente.
	if _, err := store.GetConversation(ctx, accountA, conversationCompose); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("public hidden conversation=%v, want no rows", err)
	}
	composeRow, err := store.GetConversationForCompose(ctx, accountA, conversationCompose)
	if err != nil || len(composeRow.LastMessage) != 0 && string(composeRow.LastMessage) != "null" ||
		composeRow.CreatedAt.Before(reset.HiddenBefore) || composeRow.UpdatedAt.Before(reset.HiddenBefore) ||
		composeRow.LastMessageAt.Before(reset.HiddenBefore) {
		t.Fatalf("compose metadata leaked preview=%s err=%v", composeRow.LastMessage, err)
	}
	principal := auth.Principal{UserID: actor, AccountID: accountA, Role: auth.RolePlatformAdmin}
	readService := NewService(store)
	sendService := NewSendService(store, nil, nil, nil)
	actions := NewActionsService(store, readService, sendService, nil, nil, nil)
	opened, err := actions.OpenContactConversation(ctx, accountA, principal, contactCompose)
	if err != nil || opened.ID != conversationCompose || opened.LastMessage != nil ||
		opened.CreatedAt.Before(reset.HiddenBefore) || opened.UpdatedAt.Before(reset.HiddenBefore) ||
		opened.LastMessageAt.Before(reset.HiddenBefore) {
		t.Fatalf("open hidden contact=%+v err=%v", opened, err)
	}
	var composeMessagesBefore int
	if err := pool.QueryRow(ctx, `select count(*) from messaging.messages
		where account_id=$1::uuid and conversation_id=$2::uuid`, accountA, conversationCompose).
		Scan(&composeMessagesBefore); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sendService.SendMessage(ctx, accountA, principal, conversationCompose, SendMessageInput{
		Type: "TEXT", Content: "nao deve citar", ReplyToMessageID: composeOldMessage,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old reply from compose=%v, want not found", err)
	}
	var composeMessagesAfterRejected int
	if err := pool.QueryRow(ctx, `select count(*) from messaging.messages
		where account_id=$1::uuid and conversation_id=$2::uuid`, accountA, conversationCompose).
		Scan(&composeMessagesAfterRejected); err != nil {
		t.Fatal(err)
	}
	if composeMessagesAfterRejected != composeMessagesBefore {
		t.Fatalf("rejected old reply mutated messages: before=%d after=%d", composeMessagesBefore, composeMessagesAfterRejected)
	}
	composeFresh, _, err := sendService.SendMessage(ctx, accountA, principal, conversationCompose,
		SendMessageInput{Type: "TEXT", Content: "compose fresh", IdempotencyKey: "p0-compose-fresh"})
	if err != nil {
		t.Fatalf("fresh compose send: %v", err)
	}
	if _, err := store.GetConversation(ctx, accountA, conversationCompose); err != nil {
		t.Fatalf("fresh outbound did not restore public conversation: %v", err)
	}
	composeVisible, err := store.ListMessages(ctx, accountA, actor, conversationCompose, MessagePageFilter{Limit: 100})
	if err != nil || len(composeVisible) != 1 || composeVisible[0].ID != composeFresh.ID ||
		strings.Contains(composeVisible[0].Content, "old-secret") {
		t.Fatalf("compose visible history=%v err=%v", messageIDs(composeVisible), err)
	}
	composeEvidence, err := store.ListCustomerMessageEvidence(ctx, CustomerEvidenceRequest{
		AccountID: accountA, ClientAccountID: accountA, LookbackDays: 90, Limit: 100,
	}, []string{contactCompose})
	if err != nil || len(composeEvidence) != 1 || composeEvidence[0].MessageID != composeFresh.ID ||
		strings.Contains(composeEvidence[0].Content, "old-secret") {
		t.Fatalf("compose customer evidence=%+v err=%v", composeEvidence, err)
	}

	_ = seedMessage(accountA, conversationA, instanceA, "equal-cutoff", "INBOUND", "contact", reset.HiddenBefore)
	newMessage := seedMessage(accountA, conversationA, instanceA, "new-primary-visible", "INBOUND", "contact", reset.HiddenBefore.Add(time.Microsecond))
	projectedAt := reset.HiddenBefore.Add(2 * time.Microsecond)
	var freshHandoffID string
	if err := pool.QueryRow(ctx, `with fresh as (
		insert into messaging.handoffs
		(account_id,conversation_id,status,reason_code,summary,collected_fields,requested_at)
		values ($1::uuid,$2::uuid,'closed','policy','fresh-handoff',
		        '{"fresh":true}'::jsonb,$3) returning id
	)
	insert into messaging.sla_events
	(account_id,conversation_id,handoff_id,event_type,idempotency_key,occurred_at,metadata)
	select $1::uuid,$2::uuid,id,'started','p0-fresh-sla',$3,'{"fresh":true}'::jsonb from fresh
	returning handoff_id::text`, accountA, conversationA, projectedAt).Scan(&freshHandoffID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into messaging.routing_decisions
		(account_id,conversation_id,outcome,reason,input,decided_at)
		values ($1::uuid,$2::uuid,'unrouted','fresh-routing','{"fresh":true}'::jsonb,$3)`,
		accountA, conversationA, projectedAt); err != nil {
		t.Fatal(err)
	}
	projectedHandoffs, err := store.ListHandoffs(ctx, accountA, conversationA)
	if err != nil || len(projectedHandoffs) != 1 || projectedHandoffs[0].ID != freshHandoffID ||
		strings.Contains(projectedHandoffs[0].Summary, "old-handoff-secret") {
		t.Fatalf("operational handoffs=%+v err=%v", projectedHandoffs, err)
	}
	if open, err := store.GetOpenHandoff(ctx, accountA, conversationA); !errors.Is(err, ErrNotFound) {
		t.Fatalf("open handoff=%+v err=%v, want hidden old handoff", open, err)
	}
	projectedSLA, err := store.ListSLAEvents(ctx, accountA, conversationA)
	if err != nil || len(projectedSLA) != 1 || projectedSLA[0].HandoffID == nil ||
		*projectedSLA[0].HandoffID != freshHandoffID || strings.Contains(string(projectedSLA[0].Metadata), "old") {
		t.Fatalf("operational SLA=%+v err=%v", projectedSLA, err)
	}
	projectedDecisions, err := store.ListRoutingDecisions(ctx, accountA, conversationA)
	if err != nil || len(projectedDecisions) != 1 || projectedDecisions[0].Reason != "fresh-routing" {
		t.Fatalf("operational routing=%+v err=%v", projectedDecisions, err)
	}
	var physicalHandoffs, physicalSLA, physicalDecisions int
	if err := pool.QueryRow(ctx, `select
		(select count(*) from messaging.handoffs where account_id=$1::uuid and conversation_id=$2::uuid),
		(select count(*) from messaging.sla_events where account_id=$1::uuid and conversation_id=$2::uuid),
		(select count(*) from messaging.routing_decisions where account_id=$1::uuid and conversation_id=$2::uuid)`,
		accountA, conversationA).Scan(&physicalHandoffs, &physicalSLA, &physicalDecisions); err != nil {
		t.Fatal(err)
	}
	if physicalHandoffs != 2 || physicalSLA != 2 || physicalDecisions != 2 {
		t.Fatalf("preserved projections handoffs=%d sla=%d decisions=%d", physicalHandoffs, physicalSLA, physicalDecisions)
	}
	var newLegacyEvidenceMessage string
	if err := pool.QueryRow(ctx, `insert into messaging.messages
		(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
		 content,status,origin,created_at)
		values ($1::uuid,$2::uuid,null,'Crow Principal','INBOUND','TEXT','legacy-new-visible','SENT','contact',$3)
		returning id::text`, accountA, conversationLegacy, reset.HiddenBefore.Add(time.Microsecond)).
		Scan(&newLegacyEvidenceMessage); err != nil {
		t.Fatal(err)
	}
	legacyEvidence, err := store.ListCustomerMessageEvidence(ctx, CustomerEvidenceRequest{
		AccountID: accountA, ClientAccountID: accountA, LookbackDays: 90, Limit: 100,
	}, []string{contactLegacy})
	if err != nil || len(legacyEvidence) != 1 || legacyEvidence[0].MessageID != newLegacyEvidenceMessage ||
		strings.Contains(legacyEvidence[0].Content, "legacy-secret") {
		t.Fatalf("legacy customer evidence=%+v err=%v", legacyEvidence, err)
	}
	for _, memoryCase := range []struct {
		contactID, conversationID, sentinel, expectedName string
	}{
		{contactCompose, conversationCompose, "ci-old-secret-compose", "Contato compose"},
		{contactLegacy, conversationLegacy, "ci-old-secret-legacy", "Contato legacy"},
	} {
		preserved, loadErr := store.GetContactIntelligence(ctx, accountA, memoryCase.contactID)
		if loadErr != nil || preserved.Summary != memoryCase.sentinel ||
			!strings.Contains(string(preserved.Facts), memoryCase.sentinel) {
			t.Fatalf("CRM intelligence was not preserved for %s: %+v err=%v",
				memoryCase.contactID, preserved, loadErr)
		}
		operational, loadErr := store.GetOperationalContactIntelligence(
			ctx, accountA, memoryCase.contactID, memoryCase.conversationID,
		)
		if loadErr != nil || !operational.DerivedMemorySuppressed || operational.Summary != "" ||
			string(operational.Facts) != "{}" || string(operational.Preferences) != "{}" ||
			operational.InteractionCount != 0 || operational.AIReplyCount != 0 ||
			operational.HandoffCount != 0 || operational.LastIntent != "" ||
			operational.LastSentiment != "" || operational.LastConfidence != nil ||
			operational.LastOutcome != "" || operational.LastConversationID != nil ||
			operational.LastLearnedAt != nil || operational.UpdatedAt != nil ||
			operational.PreferredName == nil || *operational.PreferredName != memoryCase.expectedName ||
			operational.RelationshipStatus != "customer" ||
			!strings.Contains(string(operational.Tags), "vip-safe") {
			t.Fatalf("operational intelligence leaked derived memory for %s: %+v err=%v",
				memoryCase.contactID, operational, loadErr)
		}
		prompt := buildUserPromptWithContactIntelligence(
			nil, *operational.PreferredName, nil, nil, &operational,
		)
		if strings.Contains(prompt, memoryCase.sentinel) ||
			strings.Contains(prompt, "Memoria autoritativa do contato") ||
			!strings.Contains(prompt, memoryCase.expectedName) {
			t.Fatalf("operational prompt leaked memory for %s: %s", memoryCase.contactID, prompt)
		}
		brain := buildBrainRequestV2(triageParams{
			AccountID:           accountA,
			ConversationID:      &memoryCase.conversationID,
			ContactID:           memoryCase.contactID,
			ContactName:         *operational.PreferredName,
			ContactIntelligence: &operational,
			Agent:               agentRow{ID: agentA},
			Version:             versionRow{ID: versionA, Model: "test"},
		}, nil)
		brainJSON, marshalErr := json.Marshal(brain)
		if marshalErr != nil || strings.Contains(string(brainJSON), memoryCase.sentinel) ||
			brain.Contact.Summary != nil || brain.Contact.RelationshipStatus != "customer" ||
			len(brain.Contact.Tags) != 1 || brain.Contact.Tags[0] != "vip-safe" ||
			brain.Contact.Name == nil || *brain.Contact.Name != memoryCase.expectedName {
			t.Fatalf("operational brain leaked memory for %s: %s err=%v",
				memoryCase.contactID, brainJSON, marshalErr)
		}
	}
	if _, err := pool.Exec(ctx, `update messaging.messages
		set media_source_kind='disk',media_storage_key='visible.bin',external_message_id='new-visible-external'
		where account_id=$1::uuid and id=$2::uuid;
		update messaging.messages set external_message_id='old-hidden-external'
		where account_id=$1::uuid and id=$3::uuid`, accountA, newMessage, oldInbound); err != nil {
		t.Fatal(err)
	}
	visible, err := store.ListMessages(ctx, accountA, actor, conversationA, MessagePageFilter{Limit: 100})
	if err != nil || len(visible) != 1 || visible[0].ID != newMessage {
		t.Fatalf("strict messages=%v err=%v", messageIDs(visible), err)
	}
	if conversations, err := store.ListConversations(ctx, accountA, ConversationFilter{
		ConversationPageFilter: ConversationPageFilter{Limit: 100},
	}); err != nil || !containsConversation(conversations, conversationA) {
		t.Fatalf("new message did not restore conversation: %v err=%v", conversationIDs(conversations), err)
	}
	// Mesmo que workers antigos persistam run/dispatch depois do cutoff, as projecoes
	// operacionais so podem reutilizar referencias cuja mensagem capturada permanece visivel.
	// O handoff historico continua preservado na tabela, mas sem summary/fields no card atual.
	const lateOldRun = "14141414-1414-4141-8141-141414141414"
	const lateOldDispatch = "15151515-1515-4151-8151-151515151515"
	if _, err := pool.Exec(ctx, `insert into messaging.ai_runs
		(id,account_id,conversation_id,agent_id,agent_version_id,message_id,status,provider,model,error,created_at)
		values ($6::uuid,$1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,
		        'provider_error','openai','test','old-run-after-reset',$8);
		insert into messaging.ai_dispatches
		(id,account_id,conversation_id,agent_version_id,generation,status,message_ids,
		 run_after,idempotency_key,last_error,created_at,updated_at)
		values ($7::uuid,$1::uuid,$2::uuid,$4::uuid,1,'failed',array[$5::uuid],
		        $8,'p0-late-old-dispatch','old-dispatch-after-reset',$8,$8)`,
		accountA, conversationA, agentA, versionA, oldInbound, lateOldRun, lateOldDispatch,
		reset.HiddenBefore.Add(4*time.Microsecond)); err != nil {
		t.Fatal(err)
	}
	if interventions, err := store.ListAutomationInterventions(ctx, accountA, accountA, 100); err != nil || len(interventions) != 0 {
		t.Fatalf("old handoff leaked into interventions: %+v err=%v", interventions, err)
	}
	attendances, err := store.ListAutomationAttendances(ctx, accountA, accountA, 100)
	if err != nil {
		t.Fatal(err)
	}
	var currentAttendance *automationAttendanceRow
	for i := range attendances {
		if attendances[i].ConversationID == conversationA {
			currentAttendance = &attendances[i]
			break
		}
	}
	if currentAttendance == nil {
		t.Fatalf("restored conversation missing from attendances: %+v", attendances)
	}
	if currentAttendance.HandoffID != nil || currentAttendance.ReasonCode != "" ||
		currentAttendance.Summary != "" || currentAttendance.DispatchStatus != "" ||
		currentAttendance.AIRunStatus != "" || currentAttendance.AIRunError != "" ||
		currentAttendance.UnansweredCount != 1 || currentAttendance.PendingMessagePreview != "new-primary-visible" {
		t.Fatalf("old operational projection leaked after reset: %+v", *currentAttendance)
	}
	hiddenReply := seedMessage(accountA, conversationA, instanceA, "reply-with-hidden-target", "OUTBOUND", "human",
		reset.HiddenBefore.Add(2*time.Microsecond))
	visibleReply := seedMessage(accountA, conversationA, instanceA, "reply-with-visible-target", "OUTBOUND", "human",
		reset.HiddenBefore.Add(3*time.Microsecond))
	if _, err := pool.Exec(ctx, `update messaging.messages
		set reply_to_message_id=$3::uuid,reply_to_external_message_id='old-hidden-external',
		    metadata_json='{"replySnapshot":{"content":"old-secret","messageType":"TEXT","participant":"old-sender"}}'::jsonb
		where account_id=$1::uuid and id=$2::uuid;
		update messaging.messages
		set reply_to_message_id=$4::uuid,reply_to_external_message_id='new-visible-external',
		    metadata_json='{"replySnapshot":{"content":"new-primary-visible","messageType":"TEXT","participant":"new-sender"}}'::jsonb
		where account_id=$1::uuid and id=$5::uuid`,
		accountA, hiddenReply, oldInbound, newMessage, visibleReply); err != nil {
		t.Fatal(err)
	}
	replyRows, err := store.ListMessages(ctx, accountA, actor, conversationA, MessagePageFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	hiddenReplyView := findMessage(replyRows, hiddenReply)
	visibleReplyView := findMessage(replyRows, visibleReply)
	if hiddenReplyView == nil {
		t.Fatal("hidden-target reply missing from visible page")
	}
	if hiddenReplyView.ReplyTo != nil || strings.Contains(string(hiddenReplyView.MetadataJSON), "replySnapshot") ||
		strings.Contains(string(hiddenReplyView.MetadataJSON), "old-secret") {
		t.Fatalf("hidden reply leaked: %+v metadata=%s", hiddenReplyView, hiddenReplyView.MetadataJSON)
	}
	if visibleReplyView == nil || visibleReplyView.ReplyTo == nil ||
		visibleReplyView.ReplyTo.MessageID == nil || *visibleReplyView.ReplyTo.MessageID != newMessage ||
		visibleReplyView.ReplyTo.Content != "new-primary-visible" {
		t.Fatalf("visible reply degraded: %+v", visibleReplyView)
	}
	if direct, err := store.GetMessage(ctx, accountA, actor, conversationA, hiddenReply); err != nil ||
		direct.ReplyTo != nil || strings.Contains(string(direct.MetadataJSON), "old-secret") {
		t.Fatalf("direct hidden reply leaked: %+v err=%v", direct, err)
	}
	replyProviderCalls := 0
	if _, err := store.DispatchOutbound(ctx, accountA, hiddenReply, func(data outboundSendData) (string, error) {
		replyProviderCalls++
		if data.ReplyExternalID != nil || data.ReplyContent != nil || data.ReplyMessageType != nil {
			t.Fatalf("provider received hidden quote: %+v", data)
		}
		return "reply-sent", nil
	}); err != nil || replyProviderCalls != 1 {
		t.Fatalf("hidden reply dispatch err=%v calls=%d", err, replyProviderCalls)
	}
	page, err := store.ListMessages(ctx, accountA, actor, conversationA, MessagePageFilter{Limit: 1})
	if err != nil || len(page) != 1 || page[0].ID != visibleReply {
		t.Fatalf("first page=%v err=%v", messageIDs(page), err)
	}
	older, err := store.ListMessages(ctx, accountA, actor, conversationA,
		MessagePageFilter{Limit: 100, BeforeID: page[0].ID})
	if err != nil || containsMessage(older, oldInbound) || containsMessage(older, oldOutbound) ||
		!containsMessage(older, newMessage) {
		t.Fatalf("pagination crossed cutoff: %v err=%v", messageIDs(older), err)
	}
	fromHiddenCursor, err := store.ListMessages(ctx, accountA, actor, conversationA,
		MessagePageFilter{Limit: 100, BeforeID: oldInbound})
	if err != nil || containsMessage(fromHiddenCursor, oldInbound) || containsMessage(fromHiddenCursor, oldOutbound) {
		t.Fatalf("hidden beforeId crossed cutoff: %v err=%v", messageIDs(fromHiddenCursor), err)
	}
	if rows, err := store.ListConversations(ctx, accountA, ConversationFilter{ConversationPageFilter: ConversationPageFilter{Search: "old-secret", Limit: 100}}); err != nil || len(rows) != 0 {
		t.Fatalf("old search=%v err=%v", conversationIDs(rows), err)
	}
	if rows, err := store.ListConversations(ctx, accountA, ConversationFilter{ConversationPageFilter: ConversationPageFilter{Search: "new-primary-visible", Limit: 100}}); err != nil || len(rows) != 1 || rows[0].ID != conversationA {
		t.Fatalf("new search=%v err=%v", conversationIDs(rows), err)
	}
	if _, err := store.GetMediaDescriptor(ctx, accountA, actor, conversationA, newMessage); err != nil {
		t.Fatalf("new media descriptor=%v", err)
	}
	if history, err := store.RecentMessages(ctx, accountA, conversationA, 20); err != nil ||
		containsSimMessage(history, oldInbound) || containsSimMessage(history, oldOutbound) ||
		!containsSimMessage(history, newMessage) {
		t.Fatalf("AI history crossed cutoff=%v err=%v", simMessageIDs(history), err)
	}

	// Renomear depois do reset nao pode romper o vinculo reparado nem ressuscitar o legado.
	mainRenamedDisplay := "Crow Principal Renamed"
	if err := store.UpdateInstance(ctx, accountA, instanceA, instanceWrite{
		InstanceName: "Crow Principal Renamed", DisplayName: &mainRenamedDisplay,
		IsActive: true, UserScopePolicy: userScopePolicyMultiInstance,
	}); err != nil {
		t.Fatalf("rename reset instance: %v", err)
	}
	if _, err := store.GetConversation(ctx, accountA, conversationLegacy); err != nil {
		t.Fatalf("fresh legacy conversation disappeared after rename: %v", err)
	}
	legacyAfterRename, err := store.ListMessages(
		ctx, accountA, actor, conversationLegacy, MessagePageFilter{Limit: 100},
	)
	if err != nil || len(legacyAfterRename) != 1 ||
		legacyAfterRename[0].ID != newLegacyEvidenceMessage || containsMessage(legacyAfterRename, legacyMessage) {
		t.Fatalf("legacy history resurrected after rename: messages=%v err=%v",
			messageIDs(legacyAfterRename), err)
	}

	// Uma conversa 0200 ainda sem FK tambem bloqueia hard-delete, e o rename anterior ao reset
	// repara instance_id usando o nome antigo dentro da mesma transacao.
	const legacyRenameInstance = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaac"
	if _, err := pool.Exec(ctx, `insert into messaging.whatsapp_instances
		(id,account_id,instance_name,provider,display_name,is_active)
		values ($2::uuid,$1::uuid,'Legacy Before Reset','evolution','Legacy Before Reset',true);
		insert into messaging.whatsapp_instance_user_grants
		(account_id,instance_id,user_id,access_level,is_active,granted_by_user_id,updated_by_user_id)
		values ($1::uuid,$2::uuid,$3::uuid,'manage',true,$3::uuid,$3::uuid)`,
		accountA, legacyRenameInstance, actor); err != nil {
		t.Fatal(err)
	}
	var renameLegacyConversation, renameLegacyMessage string
	if err := pool.QueryRow(ctx, `insert into messaging.conversations
		(account_id,instance_id,instance_scope_key,channel,external_id,state,last_message_at,created_at)
		values ($1::uuid,null,'Legacy Before Reset','WHATSAPP','legacy-before-reset','queued',$2,$2)
		returning id::text`, accountA, oldAt).Scan(&renameLegacyConversation); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `insert into messaging.messages
		(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,content,status,origin,created_at)
		values ($1::uuid,$2::uuid,null,'Legacy Before Reset','INBOUND','TEXT','rename-old-secret','SENT','contact',$3)
		returning id::text`, accountA, renameLegacyConversation, oldAt).Scan(&renameLegacyMessage); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteInstance(ctx, accountA, Caller{UserID: actor, IsAdmin: true}, legacyRenameInstance); !errors.Is(err, ErrInstanceHasConversations) {
		t.Fatalf("legacy hard-delete guard=%v", err)
	}
	legacyRenamedDisplay := "Legacy After Rename"
	if err := store.UpdateInstance(ctx, accountA, legacyRenameInstance, instanceWrite{
		InstanceName: "Legacy After Rename", DisplayName: &legacyRenamedDisplay,
		IsActive: true, UserScopePolicy: userScopePolicyMultiInstance,
	}); err != nil {
		t.Fatalf("rename before reset: %v", err)
	}
	var repairedInstanceID *string
	if err := pool.QueryRow(ctx, `select instance_id::text from messaging.conversations
		where account_id=$1::uuid and id=$2::uuid`, accountA, renameLegacyConversation).
		Scan(&repairedInstanceID); err != nil || repairedInstanceID == nil || *repairedInstanceID != legacyRenameInstance {
		t.Fatalf("legacy rename repair instance=%v err=%v", repairedInstanceID, err)
	}
	legacyReset, err := svc.ResetInstanceHistory(ctx, accountA, legacyRenameInstance, Caller{UserID: actor},
		InstanceHistoryResetInput{Confirmation: "Legacy After Rename", ExpectedRevision: int64ptr(0)})
	if err != nil || legacyReset.ResetRevision != 1 {
		t.Fatalf("legacy renamed reset=%+v err=%v", legacyReset, err)
	}
	if _, err := store.GetMessage(ctx, accountA, actor, renameLegacyConversation, renameLegacyMessage); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("renamed legacy message visible after reset: %v", err)
	}

	// Corrida deterministica: o reset entra primeiro na fila do lock da instancia; dispatch e
	// inbound ficam atras dele. Depois do commit, o dispatch antigo morre sem IA e o evento
	// atrasado e preservado sem FSM/outbox. Somente um evento > cutoff volta a automatizar.
	const raceInstance = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaad"
	if _, err := pool.Exec(ctx, `insert into messaging.whatsapp_instances
		(id,account_id,instance_name,provider,display_name,is_active)
		values ($2::uuid,$1::uuid,'P0 Race','evolution','P0 Race',true)`, accountA, raceInstance); err != nil {
		t.Fatal(err)
	}
	var raceConversation, raceSeedMessage string
	if err := pool.QueryRow(ctx, `insert into messaging.conversations
		(account_id,instance_id,instance_scope_key,channel,external_id,state,last_message_at,created_at)
		values ($1::uuid,$2::uuid,'P0 Race','WHATSAPP','race-contact@s.whatsapp.net','ai_active',$3,$3)
		returning id::text`, accountA, raceInstance, oldAt).Scan(&raceConversation); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `insert into messaging.messages
		(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,content,status,origin,created_at)
		values ($1::uuid,$2::uuid,$3::uuid,'P0 Race','INBOUND','TEXT','race-seed-old','SENT','contact',$4)
		returning id::text`, accountA, raceConversation, raceInstance, oldAt).Scan(&raceSeedMessage); err != nil {
		t.Fatal(err)
	}
	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var lockedRaceID string
	if err := blocker.QueryRow(ctx, `select id::text from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid for update`, accountA, raceInstance).Scan(&lockedRaceID); err != nil {
		t.Fatal(err)
	}
	resetPool := newHistoryNamedPool(t, ctx, dsn, "p0-history-reset-wait")
	dispatchPool := newHistoryNamedPool(t, ctx, dsn, "p0-history-dispatch-wait")
	inboundPool := newHistoryNamedPool(t, ctx, dsn, "p0-history-inbound-wait")
	defer func() {
		_ = blocker.Rollback(context.Background())
		inboundPool.Close()
		dispatchPool.Close()
		resetPool.Close()
	}()

	type resetRaceResult struct {
		result historyResetResult
		err    error
	}
	resetRaceCh := make(chan resetRaceResult, 1)
	go func() {
		result, resetErr := NewStore(resetPool).ResetInstanceHistory(ctx, historyResetWrite{
			AccountID: accountA, InstanceID: raceInstance, ActorUserID: actor,
			Confirmation: "P0 Race", ExpectedRevision: 0,
		})
		resetRaceCh <- resetRaceResult{result: result, err: resetErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-history-reset-wait")

	dispatchRaceCh := make(chan error, 1)
	go func() {
		_, dispatchErr := NewStore(dispatchPool).UpsertAIDispatch(ctx, accountA, raceConversation,
			versionA, raceSeedMessage, time.Now().UTC())
		dispatchRaceCh <- dispatchErr
	}()
	waitForHistoryLock(t, ctx, pool, "p0-history-dispatch-wait")

	var oldDecideCalls atomic.Int32
	oldInboundWrite := inboundWrite{
		AccountID: accountA, Provider: "evolution", ExternalEventID: "p0-race-old-event",
		EventKind: "message_received", InstanceName: "P0 Race", InstanceID: raceInstance,
		PayloadMasked: json.RawMessage(`{"event":"old"}`), EnqueueAutomation: true,
		Message: &inboundMessageWrite{
			ExternalMessageID: "p0-race-old-message", Channel: "WHATSAPP",
			ContactExternalID: "race-contact@s.whatsapp.net", ContactPhone: "551177771111",
			ContactName: "Race contact", MessageType: "TEXT", Content: "race delayed old",
			OccurredAt: oldAt,
		},
	}
	type inboundRaceResult struct {
		result inboundResult
		err    error
	}
	inboundRaceCh := make(chan inboundRaceResult, 1)
	go func() {
		result, inboundErr := NewStore(inboundPool).PersistInboundWithTransition(ctx, oldInboundWrite,
			func(convSnapshot) (stateUpdate, *decisionRecord, error) {
				oldDecideCalls.Add(1)
				return stateUpdate{NoChange: true}, nil, nil
			})
		inboundRaceCh <- inboundRaceResult{result: result, err: inboundErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-history-inbound-wait")
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	resetRace := receiveHistoryResult(t, resetRaceCh)
	if resetRace.err != nil || resetRace.result.Revision != 1 {
		t.Fatalf("queued reset=%+v err=%v", resetRace.result, resetRace.err)
	}
	if dispatchErr := receiveHistoryResult(t, dispatchRaceCh); !errors.Is(dispatchErr, ErrHistoryResetInvalidated) {
		t.Fatalf("queued dispatch=%v, want history reset", dispatchErr)
	}
	oldRace := receiveHistoryResult(t, inboundRaceCh)
	if oldRace.err != nil || !oldRace.result.MessageCreated || !oldRace.result.HistorySuppressed || oldDecideCalls.Load() != 0 {
		t.Fatalf("queued old inbound=%+v err=%v decideCalls=%d", oldRace.result, oldRace.err, oldDecideCalls.Load())
	}
	var oldAutomationJobs int
	if err := pool.QueryRow(ctx, `select count(*) from messaging.outbox
		where account_id=$1::uuid and kind=$2 and payload->>'messageId'=$3`,
		accountA, AIInboundJobKind, oldRace.result.MessageID).Scan(&oldAutomationJobs); err != nil {
		t.Fatal(err)
	}
	if oldAutomationJobs != 0 {
		t.Fatalf("suppressed inbound enqueued %d AI jobs", oldAutomationJobs)
	}
	if triage, err := store.ConvTriageContext(ctx, accountA, raceConversation); err != nil || triage.Found {
		t.Fatalf("suppressed conversation leaked to triage=%+v err=%v", triage, err)
	}

	var newDecideCalls atomic.Int32
	newInboundWrite := oldInboundWrite
	newInboundWrite.ExternalEventID = "p0-race-new-event"
	newInboundWrite.Message = &inboundMessageWrite{
		ExternalMessageID: "p0-race-new-message", Channel: "WHATSAPP",
		ContactExternalID: "race-contact@s.whatsapp.net", ContactPhone: "551177771111",
		ContactName: "Race contact", MessageType: "TEXT", Content: "race fresh",
		OccurredAt: resetRace.result.Cutoff.Add(time.Microsecond),
	}
	newRace, err := store.PersistInboundWithTransition(ctx, newInboundWrite,
		func(convSnapshot) (stateUpdate, *decisionRecord, error) {
			newDecideCalls.Add(1)
			return stateUpdate{NoChange: true}, nil, nil
		})
	if err != nil || !newRace.MessageCreated || newRace.HistorySuppressed || newDecideCalls.Load() != 1 {
		t.Fatalf("fresh inbound=%+v err=%v decideCalls=%d", newRace, err, newDecideCalls.Load())
	}
	var newAutomationJobs int
	if err := pool.QueryRow(ctx, `select count(*) from messaging.outbox
		where account_id=$1::uuid and kind=$2 and payload->>'messageId'=$3`,
		accountA, AIInboundJobKind, newRace.MessageID).Scan(&newAutomationJobs); err != nil {
		t.Fatal(err)
	}
	if newAutomationJobs != 1 {
		t.Fatalf("fresh inbound AI jobs=%d, want 1", newAutomationJobs)
	}
	raceMessages, err := store.ListMessages(ctx, accountA, actor, raceConversation, MessagePageFilter{Limit: 100})
	if err != nil || len(raceMessages) != 1 || raceMessages[0].ID != newRace.MessageID {
		t.Fatalf("race visible messages=%v err=%v", messageIDs(raceMessages), err)
	}
}

// TestHistoryExternalEffectFenceIntegration prova a ordem concorrente que nao pode ser coberta
// por mocks: reset-first invalida callbacks; effect-first segura o reset ate o retorno externo;
// e o gateway NOWAIT chamado sob um lease n8n nunca forma espera circular.
func TestHistoryExternalEffectFenceIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OMNI_E1_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("OMNI_E1_TEST_DATABASE_URL nao definido")
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool, err := newHistoryFixturePool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		pool.Close()
	})
	appPool, err := newHistoryPool(ctx, dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		appPool.Close()
	})
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
	t.Log("fence: migrations ready")

	const (
		accountID = "31313131-3131-4131-8131-313131313131"
		agentID   = "32323232-3232-4232-8232-323232323232"
		versionID = "33333333-3333-4333-8333-333333333333"
		actorID   = "30303030-3030-4030-8030-303030303030"
	)
	_, _ = pool.Exec(ctx, `delete from core.accounts where id=$1::uuid; delete from core.users where id=$2::uuid`, accountID, actorID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `delete from core.accounts where id=$1::uuid; delete from core.users where id=$2::uuid`, accountID, actorID)
	})
	if _, err := pool.Exec(ctx, `insert into core.accounts(id,slug,name)
		values ($1::uuid,'p0-effect-fence','P0 effect fence');
		insert into core.users(id,email,display_name)
		values ($4::uuid,'p0-effect-actor@example.invalid','P0 effect actor');
		insert into core.account_users(account_id,user_id) values ($1::uuid,$4::uuid);
		insert into messaging.ai_agents(id,account_id,slug,name,enabled)
		values ($2::uuid,$1::uuid,'p0-effect-agent','P0 effect agent',true);
		insert into messaging.ai_agent_versions(id,account_id,agent_id,version,status,provider,model)
		values ($3::uuid,$1::uuid,$2::uuid,1,'published','openai','test')`, accountID, agentID, versionID, actorID); err != nil {
		t.Fatal(err)
	}

	type fenceScope struct {
		instanceID, name, conversationID, inboundID, outboundID, dispatchID, outboxID string
	}
	seedScope := func(instanceID, name, conversationID, inboundID, outboundID, dispatchID, outboxID string) fenceScope {
		t.Helper()
		at := time.Now().UTC().Add(-time.Hour)
		if _, err := pool.Exec(ctx, `insert into messaging.whatsapp_instances
			(id,account_id,instance_name,provider,display_name,is_active)
			values ($2::uuid,$1::uuid,$3,'evolution',$3,true);
			insert into messaging.conversations
			(id,account_id,instance_id,instance_scope_key,channel,external_id,contact_phone,
			 contact_name,state,ai_generation,last_message_at,created_at)
			values ($4::uuid,$1::uuid,$2::uuid,$3,'WHATSAPP',$5,$5,$5,'ai_active',1,$6,$6);
			insert into messaging.messages
			(id,account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
			 content,status,origin,created_at)
			values ($7::uuid,$1::uuid,$4::uuid,$2::uuid,$3,'INBOUND','TEXT','old inbound','SENT','contact',$6),
			       ($8::uuid,$1::uuid,$4::uuid,$2::uuid,$3,'OUTBOUND','TEXT','old outbound','PENDING','human',$6);
			insert into messaging.ai_dispatches
			(id,account_id,conversation_id,agent_version_id,generation,status,message_ids,run_after,idempotency_key,created_at)
			values ($9::uuid,$1::uuid,$4::uuid,$10::uuid,1,'processing',array[$7::uuid],$6,$9,$6);
			insert into messaging.outbox
			(id,account_id,ordering_key,idempotency_key,kind,payload,status,attempts)
			values ($11::uuid,$1::uuid,$4,$11,'omnichannel.outbound',jsonb_build_object('messageId',$8::text),'processing',1)`,
			accountID, instanceID, name, conversationID, name+"@s.whatsapp.net", at,
			inboundID, outboundID, dispatchID, versionID, outboxID); err != nil {
			t.Fatal(err)
		}
		return fenceScope{instanceID: instanceID, name: name, conversationID: conversationID,
			inboundID: inboundID, outboundID: outboundID, dispatchID: dispatchID, outboxID: outboxID}
	}
	prepareMediaGateway := func(scope fenceScope) (*MediaAnalysisGateway, string, string) {
		t.Helper()
		const secret = "old-media-secret"
		media := NewDiskMediaStorage(t.TempDir())
		stored, err := media.Save(ctx, accountID, scope.conversationID, "IMAGE", "image/png", "old.png",
			"data:image/png;base64,b2xkLW1lZGlhLXNlY3JldA==", 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `update messaging.messages set message_type='IMAGE',
			media_storage_key=$3,media_source_kind='disk',media_mime_type=$4,
			media_file_name=$5,media_file_size_bytes=$6
			where account_id=$1::uuid and id=$2::uuid`, accountID, scope.inboundID,
			stored.StorageKey, stored.MimeType, stored.FileName, stored.SizeBytes); err != nil {
			t.Fatal(err)
		}
		analysis, created, err := NewStore(appPool).CreateMediaAnalysis(ctx, accountID, mediaAnalysisCreate{
			MessageID: scope.inboundID, ConversationID: scope.conversationID,
			Kind: MediaAnalysisKindVision, ContentHash: strings.Repeat("a", 64),
			Provider: "openai", Model: "test", AgentVersionID: versionID,
		})
		if err != nil || !created {
			t.Fatalf("create media analysis=%+v created=%v err=%v", analysis, created, err)
		}
		box, err := secretbox.New(make([]byte, 32))
		if err != nil {
			t.Fatal(err)
		}
		token, err := IssueMediaStreamToken(box, accountID, scope.inboundID, analysis.ID, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		return newMediaAnalysisGateway(box, NewStore(appPool), media), token, secret
	}

	resetFirst := seedScope(
		"34343434-3434-4434-8434-343434343434", "Fence Reset First",
		"35353535-3535-4535-8535-353535353535", "36363636-3636-4636-8636-363636363636",
		"37373737-3737-4737-8737-373737373737", "38383838-3838-4838-8838-383838383838",
		"39393939-3939-4939-8939-393939393939")
	resetFirstGateway, resetFirstToken, resetFirstSecret := prepareMediaGateway(resetFirst)
	if rows, err := NewStore(appPool).ListMediaAnalyses(ctx, accountID, resetFirst.inboundID); err != nil || len(rows) != 1 {
		t.Fatalf("visible media analyses=%d err=%v", len(rows), err)
	}
	const (
		toolBindingID  = "67676767-6767-4767-8767-676767676767"
		toolRunID      = "68686868-6868-4868-8868-686868686868"
		toolApprovalID = "69696969-6969-4969-8969-696969696969"
	)
	if _, err := pool.Exec(ctx, `insert into messaging.ai_tool_bindings
		(id,account_id,agent_id,tool_id,is_enabled,mode,allowed_operations)
		values ($2::uuid,$1::uuid,$3::uuid,'p0-tool',true,'propose_write','["write"]'::jsonb);
		insert into messaging.ai_tool_runs
		(id,account_id,conversation_id,dispatch_id,binding_id,call_id,status,operation,input_masked)
		values ($4::uuid,$1::uuid,$5::uuid,$6::uuid,$2::uuid,'p0-call','requested','write','{"old":true}'::jsonb);
		insert into messaging.ai_tool_approvals
		(id,account_id,tool_run_id,binding_id,agent_id,conversation_id,dispatch_id,call_id,operation,arguments_ciphertext)
		values ($7::uuid,$1::uuid,$4::uuid,$2::uuid,$3::uuid,$5::uuid,$6::uuid,'p0-call','write','ciphertext')`,
		accountID, toolBindingID, agentID, toolRunID, resetFirst.conversationID,
		resetFirst.dispatchID, toolApprovalID); err != nil {
		t.Fatal(err)
	}
	if approvals, err := NewStore(appPool).ListAIToolApprovals(ctx, accountID, agentID, 20, ""); err != nil || len(approvals) != 1 || approvals[0].ID != toolApprovalID {
		t.Fatalf("visible tool approvals before reset=%+v err=%v", approvals, err)
	}
	t.Log("fence: reset-first fixture ready")
	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var lockedID string
	if err := blocker.QueryRow(ctx, `select id::text from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid for update`, accountID, resetFirst.instanceID).Scan(&lockedID); err != nil {
		t.Fatal(err)
	}

	resetPool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-reset-first")
	messagePool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-message-first")
	aiPool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-ai-first")
	outboundPool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-outbound-first")
	nowaitPool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-nowait-first")
	t.Cleanup(func() {
		_ = blocker.Rollback(context.Background())
		cancel()
		nowaitPool.Close()
		outboundPool.Close()
		aiPool.Close()
		messagePool.Close()
		resetPool.Close()
	})

	type resetRaceResult struct {
		result historyResetResult
		err    error
	}
	resetCh := make(chan resetRaceResult, 1)
	go func() {
		result, resetErr := NewStore(resetPool).ResetInstanceHistory(ctx, historyResetWrite{
			AccountID: accountID, InstanceID: resetFirst.instanceID,
			ActorUserID: actorID, Confirmation: resetFirst.name, ExpectedRevision: 0,
		})
		resetCh <- resetRaceResult{result: result, err: resetErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-fence-reset-first")
	t.Log("fence: reset queued")

	var messageCalls, aiCalls, providerCalls, nowaitCalls atomic.Int32
	type leaseRaceResult struct {
		allowed bool
		err     error
	}
	messageCh := make(chan leaseRaceResult, 1)
	go func() {
		allowed, leaseErr := NewStore(messagePool).WithMessageExternalEffectLease(ctx, accountID,
			resetFirst.conversationID, resetFirst.inboundID, func() error {
				messageCalls.Add(1)
				return nil
			})
		messageCh <- leaseRaceResult{allowed: allowed, err: leaseErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-fence-message-first")
	aiCh := make(chan leaseRaceResult, 1)
	go func() {
		allowed, leaseErr := NewStore(aiPool).WithAIDispatchExternalEffectLease(ctx, accountID,
			resetFirst.dispatchID, 1, func() error {
				aiCalls.Add(1)
				return nil
			})
		aiCh <- leaseRaceResult{allowed: allowed, err: leaseErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-fence-ai-first")
	outboundCh := make(chan error, 1)
	go func() {
		_, dispatchErr := NewStore(outboundPool).DispatchOutbound(ctx, accountID, resetFirst.outboundID,
			func(outboundSendData) (string, error) {
				providerCalls.Add(1)
				return "must-not-send", nil
			})
		outboundCh <- dispatchErr
	}()
	waitForHistoryLock(t, ctx, pool, "p0-fence-outbound-first")
	t.Log("fence: effects queued")

	mediaRequest := httptest.NewRequestWithContext(ctx, http.MethodGet,
		"/v1/runtime/omnichannel/media/"+resetFirst.inboundID, nil)
	mediaRequest.SetPathValue("messageId", resetFirst.inboundID)
	mediaRequest.Header.Set("Authorization", "Bearer "+resetFirstToken)
	mediaRecorder := httptest.NewRecorder()
	resetFirstGateway.handle(mediaRecorder, mediaRequest)
	if mediaRecorder.Code != http.StatusNotFound || strings.Contains(mediaRecorder.Body.String(), resetFirstSecret) {
		t.Fatalf("reset-first media status=%d body=%q", mediaRecorder.Code, mediaRecorder.Body.String())
	}

	nowaitStarted := time.Now()
	nowaitAllowed, nowaitErr := NewStore(nowaitPool).WithMessageExternalEffectLeaseNowait(ctx,
		accountID, resetFirst.conversationID, resetFirst.inboundID, func() error {
			nowaitCalls.Add(1)
			return nil
		})
	if nowaitErr != nil || nowaitAllowed || nowaitCalls.Load() != 0 || time.Since(nowaitStarted) > time.Second {
		t.Fatalf("reset-first NOWAIT allowed=%v calls=%d err=%v elapsed=%s",
			nowaitAllowed, nowaitCalls.Load(), nowaitErr, time.Since(nowaitStarted))
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	resetResult := receiveHistoryResult(t, resetCh)
	if resetResult.err != nil || resetResult.result.Revision != 1 {
		t.Fatalf("reset-first result=%+v err=%v", resetResult.result, resetResult.err)
	}
	if rows, err := NewStore(appPool).ListMediaAnalyses(ctx, accountID, resetFirst.inboundID); err != nil || len(rows) != 0 {
		t.Fatalf("hidden media analyses=%d err=%v", len(rows), err)
	}
	var toolRunStatus, toolRunError, approvalStatus, approvalReason string
	if err := pool.QueryRow(ctx, `select r.status,r.error,a.status,a.reason
		from messaging.ai_tool_runs r
		join messaging.ai_tool_approvals a on a.account_id=r.account_id and a.tool_run_id=r.id
		where r.account_id=$1::uuid and r.id=$2::uuid`, accountID, toolRunID).
		Scan(&toolRunStatus, &toolRunError, &approvalStatus, &approvalReason); err != nil {
		t.Fatal(err)
	}
	if toolRunStatus != "failed" || toolRunError != "history_reset" ||
		approvalStatus != "expired" || approvalReason != "history_reset" {
		t.Fatalf("tool reset run=%s/%s approval=%s/%s", toolRunStatus, toolRunError, approvalStatus, approvalReason)
	}
	if approvals, err := NewStore(appPool).ListAIToolApprovals(ctx, accountID, agentID, 20, ""); err != nil || len(approvals) != 0 {
		t.Fatalf("old tool approval remained operational=%+v err=%v", approvals, err)
	}
	if _, err := NewStore(appPool).ApprovalView(ctx, accountID, agentID, toolApprovalID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("old direct tool approval=%v, want no rows", err)
	}
	if err := NewStore(appPool).DecideAIToolApproval(ctx, accountID, agentID, toolApprovalID,
		actorID, true, "late"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("late tool approval=%v, want not found", err)
	}
	const (
		freshToolMessageID  = "86868686-8686-4686-8686-868686868686"
		freshToolDispatchID = "87878787-8787-4787-8787-878787878787"
		freshToolRunID      = "88888888-8888-4888-8888-888888888888"
		freshToolApprovalID = "89898989-8989-4989-8989-898989898989"
	)
	if _, err := pool.Exec(ctx, `insert into messaging.messages
		(id,account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
		 content,status,origin,created_at)
		values ($2::uuid,$1::uuid,$3::uuid,$4::uuid,$5,'INBOUND','TEXT','fresh tool context',
		 'SENT','contact',$6);
		insert into messaging.ai_dispatches
		(id,account_id,conversation_id,agent_version_id,generation,status,message_ids,
		 run_after,idempotency_key,created_at)
		values ($7::uuid,$1::uuid,$3::uuid,$8::uuid,2,'processing',array[$2::uuid],$6,$7,$6);
		insert into messaging.ai_tool_runs
		(id,account_id,conversation_id,dispatch_id,binding_id,call_id,status,operation,input_masked)
		values ($9::uuid,$1::uuid,$3::uuid,$7::uuid,$10::uuid,'fresh-call','requested','write',
		 '{"fresh":true}'::jsonb);
		insert into messaging.ai_tool_approvals
		(id,account_id,tool_run_id,binding_id,agent_id,conversation_id,dispatch_id,call_id,
		 operation,arguments_ciphertext,requested_at)
		values ($11::uuid,$1::uuid,$9::uuid,$10::uuid,$12::uuid,$3::uuid,$7::uuid,
		 'fresh-call','write','ciphertext',$6)`, accountID, freshToolMessageID,
		resetFirst.conversationID, resetFirst.instanceID, resetFirst.name,
		resetResult.result.Cutoff.Add(time.Microsecond), freshToolDispatchID, versionID,
		freshToolRunID, toolBindingID, freshToolApprovalID, agentID); err != nil {
		t.Fatal(err)
	}
	if approvals, err := NewStore(appPool).ListAIToolApprovals(ctx, accountID, agentID, 20, ""); err != nil || len(approvals) != 1 || approvals[0].ID != freshToolApprovalID {
		t.Fatalf("fresh tool approval not projected=%+v err=%v", approvals, err)
	}
	var physicalToolApprovals int
	if err := pool.QueryRow(ctx, `select count(*) from messaging.ai_tool_approvals
		where account_id=$1::uuid and agent_id=$2::uuid`, accountID, agentID).
		Scan(&physicalToolApprovals); err != nil || physicalToolApprovals != 2 {
		t.Fatalf("tool approvals not preserved=%d err=%v", physicalToolApprovals, err)
	}
	messageResult := receiveHistoryResult(t, messageCh)
	aiResult := receiveHistoryResult(t, aiCh)
	outboundErr := receiveHistoryResult(t, outboundCh)
	if messageResult.err != nil || messageResult.allowed || messageCalls.Load() != 0 {
		t.Fatalf("message reset-first allowed=%v calls=%d err=%v", messageResult.allowed, messageCalls.Load(), messageResult.err)
	}
	if aiResult.err != nil || aiResult.allowed || aiCalls.Load() != 0 {
		t.Fatalf("AI reset-first allowed=%v calls=%d err=%v", aiResult.allowed, aiCalls.Load(), aiResult.err)
	}
	if !errors.Is(outboundErr, ErrHistoryResetInvalidated) || providerCalls.Load() != 0 {
		t.Fatalf("outbound reset-first calls=%d err=%v", providerCalls.Load(), outboundErr)
	}

	// Um job ja claimed/processing recebe erro terminal, e o engine o dead-lettera sem provider.
	payload, _ := json.Marshal(outboundJobPayload{MessageID: resetFirst.outboundID})
	handlerErr := NewOutboundHandler(NewStore(appPool), nil, nil, nil, nil).Handle(ctx, jobs.Job{
		ID: resetFirst.outboxID, AccountID: accountID, OrderingKey: resetFirst.conversationID,
		Kind: OutboundJobKind, Payload: payload, Attempts: 1, MaxAttempts: 5,
	})
	if !errors.Is(handlerErr, ErrHistoryResetInvalidated) || !jobs.Classify(handlerErr).Unrecoverable {
		t.Fatalf("claimed outbound handler=%v, want unrecoverable history_reset", handlerErr)
	}
	jobStore, err := jobs.NewPostgresStore(appPool, jobs.DefaultTable)
	if err != nil {
		t.Fatal(err)
	}
	if err := jobStore.MarkDead(ctx, accountID, resetFirst.outboxID, "history_reset"); err != nil {
		t.Fatal(err)
	}
	var messageStatus, messageCode, outboxStatus, outboxError string
	if err := pool.QueryRow(ctx, `select status,provider_error_code from messaging.messages
		where account_id=$1::uuid and id=$2::uuid`, accountID, resetFirst.outboundID).
		Scan(&messageStatus, &messageCode); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `select status,last_error from messaging.outbox
		where account_id=$1::uuid and id=$2::uuid`, accountID, resetFirst.outboxID).
		Scan(&outboxStatus, &outboxError); err != nil {
		t.Fatal(err)
	}
	if messageStatus != "FAILED" || messageCode != "history_reset" || outboxStatus != "dead" || outboxError != "history_reset" {
		t.Fatalf("claimed terminal message=%s/%s outbox=%s/%s", messageStatus, messageCode, outboxStatus, outboxError)
	}
	t.Log("fence: reset-first assertions passed")

	// Effect-first: o reset nao avanca enquanto o callback mantem o fence compartilhado.
	effectFirst := seedScope(
		"41414141-4141-4141-8141-414141414141", "Fence Effect First",
		"42424242-4242-4242-8242-424242424242", "43434343-4343-4343-8343-434343434343",
		"44444444-4444-4444-8444-444444444444", "45454545-4545-4545-8545-454545454545",
		"46464646-4646-4646-8646-464646464646")
	effectPool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-effect-holder")
	effectResetPool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-effect-reset")
	effectEntered := make(chan struct{})
	releaseEffect := make(chan struct{})
	var releaseEffectOnce sync.Once
	releaseEffectFn := func() { releaseEffectOnce.Do(func() { close(releaseEffect) }) }
	t.Cleanup(func() {
		releaseEffectFn()
		cancel()
		effectResetPool.Close()
		effectPool.Close()
	})
	effectCh := make(chan leaseRaceResult, 1)
	go func() {
		allowed, leaseErr := NewStore(effectPool).WithMessageExternalEffectLease(ctx, accountID,
			effectFirst.conversationID, effectFirst.inboundID, func() error {
				close(effectEntered)
				<-releaseEffect
				return nil
			})
		effectCh <- leaseRaceResult{allowed: allowed, err: leaseErr}
	}()
	select {
	case <-effectEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("effect-first callback nao adquiriu fence")
	}
	effectResetCh := make(chan resetRaceResult, 1)
	go func() {
		result, resetErr := NewStore(effectResetPool).ResetInstanceHistory(ctx, historyResetWrite{
			AccountID: accountID, InstanceID: effectFirst.instanceID,
			ActorUserID: actorID, Confirmation: effectFirst.name, ExpectedRevision: 0,
		})
		effectResetCh <- resetRaceResult{result: result, err: resetErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-fence-effect-reset")
	select {
	case result := <-effectResetCh:
		t.Fatalf("reset atravessou effect-first: %+v", result)
	case <-time.After(100 * time.Millisecond):
	}
	releaseEffectFn()
	effectResult := receiveHistoryResult(t, effectCh)
	if effectResult.err != nil || !effectResult.allowed {
		t.Fatalf("effect-first allowed=%v err=%v", effectResult.allowed, effectResult.err)
	}
	if result := receiveHistoryResult(t, effectResetCh); result.err != nil || result.result.Revision != 1 {
		t.Fatalf("effect-first reset=%+v err=%v", result.result, result.err)
	}
	t.Log("fence: effect-first assertions passed")

	// Boundary real de stream: o gateway segura o fence ate o ultimo byte; reset-first ja foi
	// provado acima com token valido e resposta 404 sem o segredo da midia.
	mediaStream := seedScope(
		"61616161-6161-4161-8161-616161616161", "Fence Media Stream",
		"62626262-6262-4262-8262-626262626262", "63636363-6363-4363-8363-636363636363",
		"64646464-6464-4464-8464-646464646464", "65656565-6565-4565-8565-656565656565",
		"66666666-6666-4666-8666-666666666666")
	mediaGateway, mediaToken, mediaSecret := prepareMediaGateway(mediaStream)
	mediaResetPool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-media-reset")
	writer := newBlockingMediaResponseWriter()
	var releaseWriterOnce sync.Once
	releaseWriter := func() { releaseWriterOnce.Do(func() { close(writer.release) }) }
	t.Cleanup(func() {
		releaseWriter()
		cancel()
		mediaResetPool.Close()
	})
	streamDone := make(chan struct{})
	go func() {
		request := httptest.NewRequestWithContext(ctx, http.MethodGet,
			"/v1/runtime/omnichannel/media/"+mediaStream.inboundID, nil)
		request.SetPathValue("messageId", mediaStream.inboundID)
		request.Header.Set("Authorization", "Bearer "+mediaToken)
		mediaGateway.handle(writer, request)
		close(streamDone)
	}()
	select {
	case <-writer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("media gateway nao iniciou o stream")
	}
	mediaResetCh := make(chan resetRaceResult, 1)
	go func() {
		result, resetErr := NewStore(mediaResetPool).ResetInstanceHistory(ctx, historyResetWrite{
			AccountID: accountID, InstanceID: mediaStream.instanceID,
			ActorUserID: actorID, Confirmation: mediaStream.name, ExpectedRevision: 0,
		})
		mediaResetCh <- resetRaceResult{result: result, err: resetErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-fence-media-reset")
	select {
	case result := <-mediaResetCh:
		t.Fatalf("reset atravessou stream ativo: %+v", result)
	case <-time.After(100 * time.Millisecond):
	}
	releaseWriter()
	select {
	case <-streamDone:
	case <-time.After(5 * time.Second):
		t.Fatal("media stream nao terminou depois do release")
	}
	if writer.statusCode() != http.StatusOK || writer.body.String() != mediaSecret {
		t.Fatalf("media stream status=%d body=%q", writer.statusCode(), writer.body.String())
	}
	if result := receiveHistoryResult(t, mediaResetCh); result.err != nil || result.result.Revision != 1 {
		t.Fatalf("media stream reset=%+v err=%v", result.result, result.err)
	}
	t.Log("fence: media stream assertions passed")

	// Lease externo n8n + gateway NOWAIT: reset aguardando no meio nao pode causar deadlock.
	nested := seedScope(
		"47474747-4747-4747-8747-474747474747", "Fence Nested",
		"48484848-4848-4848-8848-484848484848", "49494949-4949-4949-8949-494949494949",
		"51515151-5151-4151-8151-515151515151", "52525252-5252-4252-8252-525252525252",
		"53535353-5353-4353-8353-535353535353")
	outerPool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-nested-outer")
	innerPool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-nested-inner")
	nestedResetPool := newHistoryNamedPool(t, ctx, dsn, "p0-fence-nested-reset")
	outerEntered := make(chan struct{})
	callInner := make(chan struct{})
	var callInnerOnce sync.Once
	callInnerFn := func() { callInnerOnce.Do(func() { close(callInner) }) }
	t.Cleanup(func() {
		callInnerFn()
		cancel()
		nestedResetPool.Close()
		innerPool.Close()
		outerPool.Close()
	})
	var innerAllowed bool
	var innerErr error
	var innerCalls atomic.Int32
	outerCh := make(chan leaseRaceResult, 1)
	go func() {
		allowed, leaseErr := NewStore(outerPool).WithMessageExternalEffectLease(ctx, accountID,
			nested.conversationID, nested.inboundID, func() error {
				close(outerEntered)
				<-callInner
				innerAllowed, innerErr = NewStore(innerPool).WithMessageExternalEffectLeaseNowait(ctx,
					accountID, nested.conversationID, nested.inboundID, func() error {
						innerCalls.Add(1)
						return nil
					})
				return nil
			})
		outerCh <- leaseRaceResult{allowed: allowed, err: leaseErr}
	}()
	select {
	case <-outerEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("nested outer nao adquiriu fence")
	}
	nestedResetCh := make(chan resetRaceResult, 1)
	go func() {
		result, resetErr := NewStore(nestedResetPool).ResetInstanceHistory(ctx, historyResetWrite{
			AccountID: accountID, InstanceID: nested.instanceID,
			ActorUserID: actorID, Confirmation: nested.name, ExpectedRevision: 0,
		})
		nestedResetCh <- resetRaceResult{result: result, err: resetErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-fence-nested-reset")
	callInnerFn()
	outerResult := receiveHistoryResult(t, outerCh)
	if outerResult.err != nil || !outerResult.allowed || innerErr != nil || innerAllowed != (innerCalls.Load() == 1) {
		t.Fatalf("nested outer=%v/%v inner=%v calls=%d err=%v",
			outerResult.allowed, outerResult.err, innerAllowed, innerCalls.Load(), innerErr)
	}
	if result := receiveHistoryResult(t, nestedResetCh); result.err != nil || result.result.Revision != 1 {
		t.Fatalf("nested reset=%+v err=%v", result.result, result.err)
	}
	t.Log("fence: nested assertions passed")

	// Scheduler de SLA usa o mesmo fence da instancia. Reset-first suprime a
	// materializacao de um handoff antigo; scheduler-first conclui o insert
	// anterior ao cutoff e o reset espera esse commit.
	const (
		slaDepartmentID = "71717171-7171-4171-8171-717171717171"
		slaQueueID      = "72727272-7272-4272-8272-727272727272"
	)
	if _, err := pool.Exec(ctx, `insert into messaging.departments(id,account_id,slug,name)
		values ($2::uuid,$1::uuid,'p0-sla-fence','P0 SLA fence');
		insert into messaging.queues(id,account_id,department_id,slug,name)
		values ($3::uuid,$1::uuid,$2::uuid,'p0-sla-fence','P0 SLA fence');
		insert into messaging.queue_sla_policies
		(account_id,queue_id,first_response_seconds,resolution_seconds,is_active)
		values ($1::uuid,$3::uuid,1,60,true)`, accountID, slaDepartmentID, slaQueueID); err != nil {
		t.Fatal(err)
	}
	seedSLAScope := func(instanceID, name, conversationID, inboundID, outboundID, dispatchID, outboxID string) (fenceScope, string) {
		t.Helper()
		scope := seedScope(instanceID, name, conversationID, inboundID, outboundID, dispatchID, outboxID)
		var handoffID string
		if err := pool.QueryRow(ctx, `insert into messaging.handoffs
			(account_id,conversation_id,status,reason_code,target_queue_id,requested_at)
			values ($1::uuid,$2::uuid,'queued','policy',$3::uuid,$4)
			returning id::text`, accountID, conversationID, slaQueueID, time.Now().UTC().Add(-time.Hour)).
			Scan(&handoffID); err != nil {
			t.Fatal(err)
		}
		return scope, handoffID
	}
	slaResetFirst, slaResetHandoff := seedSLAScope(
		"73737373-7373-4373-8373-737373737373", "SLA Reset First",
		"74747474-7474-4474-8474-747474747474", "75757575-7575-4575-8575-757575757575",
		"76767676-7676-4676-8676-767676767676", "77777777-7777-4777-8777-777777777777",
		"78787878-7878-4878-8878-787878787878")
	slaBlocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := slaBlocker.QueryRow(ctx, `select id::text from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid for update`, accountID, slaResetFirst.instanceID).
		Scan(&lockedID); err != nil {
		t.Fatal(err)
	}
	slaResetPool := newHistoryNamedPool(t, ctx, dsn, "p0-sla-reset-first-reset")
	slaEvalPool := newHistoryNamedPool(t, ctx, dsn, "p0-sla-reset-first-eval")
	handoffAfterResetPool := newHistoryNamedPool(t, ctx, dsn, "p0-handoff-reset-first")
	defer func() {
		_ = slaBlocker.Rollback(context.Background())
		handoffAfterResetPool.Close()
		slaEvalPool.Close()
		slaResetPool.Close()
	}()
	slaResetCh := make(chan resetRaceResult, 1)
	go func() {
		result, resetErr := NewStore(slaResetPool).ResetInstanceHistory(ctx, historyResetWrite{
			AccountID: accountID, InstanceID: slaResetFirst.instanceID,
			ActorUserID: actorID, Confirmation: slaResetFirst.name, ExpectedRevision: 0,
		})
		slaResetCh <- resetRaceResult{result: result, err: resetErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-sla-reset-first-reset")
	type slaEvaluationResult struct {
		created int
		err     error
	}
	slaEvalCh := make(chan slaEvaluationResult, 1)
	go func() {
		created, evalErr := NewStore(slaEvalPool).EvaluateSLAs(ctx, time.Now().UTC().Unix())
		slaEvalCh <- slaEvaluationResult{created: created, err: evalErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-sla-reset-first-eval")
	handoffAfterResetCh := make(chan error, 1)
	handoffTargetQueueID := slaQueueID
	go func() {
		_, createErr := NewStore(handoffAfterResetPool).CreateHandoff(
			ctx, accountID, slaResetFirst.conversationID, actorID,
			HandoffRequest{
				ReasonCode: "policy", Summary: "must-not-create-after-reset",
				CollectedFields: json.RawMessage(`{"old":"must-not-create-after-reset"}`),
				TargetQueueID:   &handoffTargetQueueID,
				IdempotencyKey:  "p0-reset-first-late-handoff",
			},
		)
		handoffAfterResetCh <- createErr
	}()
	waitForHistoryLock(t, ctx, pool, "p0-handoff-reset-first")
	if err := slaBlocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	slaResetResult := receiveHistoryResult(t, slaResetCh)
	if slaResetResult.err != nil || slaResetResult.result.Revision != 1 {
		t.Fatalf("SLA reset-first reset=%+v err=%v", slaResetResult.result, slaResetResult.err)
	}
	if result := receiveHistoryResult(t, slaEvalCh); result.err != nil || result.created != 0 {
		t.Fatalf("SLA reset-first evaluation=%+v", result)
	}
	if createErr := receiveHistoryResult(t, handoffAfterResetCh); !errors.Is(createErr, ErrNotFound) {
		t.Fatalf("handoff reset-first err=%v, want not found", createErr)
	}
	var lateHandoffs int
	if err := pool.QueryRow(ctx, `select count(*) from messaging.handoffs
		where account_id=$1::uuid and idempotency_key='p0-reset-first-late-handoff'`, accountID).
		Scan(&lateHandoffs); err != nil || lateHandoffs != 0 {
		t.Fatalf("handoff created after reset: count=%d err=%v", lateHandoffs, err)
	}
	if _, err := pool.Exec(ctx, `insert into messaging.messages
		(account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,
		 content,status,origin,created_at)
		values ($1::uuid,$2::uuid,$3::uuid,$4,'INBOUND','TEXT','sla fresh','SENT','contact',$5)`,
		accountID, slaResetFirst.conversationID, slaResetFirst.instanceID, slaResetFirst.name,
		slaResetResult.result.Cutoff.Add(time.Microsecond)); err != nil {
		t.Fatal(err)
	}
	if created, err := NewStore(appPool).EvaluateSLAs(ctx, time.Now().UTC().Unix()); err != nil || created != 0 {
		t.Fatalf("old SLA handoff rematerialized after fresh inbound: created=%d err=%v", created, err)
	}
	var resetFirstSLAEvents int
	if err := pool.QueryRow(ctx, `select count(*) from messaging.sla_events
		where account_id=$1::uuid and handoff_id=$2::uuid`, accountID, slaResetHandoff).
		Scan(&resetFirstSLAEvents); err != nil || resetFirstSLAEvents != 0 {
		t.Fatalf("SLA reset-first physical events=%d err=%v", resetFirstSLAEvents, err)
	}

	slaEffectFirst, slaEffectHandoff := seedSLAScope(
		"79797979-7979-4979-8979-797979797979", "SLA Effect First",
		"81818181-8181-4181-8181-818181818181", "82828282-8282-4282-8282-828282828282",
		"83838383-8383-4383-8383-838383838383", "84848484-8484-4484-8484-848484848484",
		"85858585-8585-4585-8585-858585858585")
	slaTableBlocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := slaTableBlocker.Exec(ctx, `lock table messaging.sla_events in access exclusive mode`); err != nil {
		t.Fatal(err)
	}
	slaEffectEvalPool := newHistoryNamedPool(t, ctx, dsn, "p0-sla-effect-first-eval")
	slaEffectResetPool := newHistoryNamedPool(t, ctx, dsn, "p0-sla-effect-first-reset")
	defer func() {
		_ = slaTableBlocker.Rollback(context.Background())
		slaEffectResetPool.Close()
		slaEffectEvalPool.Close()
	}()
	slaEffectEvalCh := make(chan slaEvaluationResult, 1)
	go func() {
		created, evalErr := NewStore(slaEffectEvalPool).EvaluateSLAs(ctx, time.Now().UTC().Unix())
		slaEffectEvalCh <- slaEvaluationResult{created: created, err: evalErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-sla-effect-first-eval")
	slaEffectResetCh := make(chan resetRaceResult, 1)
	go func() {
		result, resetErr := NewStore(slaEffectResetPool).ResetInstanceHistory(ctx, historyResetWrite{
			AccountID: accountID, InstanceID: slaEffectFirst.instanceID,
			ActorUserID: actorID, Confirmation: slaEffectFirst.name, ExpectedRevision: 0,
		})
		slaEffectResetCh <- resetRaceResult{result: result, err: resetErr}
	}()
	waitForHistoryLock(t, ctx, pool, "p0-sla-effect-first-reset")
	select {
	case result := <-slaEffectResetCh:
		t.Fatalf("SLA reset crossed scheduler-first: %+v", result)
	case <-time.After(100 * time.Millisecond):
	}
	if err := slaTableBlocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if result := receiveHistoryResult(t, slaEffectEvalCh); result.err != nil || result.created < 1 {
		t.Fatalf("SLA scheduler-first evaluation=%+v", result)
	}
	if result := receiveHistoryResult(t, slaEffectResetCh); result.err != nil || result.result.Revision != 1 {
		t.Fatalf("SLA scheduler-first reset=%+v err=%v", result.result, result.err)
	}
	var effectFirstSLAEvents int
	if err := pool.QueryRow(ctx, `select count(*) from messaging.sla_events
		where account_id=$1::uuid and handoff_id=$2::uuid`, accountID, slaEffectHandoff).
		Scan(&effectFirstSLAEvents); err != nil || effectFirstSLAEvents < 1 {
		t.Fatalf("SLA scheduler-first physical events=%d err=%v", effectFirstSLAEvents, err)
	}
	t.Log("fence: SLA scheduler assertions passed")
}

type blockingMediaResponseWriter struct {
	header  http.Header
	status  int
	body    bytes.Buffer
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingMediaResponseWriter() *blockingMediaResponseWriter {
	return &blockingMediaResponseWriter{
		header: make(http.Header), entered: make(chan struct{}), release: make(chan struct{}),
	}
}

func (w *blockingMediaResponseWriter) Header() http.Header { return w.header }

func (w *blockingMediaResponseWriter) WriteHeader(status int) { w.status = status }

func (w *blockingMediaResponseWriter) Write(raw []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return w.body.Write(raw)
}

func (w *blockingMediaResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func int64ptr(value int64) *int64 { return &value }

func conversationIDs(rows []conversationRow) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func messageIDs(rows []MessageView) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func containsConversation(rows []conversationRow, id string) bool {
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

func containsMessage(rows []MessageView, id string) bool {
	return findMessage(rows, id) != nil
}

func findMessage(rows []MessageView, id string) *MessageView {
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}

func containsSimMessage(rows []SimMessage, id string) bool {
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

func simMessageIDs(rows []SimMessage) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func newHistoryNamedPool(t *testing.T, ctx context.Context, dsn, applicationName string) *pgxpool.Pool {
	t.Helper()
	pool, err := newHistoryPool(ctx, dsn, applicationName)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

func newHistoryPool(ctx context.Context, dsn, applicationName string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if applicationName != "" {
		config.MaxConns = 1
	}
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	if applicationName != "" {
		config.ConnConfig.RuntimeParams["application_name"] = applicationName
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	return pool, err
}

func newHistoryFixturePool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 8
	// Somente as fixtures usam batches parametrizados de multiplos statements. O Store roda em
	// outro pool, com protocolo extended igual a producao, para nao distorcer json.RawMessage.
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	return pgxpool.NewWithConfig(ctx, config)
}

func waitForHistoryLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, applicationName string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `select exists(select 1 from pg_stat_activity
			where datname=current_database() and application_name=$1 and wait_event_type='Lock')`,
			applicationName).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("session %s did not reach deterministic lock wait", applicationName)
		case <-ticker.C:
		}
	}
}

func receiveHistoryResult[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(10 * time.Second):
		var zero T
		t.Fatal("timed out waiting for history concurrency result")
		return zero
	}
}
