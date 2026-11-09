package finance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

func TestAccountingSourceExplicitSharingAndOwnerIsolation(t *testing.T) {
	p := financeTestPool(t)
	ctx := context.Background()
	account := financeTestAccount(t, p)
	var user, other string
	if e := p.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); e != nil {
		t.Fatal(e)
	}
	if e := p.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&other); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = p.Exec(ctx, `delete from core.users where id=any($1::uuid[])`, []string{user, other}) })
	m := New()
	if _, e := m.Build(modules.Dependencies{Pool: p}); e != nil {
		t.Fatal(e)
	}
	profile, e := m.handle.fiscal.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: FiscalProfile{LegalName: "Synthetic profile"}})
	if e != nil {
		t.Fatal(e)
	}
	draft, e := m.handle.documents.store.Create(ctx, account, user, "synthetic-source-hash", "synthetic.txt", "synthetic", DocumentFields{Service: FiscalTemplateFields{ServiceDescription: "Shared service", ServiceCode: "synthetic-code"}})
	if e != nil {
		t.Fatal(e)
	}
	input := AccountingSourceInput{Owner: "finance", DraftID: draft.ID, ExpectedDraftUpdatedAt: draft.UpdatedAt, ExpectedProfileUpdatedAt: profile.UpdatedAt, SelectedFields: []string{"service.description"}, ConfirmShareWithIssuer: true}
	out, e := m.ReadAccountingSource(ctx, account, user, input)
	if e != nil || out.Service.Description == nil || *out.Service.Description != "Shared service" || out.Service.ServiceCode != nil {
		t.Fatalf("projection: %v", e)
	}
	b, _ := json.Marshal(out)
	if strings.Contains(string(b), "portal") || strings.Contains(string(b), "password") {
		t.Fatal("private access in projection")
	}
	if _, e = m.ReadAccountingSource(ctx, account, other, input); !errors.Is(e, ErrFiscalNotFound) {
		t.Fatal("another operator read personal source")
	}
	input.ConfirmShareWithIssuer = false
	if _, e = m.ReadAccountingSource(ctx, account, user, input); !errors.Is(e, ErrFiscalInvalid) {
		t.Fatal("implicit sharing")
	}
	input.ConfirmShareWithIssuer = true
	input.ExpectedDraftUpdatedAt = "stale"
	if _, e = m.ReadAccountingSource(ctx, account, user, input); !errors.Is(e, ErrFiscalConflict) {
		t.Fatal("stale source")
	}
}
