package finance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

func newPlusFixture(t *testing.T) (*PlusService, PlusScope) {
	t.Helper()
	p := financeTestPool(t)
	ctx := context.Background()
	account := financeTestAccount(t, p)
	var user string
	if e := p.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = p.Exec(ctx, `delete from core.users where id=$1::uuid`, user) })
	if _, e := p.Exec(ctx, `create schema if not exists calendar;create table if not exists calendar.chat_conversations(entry_surface text);create table if not exists calendar.chat_ask_requests(entry_surface text);`); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile("../../platform/database/migrations/0305_finance_assistant.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, string(b)); e != nil {
		t.Fatal(e)
	}
	owner := NewService(NewPostgresSheetStore(p), NewPostgresConfigStore(p))
	box, e := secretbox.New(make([]byte, 32))
	if e != nil {
		t.Fatal(e)
	}
	fiscal := NewFiscalService(NewPostgresFiscalStore(p), box)
	documents := newDocumentService(&DocumentStore{pool: p}, fiscal)
	assistant := &AssistantService{pool: p, finance: owner, fiscal: fiscal, documents: documents, access: &AccessConversationService{pool: p, fiscal: fiscal, documents: documents}}
	_, e = assistant.SaveConfig(ctx, account, AssistantConfigInput{Config: AssistantConfig{HistoryWindow: 5, Plus: PlusConfig{Enabled: true, ToolPolicyVersion: 1}}})
	if e != nil {
		t.Fatal(e)
	}
	svc := NewPlusService(assistant, PlusPorts{Authorize: func(context.Context, PlusScope, string) error { return nil }}, nil)
	return svc, PlusScope{AccountID: account, ActorID: user, RequestID: "00000000-0000-4000-8000-000000000091"}
}
func TestPlusConfigAndFinancialToolsWorkWithoutAI(t *testing.T) {
	s, sc := newPlusFixture(t)
	ctx := context.Background()
	period := "2026-11"
	sheet, e := s.assistant.finance.CreateSheet(ctx, sc.AccountID, SheetInput{Period: &period, Saidas: []Line{{Description: "Synthetic payment", Amount: 125.5, Effective: true, EffectiveDate: "2026-11-04"}}})
	if e != nil {
		t.Fatal(e)
	}
	out, e := s.Execute(ctx, sc, PlusInvocation{Question: "o que paguei", FinanceSheetID: sheet.ID})
	if e != nil || out.Status != "completed" {
		t.Fatalf("read without IA: %v %+v", e, out)
	}
	var data struct {
		Totals            map[string]string `json:"totals"`
		DueDatesAvailable bool              `json:"dueDatesAvailable"`
		PrecisionSource   string            `json:"precisionSource"`
	}
	if e = json.Unmarshal(out.Data, &data); e != nil {
		t.Fatal(e)
	}
	if data.Totals["effectiveOut"] != "125.50" || data.DueDatesAvailable || data.PrecisionSource != "finance_legacy" {
		t.Fatal("wrong source/totals")
	}
	out, e = s.Execute(ctx, sc, PlusInvocation{Question: "o que vence amanhã"})
	if e != nil || out.Status != "capability_unavailable" {
		t.Fatal("invented due dates")
	}
	out, e = s.Execute(ctx, sc, PlusInvocation{Question: "resumo 2026-10", FinanceSheetID: sheet.ID})
	var domain *PlusError
	if e != nil || out.Status != "missing_data" {
		t.Fatal("ignored period")
	}
	out, e = s.Execute(ctx, sc, PlusInvocation{Question: "explique imposto"})
	if e != nil || out.Status != "insufficient_evidence" {
		t.Fatal("invented interpretation")
	}
	cfg, e := s.assistant.Config(ctx, sc.AccountID)
	if e != nil || cfg.Enabled || !cfg.Plus.Enabled {
		t.Fatal("Plus depends on LLM config")
	}
	cfg.Plus.Enabled = false
	if _, e = s.assistant.SaveConfig(ctx, sc.AccountID, AssistantConfigInput{Config: cfg, ExpectedUpdatedAt: cfg.UpdatedAt}); e != nil {
		t.Fatal(e)
	}
	_, e = s.Execute(ctx, sc, PlusInvocation{Question: "saldo", FinanceSheetID: sheet.ID})
	if !errors.As(e, &domain) || domain.Code != "plus_disabled" {
		t.Fatal("ignored kill switch")
	}
}
func TestPlusRejectsUnregisteredAndForbiddenTools(t *testing.T) {
	s, sc := newPlusFixture(t)
	ctx := context.Background()
	_, e := s.Execute(ctx, sc, PlusInvocation{Question: "executar", Action: &PlusAction{ToolID: "sql.run.v1", Input: json.RawMessage(`{}`)}})
	var domain *PlusError
	if !errors.As(e, &domain) || domain.Code != "tool_not_registered" {
		t.Fatal("arbitrary tool")
	}
	calls := 0
	s.ports.Accounting = func(context.Context, PlusScope, PlusAction) (json.RawMessage, error) {
		calls++
		return json.RawMessage(`{}`), nil
	}
	s.ports.Authorize = func(_ context.Context, _ PlusScope, perm string) error {
		if perm == "accounting.documents.manage" {
			return plusError("permission_denied", 403)
		}
		return nil
	}
	sc.IssuerID = "00000000-0000-4000-8000-000000000099"
	_, e = s.Execute(ctx, sc, PlusInvocation{Question: "Preparar documento", Action: &PlusAction{ToolID: "accounting.document.prepare.v1", Input: json.RawMessage(`{}`)}})
	if !errors.As(e, &domain) || domain.Code != "permission_denied" || calls != 1 {
		t.Fatal("scope widened")
	}
	out, e := s.Execute(ctx, sc, PlusInvocation{Question: "não preparar", Action: &PlusAction{ToolID: "accounting.document.prepare.v1", Input: json.RawMessage(`{}`)}})
	if e != nil || out.Status != "missing_data" {
		t.Fatal("negated write executed")
	}
	out, e = s.Execute(ctx, sc, PlusInvocation{Question: "buscar fonte regra"})
	if e != nil || out.Status != "capability_unavailable" {
		t.Fatal("missing knowledge fabricated")
	}
}

