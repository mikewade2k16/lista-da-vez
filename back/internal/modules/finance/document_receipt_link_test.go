package finance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

type receiptFixture struct {
	ctx        context.Context
	account    string
	user       string
	other      string
	service    *Service
	documents  *DocumentService
	chat       *AccessConversationService
	canManage  bool
	sheetID    string
	sheetStamp string
}

func newReceiptFixture(t *testing.T) *receiptFixture {
	t.Helper()
	pool := financeTestPool(t)
	f := &receiptFixture{ctx: context.Background(), canManage: true}
	raw, err := os.ReadFile("../../platform/database/migrations/0305_finance_assistant.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(f.ctx, strings.Split(string(raw), "alter table")[0]); err != nil {
		t.Fatal(err)
	}
	f.account = financeTestAccount(t, pool)
	for _, target := range []*string{&f.user, &f.other} {
		if err = pool.QueryRow(f.ctx, `insert into core.users default values returning id::text`).Scan(target); err != nil {
			t.Fatal(err)
		}
		id := *target
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `delete from core.users where id=$1::uuid`, id) })
	}
	fiscal := NewFiscalService(NewPostgresFiscalStore(pool), nil)
	f.service = NewService(NewPostgresSheetStore(pool), NewPostgresConfigStore(pool))
	f.documents = newDocumentService(&DocumentStore{pool: pool}, fiscal)
	assistant := &AssistantService{pool: pool, fiscal: fiscal, finance: f.service}
	f.chat = &AccessConversationService{pool: pool, fiscal: fiscal, documents: f.documents}
	assistant.access = f.chat
	plus := NewPlusService(assistant, PlusPorts{Authorize: func(_ context.Context, sc PlusScope, permission string) error {
		if sc.AccountID != f.account || (permission == "finance.sheets.manage" && !f.canManage) {
			return ErrFiscalNotFound
		}
		return nil
	}}, nil)
	f.chat.orchestrator = &ConversationOrchestrator{plus: plus}
	title, period := "Casa novembro", "2026-11"
	sheet, err := f.service.CreateSheet(f.ctx, f.account, SheetInput{Title: &title, Period: &period, Saidas: []Line{{Description: "Uber antigo", Amount: 15}}})
	if err != nil {
		t.Fatal(err)
	}
	f.sheetID, f.sheetStamp = sheet.ID, sheet.UpdatedAt
	return f
}

