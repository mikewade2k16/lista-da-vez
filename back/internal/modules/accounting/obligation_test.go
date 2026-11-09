package accounting

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"sync"
	"testing"
)

func obligationFixture(t *testing.T) (*Service, Scope, *pgxpool.Pool) {
	t.Helper()
	s, sc, p := fixture(t)
	if _, e := p.Exec(context.Background(), `alter table core.accounts add column if not exists name text not null default ''`); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"0187_finance_module.sql", "0314_accounting_obligations.sql", "0321_accounting_obligation_rule_lifecycle.sql"} {
		b, e := os.ReadFile("../../platform/database/migrations/" + name)
		if e != nil {
			t.Fatal(e)
		}
		for range 2 {
			if _, e = p.Exec(context.Background(), string(b)); e != nil {
				t.Fatal(e)
			}
		}
	}
	return s, sc, p
}
func obligationValue(t *testing.T, b []byte, e error) Obligation {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
	var o Obligation
	if e = json.Unmarshal(b, &o); e != nil {
		t.Fatal(e)
	}
	return o
}
func saveTestObligation(t *testing.T, s *Service, sc Scope, key string, in ObligationInput) Obligation {
	t.Helper()
	b, e := s.SaveObligation(context.Background(), sc, key, in)
	return obligationValue(t, b, e)
}
func syntheticObligationRule(t *testing.T, s *Service, sc Scope) {
	t.Helper()
	raw := `{"formula":"base_percent_fixed_v1","rate":"1.333333","fixed":"0.01","rounding":"half_up_2_v1","dueDates":{"2026-11":"2026-12-21"}}`
	_, e := s.store.pool.Exec(context.Background(), `insert into accounting.obligation_rules(account_id,issuer_id,code,version,regime,jurisdiction,valid_from,valid_to,parameters,source_ref,technical_review_ref,reviewed_by)values($1::uuid,$2::uuid,'mei.das',1,'synthetic','synthetic','2026-01-01','2026-12-31',$3,'fixture://synthetic-only','fixture://review',$4::uuid)`, sc.AccountID, sc.IssuerID, raw, sc.ActorID)
	if e != nil {
		t.Fatal(e)
	}
	// Synthetic activation straight in the fixture; the service path (four eyes) is in obligation_rules_test.go.
	if _, e = s.store.pool.Exec(context.Background(), `insert into accounting.obligation_rule_events(account_id,issuer_id,rule_id,action,actor_id,reason) select account_id,issuer_id,id,'activate',reviewed_by,'fixture activation' from accounting.obligation_rules where account_id=$1::uuid and issuer_id=$2::uuid and code='mei.das' and version=1`, sc.AccountID, sc.IssuerID); e != nil {
		t.Fatal(e)
	}
}
func TestObligationExactMemoryAndRounding(t *testing.T) {
	for _, c := range []struct{ base, rate, fixed, want string }{{"100.00", "1.333333", "0.01", "1.34"}, {"0.50", "1", "0.00", "0.01"}, {"1000.00", "2.555555", "0.00", "25.56"}, {"0.00", "0", "0.00", "0.00"}} {
		p := ObligationParameters{Formula: "base_percent_fixed_v1", Rounding: "half_up_2_v1", Rate: c.rate, Fixed: c.fixed}
		v, m, e := obligationCalculation(c.base, p)
		if e != nil || v != c.want || !json.Valid(m) {
			t.Fatal(c, v, e)
		}
	}
	for _, v := range []string{"-1.00", "NaN", "1e2", "1.001"} {
		if _, _, e := obligationCalculation(v, ObligationParameters{}); e == nil {
			t.Fatal("invalid input calculated")
		}
	}
}
func TestObligationPendingReplayIsolationAndConcurrentBusinessIdentity(t *testing.T) {
	s, sc, p := obligationFixture(t)
	ctx := context.Background()
	in := ObligationInput{Code: "company.tax_review", Period: "2026-11"}
	o := saveTestObligation(t, s, sc, "obligation-pending-001", in)
	if o.Amount != nil || o.DueDate != nil || o.State != "pending_data" || o.Pending[0] != "rule_unavailable" {
		t.Fatal(o)
	}
	restarted := NewService(NewStore(p), &testPolicy{}, s.queue)
	again := saveTestObligation(t, restarted, sc, "obligation-pending-001", in)
	if again.ID != o.ID || again.Version != 1 {
		t.Fatal("replay changed")
	}
	other := sc
	other.AccountID = "00000000-0000-4000-8000-000000000099"
	_, e := s.ReadObligations(ctx, other, "", o.ID, 1, 20)
	expectCode(t, e, "not_found")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, key := range []string{"concurrent-obligation-1", "concurrent-obligation-2"} {
		wg.Go(func() {
			_, e := s.SaveObligation(ctx, sc, key, ObligationInput{Code: "mei.dasn_simei", Period: "2026"})
			results <- e
		})
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else {
			expectCode(t, e, "version_conflict")
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	_, e = s.ObligationCommand(ctx, sc, o.ID, "review", "pending-review-key", ObligationAction{ExpectedVersion: 1, Confirmed: true})
	expectCode(t, e, "calculation_required")
}
func TestObligationQualificationValidityAndImmutableReviewedHistory(t *testing.T) {
	s, sc, p := obligationFixture(t)
	ctx := context.Background()
	syntheticObligationRule(t, s, sc)
	in := ObligationInput{Code: "mei.das", Period: "2026-11", Base: ptr("100.00")}
	o := saveTestObligation(t, s, sc, "unqualified-obligation", in)
	if o.State != "pending_data" || o.Amount != nil {
		t.Fatal("unqualified rule calculated")
	}
	s.WithObligationQualification(func(context.Context, Scope, Profile, int64, ObligationRule, string) error { return nil })
	in.ExpectedVersion = 1
	o = saveTestObligation(t, s, sc, "qualified-obligation", in)
	if o.State != "calculated" || *o.Amount != "1.34" || *o.DueDate != "2026-12-21" {
		t.Fatal(o)
	}
	b, e := s.ObligationCommand(ctx, sc, o.ID, "review", "reviewed-obligation", ObligationAction{ExpectedVersion: o.Version, Confirmed: true})
	o = obligationValue(t, b, e)
	if o.State != "reviewed" {
		t.Fatal(o)
	}
	in.ExpectedVersion = o.Version
	_, e = s.SaveObligation(ctx, sc, "locked-obligation", in)
	expectCode(t, e, "obligation_locked")
	b, e = s.ObligationCommand(ctx, sc, o.ID, "report-payment", "reported-obligation", ObligationAction{ExpectedVersion: o.Version, Confirmed: true, PaymentDate: "2026-12-10", EvidenceRef: "fixture://operator-report"})
	o = obligationValue(t, b, e)
	if o.State != "payment_reported" {
		t.Fatal(o)
	}
	b, e = s.ObligationCommand(ctx, sc, o.ID, "reopen", "reopen-obligation", ObligationAction{ExpectedVersion: o.Version, Confirmed: true, EvidenceRef: "fixture://adjustment-request"})
	o = obligationValue(t, b, e)
	if o.State != "pending_data" || o.PaymentDate != nil {
		t.Fatal(o)
	}
	var count int
	if e = p.QueryRow(ctx, `select count(*) from accounting.obligation_events where account_id=$1::uuid and issuer_id=$2::uuid and obligation_id=$3::uuid`, sc.AccountID, sc.IssuerID, o.ID).Scan(&count); e != nil || count != 5 {
		t.Fatal(count, e)
	}
	past := saveTestObligation(t, s, sc, "outside-validity-key", ObligationInput{Code: "mei.das", Period: "2025-11", Base: ptr("100.00")})
	if past.Amount != nil || past.State != "pending_data" {
		t.Fatal(past)
	}
}
