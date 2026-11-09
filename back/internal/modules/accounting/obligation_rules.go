package accounting

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// CT-15 pacote 7 (P15-16): authoritative lifecycle of obligation rules.
// A version is registered as draft by its technical reviewer, activated by a
// different actor (four eyes) and can be retired. Versions are immutable (0321)
// and calculation only uses active versions. This flow never supplies a real
// rule: parameters, sources and review references come from the operator.

type ObligationRuleInput struct {
	Code               string               `json:"code"`
	Regime             string               `json:"regime"`
	Jurisdiction       string               `json:"jurisdiction"`
	ValidFrom          string               `json:"validFrom"`
	ValidTo            string               `json:"validTo"`
	Parameters         ObligationParameters `json:"parameters"`
	SourceRef          string               `json:"sourceRef"`
	TechnicalReviewRef string               `json:"technicalReviewRef"`
}
type ObligationRuleTransition struct {
	Reason string `json:"reason"`
}
type ObligationRuleView struct {
	ID                 string               `json:"id"`
	Code               string               `json:"code"`
	Version            int64                `json:"version"`
	Regime             string               `json:"regime"`
	Jurisdiction       string               `json:"jurisdiction"`
	ValidFrom          string               `json:"validFrom"`
	ValidTo            string               `json:"validTo"`
	Parameters         ObligationParameters `json:"parameters"`
	SourceRef          string               `json:"sourceRef"`
	TechnicalReviewRef string               `json:"technicalReviewRef"`
	ReviewedBy         string               `json:"reviewedBy"`
	Status             string               `json:"status"` // draft | active | retired
}

// ruleStatusSQL derives the status from the latest lifecycle event.
const ruleStatusSQL = `coalesce((select case e.action when 'activate' then 'active' else 'retired' end from accounting.obligation_rule_events e
 where e.account_id=r.account_id and e.issuer_id=r.issuer_id and e.rule_id=r.id order by e.created_at desc,e.id desc limit 1),'draft')`
const ruleViewColumns = `r.id::text,r.code,r.version,r.regime,r.jurisdiction,r.valid_from::text,r.valid_to::text,r.parameters,r.source_ref,r.technical_review_ref,r.reviewed_by::text,` + ruleStatusSQL

func scanRuleView(row pgx.Row) (ObligationRuleView, error) {
	var v ObligationRuleView
	var raw []byte
	if e := row.Scan(&v.ID, &v.Code, &v.Version, &v.Regime, &v.Jurisdiction, &v.ValidFrom, &v.ValidTo, &raw, &v.SourceRef, &v.TechnicalReviewRef, &v.ReviewedBy, &v.Status); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return v, failure("not_found", 404)
		}
		return v, e
	}
	return v, json.Unmarshal(raw, &v.Parameters)
}

func boundedText(v string, max int) bool {
	v = strings.TrimSpace(v)
	return v != "" && len(v) <= max
}

func validRuleInput(in ObligationRuleInput) bool {
	from, to := in.ValidFrom, in.ValidTo
	if !obligationCode(in.Code) || !boundedText(in.Regime, 120) || !boundedText(in.Jurisdiction, 120) || !dateValid(&from) || !dateValid(&to) || from == "" || to == "" || to < from {
		return false
	}
	if !strings.HasPrefix(in.SourceRef, "https://") || len(in.SourceRef) > 500 || !boundedText(in.TechnicalReviewRef, 500) {
		return false
	}
	if _, _, e := obligationCalculation("0.00", in.Parameters); e != nil || len(in.Parameters.DueDates) > 120 {
		return false
	}
	for period, due := range in.Parameters.DueDates {
		start, end, ok := obligationPeriod(period)
		d := due
		if !ok || !dateValid(&d) || d == "" || start < from || end > to {
			return false
		}
	}
	return true
}

func (s *Service) CreateObligationRule(ctx context.Context, sc Scope, key string, in ObligationRuleInput) (json.RawMessage, error) {
	if e := s.obligationAccess(ctx, sc, true); e != nil {
		return nil, e
	}
	if !validRuleInput(in) {
		return nil, failure("invalid_rule", 400)
	}
	return s.store.command(ctx, sc, "obligation_rule.create", key, in, "manage", func(tx pgx.Tx) (any, error) {
		raw, e := json.Marshal(in.Parameters)
		if e != nil {
			return nil, e
		}
		var id string
		e = tx.QueryRow(ctx, `insert into accounting.obligation_rules(account_id,issuer_id,code,version,regime,jurisdiction,valid_from,valid_to,parameters,source_ref,technical_review_ref,reviewed_by)
 select $1::uuid,$2::uuid,$3,coalesce(max(version),0)+1,$4,$5,$6::date,$7::date,$8::jsonb,$9,$10,$11::uuid from accounting.obligation_rules where account_id=$1::uuid and issuer_id=$2::uuid and code=$3 returning id::text`,
			sc.AccountID, sc.IssuerID, in.Code, strings.TrimSpace(in.Regime), strings.TrimSpace(in.Jurisdiction), in.ValidFrom, in.ValidTo, raw, in.SourceRef, strings.TrimSpace(in.TechnicalReviewRef), sc.ActorID).Scan(&id)
		if e != nil {
			return nil, e
		}
		return loadRuleView(ctx, tx, sc, id, false)
	})
}

