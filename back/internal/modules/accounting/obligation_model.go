package accounting

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

type Obligation struct {
	ID             string          `json:"id"`
	Code           string          `json:"code"`
	Period         string          `json:"period"`
	Version        int64           `json:"version"`
	ProfileVersion int64           `json:"profileVersion"`
	State          string          `json:"state"`
	Base           *string         `json:"base"`
	Amount         *string         `json:"amount"`
	DueDate        *string         `json:"dueDate"`
	RuleID         *string         `json:"ruleId"`
	Pending        []string        `json:"pending"`
	Memory         json.RawMessage `json:"memory"`
	PaymentDate    *string         `json:"paymentDate"`
	PaymentRef     *string         `json:"paymentRef"`
}
type ObligationInput struct {
	ExpectedVersion int64   `json:"expectedVersion"`
	Code            string  `json:"code"`
	Period          string  `json:"period"`
	Base            *string `json:"base"`
}
type ObligationAction struct {
	ExpectedVersion        int64  `json:"expectedVersion"`
	Confirmed              bool   `json:"confirmed"`
	SheetID                string `json:"sheetId"`
	ExpectedSheetUpdatedAt string `json:"expectedSheetUpdatedAt"`
	ExistingLineID         string `json:"existingLineId"`
	PaymentDate            string `json:"paymentDate"`
	EvidenceRef            string `json:"evidenceRef"`
}
type ObligationRule struct {
	ID, Code, Regime, Jurisdiction, ValidFrom, ValidTo, SourceRef, TechnicalReviewRef string
	Version                                                                           int64
	Parameters                                                                        ObligationParameters
}
type ObligationParameters struct {
	Formula  string            `json:"formula"`
	Rate     string            `json:"rate"`
	Fixed    string            `json:"fixed"`
	Rounding string            `json:"rounding"`
	DueDates map[string]string `json:"dueDates"`
}

// Qualification must be local/offline and bind confirmed regime, jurisdiction,
// period, current profile version, reviewed rule and calendar sources. Nil blocks
// real calculation. No implementation is registered until rules are qualified.
type ObligationQualification func(context.Context, Scope, Profile, int64, ObligationRule, string) error
type ObligationFinanceInput struct {
	ObligationID, Code, Period, Amount, SheetID, ExpectedUpdatedAt, ExistingLineID string
	Version                                                                        int64
}
type ObligationFinanceLink struct {
	SheetID          string `json:"sheetId"`
	LineID           string `json:"lineId"`
	Amount           string `json:"amount"`
	UpdatedAt        string `json:"updatedAt"`
	PaymentConfirmed bool   `json:"paymentConfirmed"`
}
type ObligationFinance interface {
	PrepareObligationTx(context.Context, pgx.Tx, Scope, ObligationFinanceInput) (ObligationFinanceLink, error)
}

func (s *Service) WithObligationQualification(q ObligationQualification) *Service {
	s.obligationQualification = q
	return s
}
func (s *Service) WithObligationFinance(f ObligationFinance) *Service {
	s.obligationFinance = f
	return s
}
