package accounting

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

func (s *Service) obligationAccess(ctx context.Context, sc Scope, write bool) error {
	permission := "accounting.obligations.view"
	if write {
		permission = "accounting.obligations.manage"
	}
	return s.check(ctx, sc, permission)
}
func (s *Service) obligationEvaluate(ctx context.Context, tx pgx.Tx, sc Scope, o Obligation) (Obligation, error) {
	p, pv, e := profile(ctx, tx, sc)
	if e != nil {
		return o, e
	}
	o.ProfileVersion = pv
	o.State = "pending_data"
	o.Amount = nil
	o.DueDate = nil
	o.RuleID = nil
	o.PaymentDate = nil
	o.PaymentRef = nil
	o.Memory = json.RawMessage(`{}`)
	o.Pending = []string{}
	r, e := loadObligationRule(ctx, tx, sc, o.Code, o.Period)
	if obligationAbsent(e) {
		o.Pending = append(o.Pending, "rule_unavailable")
		return o, nil
	}
	if e != nil {
		return o, e
	}
	o.RuleID = &r.ID
	start, end, ok := obligationPeriod(o.Period)
	if !ok || start < r.ValidFrom || end > r.ValidTo {
		o.Pending = append(o.Pending, "rule_outside_validity")
	}
	if !nonempty(p.TaxRegime) || *p.TaxRegime != r.Regime {
		o.Pending = append(o.Pending, "regime_not_confirmed")
	}
	if s.obligationQualification == nil || s.obligationQualification(ctx, sc, p, pv, r, o.Period) != nil {
		o.Pending = append(o.Pending, "rule_context_calendar_unqualified")
	}
	due := r.Parameters.DueDates[o.Period]
	if !dateValid(&due) || due == "" {
		o.Pending = append(o.Pending, "due_date_unverified")
	}
	if o.Base == nil {
		o.Pending = append(o.Pending, "base_missing")
	}
	if len(o.Pending) > 0 {
		return o, nil
	}
	amount, memory, e := obligationCalculation(*o.Base, r.Parameters)
	if e != nil {
		o.Pending = append(o.Pending, "missing_rule")
		return o, nil
	}
	o.Amount = &amount
	o.Memory, _ = json.Marshal(map[string]any{"calculation": memory, "ruleId": r.ID, "ruleVersion": r.Version, "sourceRef": r.SourceRef, "technicalReviewRef": r.TechnicalReviewRef, "profileVersion": pv})
	o.DueDate = &due
	o.State = "calculated"
	return o, nil
}
func (s *Service) SaveObligation(ctx context.Context, sc Scope, key string, in ObligationInput) (json.RawMessage, error) {
	if e := s.obligationAccess(ctx, sc, true); e != nil {
		return nil, e
	}
	_, _, ok := obligationPeriod(in.Period)
	if !ok || !obligationCode(in.Code) || in.ExpectedVersion < 0 || (in.Base != nil && !moneyPattern.MatchString(*in.Base)) {
		return nil, failure("invalid_input", 400)
	}
	return s.store.command(ctx, sc, "obligation.save", key, in, "manage", func(tx pgx.Tx) (any, error) {
		o, e := scanObligation(tx.QueryRow(ctx, `select `+obligationColumns+` from accounting.obligations where account_id=$1::uuid and issuer_id=$2::uuid and code=$3 and period=$4 for update`, sc.AccountID, sc.IssuerID, in.Code, in.Period))
		if e != nil {
			if v, ok := e.(*Error); !ok || v.Code != "not_found" {
				return nil, e
			}
			o = Obligation{Code: in.Code, Period: in.Period}
		}
		if o.Version != in.ExpectedVersion {
			return nil, failure("version_conflict", 409)
		}
		if o.State == "reviewed" || o.State == "payment_reported" {
			return nil, failure("obligation_locked", 409)
		}
		o.Base = in.Base
		o, e = s.obligationEvaluate(ctx, tx, sc, o)
		if e != nil {
			return nil, e
		}
		return saveObligation(ctx, tx, sc, o, "calculate")
	})
}
func (s *Service) ReadObligations(ctx context.Context, sc Scope, period, id string, page, limit int) (any, error) {
	if e := s.obligationAccess(ctx, sc, false); e != nil {
		return nil, e
	}
	if page < 1 || page > 100000 || limit < 1 || limit > 100 {
		return nil, failure("invalid_page", 400)
	}
	if period != "" {
		if _, _, ok := obligationPeriod(period); !ok {
			return nil, failure("invalid_period", 400)
		}
	}
	tx, e := s.store.begin(ctx)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e = issuerLock(ctx, tx, sc, "view"); e != nil {
		return nil, e
	}
	if id != "" {
		return loadObligation(ctx, tx, sc, id)
	}
	rows, e := tx.Query(ctx, `select `+obligationColumns+` from accounting.obligations where account_id=$1::uuid and issuer_id=$2::uuid and ($3='' or period=$3) order by period,id limit $4 offset $5`, sc.AccountID, sc.IssuerID, period, limit, (page-1)*limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Obligation{}
	for rows.Next() {
		o, e := scanObligation(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, o)
	}
	return map[string]any{"items": out, "page": page, "limit": limit, "calculationAvailable": s.obligationQualification != nil, "guideAvailable": false, "bankPaymentAvailable": false}, rows.Err()
}
func (s *Service) obligationCurrent(ctx context.Context, tx pgx.Tx, sc Scope, o Obligation) error {
	current, e := s.obligationEvaluate(ctx, tx, sc, o)
	if e != nil {
		return e
	}
	if current.State != "calculated" || current.ProfileVersion != o.ProfileVersion || !sameText(current.RuleID, o.RuleID) || !sameText(current.Amount, o.Amount) || !sameText(current.DueDate, o.DueDate) {
		return failure("obligation_stale_or_unqualified", 409)
	}
	return nil
}
func sameText(a, b *string) bool { return a != nil && b != nil && *a == *b }

func (s *Service) ObligationHistory(ctx context.Context, sc Scope, id string, page, limit int) (any, error) {
	if e := s.obligationAccess(ctx, sc, false); e != nil {
		return nil, e
	}
	if page < 1 || page > 100000 || limit < 1 || limit > 100 {
		return nil, failure("invalid_page", 400)
	}
	tx, e := s.store.begin(ctx)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e = issuerLock(ctx, tx, sc, "view"); e != nil {
		return nil, e
	}
	if _, e = loadObligation(ctx, tx, sc, id); e != nil {
		return nil, e
	}
	rows, e := tx.Query(ctx, `select version,actor_id::text,operation,snapshot from accounting.obligation_events where account_id=$1::uuid and issuer_id=$2::uuid and obligation_id=$3::uuid order by version desc limit $4 offset $5`, sc.AccountID, sc.IssuerID, id, limit, (page-1)*limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	type event struct {
		Version   int64           `json:"version"`
		ActorID   string          `json:"actorId"`
		Operation string          `json:"operation"`
		Snapshot  json.RawMessage `json:"snapshot"`
	}
	out := []event{}
	for rows.Next() {
		var v event
		if e = rows.Scan(&v.Version, &v.ActorID, &v.Operation, &v.Snapshot); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return map[string]any{"items": out, "page": page, "limit": limit}, rows.Err()
}
