package app

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
)

type accountingObligationFinance struct {
	module    *finance.Module
	authorize func(context.Context, finance.PlusScope, string) error
}

func (a accountingObligationFinance) PrepareObligationTx(ctx context.Context, tx pgx.Tx, sc accounting.Scope, in accounting.ObligationFinanceInput) (accounting.ObligationFinanceLink, error) {
	var out accounting.ObligationFinanceLink
	if e := a.authorize(ctx, finance.PlusScope{AccountID: sc.AccountID, ActorID: sc.ActorID}, "finance.sheets.manage"); e != nil {
		var denied *finance.PlusError
		if errors.As(e, &denied) {
			return out, &accounting.Error{Code: denied.Code, Status: denied.Status}
		}
		return out, e
	}
	v, e := a.module.PrepareObligationTx(ctx, tx, finance.ObligationEntryInput{AccountID: sc.AccountID, ActorID: sc.ActorID, IssuerID: sc.IssuerID, ObligationID: in.ObligationID, Code: in.Code, Period: in.Period, Amount: in.Amount, Version: in.Version, SheetID: in.SheetID, ExpectedUpdatedAt: in.ExpectedUpdatedAt, ExistingLineID: in.ExistingLineID})
	if e != nil {
		switch {
		case errors.Is(e, finance.ErrSheetNotFound), errors.Is(e, finance.ErrLineNotFound):
			return out, &accounting.Error{Code: "not_found", Status: 404}
		case errors.Is(e, finance.ErrSheetClosed):
			return out, &accounting.Error{Code: "finance_sheet_closed", Status: 409}
		case errors.Is(e, finance.ErrSheetConflict):
			return out, &accounting.Error{Code: "finance_association_or_revision_required", Status: 409}
		case errors.Is(e, finance.ErrFiscalInvalid):
			return out, &accounting.Error{Code: "invalid_finance_input", Status: 400}
		}
		return out, e
	}
	return accounting.ObligationFinanceLink{SheetID: v.SheetID, LineID: v.LineID, Amount: v.Amount, UpdatedAt: v.UpdatedAt}, nil
}
