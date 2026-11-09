package finance

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

// Create insere a planilha e suas linhas numa transacao.
func (r *PostgresSheetStore) Create(ctx context.Context, accountID string, d SheetDetail) (SheetDetail, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return SheetDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	if err := tx.QueryRow(ctx, `
		insert into finance.sheets (account_id, core_tenant_id, title, period, status, notes)
		values ($1::uuid, $2, $3, $4, $5, $6)
		returning id::text`,
		accountID, d.CoreTenantID, d.Title, d.Period, d.Status, d.Notes).Scan(&id); err != nil {
		return SheetDetail{}, err
	}
	if err := replaceLines(ctx, tx, id, d.Entradas, d.Saidas); err != nil {
		return SheetDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SheetDetail{}, err
	}
	return r.Get(ctx, accountID, id)
}

// Update faz o full-replace da planilha (404 se fora da account).
func (r *PostgresSheetStore) Update(ctx context.Context, accountID string, d SheetDetail) (SheetDetail, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return SheetDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockSheetForWrite(ctx, tx, accountID, d.ID, d.ExpectedUpdatedAt, false); err != nil {
		return SheetDetail{}, err
	}
	// period '' preserva o valor atual (coalesce nullif).
	var id string
	err = tx.QueryRow(ctx, `
		update finance.sheets
		set title = $3, period = coalesce(nullif($4,''), period), status = $5,
		    notes = $6, core_tenant_id = $7, updated_at = now()
		where id = $1::uuid and account_id = $2::uuid
		returning id::text`,
		d.ID, accountID, d.Title, d.Period, d.Status, d.Notes, d.CoreTenantID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SheetDetail{}, ErrSheetNotFound
		}
		return SheetDetail{}, err
	}
	if _, err := tx.Exec(ctx, `delete from finance.lines where sheet_id = $1::uuid`, id); err != nil {
		return SheetDetail{}, err
	}
	if err := replaceLines(ctx, tx, id, d.Entradas, d.Saidas); err != nil {
		return SheetDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SheetDetail{}, err
	}
	return r.Get(ctx, accountID, id)
}

// replaceLines insere entradas+saidas (na ordem do array = position) e adjustments.
func replaceLines(ctx context.Context, tx pgx.Tx, sheetID string, entradas, saidas []Line) error {
	insert := func(lines []Line, kind string) error {
		for i, l := range lines {
			var lineID string
			if err := tx.QueryRow(ctx, `
				insert into finance.lines
				  (id, sheet_id, kind, description, category, effective, effective_date,
				   amount, adjustment_amount, fixed_account_id, details, position)
				values (`+fmt.Sprintf(newIDExpr, 1)+`, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				returning id::text`,
				l.ID, sheetID, kind, l.Description, l.Category, l.Effective, dateArg(l.EffectiveDate),
				l.Amount, l.AdjustmentAmount, l.FixedAccountID, l.Details, i).Scan(&lineID); err != nil {
				return err
			}
			for j, a := range l.Adjustments {
				if _, err := tx.Exec(ctx, `
					insert into finance.line_adjustments (id, line_id, amount, note, date, position)
					values (`+fmt.Sprintf(newIDExpr, 1)+`, $2::uuid, $3, $4, $5, $6)`,
					a.ID, lineID, a.Amount, a.Note, dateArg(a.Date), j); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := insert(entradas, "entrada"); err != nil {
		return err
	}
	return insert(saidas, "saida")
}

// Delete remove a planilha (404 se fora da account).
func (r *PostgresSheetStore) Delete(ctx context.Context, accountID, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockSheetForWrite(ctx, tx, accountID, id, "", false); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `delete from finance.sheets where id=$1::uuid and account_id=$2::uuid`, id, accountID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PatchLine atualiza effective/effective_date de uma linha e toca updated_at do
// sheet. 404 se a linha nao pertence a uma planilha da account.
func (r *PostgresSheetStore) PatchLine(ctx context.Context, accountID, sheetID, lineID string, effective bool, effectiveDate string) (Line, string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Line{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSheetForWrite(ctx, tx, accountID, sheetID, "", false); err != nil {
		return Line{}, "", err
	}

	tag, err := tx.Exec(ctx, `
		update finance.lines l
		set effective = $3, effective_date = $4
		from finance.sheets s
		where l.id = $1::uuid and l.sheet_id = s.id
		  and s.id = $2::uuid and s.account_id = $5::uuid`,
		lineID, sheetID, effective, dateArg(effectiveDate), accountID)
	if err != nil {
		return Line{}, "", err
	}
	if tag.RowsAffected() == 0 {
		return Line{}, "", ErrLineNotFound
	}

	var updated time.Time
	if err := tx.QueryRow(ctx, `
		update finance.sheets set updated_at = now()
		where id = $1::uuid and account_id = $2::uuid
		returning updated_at`, sheetID, accountID).Scan(&updated); err != nil {
		return Line{}, "", err
	}

	var l Line
	var effDate pgtype.Date
	if err := tx.QueryRow(ctx, `
		select id::text, kind, description, category, effective, effective_date,
		       amount, adjustment_amount, fixed_account_id, details
		from finance.lines where id = $1::uuid`, lineID).
		Scan(&l.ID, &l.Kind, &l.Description, &l.Category, &l.Effective, &effDate,
			&l.Amount, &l.AdjustmentAmount, &l.FixedAccountID, &l.Details); err != nil {
		return Line{}, "", err
	}
	if effDate.Valid {
		l.EffectiveDate = effDate.Time.Format("2006-01-02")
	}
	l.Adjustments = []Adjustment{}
	if err := loadLineAdjustments(ctx, tx, &l); err != nil {
		return Line{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return Line{}, "", err
	}
	return l, tsString(updated), nil
}

// loadLineAdjustments preenche os adjustments de uma linha dentro da tx.
func loadLineAdjustments(ctx context.Context, tx pgx.Tx, l *Line) error {
	rows, err := tx.Query(ctx, `
		select id::text, amount, note, date from finance.line_adjustments
		where line_id = $1::uuid order by position`, l.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var adj Adjustment
		var date pgtype.Date
		if err := rows.Scan(&adj.ID, &adj.Amount, &adj.Note, &date); err != nil {
			return err
		}
		if date.Valid {
			adj.Date = date.Time.Format("2006-01-02")
		}
		l.Adjustments = append(l.Adjustments, adj)
	}
	return rows.Err()
}

func lockSheetForWrite(ctx context.Context, tx pgx.Tx, accountID, id, expected string, allowClosed bool) error {
	var status string
	var updated time.Time
	err := tx.QueryRow(ctx, `select status,updated_at from finance.sheets where id=$1::uuid and account_id=$2::uuid for update`, id, accountID).Scan(&status, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSheetNotFound
	}
	if err != nil {
		return err
	}
	if status == "fechada" && !allowClosed {
		return ErrSheetClosed
	}
	if expected != "" {
		stamp, err := time.Parse(time.RFC3339Nano, expected)
		if err != nil || !updated.Equal(stamp) {
			return ErrSheetConflict
		}
	}
	return nil
}
func (r *PostgresSheetStore) ChangeStatus(ctx context.Context, accountID, id, status, expected string) (SheetDetail, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return SheetDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockSheetForWrite(ctx, tx, accountID, id, expected, true); err != nil {
		return SheetDetail{}, err
	}
	if _, err = tx.Exec(ctx, `update finance.sheets set status=$3,updated_at=now() where id=$1::uuid and account_id=$2::uuid`, id, accountID, status); err != nil {
		return SheetDetail{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SheetDetail{}, err
	}
	return r.Get(ctx, accountID, id)
}
