package finance

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"testing"
)

func TestDocumentDraftPersistenceAndIsolation(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `create table if not exists core.users(id uuid primary key default gen_random_uuid())`); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../platform/database/migrations/0306_finance_document_drafts.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = pool.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	account := financeTestAccount(t, pool)
	otherAccount := financeTestAccount(t, pool)
	var user, otherUser string
	if err = pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&otherUser); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `delete from core.users where id=any($1::uuid[])`, []string{user, otherUser})
	})
	s := newDocumentService(&DocumentStore{pool: pool}, NewFiscalService(NewPostgresFiscalStore(pool), nil))
	calls := 0
	s.ocr = func(context.Context, []byte) (string, error) {
		calls++
		return "Valor R$ 100,00\n08 OUT 2026\nPix\nEstabelecimento Empresa QA\nCNPJ 13017462000163\nNSU 123\nDados do titular do cartão\nNome Pessoa QA\nCPF ***", nil
	}
	var buf bytes.Buffer
	if err = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 40, 40))); err != nil {
		t.Fatal(err)
	}
	first, err := s.Extract(ctx, account, user, "receipt.png", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if first.Fields.PayeeDocument != "13017462000163" || first.Fields.PayerDocument != "" {
		t.Fatal("party documents not persisted")
	}
	first.Fields.CustomerName = "QA Customer"
	saved, err := s.Save(ctx, account, user, first.ID, DocumentDraftInput{Fields: first.Fields, ExpectedUpdatedAt: first.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := s.Extract(ctx, account, user, "renamed.png", buf.Bytes())
	if err != nil || duplicate.ID != first.ID || duplicate.Fields.CustomerName != "QA Customer" || calls != 1 {
		t.Fatal("duplicate image overwrote review or reran OCR")
	}
	if _, err = s.Save(ctx, account, user, first.ID, DocumentDraftInput{Fields: first.Fields, ExpectedUpdatedAt: first.UpdatedAt}); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("stale draft overwrite allowed")
	}
	saved.Fields.PayerName = ""
	saved.Fields.PayeeName = ""
	saved.Fields.PayeeDocument = ""
	blank, err := s.Save(ctx, account, user, saved.ID, DocumentDraftInput{Fields: saved.Fields, ExpectedUpdatedAt: saved.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := s.ReidentifyParties(ctx, account, user, saved.ID, blank.UpdatedAt)
	if err != nil || recovered.Fields.PayerName != "Pessoa QA" || recovered.Fields.PayeeName != "Empresa QA" || recovered.Fields.CustomerName != "QA Customer" || recovered.Fields.PayeeDocument != "13017462000163" {
		t.Fatal("saved OCR recovery failed", err)
	}
	if _, err = s.ReidentifyParties(ctx, account, user, saved.ID, blank.UpdatedAt); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("recovery accepted stale version")
	}
	if _, err = s.ReidentifyParties(ctx, otherAccount, user, saved.ID, recovered.UpdatedAt); !errors.Is(err, ErrFiscalNotFound) {
		t.Fatal("recovery leaked foreign document")
	}

	recovered.Fields.PayerName = "Nome manual preservado"
	recovered.Fields.PayeeName = ""
	blank, err = s.Save(ctx, account, user, recovered.ID, DocumentDraftInput{Fields: recovered.Fields, ExpectedUpdatedAt: recovered.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := s.Extract(ctx, account, user, "renamed.png", buf.Bytes())
	if err != nil || refreshed.ID != first.ID || refreshed.Fields.PayerName != "Nome manual preservado" || refreshed.Fields.PayeeName != "Empresa QA" || calls != 1 {
		t.Fatal("reupload did not fill blanks while preserving manual corrections", err)
	}
	if _, err = s.ReidentifyParties(ctx, account, user, blank.ID, blank.UpdatedAt); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("automatic recovery did not revise the document")
	}

	for _, scope := range [][2]string{{account, otherUser}, {otherAccount, user}} {
		docs, e := s.List(ctx, scope[0], scope[1])
		if e != nil || len(docs) != 0 {
			t.Fatal("list leaks")
		}
		if _, e = s.Save(ctx, scope[0], scope[1], saved.ID, DocumentDraftInput{Fields: saved.Fields, ExpectedUpdatedAt: saved.UpdatedAt}); !errors.Is(e, ErrFiscalNotFound) {
			t.Fatal("foreign draft update")
		}
	}
	// Reupload an old unreadable image: replace OCR, fill blanks, retain manual fields.
	var small bytes.Buffer
	if err = png.Encode(&small, image.NewRGBA(image.Rect(0, 0, 41, 41))); err != nil {
		t.Fatal(err)
	}
	s.ocr = func(context.Context, []byte) (string, error) { return "Texto incompleto antigo", nil }
	unreadable, err := s.Extract(ctx, account, user, "small.png", small.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	amount := 12.0
	unreadable.Fields.Amount = &amount
	unreadable.Fields.CustomerName = "Escolha manual"
	unreadable, err = s.Save(ctx, account, user, unreadable.ID, DocumentDraftInput{Fields: unreadable.Fields, ExpectedUpdatedAt: unreadable.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	s.ocr = func(context.Context, []byte) (string, error) {
		return "Comprovante Pix\n15 OUT 2026\nValor AS 57,58\nDestino\nNome Empresa QA\nCNPJ 15754475000140\nOrigem\nNome Pessoa QA\nCPF ***", nil
	}
	reread, err := s.Extract(ctx, account, user, "small.png", small.Bytes())
	if err != nil || reread.ID != unreadable.ID || reread.ExtractedText == unreadable.ExtractedText || reread.Fields.Amount == nil || *reread.Fields.Amount != 12 || reread.Fields.PaymentDate != "2026-10-15" || reread.Fields.CustomerName != "Escolha manual" || reread.Fields.PayeeDocument != "15754475000140" {
		t.Fatal("incomplete OCR was not refreshed safely", err)
	}
}
