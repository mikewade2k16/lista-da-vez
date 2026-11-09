package finance

import (
	"context"
	"encoding/json"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type conversationLLM struct {
	calls int
	fail  bool
	after func()
}

func (m *conversationLLM) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	m.calls++
	if m.after != nil {
		m.after()
	}
	if !req.NoFormatFallback || req.MaxOutputTokens == 0 {
		panic("unbounded call")
	}
	if m.fail {
		return llm.Response{}, llm.ErrProviderUnavailable
	}
	return llm.Response{JSON: json.RawMessage(`{"answer":"Olá! Como posso ajudar?"}`), Model: req.Model}, nil
}
func TestUnifiedConversationChoosesToolsAIAndFallback(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	raw, e := os.ReadFile("../../platform/database/migrations/0305_finance_assistant.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, strings.Split(string(raw), "alter table")[0]); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"0313_finance_plus_llm_runs.sql", "0316_finance_unified_conversation_runs.sql"} {
		raw, e = os.ReadFile("../../platform/database/migrations/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(raw)); e != nil {
			t.Fatal(e)
		}
	}
	account := financeTestAccount(t, pool)
	var user string
	if e = pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `delete from finance.plus_llm_runs where account_id=$1::uuid`, account)
		_, _ = pool.Exec(ctx, `delete from core.users where id=$1::uuid`, user)
	})
	fiscal := NewFiscalService(NewPostgresFiscalStore(pool), nil)
	service := NewService(NewPostgresSheetStore(pool), NewPostgresConfigStore(pool))
	assistant := &AssistantService{pool: pool, fiscal: fiscal, finance: service, resolver: func(context.Context, string, AssistantConfig) (AssistantRuntime, error) {
		return AssistantRuntime{Provider: "openai", Model: "fixture", APIKey: "synthetic-only"}, nil
	}}
	chat := &AccessConversationService{pool: pool, fiscal: fiscal, documents: newDocumentService(&DocumentStore{pool: pool}, fiscal)}
	assistant.access = chat
	model := &conversationLLM{}
	allowed := true
	plus := NewPlusService(assistant, PlusPorts{LLM: model, Authorize: func(_ context.Context, sc PlusScope, permission string) error {
		if !allowed || sc.AccountID != account || sc.ActorID != user || permission != "finance.sheets.view" {
			return ErrFiscalNotFound
		}
		return nil
	}}, nil)
	chat.orchestrator = &ConversationOrchestrator{plus: plus}
	send := func(id, text, sheet string) AccessMessage {
		t.Helper()
		out, err := chat.Send(ctx, account, user, AccessConversationInput{RequestID: id, Text: text, FinanceSheetID: sheet})
		if err != nil {
			t.Fatal(err)
		}
		return out.Messages[len(out.Messages)-1]
	}
	send("00000000-0000-4000-8000-000000000201", "olá", "")
	if model.calls != 0 {
		t.Fatal("called model with AI disabled")
	}
	cfg, e := assistant.Config(ctx, account)
	if e != nil {
		t.Fatal(e)
	}
	cfg.Enabled = true
	cfg.Model = "fixture"
	cfg.CredentialID = "00000000-0000-4000-8000-000000000301"
	if _, e = assistant.SaveConfig(ctx, account, AssistantConfigInput{Config: cfg, ExpectedUpdatedAt: cfg.UpdatedAt}); e != nil {
		t.Fatal(e)
	}
	reply := send("00000000-0000-4000-8000-000000000202", "olá", "")
	if model.calls != 1 || !strings.Contains(reply.Text, "Olá!") {
		t.Fatal("AI not selected", reply.Text)
	}
	replay := send("00000000-0000-4000-8000-000000000202", "olá", "")
	if model.calls != 1 || replay.ID != reply.ID {
		t.Fatal("paid replay")
	}
	period := "2026-11"
	sheet, e := service.CreateSheet(ctx, account, SheetInput{Period: &period, Entradas: []Line{{Amount: 100}}, Saidas: []Line{{Amount: 30}}})
	if e != nil {
		t.Fatal(e)
	}
	financial := send("00000000-0000-4000-8000-000000000203", "resumo", sheet.ID)
	if model.calls != 1 || !strings.Contains(financial.Text, "70,00") {
		t.Fatal("financial query used AI or lost totals", financial.Text)
	}
	model.fail = true
	failed := send("00000000-0000-4000-8000-000000000204", "oi", "")
	if model.calls != 2 || !strings.Contains(failed.Text, "Posso continuar") {
		t.Fatal("no local fallback", failed.Text)
	}
	send("00000000-0000-4000-8000-000000000205", "criar nota", "")
	if model.calls != 2 {
		t.Fatal("preparation called model")
	}
	var count int
	if e = pool.QueryRow(ctx, `select count(*) from finance.plus_llm_runs where account_id=$1::uuid and actor_id=$2::uuid and issuer_id is null and task_class='conversation'`, account, user).Scan(&count); e != nil || count != 2 {
		t.Fatal("ledger missing", count, e)
	}

	send("00000000-0000-4000-8000-000000000206", "cancelar", "")
	cfg, e = assistant.Config(ctx, account)
	if e != nil {
		t.Fatal(e)
	}
	cfg.Plus.Explanation.DailyRequests = 2
	if _, e = assistant.SaveConfig(ctx, account, AssistantConfigInput{Config: cfg, ExpectedUpdatedAt: cfg.UpdatedAt}); e != nil {
		t.Fatal(e)
	}
	send("00000000-0000-4000-8000-000000000207", "olá", "")
	if model.calls != 2 {
		t.Fatal("budget ignored")
	}
	cfg, e = assistant.Config(ctx, account)
	if e != nil {
		t.Fatal(e)
	}
	cfg.Plus.Explanation.DailyRequests = 20
	if _, e = assistant.SaveConfig(ctx, account, AssistantConfigInput{Config: cfg, ExpectedUpdatedAt: cfg.UpdatedAt}); e != nil {
		t.Fatal(e)
	}
	model.fail = false
	model.after = func() { allowed = false }
	if _, e = chat.Send(ctx, account, user, AccessConversationInput{RequestID: "00000000-0000-4000-8000-000000000208", Text: "olá"}); e == nil {
		t.Fatal("revocation exposed a model response")
	}
}

func TestUnifiedConversationMapsScopeAndPermissionErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{ErrSheetNotFound, 404}, {&PlusError{Code: "permission_denied", Status: 403}, 403}} {
		recorder := httptest.NewRecorder()
		conversationResult(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/finance/conversation", nil), nil, tc.err)
		if recorder.Code != tc.status {
			t.Fatal("unexpected error status", recorder.Code)
		}
	}
}
