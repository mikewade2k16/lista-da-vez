package accounting

import (
	"context"
	"encoding/json"
	"testing"
)

func syntheticRuleInput(from, to string) ObligationRuleInput {
	return ObligationRuleInput{Code: "mei.das", Regime: "synthetic", Jurisdiction: "synthetic", ValidFrom: from, ValidTo: to,
		Parameters: ObligationParameters{Formula: "base_percent_fixed_v1", Rate: "1.333333", Fixed: "0.01", Rounding: "half_up_2_v1", DueDates: map[string]string{"2026-11": "2026-12-21"}},
		SourceRef:  "https://example.invalid/synthetic-source", TechnicalReviewRef: "synthetic review only"}
}

func ruleView(t *testing.T) func([]byte, error) ObligationRuleView {
	return func(b []byte, e error) ObligationRuleView {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
		var v ObligationRuleView
		if e = json.Unmarshal(b, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
}

// CT-15 pacote 7 (P15-16): draft is never used, four eyes for activation, no
// overlapping active versions, immutable versions and retire back to pending.
func TestObligationRuleLifecycle(t *testing.T) {
	s, reviewer, p := obligationFixture(t)
	ctx := context.Background()
	approver := reviewer
	if e := p.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&approver.ActorID); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Exec(ctx, `insert into accounting.issuer_members(account_id,issuer_id,user_id,grants) values($1::uuid,$2::uuid,$3::uuid,array['view','manage'])`, approver.AccountID, approver.IssuerID, approver.ActorID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.CreateObligationRule(ctx, reviewer, "rule-bad-0001", ObligationRuleInput{Code: "mei.das"}); e == nil {
		t.Fatal("incomplete rule accepted")
	}
	bad := syntheticRuleInput("2026-01-01", "2026-12-31")
	bad.SourceRef = "fixture://not-official"
	if _, e := s.CreateObligationRule(ctx, reviewer, "rule-bad-0002", bad); e == nil {
		t.Fatal("non-https source accepted")
	}
	v1 := ruleView(t)(s.CreateObligationRule(ctx, reviewer, "rule-create-0001", syntheticRuleInput("2026-01-01", "2026-12-31")))
	if v1.Status != "draft" || v1.Version != 1 {
		t.Fatal(v1)
	}
	base := "100.00"
	pending := saveTestObligation(t, s, reviewer, "rule-obl-0001", ObligationInput{Code: "mei.das", Period: "2026-11", Base: &base})
	if pending.Pending[0] != "rule_unavailable" {
		t.Fatal("draft rule used in calculation", pending.Pending)
	}
	_, e := s.TransitionObligationRule(ctx, reviewer, v1.ID, "activate", "rule-act-0001", ObligationRuleTransition{Reason: "self approval"})
	expectCode(t, e, "four_eyes_required")
	active := ruleView(t)(s.TransitionObligationRule(ctx, approver, v1.ID, "activate", "rule-act-0002", ObligationRuleTransition{Reason: "reviewed synthetic"}))
	if active.Status != "active" {
		t.Fatal(active)
	}
	replay := ruleView(t)(s.TransitionObligationRule(ctx, approver, v1.ID, "activate", "rule-act-0002", ObligationRuleTransition{Reason: "reviewed synthetic"}))
	if replay.Status != "active" {
		t.Fatal("idempotent replay failed", replay)
	}
	v2 := ruleView(t)(s.CreateObligationRule(ctx, reviewer, "rule-create-0002", syntheticRuleInput("2026-06-01", "2027-05-31")))
	_, e = s.TransitionObligationRule(ctx, approver, v2.ID, "activate", "rule-act-0003", ObligationRuleTransition{Reason: "overlap attempt"})
	expectCode(t, e, "active_rule_overlap")
	if _, e = p.Exec(ctx, `update accounting.obligation_rules set parameters='{}' where id=$1::uuid`, v1.ID); e == nil {
		t.Fatal("rule version is mutable")
	}
	if _, e = p.Exec(ctx, `update accounting.obligation_rule_events set reason='changed' where rule_id=$1::uuid`, v1.ID); e == nil {
		t.Fatal("rule history is mutable")
	}
	list, e := s.ListObligationRules(ctx, reviewer, "mei.das")
	if e != nil || len(list) != 2 || list[0].Status != "draft" || list[1].Status != "active" {
		t.Fatal("list", list, e)
	}
	retired := ruleView(t)(s.TransitionObligationRule(ctx, approver, v1.ID, "retire", "rule-ret-0001", ObligationRuleTransition{Reason: "superseded"}))
	if retired.Status != "retired" {
		t.Fatal(retired)
	}
	_, e = s.TransitionObligationRule(ctx, approver, v1.ID, "activate", "rule-act-0004", ObligationRuleTransition{Reason: "reactivate"})
	expectCode(t, e, "rule_not_draft")
	again := saveTestObligation(t, s, reviewer, "rule-obl-0002", ObligationInput{Code: "mei.das", Period: "2026-11", Base: &base, ExpectedVersion: pending.Version})
	if again.Pending[0] != "rule_unavailable" {
		t.Fatal("retired rule used", again.Pending)
	}
	// A different issuer member without access sees nothing of this issuer.
	outsider := reviewer
	if e = p.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&outsider.ActorID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ListObligationRules(ctx, outsider, ""); e == nil {
		t.Fatal("rules visible without issuer membership")
	}
}
