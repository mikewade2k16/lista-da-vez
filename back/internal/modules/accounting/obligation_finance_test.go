package accounting

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
	"sync"
	"testing"
)

type obligationFinanceTestPort struct {
	service *finance.Service
	fail    bool
}

func (p *obligationFinanceTestPort) PrepareObligationTx(ctx context.Context, tx pgx.Tx, sc Scope, in ObligationFinanceInput) (ObligationFinanceLink, error) {
	v, e := p.service.PrepareObligationTx(ctx, tx, finance.ObligationEntryInput{AccountID: sc.AccountID, ActorID: sc.ActorID, IssuerID: sc.IssuerID, ObligationID: in.ObligationID, Code: in.Code, Period: in.Period, Amount: in.Amount, Version: in.Version, SheetID: in.SheetID, ExpectedUpdatedAt: in.ExpectedUpdatedAt, ExistingLineID: in.ExistingLineID})
	if e != nil {
		return ObligationFinanceLink{}, e
	}
	if p.fail {
		return ObligationFinanceLink{}, errors.New("synthetic rollback after Finance write")
	}
	return ObligationFinanceLink{SheetID: v.SheetID, LineID: v.LineID, Amount: v.Amount, UpdatedAt: v.UpdatedAt}, nil
}
func reviewedSyntheticObligation(t *testing.T, s *Service, sc Scope) Obligation {
	t.Helper()
	syntheticObligationRule(t, s, sc)
	s.WithObligationQualification(func(context.Context, Scope, Profile, int64, ObligationRule, string) error { return nil })
	o := saveTestObligation(t, s, sc, "finance-source-calc", ObligationInput{Code: "mei.das", Period: "2026-11", Base: ptr("100.00")})
	b, e := s.ObligationCommand(context.Background(), sc, o.ID, "review", "finance-source-review", ObligationAction{ExpectedVersion: o.Version, Confirmed: true})
	return obligationValue(t, b, e)
}
func TestObligationFinanceAtomicReplayAndClosure(t *testing.T) {
	s, sc, p := obligationFixture(t)
	ctx := context.Background()
	o := reviewedSyntheticObligation(t, s, sc)
	f := finance.NewService(finance.NewPostgresSheetStore(p), finance.NewPostgresConfigStore(p))
	sheet, e := f.CreateSheet(ctx, sc.AccountID, finance.SheetInput{Period: ptr("2026-11")})
	if e != nil {
		t.Fatal(e)
	}
	port := &obligationFinanceTestPort{service: f, fail: true}
	s.WithObligationFinance(port)
	in := ObligationAction{ExpectedVersion: o.Version, Confirmed: true, SheetID: sheet.ID, ExpectedSheetUpdatedAt: sheet.UpdatedAt}
	if _, e = s.ObligationCommand(ctx, sc, o.ID, "prepare-finance", "finance-rollback-key", in); e == nil {
		t.Fatal("rollback not injected")
	}
	var n int
	if e = p.QueryRow(ctx, `select count(*) from finance.obligation_links where account_id=$1::uuid`, sc.AccountID).Scan(&n); e != nil || n != 0 {
		t.Fatal("Finance escaped rollback", n, e)
	}
	port.fail = false
	raw, e := s.ObligationCommand(ctx, sc, o.ID, "prepare-finance", "finance-prepare-key", in)
	if e != nil {
		t.Fatal(e)
	}
	var link ObligationFinanceLink
	if e = json.Unmarshal(raw, &link); e != nil {
		t.Fatal(e)
	}
	raw, e = s.ObligationCommand(ctx, sc, o.ID, "prepare-finance", "finance-reprocess-key", in)
	if e != nil {
		t.Fatal(e)
	}
	var again ObligationFinanceLink
	if e = json.Unmarshal(raw, &again); e != nil || again.LineID != link.LineID || again.PaymentConfirmed {
		t.Fatal("duplicate or fake payment", e)
	}
	var concurrent sync.WaitGroup
	results := make(chan error, 2)
	for _, key := range []string{"finance-concurrent-one", "finance-concurrent-two"} {
		concurrent.Go(func() { _, err := s.ObligationCommand(ctx, sc, o.ID, "prepare-finance", key, in); results <- err })
	}
	concurrent.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	detail, e := f.GetSheet(ctx, sc.AccountID, sheet.ID)
	if e != nil || len(detail.Saidas) != 1 || detail.Saidas[0].Effective {
		t.Fatal(detail, e)
	}
	_, e = f.ChangeStatus(ctx, sc.AccountID, sheet.ID, "fechada", detail.UpdatedAt)
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.ObligationCommand(ctx, sc, o.ID, "prepare-finance", "finance-closed-key", in)
	if !errors.Is(e, finance.ErrSheetClosed) {
		t.Fatal("closed sheet mutated", e)
	}
}
func TestObligationFinanceManualAssociationAndStaleSource(t *testing.T) {
	s, sc, p := obligationFixture(t)
	ctx := context.Background()
	o := reviewedSyntheticObligation(t, s, sc)
	f := finance.NewService(finance.NewPostgresSheetStore(p), finance.NewPostgresConfigStore(p))
	sheet, e := f.CreateSheet(ctx, sc.AccountID, finance.SheetInput{Period: ptr("2026-11"), Saidas: []finance.Line{{Description: "Manually reported payment", Amount: 1.34, Effective: true, EffectiveDate: "2026-11-05"}}})
	if e != nil {
		t.Fatal(e)
	}
	s.WithObligationFinance(&obligationFinanceTestPort{service: f})
	in := ObligationAction{ExpectedVersion: o.Version, Confirmed: true, SheetID: sheet.ID, ExpectedSheetUpdatedAt: sheet.UpdatedAt}
	_, e = s.ObligationCommand(ctx, sc, o.ID, "prepare-finance", "manual-selection-required", in)
	if !errors.Is(e, finance.ErrSheetConflict) {
		t.Fatal("manual line duplicated", e)
	}
	in.ExistingLineID = sheet.Saidas[0].ID
	raw, e := s.ObligationCommand(ctx, sc, o.ID, "prepare-finance", "manual-selection-key", in)
	if e != nil {
		t.Fatal(e)
	}
	var link ObligationFinanceLink
	if e = json.Unmarshal(raw, &link); e != nil || link.PaymentConfirmed {
		t.Fatal(e)
	}
	got, e := s.ReadObligations(ctx, sc, "", o.ID, 1, 20)
	if e != nil || got.(Obligation).State != "reviewed" {
		t.Fatal("effective was treated as settlement", e)
	}
	if _, e = s.UpdateIssuer(ctx, sc, "updated-profile-for-obligation", Command{ExpectedVersion: 1, Profile: &Profile{LegalName: "Synthetic revised", TaxRegime: ptr("different")}}); e != nil {
		t.Fatal(e)
	}
	_, e = s.ObligationCommand(ctx, sc, o.ID, "prepare-finance", "stale-obligation-finance", in)
	expectCode(t, e, "obligation_stale_or_unqualified")
}
