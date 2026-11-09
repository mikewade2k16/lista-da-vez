package finance

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

// CT-15 pacote 5: the chat picks its own sheet/period inside the chat account,
// independent from the page, and never reads a sheet from another account.
func TestChatChoosesSheetIndependentlyOfPage(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	raw, err := os.ReadFile("../../platform/database/migrations/0305_finance_assistant.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, strings.Split(string(raw), "alter table")[0]); err != nil {
		t.Fatal(err)
	}
	accountA, accountB := financeTestAccount(t, pool), financeTestAccount(t, pool)
	var user string
	if err = pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `delete from core.users where id=$1::uuid`, user) })
	fiscal := NewFiscalService(NewPostgresFiscalStore(pool), nil)
	service := NewService(NewPostgresSheetStore(pool), NewPostgresConfigStore(pool))
	assistant := &AssistantService{pool: pool, fiscal: fiscal, finance: service}
	chat := &AccessConversationService{pool: pool, fiscal: fiscal, documents: newDocumentService(&DocumentStore{pool: pool}, fiscal)}
	assistant.access = chat
	plus := NewPlusService(assistant, PlusPorts{Authorize: func(_ context.Context, sc PlusScope, permission string) error {
		if sc.ActorID != user || (sc.AccountID != accountA && sc.AccountID != accountB) || permission != "finance.sheets.view" {
			return ErrFiscalNotFound
		}
		return nil
	}}, nil)
	chat.orchestrator = &ConversationOrchestrator{plus: plus}
	create := func(account, title, period, outDescription string, in, out float64) string {
		t.Helper()
		s, e := service.CreateSheet(ctx, account, SheetInput{Title: &title, Period: &period, Entradas: []Line{{Description: "Entrada " + title, Amount: in}}, Saidas: []Line{{Description: outDescription, Amount: out}}})
		if e != nil {
			t.Fatal(e)
		}
		return s.ID
	}
	create(accountA, "Casa outubro", "2026-10", "Aluguel outubro", 1000, 400)
	create(accountA, "Casa novembro", "2026-11", "Aluguel novembro", 2000, 500)
	novB := create(accountA, "Estúdio novembro", "2026-11", "Equipamento estúdio", 3000, 700)
	foreign := create(accountB, "Conta B", "2026-11", "Despesa da conta B", 9000, 1)
	n := 600
	send := func(account, text, pageSheet string) AccessMessage {
		t.Helper()
		n++
		out, e := chat.Send(ctx, account, user, AccessConversationInput{RequestID: fmt.Sprintf("00000000-0000-4000-8000-%012d", n), Text: text, FinanceSheetID: pageSheet})
		if e != nil {
			t.Fatal(text, e)
		}
		return out.Messages[len(out.Messages)-1]
	}
	oct := send(accountA, "resumo de outubro de 2026", "")
	if !strings.Contains(oct.Text, "Planilha 2026-10") || !strings.HasPrefix(oct.Action, "sheet:") {
		t.Fatal("period in text not resolved inside the chat account", oct)
	}
	follow := send(accountA, "listar saídas", "")
	if !strings.Contains(follow.Text, "Aluguel outubro") || strings.Contains(follow.Text, "novembro") {
		t.Fatal("follow-up lost the chat sheet", follow.Text)
	}
	choice := send(accountA, "resumo 2026-11", "")
	if choice.Action != "sheetpick:2026-11" || !strings.Contains(choice.Text, "Casa novembro") || !strings.Contains(choice.Text, "Estúdio novembro") {
		t.Fatal("two sheets of the period not offered", choice)
	}
	var picked AccessMessage
	for i, line := range strings.Split(choice.Text, "\n") {
		if strings.Contains(line, "Estúdio novembro") {
			picked = send(accountA, fmt.Sprint(i), "")
		}
	}
	if picked.Action != "sheet:"+novB || !strings.Contains(picked.Text, "Equipamento estúdio") {
		t.Fatal("numbered choice not applied", picked)
	}
	// The page sends a sheet from another account: never read, chat sheet kept.
	page := send(accountA, "saldo", foreign)
	if strings.Contains(page.Text, "9000") || strings.Contains(page.Text, "conta B") || page.Action != "sheet:"+novB {
		t.Fatal("foreign page sheet read from chat", page)
	}
	missing := send(accountA, "resumo 2026-09", "")
	if !strings.Contains(missing.Text, "Não encontrei planilha de 2026-09") {
		t.Fatal(missing.Text)
	}
	other := send(accountB, "saldo", "")
	if !strings.Contains(other.Text, "8999,00") || other.Action != "sheet:"+foreign {
		t.Fatal("chat in account B did not use its only sheet", other)
	}
}
