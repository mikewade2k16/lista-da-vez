package omnichannel

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestTakeConversationE5 usa banco descartável explicitamente nomeado para
// provar a corrida do handoff sem tocar no banco do Compose.
func TestTakeConversationE5(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OMNI_E5_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("OMNI_E5_TEST_DATABASE_URL nao definido")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var db string
	if err := pool.QueryRow(context.Background(), `select current_database()`).Scan(&db); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(db, "omni_e5_test_") {
		t.Fatalf("banco recusado: %q", db)
	}
	setupE5TestSchema(t, pool)

	const (
		account  = "11111111-1111-4111-8111-111111111111"
		userA    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		userB    = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
		queue    = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
		dept     = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
		conv     = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
		message  = "ffffffff-ffff-4fff-8fff-ffffffffffff"
		dispatch = "11111111-2222-4333-8444-555555555555"
		handoff  = "22222222-3333-4444-8555-666666666666"
	)
	ctx := context.Background()
	for _, seed := range []struct {
		query string
		args  []any
	}{
		{`insert into core.accounts(id) values ($1)`, []any{account}},
		{`insert into messaging.departments(id,account_id) values ($1,$2)`, []any{dept, account}},
		{`insert into messaging.queues(id,account_id,department_id) values ($1,$2,$3)`, []any{queue, account, dept}},
		{`insert into messaging.queue_members(account_id,queue_id,user_id) values ($1,$2,$3),($1,$2,$4)`, []any{account, queue, userA, userB}},
		{`insert into messaging.conversations(id,account_id,instance_scope_key,channel,external_id,state,queue_id,department_id,last_message_at) values ($1,$2,'main','WHATSAPP','external','ai_active',$3,$4,now())`, []any{conv, account, queue, dept}},
		{`insert into messaging.messages(id,account_id,conversation_id,instance_scope_key,direction,message_type,content,status,origin) values ($1,$2,$3,'main','OUTBOUND','TEXT','ia','PENDING','ai')`, []any{message, account, conv}},
		{`insert into messaging.ai_dispatches(id,account_id,conversation_id,status) values ($1,$2,$3,'processing')`, []any{dispatch, account, conv}},
		{`insert into messaging.outbox(account_id,ordering_key,idempotency_key,kind,payload,status) values ($1,$2,'ai-out','omnichannel.outbound_message',jsonb_build_object('messageId',$3::text),'processing')`, []any{account, conv, message}},
		{`insert into messaging.handoffs(id,account_id,conversation_id,reason_code,status) values ($1,$2,$3,'max_turns','queued')`, []any{handoff, account, conv}},
	} {
		if _, err := pool.Exec(ctx, seed.query, seed.args...); err != nil {
			t.Fatal(err)
		}
	}

	store := NewStore(pool)
	store.SetAIDispatchV2Enabled(true)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, user := range []string{userA, userB} {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()
			_, e := store.TakeConversation(ctx, account, conv, user, false)
			results <- e
		}(user)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	var otherErrs []error
	for e := range results {
		switch {
		case e == nil:
			success++
		case errors.Is(e, ErrConflict):
			conflicts++
		default:
			otherErrs = append(otherErrs, e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d other=%v", success, conflicts, otherErrs)
	}
	var state, assigned, dispatchStatus, messageStatus, providerCode, handoffStatus string
	if err := pool.QueryRow(ctx, `select c.state,c.assigned_user_id::text,d.status,m.status,m.provider_error_code,h.status
		from messaging.conversations c
		join messaging.ai_dispatches d on d.id=$2
		join messaging.messages m on m.id=$3
		join messaging.handoffs h on h.id=$4
		where c.id=$1`, conv, dispatch, message, handoff).Scan(&state, &assigned, &dispatchStatus, &messageStatus, &providerCode, &handoffStatus); err != nil {
		t.Fatal(err)
	}
	if state != "human_active" || (assigned != userA && assigned != userB) || dispatchStatus != "cancelled" || messageStatus != "FAILED" || providerCode != "ai_handoff_canceled" || handoffStatus != "accepted" {
		t.Fatalf("state=%s assigned=%s dispatch=%s message=%s code=%s handoff=%s", state, assigned, dispatchStatus, messageStatus, providerCode, handoffStatus)
	}
}

func TestSelectHandoffPolicyClosesRowsBeforeContactLookup(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OMNI_E5_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("OMNI_E5_TEST_DATABASE_URL nao definido")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var db string
	if err := pool.QueryRow(context.Background(), `select current_database()`).Scan(&db); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(db, "omni_e5_test_") {
		t.Fatalf("banco recusado: %q", db)
	}
	setupE5TestSchema(t, pool)

	const (
		account = "11111111-1111-4111-8111-111111111111"
		contact = "22222222-2222-4222-8222-222222222222"
		policy  = "33333333-3333-4333-8333-333333333333"
	)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `insert into core.accounts(id) values ($1)`, account); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into messaging.contacts
		(id,account_id,relationship_status,classification_confidence,tags)
		values ($1,$2,'customer',0.9,'["vip"]'::jsonb)`, contact, account); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into messaging.handoff_policies
		(id,account_id,name,priority,is_active,conditions)
		values ($1,$2,'Clientes',1,true,'{"relationshipStatus":"customer"}'::jsonb)`, policy, account); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	selected, ok, err := NewStore(pool).selectHandoffPolicyTx(ctx, tx, account, convSnapshot{
		State: StateAIActive, ContactID: stringPtr(contact), Channel: "WHATSAPP",
		InstanceScopeKey: "main", ExtractedFields: []byte(`{}`),
	}, HandoffRequest{ReasonCode: "model_error"})
	if err != nil {
		t.Fatalf("policy selection failed: %v", err)
	}
	if !ok || selected.ID != policy {
		t.Fatalf("selected=%+v ok=%v", selected, ok)
	}
}

