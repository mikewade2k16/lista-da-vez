package accounting

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

const obligationColumns = `id::text,code,period,version,profile_version,state,base_amount::text,amount::text,to_char(due_date,'YYYY-MM-DD'),rule_id::text,pending,memory,to_char(payment_date,'YYYY-MM-DD'),payment_ref`

func scanObligation(row pgx.Row) (Obligation, error) {
	var o Obligation
	var pending []byte
	e := row.Scan(&o.ID, &o.Code, &o.Period, &o.Version, &o.ProfileVersion, &o.State, &o.Base, &o.Amount, &o.DueDate, &o.RuleID, &pending, &o.Memory, &o.PaymentDate, &o.PaymentRef)
	if e == nil {
		e = json.Unmarshal(pending, &o.Pending)
	}
	return o, dbError(e)
}
func loadObligation(ctx context.Context, tx pgx.Tx, sc Scope, id string) (Obligation, error) {
	if !validID(id) {
		return Obligation{}, failure("not_found", 404)
	}
	return scanObligation(tx.QueryRow(ctx, `select `+obligationColumns+` from accounting.obligations where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid for update`, sc.AccountID, sc.IssuerID, id))
}

// loadObligationRule uses only active versions (0321 lifecycle), preferring the one
// whose validity covers the period; an active version outside it is still returned
// so the evaluation reports rule_outside_validity instead of hiding it.
func loadObligationRule(ctx context.Context, tx pgx.Tx, sc Scope, code, period string) (ObligationRule, error) {
	var r ObligationRule
	var raw []byte
	start, end, _ := obligationPeriod(period)
	e := tx.QueryRow(ctx, `select r.id::text,r.code,r.version,r.regime,r.jurisdiction,r.valid_from::text,r.valid_to::text,r.parameters,r.source_ref,r.technical_review_ref from accounting.obligation_rules r
 where r.account_id=$1::uuid and r.issuer_id=$2::uuid and r.code=$3 and `+ruleStatusSQL+`='active'
 order by (r.valid_from<=nullif($4,'')::date and r.valid_to>=nullif($5,'')::date) desc nulls last,r.version desc limit 1`, sc.AccountID, sc.IssuerID, code, start, end).Scan(&r.ID, &r.Code, &r.Version, &r.Regime, &r.Jurisdiction, &r.ValidFrom, &r.ValidTo, &raw, &r.SourceRef, &r.TechnicalReviewRef)
	if e != nil {
		return r, e
	}
	e = json.Unmarshal(raw, &r.Parameters)
	return r, e
}
func saveObligation(ctx context.Context, tx pgx.Tx, sc Scope, o Obligation, op string) (Obligation, error) {
	pending, e := json.Marshal(o.Pending)
	if e != nil {
		return o, e
	}
	if o.ID == "" {
		e = tx.QueryRow(ctx, `insert into accounting.obligations(account_id,issuer_id,code,period,profile_version,state,base_amount,amount,due_date,rule_id,pending,memory)values($1::uuid,$2::uuid,$3,$4,$5,$6,$7::numeric,$8::numeric,$9::date,$10::uuid,$11,$12)returning id::text,version`, sc.AccountID, sc.IssuerID, o.Code, o.Period, o.ProfileVersion, o.State, o.Base, o.Amount, o.DueDate, o.RuleID, pending, o.Memory).Scan(&o.ID, &o.Version)
	} else {
		o.Version++
		_, e = tx.Exec(ctx, `update accounting.obligations set version=$4,profile_version=$5,state=$6,base_amount=$7::numeric,amount=$8::numeric,due_date=$9::date,rule_id=$10::uuid,pending=$11,memory=$12,reviewed_by=case when $6='reviewed' then $13::uuid when $6='pending_data' then null else reviewed_by end,payment_date=$14::date,payment_ref=$15,updated_at=now() where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, sc.AccountID, sc.IssuerID, o.ID, o.Version, o.ProfileVersion, o.State, o.Base, o.Amount, o.DueDate, o.RuleID, pending, o.Memory, sc.ActorID, o.PaymentDate, o.PaymentRef)
	}
	if e != nil {
		return o, e
	}
	raw, e := json.Marshal(o)
	if e != nil {
		return o, e
	}
	_, e = tx.Exec(ctx, `insert into accounting.obligation_events(account_id,issuer_id,obligation_id,version,actor_id,operation,snapshot)values($1::uuid,$2::uuid,$3::uuid,$4,$5::uuid,$6,$7)`, sc.AccountID, sc.IssuerID, o.ID, o.Version, sc.ActorID, op, raw)
	return o, e
}
func obligationAbsent(e error) bool { return errors.Is(e, pgx.ErrNoRows) }