func TestPlusPersonalFlowReusesExistingConversationWithoutCopyingHistory(t *testing.T) {
	s, sc := newPlusFixture(t)
	ctx := context.Background()
	result, e := s.Execute(ctx, sc, PlusInvocation{Question: "abrir perfil"})
	if e != nil || result.Status != "completed" {
		t.Fatalf("personal adapter: %v %+v", e, result)
	}
	var card struct {
		Action   string            `json:"action"`
		Messages []json.RawMessage `json:"messages"`
	}
	if e = json.Unmarshal(result.Data, &card); e != nil || card.Action != "profile" || len(card.Messages) != 0 {
		t.Fatalf("unexpected personal projection: %v %s", e, result.Data)
	}
	history, e := s.assistant.access.List(ctx, sc.AccountID, sc.ActorID)
	if e != nil || len(history.Messages) != 2 {
		t.Fatalf("owner history missing: %v %+v", e, history)
	}
	if _, e = s.Execute(ctx, sc, PlusInvocation{Question: "abrir perfil"}); e != nil {
		t.Fatal(e)
	}
	history, e = s.assistant.access.List(ctx, sc.AccountID, sc.ActorID)
	if e != nil || len(history.Messages) != 2 {
		t.Fatal("duplicate personal side effect")
	}
}

func TestObligationDueDatesReadWithoutLLM(t *testing.T) {
	s, sc := newPlusFixture(t)
	sc.IssuerID = "00000000-0000-4000-8000-000000000092"
	calls := 0
	s.ports.Accounting = func(_ context.Context, _ PlusScope, a PlusAction) (json.RawMessage, error) {
		if a.ToolID == "accounting.obligations.read.v1" {
			calls++
		}
		return json.RawMessage(`{"items":[],"calculationAvailable":false}`), nil
	}
	out, e := s.Execute(context.Background(), sc, PlusInvocation{Question: "quais vencimentos?", Month: "2026-11"})
	if e != nil || out.Status != "completed" || calls != 1 {
		t.Fatal(out.Status, calls, e)
	}
}
