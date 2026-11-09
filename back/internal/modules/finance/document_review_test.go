package finance

import (
	"context"
	"errors"
	"os"
	"testing"
)

func reviewTestProfile() FiscalProfile {
	return FiscalProfile{LegalName: "QA Emitente", Document: "12345678000195", IssuerType: "mei", TaxRegime: "Informado para teste", Municipality: "QA Cidade", State: "AL", Street: "Rua QA", Number: "1", District: "QA", PostalCode: "57000000", PortalURL: "https://www.nfse.gov.br/EmissorNacional/Login", PortalLoginMethod: "govbr"}
}
func reviewTestFields() DocumentFields {
	amount := 100.0
	return DocumentFields{DocumentType: "comprovante", Amount: &amount, CustomerName: "QA Tomador", CustomerDocument: "12345678901", CompetenceDate: "2026-11-02", Service: FiscalTemplateFields{ServiceDescription: "QA serviço"}}
}
func TestReviewBindsProfileAndDraftVersions(t *testing.T) {
	d := DocumentDraft{ID: "draft", UpdatedAt: "draft-v1", Fields: reviewTestFields()}
	p := FiscalProfileView{Profile: reviewTestProfile(), UpdatedAt: "profile-v1"}
	first := buildDocumentReview(d, p)
	if !first.Ready || first.EmissionAvailable {
		t.Fatal("unexpected readiness or execution permission")
	}
	p.UpdatedAt = "profile-v2"
	if buildDocumentReview(d, p).Hash == first.Hash {
		t.Fatal("profile version not bound")
	}
	p.UpdatedAt = "profile-v1"
	d.UpdatedAt = "draft-v2"
	if buildDocumentReview(d, p).Hash == first.Hash {
		t.Fatal("draft version not bound")
	}
	d.Fields.CustomerDocument = "invalid"
	if buildDocumentReview(d, p).Ready {
		t.Fatal("invalid recipient document accepted")
	}
	d.Fields = DocumentFields{}
	if len(buildDocumentReview(d, FiscalProfileView{}).MissingFields) < 10 {
		t.Fatal("missing fields hidden")
	}
}
func TestReviewPersistenceConflictsAndScope(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `create table if not exists core.users(id uuid primary key default gen_random_uuid())`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"0304_finance_user_fiscal_profile.sql", "0306_finance_document_drafts.sql", "0307_finance_document_reviews.sql"} {
		raw, err := os.ReadFile("../../platform/database/migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err = pool.Exec(ctx, string(raw)); err != nil {
				t.Fatal(err)
			}
		}
	}
	account := financeTestAccount(t, pool)
	otherAccount := financeTestAccount(t, pool)
	var user, otherUser string
	for _, target := range []*string{&user, &otherUser} {
		if err := pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `delete from core.users where id=any($1::uuid[])`, []string{user, otherUser})
	})
	fiscal := NewFiscalService(NewPostgresFiscalStore(pool), nil)
	store := &DocumentStore{pool: pool}
	svc := newDocumentService(store, fiscal)
	d, err := store.Create(ctx, account, user, "test-hash", "QA receipt.txt", "Texto QA", reviewTestFields())
	if err != nil {
		t.Fatal(err)
	}
	review, err := svc.Review(ctx, account, user, d.ID)
	if err != nil || review.Ready {
		t.Fatal("incomplete issuer ready")
	}
	input := func(r DocumentReview) DocumentReviewInput {
		return DocumentReviewInput{ReviewHash: r.Hash, ExpectedUpdatedAt: r.DraftVersion, ExpectedProfileUpdatedAt: r.ProfileVersion, Confirmed: true}
	}
	if _, err = svc.ApproveReview(ctx, account, user, d.ID, input(review)); !errors.Is(err, ErrFiscalInvalid) {
		t.Fatal("incomplete approval allowed")
	}
	p, err := fiscal.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: reviewTestProfile()})
	if err != nil {
		t.Fatal(err)
	}
	review, err = svc.Review(ctx, account, user, d.ID)
	if err != nil || !review.Ready {
		t.Fatal("ready review failed", err)
	}
	approved, err := svc.ApproveReview(ctx, account, user, d.ID, input(review))
	if err != nil || approved.ApprovalID == "" {
		t.Fatal("approval failed", err)
	}
	duplicate, err := svc.ApproveReview(ctx, account, user, d.ID, input(review))
	if err != nil || duplicate.ApprovalID != approved.ApprovalID {
		t.Fatal("duplicate approval created")
	}
	revoked, err := svc.RevokeReview(ctx, account, user, d.ID, input(approved))
	if err != nil || revoked.ApprovalID != "" {
		t.Fatal("revocation failed")
	}
	approved, err = svc.ApproveReview(ctx, account, user, d.ID, input(review))
	if err != nil {
		t.Fatal(err)
	}
	p.Profile.LegalName = "QA Emitente alterado"
	_, err = fiscal.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: p.Profile, ExpectedUpdatedAt: p.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ApproveReview(ctx, account, user, d.ID, input(review)); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("stale profile approval allowed")
	}
	fresh, err := svc.Review(ctx, account, user, d.ID)
	if err != nil || fresh.ApprovalID != "" {
		t.Fatal("old approval remained effective")
	}
	if _, _, err = store.ApproveReview(ctx, account, user, d.ID, review); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("store accepted stale profile snapshot")
	}
	if _, err = svc.ApproveReview(ctx, account, user, d.ID, input(fresh)); err != nil {
		t.Fatal(err)
	}
	d.Fields.Service.Notes = "QA edição posterior"
	_, err = svc.Save(ctx, account, user, d.ID, DocumentDraftInput{Fields: d.Fields, ExpectedUpdatedAt: d.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.ApproveReview(ctx, account, user, d.ID, fresh); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("store accepted stale draft")
	}
	fresh, err = svc.Review(ctx, account, user, d.ID)
	if err != nil || fresh.ApprovalID != "" {
		t.Fatal("edited draft inherited approval")
	}
	for _, scope := range [][2]string{{account, otherUser}, {otherAccount, user}} {
		if _, err = svc.Review(ctx, scope[0], scope[1], d.ID); !errors.Is(err, ErrFiscalNotFound) {
			t.Fatal("foreign review exposed")
		}
		if _, err = svc.ApproveReview(ctx, scope[0], scope[1], d.ID, input(fresh)); !errors.Is(err, ErrFiscalNotFound) {
			t.Fatal("foreign approval allowed")
		}
	}
	var count int
	if err = pool.QueryRow(ctx, `select count(*) from finance.document_reviews where account_id=$1::uuid and user_id=$2::uuid and draft_id=$3::uuid`, account, user, d.ID).Scan(&count); err != nil || count != 3 {
		t.Fatal("audit history missing")
	}
}
