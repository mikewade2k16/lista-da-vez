package finance

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDocumentPreparationUsesSavedDataAndExplicitConversationFields(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	account, other := financeTestAccount(t, pool), financeTestAccount(t, pool)
	var user string
	if err := pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `delete from core.users where id=$1::uuid`, user) })
	fiscal := NewFiscalService(NewPostgresFiscalStore(pool), nil)
	p := FiscalProfile{LegalName: "Issuer QA", Document: "12345678000195", IssuerType: "mei", TaxRegime: "Informado QA", Municipality: "Cidade QA", State: "AL", Street: "Rua QA", Number: "1", District: "Centro", PostalCode: "57000000", PortalURL: nationalPortalLogin, PortalLoginMethod: "portal"}
	if _, err := fiscal.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: p}); err != nil {
		t.Fatal(err)
	}
	template, err := fiscal.SaveTemplate(ctx, account, user, "", FiscalTemplateInput{Name: "Padrão QA", IsDefault: true, Fields: FiscalTemplateFields{ServiceDescription: "Serviço do template", ServiceCode: "QA01"}})
	if err != nil {
		t.Fatal(err)
	}
	store := &DocumentStore{pool: pool}
	svc := newDocumentService(store, fiscal)
	amount := 57.58
	d, err := store.Create(ctx, account, user, strings.Repeat("a", 64), "QA document", "Comprovante", DocumentFields{DocumentType: "comprovante", Amount: &amount, PaymentDate: "2026-10-15", PayerName: "Pagador QA", PayeeName: "Recebedor QA"})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.Prepare(ctx, account, user, d.ID, d.UpdatedAt)
	if err != nil || prepared.Draft.Fields.TemplateID != template.ID || prepared.Draft.Fields.Service.ServiceDescription != "Serviço do template" || prepared.Draft.Fields.CustomerName != "" || prepared.Draft.Fields.CompetenceDate != "" || prepared.Review.Ready || prepared.Review.EmissionAvailable || prepared.Session.Status != "disconnected" {
		t.Fatal("preparation inferred data or lost template", err)
	}
	if _, err = svc.Prepare(ctx, account, user, d.ID, d.UpdatedAt); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("stale preparation accepted")
	}
	if _, err = svc.Prepare(ctx, other, user, d.ID, prepared.Draft.UpdatedAt); !errors.Is(err, ErrFiscalNotFound) {
		t.Fatal("preparation leaked")
	}
	updated, changed, err := svc.UpdateFromConversation(ctx, account, user, d.ID, "Tomador: Cliente QA\nCPF/CNPJ do tomador: 98765432000198\nCompetência: 05/11/2026\nServiço: Edição de vídeo\nValor: R$ 82,50")
	if err != nil || !changed || updated.Fields.CustomerName != "Cliente QA" || updated.Fields.CompetenceDate != "2026-11-05" || updated.Fields.Amount == nil || *updated.Fields.Amount != 82.5 {
		t.Fatal("explicit data not persisted", err)
	}
	ready, err := svc.Prepare(ctx, account, user, d.ID, updated.UpdatedAt)
	if err != nil || !ready.Review.Ready || ready.Review.ApprovalID != "" || ready.Review.EmissionAvailable || ready.Draft.Fields.Service.ServiceDescription != "Edição de vídeo" {
		t.Fatal("review not ready or manual data replaced", err)
	}
	chat := &AccessConversationService{pool: pool, fiscal: fiscal, documents: svc}
	messages, err := chat.Send(ctx, account, user, AccessConversationInput{RequestID: "00000000-0000-4000-8000-000000000011", Text: "Preparar nota", DraftID: d.ID})
	if err != nil || len(messages.Messages) != 2 || messages.Messages[1].Action != "" || messages.Messages[1].DraftID != d.ID {
		t.Fatal("preparation conversation failed", err)
	}
	messages, err = chat.Send(ctx, account, user, AccessConversationInput{RequestID: "00000000-0000-4000-8000-000000000012", Text: "Serviço: Revisão solicitada"})
	if err != nil || messages.Messages[len(messages.Messages)-1].DraftID != d.ID {
		t.Fatal("selected draft not retained", err)
	}
	if _, err = chat.Send(ctx, other, user, AccessConversationInput{RequestID: "00000000-0000-4000-8000-000000000013", Text: "Preparar nota", DraftID: d.ID}); !errors.Is(err, ErrFiscalNotFound) {
		t.Fatal("foreign draft accepted")
	}
}
