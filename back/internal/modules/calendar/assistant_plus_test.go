package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func plusTestStore(t *testing.T) *assistantPlusStore {
	t.Helper()
	url := os.Getenv("PLUS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable plus_test database")
	}
	pool, e := pgxpool.New(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	if !strings.HasPrefix(pool.Config().ConnConfig.Database, "plus_test") {
		t.Fatal("refusing non-disposable database")
	}
	_, e = pool.Exec(context.Background(), `create schema if not exists core;create table if not exists core.accounts(id uuid primary key default gen_random_uuid());create table if not exists core.users(id uuid primary key default gen_random_uuid());create schema if not exists calendar;`)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"0191_calendar_chat_conversations.sql", "0287_calendar_chat_proposal_executions.sql"} {
		b, e := os.ReadFile("../../platform/database/migrations/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(context.Background(), string(b)); e != nil {
			t.Fatal(e)
		}
	}
	// Entry surface is additive in the existing chat; old test scaffolds omit it.
	if _, e = pool.Exec(context.Background(), `alter table calendar.chat_conversations add column if not exists entry_surface text not null default 'calendar';alter table calendar.chat_ask_requests drop constraint if exists chat_ask_requests_entry_surface_check;alter table calendar.chat_ask_requests add constraint chat_ask_requests_entry_surface_check check(entry_surface in ('calendar','meta_ads','global','finance'));`); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile("../../platform/database/migrations/0311_assistant_plus.sql")
	if e != nil {
		t.Fatal(e)
	}
	for range 2 {
		if _, e = pool.Exec(context.Background(), string(b)); e != nil {
			t.Fatal(e)
		}
	}
	return &assistantPlusStore{pool: pool}
}
func plusScopeFixture(t *testing.T, store *assistantPlusStore) AssistantPlusScope {
	t.Helper()
	sc := AssistantPlusScope{}
	ctx := context.Background()
	if e := store.pool.QueryRow(ctx, `insert into core.accounts default values returning id::text`).Scan(&sc.AccountID); e != nil {
		t.Fatal(e)
	}
	if e := store.pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&sc.ActorID); e != nil {
		t.Fatal(e)
	}
	return sc
}
func TestPlusPersistentHistoryReplayPrivacyAndContext(t *testing.T) {
	store := plusTestStore(t)
	ctx := context.Background()
	scope := plusScopeFixture(t, store)
	var calls atomic.Int32
	svc := &AssistantPlusService{store: store, ports: AssistantPlusPorts{Authorize: func(context.Context, AssistantPlusScope) error { return nil }, Execute: func(context.Context, AssistantPlusScope, ChatAskRequest) (json.RawMessage, error) {
		calls.Add(1)
		return json.RawMessage(`{"status":"completed","message":"Resultado sintético","data":{},"evidence":[],"proposals":[],"tools":[]}`), nil
	}}}
	req := ChatAskRequest{Mode: "plus", ContractVersion: "1.0", Surface: "finance", Question: "resumo", IdempotencyKey: "synthetic-plus-key"}
	out, e := svc.Ask(ctx, scope, req)
	if e != nil {
		t.Fatal(e)
	}
	var envelope struct {
		ConversationID string `json:"conversationId"`
	}
	if e = json.Unmarshal(out, &envelope); e != nil {
		t.Fatal(e)
	}
	for range 3 {
		replay, e := svc.Ask(ctx, scope, req)
		if e != nil || string(replay) != string(out) {
			t.Fatalf("replay: %v", e)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("effect repeated")
	}
	var count int
	if e = store.pool.QueryRow(ctx, `select count(*) from calendar.chat_messages where account_id=$1::uuid and conversation_id=$2::uuid`, scope.AccountID, envelope.ConversationID).Scan(&count); e != nil || count != 2 {
		t.Fatal("history duplicated")
	}
	changed := req
	changed.Question = "saldo"
	_, e = svc.Ask(ctx, scope, changed)
	assertPlusCode(t, e, "idempotency_conflict")
	other := plusScopeFixture(t, store)
	_, e = svc.History(ctx, other, envelope.ConversationID, 1, 20)
	assertPlusCode(t, e, "not_found")
	sameAccount := other
	sameAccount.AccountID = scope.AccountID
	_, e = svc.History(ctx, sameAccount, envelope.ConversationID, 1, 20)
	assertPlusCode(t, e, "not_found")
	moved := req
	moved.ConversationID = envelope.ConversationID
	moved.IssuerID = "00000000-0000-4000-8000-000000000099"
	moved.IdempotencyKey = "new-context-key"
	_, e = svc.Ask(ctx, scope, moved)
	assertPlusCode(t, e, "conversation_context_conflict")
	// The old transport must never reinterpret a Plus conversation as legacy.
	if _, e = NewStore(store.pool).GetConversation(ctx, envelope.ConversationID, scope.AccountID); e == nil {
		t.Fatal("legacy accessed Plus")
	}
	if e = svc.Delete(ctx, scope, envelope.ConversationID); e != nil {
		t.Fatal(e)
	}
	_, e = svc.Ask(ctx, scope, req)
	assertPlusCode(t, e, "not_found")
}
func TestPlusInFlightAndUnknownNeverRepeatTool(t *testing.T) {
	store := plusTestStore(t)
	ctx := context.Background()
	sc := plusScopeFixture(t, store)
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	svc := &AssistantPlusService{store: store, ports: AssistantPlusPorts{Authorize: func(context.Context, AssistantPlusScope) error { return nil }, Execute: func(context.Context, AssistantPlusScope, ChatAskRequest) (json.RawMessage, error) {
		calls.Add(1)
		close(started)
		<-release
		return nil, errors.New("synthetic uncertain outcome")
	}}}
	req := ChatAskRequest{Mode: "plus", ContractVersion: "1.0", Surface: "finance", Question: "preparar nota", IdempotencyKey: "inflight-plus-key"}
	done := make(chan error, 1)
	go func() { _, e := svc.Ask(ctx, sc, req); done <- e }()
	<-started
	_, e := svc.Ask(ctx, sc, req)
	assertPlusCode(t, e, "request_in_progress")
	close(release)
	if <-done == nil {
		t.Fatal("tool error hidden")
	}
	_, e = svc.Ask(ctx, sc, req)
	assertPlusCode(t, e, "request_in_progress")
	if calls.Load() != 1 {
		t.Fatal("unknown effect repeated")
	}
}
func TestPlusRejectsSecretsAndNonFinancialSurfaceBeforePersisting(t *testing.T) {
	svc := &AssistantPlusService{}
	for _, req := range []ChatAskRequest{
		{Mode: "plus", Surface: "calendar", ContractVersion: "1.0", Question: "resumo", IdempotencyKey: "synthetic-key"},
		{Mode: "plus", Surface: "finance", ContractVersion: "1.0", Question: "senha: valor-sintetico", IdempotencyKey: "synthetic-key"},
	} {
		_, e := svc.Ask(context.Background(), AssistantPlusScope{}, req)
		if e == nil {
			t.Fatal("invalid context or secret accepted")
		}
	}
}
func assertPlusCode(t *testing.T, e error, code string) {
	t.Helper()
	var p *AssistantPlusError
	if !errors.As(e, &p) || p.Code != code {
		t.Fatalf("wanted %s got %v", code, e)
	}
}
