package omnichannel

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel/mock"
	platformdb "github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
	platformmodules "github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type accessCapturePublisher struct {
	mu     sync.Mutex
	events []RealtimeEvent
}

func (publisher *accessCapturePublisher) PublishOmnichannelEvent(_ context.Context, event RealtimeEvent) {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	publisher.events = append(publisher.events, event)
}

func (publisher *accessCapturePublisher) snapshot() []RealtimeEvent {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	return append([]RealtimeEvent(nil), publisher.events...)
}

func TestInstanceAccessP1AIntegration(t *testing.T) {
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
	appPool, err := newHistoryPool(ctx, dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(appPool.Close)
	var databaseName string
	if err := fixturePool.QueryRow(ctx, `select current_database()`).Scan(&databaseName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(databaseName, "omni_e1_test_") {
		t.Fatalf("banco de teste recusado: %q", databaseName)
	}
	if err := platformdb.ApplyMigrationsWithOptions(ctx, fixturePool, platformdb.MigrationOptions{SkipDataSeeds: true}); err != nil {
		t.Fatalf("apply real migrations: %v", err)
	}

	const (
		accountA          = "71111111-1111-4111-8111-111111111111"
		accountB          = "72222222-2222-4222-8222-222222222222"
		actorA            = "73333333-3333-4333-8333-333333333333"
		managerB          = "74444444-4444-4444-8444-444444444444"
		outsider          = "75555555-5555-4555-8555-555555555555"
		roleA             = "76666666-6666-4666-8666-666666666666"
		roleB             = "77777777-7777-4777-8777-777777777777"
		legacyResponsible = "78888888-8888-4888-8888-888888888888"
		legacyCreator     = "79999999-9999-4999-8999-999999999999"
		legacyOwnerless   = "70000000-0000-4000-8000-000000000000"
	)
	cleanup := func() {
		_, _ = fixturePool.Exec(context.Background(), `delete from core.accounts where id=any($1::uuid[]);
			delete from core.users where id=any($2::uuid[])`, []string{accountA, accountB}, []string{actorA, managerB, outsider})
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := fixturePool.Exec(ctx, `insert into core.modules(id,schema_name,label)
		values ('omnichannel','messaging','Omnichannel')
		on conflict(id) do update set schema_name=excluded.schema_name,label=excluded.label;
		insert into core.permissions(key,module_id,label,scope) values
		('omnichannel.instances.manage','omnichannel','Manage instances','account'),
		('omnichannel.conversations.view','omnichannel','View conversations','account'),
		('omnichannel.conversations.reply','omnichannel','Reply conversations','account'),
		('omnichannel.conversations.privacy.manage','omnichannel','Manage privacy','account')
		on conflict(key) do update set module_id=excluded.module_id,label=excluded.label,scope=excluded.scope;
		insert into core.accounts(id,slug,name) values
		($1::uuid,'p1a-access-a','P1A access A'),($2::uuid,'p1a-access-b','P1A access B');
		insert into core.account_modules(account_id,module_id,enabled) values
		($1::uuid,'omnichannel',true),($2::uuid,'omnichannel',true);
		insert into core.users(id,email,display_name) values
		($3::uuid,'p1a-actor@example.invalid','P1A Actor'),
		($4::uuid,'p1a-manager@example.invalid','P1A Manager'),
		($5::uuid,'p1a-outsider@example.invalid','P1A Outsider');
		insert into core.account_users(account_id,user_id) values
		($1::uuid,$3::uuid),($1::uuid,$4::uuid),($2::uuid,$5::uuid);
		insert into core.roles(id,account_id,code,label) values
		($6::uuid,$1::uuid,'p1a-actor','P1A Actor'),($7::uuid,$1::uuid,'p1a-manager','P1A Manager');
		insert into core.user_role_assignments(account_id,user_id,role_id) values
		($1::uuid,$3::uuid,$6::uuid),($1::uuid,$4::uuid,$7::uuid);
		insert into core.role_permissions(role_id,permission_key)
		select role_id,permission_key from (values
		($6::uuid,'omnichannel.instances.manage'),($6::uuid,'omnichannel.conversations.view'),
		($6::uuid,'omnichannel.conversations.reply'),($6::uuid,'omnichannel.conversations.privacy.manage'),
		($7::uuid,'omnichannel.instances.manage'),($7::uuid,'omnichannel.conversations.view'),
		($7::uuid,'omnichannel.conversations.reply'),($7::uuid,'omnichannel.conversations.privacy.manage')
		) grant_seed(role_id,permission_key)`, accountA, accountB, actorA, managerB, outsider, roleA, roleB); err != nil {
		t.Fatal(err)
	}
	if _, err := fixturePool.Exec(ctx, `insert into messaging.whatsapp_instances
		(id,account_id,instance_name,provider,responsible_user_id,created_by_user_id,provider_config)
		values
		($2::uuid,$1::uuid,'P1A legacy responsible','mock',$3::uuid,$4::uuid,
		 jsonb_build_object('assignedUserIds',jsonb_build_array($4::text,$5::text,'nao-e-uuid'))),
		($6::uuid,$1::uuid,'P1A legacy creator','mock',$5::uuid,$3::uuid,'{}'::jsonb),
		($7::uuid,$1::uuid,'P1A legacy ownerless','mock',$5::uuid,null,'{}'::jsonb)`,
		accountA, legacyResponsible, actorA, managerB, outsider, legacyCreator, legacyOwnerless); err != nil {
		t.Fatal(err)
	}
	// Reexecutar somente a migration idempotente depois das fixtures simula fielmente o
	// backfill sobre dados legados sem precisar alterar o runner global.
	migrationSQL, err := os.ReadFile("../../platform/database/migrations/0298_messaging_whatsapp_instance_access.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixturePool.Exec(ctx, string(migrationSQL)); err != nil {
		t.Fatalf("rerun P1A migration with legacy fixtures: %v", err)
	}
	var legacyResponsibleLevel, legacyAssignedLevel, legacyCreatorLevel string
	if err := fixturePool.QueryRow(ctx, `select access_level from messaging.whatsapp_instance_user_grants
		where account_id=$1::uuid and instance_id=$2::uuid and user_id=$3::uuid`, accountA, legacyResponsible, actorA).
		Scan(&legacyResponsibleLevel); err != nil || legacyResponsibleLevel != "manage" {
		t.Fatalf("responsible backfill=%q err=%v", legacyResponsibleLevel, err)
	}
	if err := fixturePool.QueryRow(ctx, `select access_level from messaging.whatsapp_instance_user_grants
		where account_id=$1::uuid and instance_id=$2::uuid and user_id=$3::uuid`, accountA, legacyResponsible, managerB).
		Scan(&legacyAssignedLevel); err != nil || legacyAssignedLevel != "reply" {
		t.Fatalf("assigned backfill=%q err=%v", legacyAssignedLevel, err)
	}
	if err := fixturePool.QueryRow(ctx, `select access_level from messaging.whatsapp_instance_user_grants
		where account_id=$1::uuid and instance_id=$2::uuid and user_id=$3::uuid`, accountA, legacyCreator, actorA).
		Scan(&legacyCreatorLevel); err != nil || legacyCreatorLevel != "manage" {
		t.Fatalf("creator fallback backfill=%q err=%v", legacyCreatorLevel, err)
	}

	store := NewStore(appPool)
	report, err := store.InstanceAccessBackfillReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Shared != 0 || report.WithoutManage < 1 || report.IgnoredUsers < 3 {
		t.Fatalf("backfill report=%+v", report)
	}
	publisher := &accessCapturePublisher{}
	session := NewSessionService(store, channel.NewRegistry(mock.New()), nil, nil, nil, nil, publisher)
	provider, inactive := "mock", false
	deniedName := "P1A denied"
	if _, err := session.CreateInstance(ctx, accountB, Caller{UserID: outsider, IsAdmin: true}, InstanceWriteInput{
		InstanceName: &deniedName, Provider: &provider, IsActive: &inactive,
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("legacy admin bypass create err=%v", err)
	}
	name := "P1A criada"
	created, err := session.CreateInstance(ctx, accountA, Caller{UserID: actorA, IsAdmin: false}, InstanceWriteInput{
		InstanceName: &name, Provider: &provider, IsActive: &inactive,
	})
	if err != nil {
		t.Fatalf("permission-based create failed: %v", err)
	}
	if created.AccessPolicy != string(InstanceAccessPolicyRestricted) || created.AccessRevision != 1 {
		t.Fatalf("created access state=%s/%d", created.AccessPolicy, created.AccessRevision)
	}
	if created.ResponsibleUserID == nil || *created.ResponsibleUserID != actorA {
		t.Fatalf("created responsible=%v, want actor", created.ResponsibleUserID)
	}
	var bindingClientID, bindingSource string
	if err := fixturePool.QueryRow(ctx, `select client_account_id::text,source
		from messaging.channel_client_bindings
		where account_id=$1::uuid and whatsapp_instance_id=$2::uuid and effective_to is null`,
		accountA, created.ID).Scan(&bindingClientID, &bindingSource); err != nil {
		t.Fatalf("standalone self-binding: %v", err)
	}
	if bindingClientID != accountA || bindingSource != "standalone_default" {
		t.Fatalf("standalone self-binding client=%s source=%s", bindingClientID, bindingSource)
	}
	readiness, err := store.AutomationBindingReadiness(ctx, accountA, accountA, created.ID, outsider)
	if err != nil || !readiness.BindingReady {
		t.Fatalf("standalone binding readiness=%+v err=%v", readiness, err)
	}
	events := publisher.snapshot()
	if len(events) != 1 || events[0].Type != RealtimeEventInvalidate || events[0].Payload["reason"] != RealtimeInvalidationReasonAccessScopeChanged {
		t.Fatalf("post-commit access invalidation=%+v", events)
	}
	bootstrapSession := NewSessionService(store, channel.NewRegistry(mock.New()), nil,
		platformmodules.NewLimitReader(appPool), nil, nil, publisher)
	bootstrap, err := bootstrapSession.Bootstrap(ctx, accountA, Caller{UserID: actorA, IsAdmin: false}, SessionBootstrapInput{
		InstanceName: "P1A bootstrap", Provider: "mock",
	})
	if err != nil {
		t.Fatalf("permission-based bootstrap failed: %v", err)
	}
	var bootstrapPolicy string
	var bootstrapRevision int64
	if err := fixturePool.QueryRow(ctx, `select access_policy,access_revision
		from messaging.whatsapp_instances where account_id=$1::uuid and instance_name=$2`, accountA, bootstrap.InstanceName).
		Scan(&bootstrapPolicy, &bootstrapRevision); err != nil || bootstrapPolicy != "RESTRICTED" || bootstrapRevision != 1 {
		t.Fatalf("bootstrap access=%s/%d err=%v", bootstrapPolicy, bootstrapRevision, err)
	}
	events = publisher.snapshot()
	if len(events) != 2 || events[1].Payload["reason"] != RealtimeInvalidationReasonAccessScopeChanged {
		t.Fatalf("bootstrap post-commit invalidation=%+v", events)
	}
	var grantLevel string
	var grantActive bool
	if err := fixturePool.QueryRow(ctx, `select access_level,is_active
		from messaging.whatsapp_instance_user_grants
		where account_id=$1::uuid and instance_id=$2::uuid and user_id=$3::uuid`, accountA, created.ID, actorA).
		Scan(&grantLevel, &grantActive); err != nil || grantLevel != "manage" || !grantActive {
		t.Fatalf("initial manager grant level=%q active=%v err=%v", grantLevel, grantActive, err)
	}

	result, err := store.ReplaceInstanceAccess(ctx, InstanceAccessWrite{
		AccountID: accountA, InstanceID: created.ID, ActorUserID: actorA,
		AccessPolicy: InstanceAccessPolicyRestricted, ExpectedRevision: 1,
		Grants: []InstanceGrantInput{{UserID: actorA, AccessLevel: InstanceGrantManage}, {UserID: managerB, AccessLevel: InstanceGrantReply}},
	})
	if err != nil || !result.Changed || result.AccessRevision != 2 {
		t.Fatalf("replace result=%+v err=%v", result, err)
	}
	if _, err := store.ReplaceInstanceAccess(ctx, InstanceAccessWrite{
		AccountID: accountA, InstanceID: created.ID, ActorUserID: actorA,
		AccessPolicy: InstanceAccessPolicyRestricted, ExpectedRevision: 1,
		Grants: []InstanceGrantInput{{UserID: actorA, AccessLevel: InstanceGrantManage}},
	}); !errors.Is(err, ErrInstanceAccessRevisionConflict) {
		t.Fatalf("stale revision err=%v", err)
	}
	if _, err := store.ReplaceInstanceAccess(ctx, InstanceAccessWrite{
		AccountID: accountA, InstanceID: created.ID, ActorUserID: actorA,
		AccessPolicy: InstanceAccessPolicyRestricted, ExpectedRevision: 2,
		Grants: []InstanceGrantInput{{UserID: managerB, AccessLevel: InstanceGrantReply}},
	}); !errors.Is(err, ErrLastInstanceManager) {
		t.Fatalf("last manager err=%v", err)
	}
	if _, err := store.ReplaceInstanceAccess(ctx, InstanceAccessWrite{
		AccountID: accountA, InstanceID: created.ID, ActorUserID: actorA, ResponsibleUserID: managerB,
		AccessPolicy: InstanceAccessPolicyRestricted, ExpectedRevision: 2,
		Grants: []InstanceGrantInput{{UserID: managerB, AccessLevel: InstanceGrantManage}},
	}); err != nil {
		t.Fatalf("manager transfer: %v", err)
	}
	var actorGrantActive bool
	var actorGrantRevision int64
	var actorRevokedAt *string
	if err := fixturePool.QueryRow(ctx, `select is_active,revision,revoked_at::text
		from messaging.whatsapp_instance_user_grants
		where account_id=$1::uuid and instance_id=$2::uuid and user_id=$3::uuid`, accountA, created.ID, actorA).
		Scan(&actorGrantActive, &actorGrantRevision, &actorRevokedAt); err != nil || actorGrantActive || actorGrantRevision != 2 || actorRevokedAt == nil {
		t.Fatalf("revoked actor active=%v revision=%d revokedAt=%v err=%v", actorGrantActive, actorGrantRevision, actorRevokedAt, err)
	}

	actorScope, err := store.LoadConversationAccessScope(ctx, accountA, actorA)
	if err != nil || !actorScope.Eligible || actorScope.Instances[created.ID].Capabilities != (InstanceCapabilities{}) {
		t.Fatalf("restricted actor scope=%+v err=%v", actorScope, err)
	}
	managerScope, err := store.LoadConversationAccessScope(ctx, accountA, managerB)
	if err != nil || !managerScope.Eligible || !managerScope.Instances[created.ID].Capabilities.Manage {
		t.Fatalf("manager scope=%+v err=%v", managerScope, err)
	}
	if _, err := session.ReplaceInstanceAccess(ctx, accountA, created.ID, Caller{UserID: managerB}, InstanceAccessUpdateInput{
		ResponsibleUserID: managerB, AccessPolicy: InstanceAccessPolicyAccountShared, ExpectedRevision: 3,
		Grants: []InstanceGrantInput{{UserID: managerB, AccessLevel: InstanceGrantManage}},
	}); err != nil {
		t.Fatalf("share instance: %v", err)
	}
	events = publisher.snapshot()
	if len(events) != 3 || events[2].Payload["reason"] != RealtimeInvalidationReasonAccessScopeChanged {
		t.Fatalf("grant post-commit invalidation=%+v", events)
	}
	noChange, err := session.ReplaceInstanceAccess(ctx, accountA, created.ID, Caller{UserID: managerB}, InstanceAccessUpdateInput{
		ResponsibleUserID: managerB, AccessPolicy: InstanceAccessPolicyAccountShared, ExpectedRevision: 4,
		Grants: []InstanceGrantInput{{UserID: managerB, AccessLevel: InstanceGrantManage}},
	})
	if err != nil || noChange.Changed || len(publisher.snapshot()) != 3 {
		t.Fatalf("no-op access write=%+v err=%v events=%+v", noChange, err, publisher.snapshot())
	}
	adminView, err := session.GetInstanceAccess(ctx, accountA, created.ID, Caller{UserID: managerB})
	if err != nil || adminView.AccessRevision != 4 || adminView.AccessPolicy != "ACCOUNT_SHARED" ||
		adminView.ResponsibleUserID == nil || *adminView.ResponsibleUserID != managerB || !adminView.MyCapabilities.Manage {
		t.Fatalf("P2 access GET=%+v err=%v", adminView, err)
	}
	if len(adminView.Grants) != 2 || adminView.Grants[0].UserID != actorA || adminView.Grants[0].IsActive ||
		adminView.Grants[0].Revision != 2 || adminView.Grants[1].UserID != managerB || !adminView.Grants[1].IsActive {
		t.Fatalf("P2 access GET grants=%+v", adminView.Grants)
	}
	revision := adminView.AccessRevision
	policy := InstanceAccessPolicyAccountShared
	responsible := managerB
	grants := []InstanceGrantInput{{UserID: managerB, AccessLevel: InstanceGrantManage}}
	putView, err := session.PutInstanceAccess(ctx, accountA, created.ID, Caller{UserID: managerB}, InstanceAccessRequest{
		AccessRevision: &revision, AccessPolicy: &policy, ResponsibleUserID: &responsible, Grants: &grants,
	})
	if err != nil || putView.AccessRevision != 4 || len(publisher.snapshot()) != 3 {
		t.Fatalf("P2 access PUT no-op=%+v err=%v events=%+v", putView, err, publisher.snapshot())
	}
	staleRevision := int64(3)
	if _, err := session.PutInstanceAccess(ctx, accountA, created.ID, Caller{UserID: managerB}, InstanceAccessRequest{
		AccessRevision: &staleRevision, AccessPolicy: &policy, ResponsibleUserID: &responsible, Grants: &grants,
	}); !errors.Is(err, ErrInstanceAccessRevisionConflict) {
		t.Fatalf("P2 access PUT stale revision err=%v", err)
	}
	actorScope, err = store.LoadConversationAccessScope(ctx, accountA, actorA)
	actorCapabilities := actorScope.Instances[created.ID].Capabilities
	if err != nil || !actorCapabilities.View || !actorCapabilities.Reply || actorCapabilities.Manage {
		t.Fatalf("shared actor capabilities=%+v err=%v", actorCapabilities, err)
	}

	if _, err := store.ReplaceInstanceAccess(ctx, InstanceAccessWrite{
		AccountID: accountA, InstanceID: created.ID, ActorUserID: managerB,
		AccessPolicy: InstanceAccessPolicyRestricted, ExpectedRevision: 4,
		Grants: []InstanceGrantInput{{UserID: outsider, AccessLevel: InstanceGrantManage}},
	}); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("cross-account grant err=%v", err)
	}
	start := make(chan struct{})
	concurrentErrors := make([]error, 2)
	concurrentWrites := []InstanceAccessWrite{
		{
			AccountID: accountA, InstanceID: created.ID, ActorUserID: managerB,
			ResponsibleUserID: managerB, AccessPolicy: InstanceAccessPolicyRestricted, ExpectedRevision: 4,
			Grants: []InstanceGrantInput{{UserID: managerB, AccessLevel: InstanceGrantManage}},
		},
		{
			AccountID: accountA, InstanceID: created.ID, ActorUserID: managerB,
			ResponsibleUserID: managerB, AccessPolicy: InstanceAccessPolicyAccountShared, ExpectedRevision: 4,
			Grants: []InstanceGrantInput{{UserID: managerB, AccessLevel: InstanceGrantManage}, {UserID: actorA, AccessLevel: InstanceGrantView}},
		},
	}
	var concurrentWG sync.WaitGroup
	for index := range concurrentWrites {
		concurrentWG.Add(1)
		go func(index int) {
			defer concurrentWG.Done()
			<-start
			_, concurrentErrors[index] = store.ReplaceInstanceAccess(ctx, concurrentWrites[index])
		}(index)
	}
	close(start)
	concurrentWG.Wait()
	winners, conflicts := 0, 0
	for _, writeErr := range concurrentErrors {
		switch {
		case writeErr == nil:
			winners++
		case errors.Is(writeErr, ErrInstanceAccessRevisionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent access error: %v", writeErr)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("concurrent writes winners=%d conflicts=%d errors=%v", winners, conflicts, concurrentErrors)
	}
	var concurrentRevision int64
	if err := fixturePool.QueryRow(ctx, `select access_revision from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid`, accountA, created.ID).Scan(&concurrentRevision); err != nil || concurrentRevision != 5 {
		t.Fatalf("concurrent access revision=%d err=%v", concurrentRevision, err)
	}
	if _, err := fixturePool.Exec(ctx, `update core.account_users set is_active=false
		where account_id=$1::uuid and user_id=$2::uuid`, accountA, actorA); err != nil {
		t.Fatal(err)
	}
	actorScope, err = store.LoadConversationAccessScope(ctx, accountA, actorA)
	if err != nil || actorScope.Eligible || actorScope.Reason != "membership_inactive" {
		t.Fatalf("inactive membership scope=%+v err=%v", actorScope, err)
	}
	if _, err := fixturePool.Exec(ctx, `update core.account_modules set enabled=false
		where account_id=$1::uuid and module_id='omnichannel'`, accountA); err != nil {
		t.Fatal(err)
	}
	managerScope, err = store.LoadConversationAccessScope(ctx, accountA, managerB)
	if err != nil || managerScope.Eligible || managerScope.Reason != "module_disabled" {
		t.Fatalf("disabled module scope=%+v err=%v", managerScope, err)
	}

	var auditCount int
	if err := fixturePool.QueryRow(ctx, `select count(*) from messaging.audit_events
		where account_id=$1::uuid and event_type='WHATSAPP_INSTANCE_ACCESS_CHANGED'
		  and payload_json->>'instanceId'=$2`, accountA, created.ID).Scan(&auditCount); err != nil || auditCount != 5 {
		t.Fatalf("access audit count=%d err=%v", auditCount, err)
	}
}

func TestInstanceAccessP1BEnforcementIntegration(t *testing.T) {
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
	appPool, err := newHistoryPool(ctx, dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(appPool.Close)
	if err := platformdb.ApplyMigrationsWithOptions(ctx, fixturePool, platformdb.MigrationOptions{SkipDataSeeds: true}); err != nil {
		t.Fatalf("apply real migrations: %v", err)
	}

	const (
		accountA  = "81111111-1111-4111-8111-111111111111"
		accountB  = "82222222-2222-4222-8222-222222222222"
		manager   = "83333333-3333-4333-8333-333333333333"
		agent     = "84444444-4444-4444-8444-444444444444"
		ungranted = "85555555-5555-4555-8555-555555555555"
		outsider  = "86666666-6666-4666-8666-666666666666"

		roleManager   = "87777777-7777-4777-8777-777777777777"
		roleAgent     = "88888888-8888-4888-8888-888888888888"
		roleUngranted = "89999999-9999-4999-8999-999999999999"

		instanceManaged = "81000000-0000-4000-8000-000000000001"
		instanceAgent   = "82000000-0000-4000-8000-000000000002"
		instanceNoGrant = "83000000-0000-4000-8000-000000000003"
		instanceShared  = "84000000-0000-4000-8000-000000000004"
		instanceB       = "85000000-0000-4000-8000-000000000005"

		departmentA = "86000000-0000-4000-8000-000000000006"
		queueA      = "87000000-0000-4000-8000-000000000007"
		departmentB = "88000000-0000-4000-8000-000000000008"
		queueB      = "89000000-0000-4000-8000-000000000009"

		convManagedUnrouted = "80100000-0000-4000-8000-000000000001"
		convAgentQueued     = "80200000-0000-4000-8000-000000000002"
		convAgentUnrouted   = "80300000-0000-4000-8000-000000000003"
		convAgentAssigned   = "80400000-0000-4000-8000-000000000004"
		convNoGrantQueued   = "80500000-0000-4000-8000-000000000005"
		convSharedQueued    = "80600000-0000-4000-8000-000000000006"
		convSharedUnrouted  = "80700000-0000-4000-8000-000000000007"
		convInstagram       = "80800000-0000-4000-8000-000000000008"
		convCrossAccount    = "80900000-0000-4000-8000-000000000009"
		convInstagramHidden = "80000000-0000-4000-8000-000000000010"
		contactVisible      = "c1000000-0000-4000-8000-000000000001"
		contactHidden       = "c2000000-0000-4000-8000-000000000002"
	)
	cleanup := func() {
		_, _ = fixturePool.Exec(context.Background(), `delete from core.accounts where id=any($1::uuid[]);
			delete from core.users where id=any($2::uuid[])`, []string{accountA, accountB}, []string{manager, agent, ungranted, outsider})
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := fixturePool.Exec(ctx, `insert into core.modules(id,schema_name,label)
		values ('omnichannel','messaging','Omnichannel')
		on conflict(id) do update set schema_name=excluded.schema_name,label=excluded.label;
		insert into core.permissions(key,module_id,label,scope) values
		('omnichannel.conversations.view','omnichannel','View conversations','account'),
		('omnichannel.conversations.reply','omnichannel','Reply conversations','account'),
		('omnichannel.conversations.assign','omnichannel','Assign conversations','account'),
		('omnichannel.conversations.close','omnichannel','Close conversations','account'),
		('omnichannel.instances.manage','omnichannel','Manage instances','account'),
		('omnichannel.conversations.privacy.manage','omnichannel','Manage privacy','account'),
		('omnichannel.contacts.manage','omnichannel','Manage contacts','account'),
		('omnichannel.settings.manage','omnichannel','Manage settings','account'),
		('omnichannel.agents.manage','omnichannel','Manage agents','account'),
		('omnichannel.audit.view','omnichannel','View audit','account')
		on conflict(key) do update set module_id=excluded.module_id,label=excluded.label,scope=excluded.scope;
		insert into core.accounts(id,slug,name) values
		($1::uuid,'p1b-access-a','P1B access A'),($2::uuid,'p1b-access-b','P1B access B');
		insert into core.account_modules(account_id,module_id,enabled) values
		($1::uuid,'omnichannel',true),($2::uuid,'omnichannel',true);
		insert into core.users(id,email,display_name) values
		($3::uuid,'p1b-manager@example.invalid','P1B Manager'),
		($4::uuid,'p1b-agent@example.invalid','P1B Agent'),
		($5::uuid,'p1b-ungranted@example.invalid','P1B Ungranted'),
		($6::uuid,'p1b-outsider@example.invalid','P1B Outsider');
		insert into core.account_users(account_id,user_id) values
		($1::uuid,$3::uuid),($1::uuid,$4::uuid),($1::uuid,$5::uuid),($2::uuid,$6::uuid);
		insert into core.roles(id,account_id,code,label) values
		($7::uuid,$1::uuid,'p1b-manager','P1B Manager'),
		($8::uuid,$1::uuid,'p1b-agent','P1B Agent'),
		($9::uuid,$1::uuid,'p1b-ungranted','P1B Ungranted');
		insert into core.user_role_assignments(account_id,user_id,role_id) values
		($1::uuid,$3::uuid,$7::uuid),($1::uuid,$4::uuid,$8::uuid),($1::uuid,$5::uuid,$9::uuid);
		insert into core.role_permissions(role_id,permission_key)
		select $7::uuid,key from core.permissions where module_id='omnichannel';
		insert into core.role_permissions(role_id,permission_key) values
		($8::uuid,'omnichannel.conversations.view'),($8::uuid,'omnichannel.conversations.reply'),
		($8::uuid,'omnichannel.instances.manage'),
		($9::uuid,'omnichannel.conversations.view'),($9::uuid,'omnichannel.conversations.reply')`,
		accountA, accountB, manager, agent, ungranted, outsider, roleManager, roleAgent, roleUngranted); err != nil {
		t.Fatal(err)
	}

	if _, err := fixturePool.Exec(ctx, `insert into messaging.whatsapp_instances
		(id,account_id,instance_name,provider,responsible_user_id,created_by_user_id,is_active,access_policy,access_revision,provider_config)
		values
		($2::uuid,$1::uuid,'p1b-managed','mock',$6::uuid,$6::uuid,true,'RESTRICTED',1,'{}'),
		($3::uuid,$1::uuid,'p1b-agent','mock',$6::uuid,$6::uuid,true,'RESTRICTED',1,'{}'),
		($4::uuid,$1::uuid,'p1b-no-grant','mock',$6::uuid,$6::uuid,true,'RESTRICTED',1,'{}'),
		($5::uuid,$1::uuid,'p1b-shared','mock',$6::uuid,$6::uuid,true,'ACCOUNT_SHARED',1,'{}'),
		($8::uuid,$7::uuid,'p1b-other-account','mock',$9::uuid,$9::uuid,true,'RESTRICTED',1,'{}');
		insert into messaging.whatsapp_instance_user_grants
		(account_id,instance_id,user_id,access_level,granted_by_user_id,updated_by_user_id) values
		($1::uuid,$2::uuid,$6::uuid,'manage',$6::uuid,$6::uuid),
		($1::uuid,$3::uuid,$6::uuid,'manage',$6::uuid,$6::uuid),
		($1::uuid,$3::uuid,$10::uuid,'reply',$6::uuid,$6::uuid),
		($1::uuid,$4::uuid,$6::uuid,'manage',$6::uuid,$6::uuid),
		($1::uuid,$5::uuid,$6::uuid,'manage',$6::uuid,$6::uuid),
		($7::uuid,$8::uuid,$9::uuid,'manage',$9::uuid,$9::uuid);
		insert into messaging.departments(id,account_id,slug,name) values
		($11::uuid,$1::uuid,'p1b-dept-a','P1B Dept A'),($13::uuid,$7::uuid,'p1b-dept-b','P1B Dept B');
		insert into messaging.queues(id,account_id,department_id,slug,name) values
		($12::uuid,$1::uuid,$11::uuid,'p1b-queue-a','P1B Queue A'),
		($14::uuid,$7::uuid,$13::uuid,'p1b-queue-b','P1B Queue B');
		insert into messaging.queue_members(account_id,queue_id,user_id) values
		($1::uuid,$12::uuid,$10::uuid),($1::uuid,$12::uuid,$15::uuid),($7::uuid,$14::uuid,$9::uuid)`,
		accountA, instanceManaged, instanceAgent, instanceNoGrant, instanceShared, manager,
		accountB, instanceB, outsider, agent, departmentA, queueA, departmentB, queueB, ungranted); err != nil {
		t.Fatal(err)
	}

	if _, err := fixturePool.Exec(ctx, `insert into messaging.conversations
		(id,account_id,instance_id,instance_scope_key,channel,external_id,state,queue_id,department_id,assigned_user_id,last_message_at)
		values
		($1::uuid,$10::uuid,$11::uuid,'p1b-managed','WHATSAPP','p1b-managed-unrouted','new',null,null,null,now()),
		($2::uuid,$10::uuid,$12::uuid,'p1b-agent','WHATSAPP','p1b-agent-queued','queued',$19::uuid,$20::uuid,null,now()),
		($3::uuid,$10::uuid,$12::uuid,'p1b-agent','WHATSAPP','p1b-agent-unrouted','new',null,null,null,now()),
		($4::uuid,$10::uuid,$12::uuid,'p1b-agent','WHATSAPP','p1b-agent-assigned','human_active',null,null,$21::uuid,now()),
		($5::uuid,$10::uuid,$13::uuid,'p1b-no-grant','WHATSAPP','p1b-no-grant-queued','queued',$19::uuid,$20::uuid,null,now()),
		($6::uuid,$10::uuid,$14::uuid,'p1b-shared','WHATSAPP','p1b-shared-queued','queued',$19::uuid,$20::uuid,null,now()),
		($7::uuid,$10::uuid,$14::uuid,'p1b-shared','WHATSAPP','p1b-shared-unrouted','new',null,null,null,now()),
		($8::uuid,$10::uuid,null,'instagram-p1b','INSTAGRAM','p1b-instagram-assigned','human_active',null,null,$21::uuid,now()),
		($9::uuid,$15::uuid,$16::uuid,'p1b-other-account','WHATSAPP','p1b-cross-account','queued',$17::uuid,$18::uuid,$22::uuid,now()),
		($24::uuid,$10::uuid,null,'instagram-p1b','INSTAGRAM','p1b-instagram-unrouted','new',null,null,null,now());
		insert into messaging.messages
		(id,account_id,conversation_id,instance_id,instance_scope_key,direction,message_type,content,status,origin,created_at)
		select ('90000000-0000-4000-8000-' || lpad(row_number() over(order by conversation.id)::text,12,'0'))::uuid,
			conversation.account_id,conversation.id,conversation.instance_id,conversation.instance_scope_key,
			'INBOUND','TEXT','p1b-visible-message','SENT','contact',now()
		from messaging.conversations conversation
		where conversation.id=any($23::uuid[])`,
		convManagedUnrouted, convAgentQueued, convAgentUnrouted, convAgentAssigned, convNoGrantQueued,
		convSharedQueued, convSharedUnrouted, convInstagram, convCrossAccount, accountA, instanceManaged,
		instanceAgent, instanceNoGrant, instanceShared, accountB, instanceB, queueB, departmentB, queueA,
		departmentA, agent, outsider, []string{convManagedUnrouted, convAgentQueued, convAgentUnrouted,
			convAgentAssigned, convNoGrantQueued, convSharedQueued, convSharedUnrouted, convInstagram, convCrossAccount,
			convInstagramHidden}, convInstagramHidden); err != nil {
		t.Fatal(err)
	}
	if _, err := fixturePool.Exec(ctx, `insert into messaging.contacts(id,account_id,name,phone,source) values
		($2::uuid,$1::uuid,'Visible P1B','551100000001','WHATSAPP'),
		($3::uuid,$1::uuid,'Hidden P1B','551100000002','WHATSAPP');
		update messaging.conversations set contact_id=$2::uuid,contact_phone='551100000001'
		where account_id=$1::uuid and id=$4::uuid;
		update messaging.conversations set contact_id=$3::uuid,contact_phone='551100000002'
		where account_id=$1::uuid and id=$5::uuid`, accountA, contactVisible, contactHidden, convAgentQueued, convAgentUnrouted); err != nil {
		t.Fatal(err)
	}

	store := NewStore(appPool)
	service := NewService(store)
	publisher := &accessCapturePublisher{}
	session := NewSessionService(store, channel.NewRegistry(mock.New()), nil, nil, nil, nil, publisher)
	agentCaller := Caller{UserID: agent, IsAdmin: true}
	managerCaller := Caller{UserID: manager}

	page, err := service.ListConversations(ctx, accountA, agentCaller, ConversationPageFilter{Limit: 100})
	if err != nil {
		t.Fatalf("agent list: %v", err)
	}
	wantAgent := map[string]bool{convAgentQueued: true, convAgentAssigned: true, convSharedQueued: true, convInstagram: true}
	if len(page.Conversations) != len(wantAgent) {
		t.Fatalf("agent conversations=%v want=%v", p1bConversationViewIDs(page.Conversations), wantAgent)
	}
	for _, conversation := range page.Conversations {
		if !wantAgent[conversation.ID] {
			t.Fatalf("agent leaked conversation %s in %v", conversation.ID, p1bConversationViewIDs(page.Conversations))
		}
	}

	visibleMessages, err := service.ListMessages(ctx, accountA, agentCaller, convAgentQueued, MessagePageFilter{Limit: 20})
	if err != nil || len(visibleMessages.Messages) != 1 {
		t.Fatalf("visible message history=%+v err=%v", visibleMessages, err)
	}
	if _, err := service.ListMessages(ctx, accountA, agentCaller, convAgentUnrouted, MessagePageFilter{Limit: 20}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unrouted detail did not fail closed: %v", err)
	}
	if _, err := service.ListMessages(ctx, accountA, managerCaller, convInstagramHidden, MessagePageFilter{Limit: 20}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("settings.manage leaked unrouted Instagram: %v", err)
	}
	if _, err := service.ListMessages(ctx, accountB, agentCaller, convCrossAccount, MessagePageFilter{Limit: 20}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-account principal gate=%v", err)
	}
	visibility, err := service.resolveConversationVisibility(ctx, accountA, agent, "omnichannel.conversations.view", InstanceGrantView)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetVisibleConversation(ctx, accountA, visibility, convCrossAccount); !errors.Is(translate(err), ErrNotFound) {
		t.Fatalf("cross-account resource gate=%v", err)
	}
	contacts, err := service.ListContacts(ctx, accountA, agentCaller)
	if err != nil || len(contacts) != 1 || contacts[0].ID != contactVisible {
		t.Fatalf("visible contacts=%+v err=%v", contacts, err)
	}
	crm, err := service.ListCRMContacts(ctx, accountA, auth.Principal{UserID: agent, AccountID: accountA}, CRMContactFilter{Limit: 20})
	if err != nil || len(crm.Contacts) != 1 || crm.Contacts[0].ID != contactVisible {
		t.Fatalf("visible CRM contacts=%+v err=%v", crm, err)
	}
	var visibleMessageID, hiddenMessageID string
	if err := fixturePool.QueryRow(ctx, `select id::text from messaging.messages where account_id=$1::uuid and conversation_id=$2::uuid`,
		accountA, convAgentQueued).Scan(&visibleMessageID); err != nil {
		t.Fatal(err)
	}
	if err := fixturePool.QueryRow(ctx, `select id::text from messaging.messages where account_id=$1::uuid and conversation_id=$2::uuid`,
		accountA, convAgentUnrouted).Scan(&hiddenMessageID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetVisibleMediaDescriptor(ctx, accountA, convAgentQueued, visibleMessageID, visibility); err != nil {
		t.Fatalf("visible media descriptor: %v", err)
	}
	if _, err := store.GetVisibleMediaDescriptor(ctx, accountA, convAgentUnrouted, hiddenMessageID, visibility); !errors.Is(translate(err), ErrNotFound) {
		t.Fatalf("hidden media descriptor=%v", err)
	}

	accessible, err := service.ListAccessibleInstances(ctx, accountA, agentCaller)
	if err != nil {
		t.Fatal(err)
	}
	if got := instanceIDs(accessible.Instances); len(got) != 2 || !got[instanceAgent] || !got[instanceShared] || got[instanceNoGrant] {
		t.Fatalf("accessible instances=%v", got)
	}
	noGrantAccess, err := service.ListAccessibleInstances(ctx, accountA, Caller{UserID: ungranted})
	if err != nil {
		t.Fatal(err)
	}
	if got := instanceIDs(noGrantAccess.Instances); len(got) != 1 || !got[instanceShared] {
		t.Fatalf("single/no-grant auto access regression=%v", got)
	}

	if _, err := session.InstanceCapabilities(ctx, accountA, agentCaller, instanceAgent); !errors.Is(err, ErrNotFound) {
		t.Fatalf("legacy admin bypassed instance manage grant: %v", err)
	}
	if _, err := session.InstanceCapabilities(ctx, accountA, managerCaller, instanceAgent); err != nil {
		t.Fatalf("manager lifecycle capability: %v", err)
	}
	if _, err := session.InstanceCapabilities(ctx, accountA, managerCaller, instanceB); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-account lifecycle leak: %v", err)
	}

	result, err := session.ReplaceInstanceAccess(ctx, accountA, instanceAgent, managerCaller, InstanceAccessUpdateInput{
		ResponsibleUserID: manager, AccessPolicy: InstanceAccessPolicyRestricted, ExpectedRevision: 1,
		Grants: []InstanceGrantInput{{UserID: manager, AccessLevel: InstanceGrantManage}},
	})
	if err != nil || !result.Changed || result.AccessRevision != 2 {
		t.Fatalf("revoke agent result=%+v err=%v", result, err)
	}
	page, err = service.ListConversations(ctx, accountA, agentCaller, ConversationPageFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	wantAfterRevoke := map[string]bool{convSharedQueued: true, convInstagram: true}
	if len(page.Conversations) != len(wantAfterRevoke) {
		t.Fatalf("post-revoke conversations=%v", p1bConversationViewIDs(page.Conversations))
	}
	for _, conversation := range page.Conversations {
		if !wantAfterRevoke[conversation.ID] {
			t.Fatalf("revoked conversation remained visible: %s", conversation.ID)
		}
	}
	if _, err := service.ListMessages(ctx, accountA, agentCaller, convAgentQueued, MessagePageFilter{Limit: 20}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked history remained visible: %v", err)
	}
	events := publisher.snapshot()
	if len(events) != 1 || events[0].Payload["reason"] != RealtimeInvalidationReasonAccessScopeChanged {
		t.Fatalf("revocation invalidation=%+v", events)
	}
	if _, err := fixturePool.Exec(ctx, `delete from core.role_permissions
		where role_id=$1::uuid and permission_key='omnichannel.conversations.view';
		insert into core.user_permission_overrides(account_id,user_id,permission_key,effect,note)
		values ($2::uuid,$3::uuid,'omnichannel.conversations.view','allow','p1b allow fixture')`, roleAgent, accountA, agent); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListConversations(ctx, accountA, agentCaller, ConversationPageFilter{Limit: 100}); err != nil {
		t.Fatalf("effective allow override: %v", err)
	}
	if _, err := fixturePool.Exec(ctx, `update core.user_permission_overrides set effect='deny'
		where account_id=$1::uuid and user_id=$2::uuid and permission_key='omnichannel.conversations.view' and is_active`, accountA, agent); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListConversations(ctx, accountA, agentCaller, ConversationPageFilter{Limit: 100}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("effective deny override: %v", err)
	}
	if _, err := fixturePool.Exec(ctx, `delete from core.user_permission_overrides
		where account_id=$1::uuid and user_id=$2::uuid and permission_key='omnichannel.conversations.view';
		insert into core.role_permissions(role_id,permission_key)
		values ($3::uuid,'omnichannel.conversations.view')`, accountA, agent, roleAgent); err != nil {
		t.Fatal(err)
	}

	if _, err := fixturePool.Exec(ctx, `update core.account_modules set enabled=false
		where account_id=$1::uuid and module_id='omnichannel'`, accountA); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListConversations(ctx, accountA, agentCaller, ConversationPageFilter{Limit: 100}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("disabled module gate=%v", err)
	}
}

func p1bConversationViewIDs(conversations []ConversationView) []string {
	ids := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		ids = append(ids, conversation.ID)
	}
	return ids
}

func instanceIDs(instances []InstanceView) map[string]bool {
	ids := make(map[string]bool, len(instances))
	for _, instance := range instances {
		ids[instance.ID] = true
	}
	return ids
}