func loadRuleView(ctx context.Context, tx pgx.Tx, sc Scope, id string, lock bool) (ObligationRuleView, error) {
	if !validID(id) {
		return ObligationRuleView{}, failure("not_found", 404)
	}
	suffix := ""
	if lock {
		suffix = " for update of r"
	}
	return scanRuleView(tx.QueryRow(ctx, `select `+ruleViewColumns+` from accounting.obligation_rules r where r.account_id=$1::uuid and r.issuer_id=$2::uuid and r.id=$3::uuid`+suffix, sc.AccountID, sc.IssuerID, id))
}

// TransitionObligationRule activates or retires a version, recording actor and reason.
func (s *Service) TransitionObligationRule(ctx context.Context, sc Scope, id, action, key string, in ObligationRuleTransition) (json.RawMessage, error) {
	if e := s.obligationAccess(ctx, sc, true); e != nil {
		return nil, e
	}
	reason := strings.TrimSpace(in.Reason)
	if (action != "activate" && action != "retire") || len(reason) < 3 || len(reason) > 500 {
		return nil, failure("invalid_input", 400)
	}
	return s.store.command(ctx, sc, "obligation_rule."+action, key, map[string]string{"id": id, "reason": reason}, "manage", func(tx pgx.Tx) (any, error) {
		// Serialize transitions of the same code so overlap checks see each other.
		v, e := loadRuleView(ctx, tx, sc, id, true)
		if e != nil {
			return nil, e
		}
		if _, e = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, "accounting:rule:"+sc.AccountID+":"+sc.IssuerID+":"+v.Code); e != nil {
			return nil, e
		}
		switch {
		case action == "activate" && v.Status != "draft":
			return nil, failure("rule_not_draft", 409)
		case action == "activate" && v.ReviewedBy == sc.ActorID:
			return nil, failure("four_eyes_required", 409)
		case action == "retire" && v.Status != "active":
			return nil, failure("rule_not_active", 409)
		}
		if action == "activate" {
			var overlap bool
			if e = tx.QueryRow(ctx, `select exists(select 1 from accounting.obligation_rules r where r.account_id=$1::uuid and r.issuer_id=$2::uuid and r.code=$3 and r.id<>$4::uuid
 and r.valid_from<=$6::date and r.valid_to>=$5::date and `+ruleStatusSQL+`='active')`, sc.AccountID, sc.IssuerID, v.Code, v.ID, v.ValidFrom, v.ValidTo).Scan(&overlap); e != nil {
				return nil, e
			}
			if overlap {
				return nil, failure("active_rule_overlap", 409)
			}
		}
		if _, e = tx.Exec(ctx, `insert into accounting.obligation_rule_events(account_id,issuer_id,rule_id,action,actor_id,reason) values($1::uuid,$2::uuid,$3::uuid,$4,$5::uuid,$6)`, sc.AccountID, sc.IssuerID, v.ID, action, sc.ActorID, reason); e != nil {
			return nil, e
		}
		return loadRuleView(ctx, tx, sc, v.ID, false)
	})
}

func (s *Service) ListObligationRules(ctx context.Context, sc Scope, code string) ([]ObligationRuleView, error) {
	if e := s.obligationAccess(ctx, sc, false); e != nil {
		return nil, e
	}
	if code != "" && !obligationCode(code) {
		return nil, failure("invalid_input", 400)
	}
	tx, e := s.store.begin(ctx)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e = issuerLock(ctx, tx, sc, "view"); e != nil {
		return nil, e
	}
	rows, e := tx.Query(ctx, `select `+ruleViewColumns+` from accounting.obligation_rules r where r.account_id=$1::uuid and r.issuer_id=$2::uuid and ($3='' or r.code=$3) order by r.code,r.version desc limit 100`, sc.AccountID, sc.IssuerID, code)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ObligationRuleView{}
	for rows.Next() {
		v, e := scanRuleView(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
