package finance

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

func (s *PostgresSheetStore) prepareObligationTx(ctx context.Context, tx pgx.Tx, in ObligationEntryInput) (ObligationEntry, error) {
	out := ObligationEntry{SheetID: in.SheetID, Amount: in.Amount}
	var status, period string
	var updated time.Time
	e := tx.QueryRow(ctx, `select status,period,updated_at from finance.sheets where account_id=$1::uuid and id=$2::uuid for update`, in.AccountID, in.SheetID).Scan(&status, &period, &updated)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, ErrSheetNotFound
	}
	if e != nil {
		return out, e
	}
	if status != "aberta" {
		return out, ErrSheetClosed
	}
	if period != in.Period {
		return out, ErrSheetConflict
	}
	var linkedSheet, linkedAmount string
	e = tx.QueryRow(ctx, `select sheet_id::text,line_id::text,amount::text from finance.obligation_links where account_id=$1::uuid and issuer_id=$2::uuid and obligation_id=$3::uuid`, in.AccountID, in.IssuerID, in.ObligationID).Scan(&linkedSheet, &out.LineID, &linkedAmount)
	exists := e == nil
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	if exists {
		if linkedSheet != in.SheetID || linkedAmount != in.Amount {
			return out, ErrSheetConflict
		}
	} else {
		expected, e := time.Parse(time.RFC3339Nano, in.ExpectedUpdatedAt)
		if e != nil || !expected.Equal(updated) {
			return out, ErrSheetConflict
		}
		out.LineID = in.ExistingLineID
	}
	if out.LineID != "" {
		var valid bool
		e = tx.QueryRow(ctx, `select exists(select 1 from finance.lines l join finance.sheets s on s.id=l.sheet_id where s.account_id=$1::uuid and s.id=$2::uuid and l.id=$3::uuid and l.kind='saida' and l.amount=$4::numeric and l.adjustment_amount=0 and not exists(select 1 from finance.line_adjustments a where a.line_id=l.id))`, in.AccountID, in.SheetID, out.LineID, in.Amount).Scan(&valid)
		if e != nil {
			return out, e
		}
		if !valid {
			return out, ErrSheetConflict
		}
	} else {
		var manual bool
		e = tx.QueryRow(ctx, `select exists(select 1 from finance.lines l where l.sheet_id=$1::uuid and l.kind='saida' and l.amount=$2::numeric)`, in.SheetID, in.Amount).Scan(&manual)
		if e != nil {
			return out, e
		}
		if manual {
			return out, ErrSheetConflict
		}
		e = tx.QueryRow(ctx, `insert into finance.lines(sheet_id,kind,description,amount,position)select $1::uuid,'saida',$2,$3::numeric,coalesce(max(position)+1,0) from finance.lines where sheet_id=$1::uuid returning id::text`, in.SheetID, "Obrigação "+in.Code+" · "+in.Period, in.Amount).Scan(&out.LineID)
		if e != nil {
			return out, e
		}
	}
	if !exists {
		_, e = tx.Exec(ctx, `insert into finance.obligation_links(account_id,issuer_id,obligation_id,obligation_version,sheet_id,line_id,amount,created_by)values($1::uuid,$2::uuid,$3::uuid,$4,$5::uuid,$6::uuid,$7::numeric,$8::uuid)`, in.AccountID, in.IssuerID, in.ObligationID, in.Version, in.SheetID, out.LineID, in.Amount, in.ActorID)
		if e != nil {
			return out, e
		}
		e = tx.QueryRow(ctx, `update finance.sheets set updated_at=clock_timestamp() where account_id=$1::uuid and id=$2::uuid returning updated_at`, in.AccountID, in.SheetID).Scan(&updated)
		if e != nil {
			return out, e
		}
	}
	out.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return out, nil
}