func (f *receiptFixture) receipt(t *testing.T, user, hash string, amount float64) DocumentDraft {
	t.Helper()
	d, err := f.documents.store.Create(f.ctx, f.account, user, hash, "uber-"+hash+".pdf", "Recibo Uber "+hash, DocumentFields{DocumentType: "documento", Amount: &amount, PaymentDate: "2026-11-05", PayeeName: "Uber"})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func (f *receiptFixture) stamp(t *testing.T) string {
	t.Helper()
	s, err := f.service.GetSheet(f.ctx, f.account, f.sheetID)
	if err != nil {
		t.Fatal(err)
	}
	return s.UpdatedAt
}

func (f *receiptFixture) saidas(t *testing.T) int {
	t.Helper()
	s, err := f.service.GetSheet(f.ctx, f.account, f.sheetID)
	if err != nil {
		t.Fatal(err)
	}
	return len(s.Saidas)
}

// CT-15 pacote 6: confirmation, CAS, replay, single link, revalidation, isolation and concurrency.
func TestReceiptLinkLifecycle(t *testing.T) {
	f := newReceiptFixture(t)
	d := f.receipt(t, f.user, "a1", 32.5)
	in := ReceiptLinkInput{SheetID: f.sheetID, Kind: "saida", Scope: "pessoal", ExpectedDocumentUpdatedAt: d.UpdatedAt, ExpectedSheetUpdatedAt: f.sheetStamp}
	if _, err := f.documents.LinkReceipt(f.ctx, f.account, f.user, d.ID, in); !errors.Is(err, ErrFiscalInvalid) {
		t.Fatal("unconfirmed link accepted", err)
	}
	in.Confirmed = true
	stale := in
	stale.ExpectedSheetUpdatedAt = "2020-01-01T00:00:00Z"
	if _, err := f.documents.LinkReceipt(f.ctx, f.account, f.user, d.ID, stale); !errors.Is(err, ErrSheetConflict) {
		t.Fatal("stale sheet accepted", err)
	}
	link, err := f.documents.LinkReceipt(f.ctx, f.account, f.user, d.ID, in)
	if err != nil || !link.CreatedLine || link.Amount != "32.50" || f.saidas(t) != 2 {
		t.Fatal("link", link, err)
	}
	sheet, _ := f.service.GetSheet(f.ctx, f.account, f.sheetID)
	created := sheet.Saidas[1]
	if !created.Effective || created.EffectiveDate != "2026-11-05" || created.Description != "Uber" {
		t.Fatal("entry fields", created)
	}
	replay, err := f.documents.LinkReceipt(f.ctx, f.account, f.user, d.ID, in)
	if err != nil || replay.LineID != link.LineID || f.saidas(t) != 2 {
		t.Fatal("replay created another entry", replay, err)
	}
	title, period := "Outra", "2026-11"
	other, _ := f.service.CreateSheet(f.ctx, f.account, SheetInput{Title: &title, Period: &period})
	moved := in
	moved.SheetID, moved.ExpectedSheetUpdatedAt = other.ID, other.UpdatedAt
	if _, err = f.documents.LinkReceipt(f.ctx, f.account, f.user, d.ID, moved); !errors.Is(err, ErrReceiptAlreadyLinked) {
		t.Fatal("receipt linked twice", err)
	}
	if got, e := f.documents.GetReceiptLink(f.ctx, f.account, f.user, d.ID); e != nil || got.Status != "linked" {
		t.Fatal(got, e)
	}
	if _, err = f.documents.store.pool.Exec(f.ctx, `update finance.lines set amount=40 where id=$1::uuid`, link.LineID); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.documents.GetReceiptLink(f.ctx, f.account, f.user, d.ID); got.Status != "amount_changed" {
		t.Fatal("edited entry not reported", got.Status)
	}
	if _, err = f.documents.store.pool.Exec(f.ctx, `delete from finance.lines where id=$1::uuid`, link.LineID); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.documents.GetReceiptLink(f.ctx, f.account, f.user, d.ID); got.Status != "line_removed" {
		t.Fatal("removed entry not reported", got.Status)
	}
	// Another user of the same account cannot read or use this receipt.
	if _, e := f.documents.GetReceiptLink(f.ctx, f.account, f.other, d.ID); !errors.Is(e, ErrFiscalNotFound) {
		t.Fatal("foreign link visible", e)
	}
	if _, e := f.documents.LinkReceipt(f.ctx, f.account, f.other, d.ID, in); !errors.Is(e, ErrFiscalNotFound) {
		t.Fatal("foreign receipt linked", e)
	}
	if err = f.documents.UnlinkReceipt(f.ctx, f.account, f.user, d.ID); err != nil {
		t.Fatal(err)
	}
	// Two receipts confirmed against the same sheet version: exactly one wins (CAS).
	stamp := f.stamp(t)
	first, second := f.receipt(t, f.user, "c1", 10), f.receipt(t, f.user, "c2", 11)
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i, doc := range []DocumentDraft{first, second} {
		wg.Add(1)
		go func(i int, doc DocumentDraft) {
			defer wg.Done()
			_, results[i] = f.documents.LinkReceipt(f.ctx, f.account, f.user, doc.ID, ReceiptLinkInput{SheetID: f.sheetID, Kind: "saida", Scope: "pessoal", Confirmed: true, ExpectedDocumentUpdatedAt: doc.UpdatedAt, ExpectedSheetUpdatedAt: stamp})
		}(i, doc)
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, e := range results {
		switch {
		case e == nil:
			ok++
		case errors.Is(e, ErrSheetConflict):
			conflict++
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal("concurrent confirmations", results)
	}
}

// Chat: proposal, scope change, explicit "sim", existing entry reuse, permission and note drafts.
func TestReceiptChatConfirmation(t *testing.T) {
	f := newReceiptFixture(t)
	d := f.receipt(t, f.user, "chat", 15)
	n := 800
	send := func(text, draft string) AccessMessage {
		t.Helper()
		n++
		out, err := f.chat.Send(f.ctx, f.account, f.user, AccessConversationInput{RequestID: fmt.Sprintf("00000000-0000-4000-8000-%012d", n), Text: text, DraftID: draft})
		if err != nil {
			t.Fatal(text, err)
		}
		return out.Messages[len(out.Messages)-1]
	}
	proposal := send("lança esse comprovante na planilha", d.ID)
	if !strings.HasPrefix(proposal.Action, "receipt:") || !strings.Contains(proposal.Text, "lançamento existente “Uber antigo”") || !strings.Contains(proposal.Text, "pessoal") {
		t.Fatal("proposal", proposal)
	}
	if f.saidas(t) != 1 {
		t.Fatal("wrote before confirmation")
	}
	business := send("é despesa da empresa", "")
	if !strings.Contains(business.Text, "empresarial") || !strings.HasPrefix(business.Action, "receipt:") {
		t.Fatal("scope change", business)
	}
	f.canManage = false
	denied := send("sim", "")
	if !strings.Contains(denied.Text, "não tem permissão") {
		t.Fatal("manage permission not enforced", denied)
	}
	f.canManage = true
	send("lançar comprovante", "")
	send("empresarial", "")
	done := send("sim", "")
	if !strings.Contains(done.Text, "Vinculei o comprovante") || f.saidas(t) != 1 {
		t.Fatal("confirmation", done)
	}
	link, err := f.documents.GetReceiptLink(f.ctx, f.account, f.user, d.ID)
	if err != nil || link.Scope != "empresarial" || link.CreatedLine {
		t.Fatal("link scope", link, err)
	}
	again := send("lançar comprovante", "")
	if !strings.Contains(again.Text, "já está lançado") {
		t.Fatal("duplicate proposal", again)
	}
	note := send("criar nota", d.ID)
	if note.DraftID == d.ID {
		t.Fatal("selected expense receipt reused as note draft")
	}
	notReceipt := send("lançar comprovante", "")
	if !strings.Contains(notReceipt.Text, "não um comprovante") {
		t.Fatal("note draft treated as receipt", notReceipt)
	}
}
