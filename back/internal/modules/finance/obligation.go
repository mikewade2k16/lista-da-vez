package finance

import (
	"context"
	"github.com/jackc/pgx/v5"
	"regexp"
)

type ObligationEntryInput struct {
	AccountID, ActorID, IssuerID, ObligationID, Code, Period, Amount, SheetID, ExpectedUpdatedAt, ExistingLineID string
	Version                                                                                                      int64
}
type ObligationEntry struct{ SheetID, LineID, Amount, UpdatedAt string }

var obligationMoney = regexp.MustCompile(`^[0-9]{1,12}\.[0-9]{2}$`)

// PrepareObligationTx only prepares an unpaid expense; caller owns the shared
// transaction and must authorize finance.sheets.manage through live host policy.
func (m *Module) PrepareObligationTx(ctx context.Context, tx pgx.Tx, in ObligationEntryInput) (ObligationEntry, error) {
	if m.handle == nil {
		return ObligationEntry{}, ErrSheetNotFound
	}
	return m.handle.service.PrepareObligationTx(ctx, tx, in)
}
func (s *Service) PrepareObligationTx(ctx context.Context, tx pgx.Tx, in ObligationEntryInput) (ObligationEntry, error) {
	if !isUUID(in.AccountID) || !isUUID(in.ActorID) || !isUUID(in.IssuerID) || !isUUID(in.ObligationID) || !isUUID(in.SheetID) || in.Version < 1 || !obligationMoney.MatchString(in.Amount) || (in.ExistingLineID != "" && !isUUID(in.ExistingLineID)) {
		return ObligationEntry{}, ErrFiscalInvalid
	}
	store, ok := s.sheets.(*PostgresSheetStore)
	if !ok {
		return ObligationEntry{}, ErrSheetNotFound
	}
	return store.prepareObligationTx(ctx, tx, in)
}
