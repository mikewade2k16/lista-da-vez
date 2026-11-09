package accounting

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"strings"
)

func (s *Service) ObligationCommand(ctx context.Context, sc Scope, id, operation, key string, in ObligationAction) (json.RawMessage, error) {
	permission, grant := "accounting.obligations.manage", "manage"
	if operation == "review" {
		permission, grant = "accounting.obligations.review", "review"
	}
	if e := s.check(ctx, sc, permission); e != nil {
		return nil, e
	}
	if !in.Confirmed || in.ExpectedVersion < 1 {
		return nil, failure("confirmation_required", 400)
	}
	switch operation {
	case "review", "reopen", "prepare-finance", "report-payment":
	default:
		return nil, failure("invalid_operation", 400)
	}
	return s.store.command(ctx, sc, "obligation."+operation, key, struct {
		ID    string
		Input ObligationAction
	}{id, in}, grant, func(tx pgx.Tx) (any, error) {
		o, e := loadObligation(ctx, tx, sc, id)
		if e != nil {
			return nil, e
		}
		if o.Version != in.ExpectedVersion {
			return nil, failure("version_conflict", 409)
		}
		switch operation {
		case "reopen":
			if strings.TrimSpace(in.EvidenceRef) == "" || len(in.EvidenceRef) > 500 {
				return nil, failure("revision_reason_required", 400)
			}
			o.State = "pending_data"
			o.Pending = []string{"explicit_revision_required"}
			o.PaymentDate = nil
			o.PaymentRef = nil
			return saveObligation(ctx, tx, sc, o, "reopen:"+in.EvidenceRef)
		case "review":
			if o.State != "calculated" {
				return nil, failure("calculation_required", 409)
			}
			if e = s.obligationCurrent(ctx, tx, sc, o); e != nil {
				return nil, e
			}
			o.State = "reviewed"
			return saveObligation(ctx, tx, sc, o, "review")
		case "prepare-finance":
			if o.State != "reviewed" {
				return nil, failure("review_required", 409)
			}
			if e = s.obligationCurrent(ctx, tx, sc, o); e != nil {
				return nil, e
			}
			if s.obligationFinance == nil {
				return nil, failure("finance_unavailable", 503)
			}
			return s.obligationFinance.PrepareObligationTx(ctx, tx, sc, ObligationFinanceInput{ObligationID: o.ID, Code: o.Code, Period: o.Period, Amount: *o.Amount, Version: o.Version, SheetID: in.SheetID, ExpectedUpdatedAt: in.ExpectedSheetUpdatedAt, ExistingLineID: in.ExistingLineID})
		case "report-payment":
			if o.State != "reviewed" {
				return nil, failure("review_required", 409)
			}
			if e = s.obligationCurrent(ctx, tx, sc, o); e != nil {
				return nil, e
			}
			if !dateValid(&in.PaymentDate) || in.PaymentDate == "" || strings.TrimSpace(in.EvidenceRef) == "" || len(in.EvidenceRef) > 500 {
				return nil, failure("payment_evidence_required", 400)
			}
			o.State = "payment_reported"
			o.PaymentDate = &in.PaymentDate
			o.PaymentRef = &in.EvidenceRef
			return saveObligation(ctx, tx, sc, o, "report-payment")
		}
		return nil, failure("invalid_operation", 400)
	})
}