func setupE5TestSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ddl := `create extension if not exists pgcrypto;
		drop schema if exists messaging cascade; drop schema if exists core cascade;
		create schema core; create schema messaging;
		create table core.accounts(id uuid primary key);
		create table messaging.departments(id uuid primary key,account_id uuid not null,is_default bool not null default false,is_active bool not null default true);
		create table messaging.queues(id uuid primary key,account_id uuid not null,department_id uuid not null,is_default bool not null default false,is_active bool not null default true);
		create table messaging.queue_members(id uuid primary key default gen_random_uuid(),account_id uuid not null,queue_id uuid not null,user_id uuid not null,is_active bool not null default true);
		create table messaging.whatsapp_instances(id uuid primary key,account_id uuid not null,instance_name text not null,display_name text,is_active bool not null default true);
		create table messaging.contacts(id uuid primary key,account_id uuid not null,relationship_status text not null,classification_confidence double precision,tags jsonb not null default '[]'::jsonb);
		create table messaging.handoff_policies(id uuid primary key,account_id uuid not null,name text not null,priority int not null,is_active bool not null default true,conditions jsonb not null default '{}'::jsonb,target_queue_id uuid,fallback_queue_id uuid,customer_notice_template text not null default '',created_at timestamptz not null default now(),updated_at timestamptz not null default now());
		create table messaging.conversations(id uuid primary key,account_id uuid not null,instance_id uuid,instance_scope_key text not null,assigned_to_id text,contact_id uuid,channel text not null,state text not null,queue_id uuid,department_id uuid,assigned_user_id uuid,ai_generation bigint not null default 0,extracted_fields jsonb not null default '{}',contact_name text,contact_avatar_url text,contact_phone text,external_id text not null,last_message_at timestamptz not null,created_at timestamptz not null default now(),updated_at timestamptz not null default now());
		create table messaging.messages(id uuid primary key,account_id uuid not null,conversation_id uuid not null,instance_id uuid,instance_scope_key text not null,direction text not null,message_type text not null,content text not null,media_url text,status text not null,origin text not null,provider_error_code text not null default '',created_at timestamptz not null default now(),updated_at timestamptz not null default now());
		create table messaging.outbox(id uuid primary key default gen_random_uuid(),account_id uuid not null,ordering_key text not null,idempotency_key text not null,kind text not null,payload jsonb not null,status text not null,locked_at timestamptz,locked_by text not null default '',last_error text not null default '',updated_at timestamptz not null default now());
		create table messaging.ai_dispatches(id uuid primary key,account_id uuid not null,conversation_id uuid not null,status text not null,locked_at timestamptz,last_error text not null default '',updated_at timestamptz not null default now());
		create table messaging.handoffs(id uuid primary key,account_id uuid not null,conversation_id uuid not null,reason_code text not null,status text not null,accepted_by_user_id uuid,accepted_at timestamptz,created_at timestamptz not null default now(),updated_at timestamptz not null default now());
		create table messaging.audit_events(id uuid primary key default gen_random_uuid(),account_id uuid not null,actor_user_id uuid,conversation_id uuid,message_id uuid,event_type text not null,payload_json jsonb,created_at timestamptz not null default now());`
	if _, err := pool.Exec(context.Background(), ddl); err != nil {
		t.Fatalf("fixture schema: %v", err)
	}
}
