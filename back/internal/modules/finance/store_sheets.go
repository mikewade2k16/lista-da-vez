package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresSheetStore persiste planilhas em finance.sheets/lines/line_adjustments.
type PostgresSheetStore struct {
	pool *pgxpool.Pool
}

// NewPostgresSheetStore cria o store.
func NewPostgresSheetStore(pool *pgxpool.Pool) *PostgresSheetStore {
	return &PostgresSheetStore{pool: pool}
}

// idOrNew injeta o id validado ou gera um novo no SQL. Retorna a expressao e o arg.
const newIDExpr = "coalesce(nullif($%d,'')::uuid, gen_random_uuid())"

// tsString formata timestamptz para RFC3339 (contrato do front).
func tsString(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// dateArg converte 'YYYY-MM-DD' em *string para o banco (” vira NULL).
func dateArg(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	v := s
	return &v
}

// List devolve as planilhas da account (com clientName via join core.accounts).
func (r *PostgresSheetStore) List(ctx context.Context, accountID string, f ListFilter) ([]SheetListItem, int, error) {
	args := []any{accountID}
	conds := []string{"s.account_id = $1::uuid"}
	n := 2
	if f.CoreTenantID != "" {
		conds = append(conds, fmt.Sprintf("s.core_tenant_id = $%d", n))
		args = append(args, f.CoreTenantID)
		n++
	}
	if f.Period != "" {
		conds = append(conds, fmt.Sprintf("s.period = $%d", n))
		args = append(args, f.Period)
		n++
	}
	if f.Q != "" {
		pattern := "%" + strings.ToLower(f.Q) + "%"
		conds = append(conds, fmt.Sprintf("(lower(s.title) like $%d or lower(coalesce(a.name,'')) like $%d)", n, n))
		args = append(args, pattern)
		n++
	}
	where := strings.Join(conds, " and ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"select count(*) from finance.sheets s "+clientJoin()+" where "+where,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	q := fmt.Sprintf(`
		select s.id::text, s.title, s.period, s.status, s.notes, s.core_tenant_id,
		       coalesce(a.name,''), s.created_at, s.updated_at
		from finance.sheets s %s
		where %s
		order by s.updated_at desc
		limit $%d offset $%d`, clientJoin(), where, n, n+1)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	heads := make([]SheetListItem, 0)
	ids := make([]string, 0)
	for rows.Next() {
		var it SheetListItem
		var created, updated time.Time
		if err := rows.Scan(&it.ID, &it.Title, &it.Period, &it.Status, &it.Notes,
			&it.CoreTenantID, &it.ClientName, &created, &updated); err != nil {
			return nil, 0, err
		}
		it.CreatedAt = tsString(created)
		it.UpdatedAt = tsString(updated)
		heads = append(heads, it)
		ids = append(ids, it.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Summary/preview por planilha: carrega as linhas de todas de uma vez.
	linesByID, err := r.loadLinesForSheets(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range heads {
		var entradas, saidas []Line
		if grp := linesByID[heads[i].ID]; grp != nil {
			entradas, saidas = grp.entradas, grp.saidas
		}
		summary := computeSummary(entradas, saidas)
		heads[i].Summary = summary
		heads[i].Preview = computePreview(summary)
	}
	return heads, total, nil
}

// clientJoin resolve o nome do cliente quando core_tenant_id e uuid de account.
func clientJoin() string {
	return "left join core.accounts a on a.id = s.account_id and a.id::text = s.core_tenant_id"
}

type lineGroup struct {
	entradas []Line
	saidas   []Line
}

// loadLinesForSheets carrega linhas + adjustments de varias planilhas.
func (r *PostgresSheetStore) loadLinesForSheets(ctx context.Context, sheetIDs []string) (map[string]*lineGroup, error) {
	out := make(map[string]*lineGroup)
	if len(sheetIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		select l.id::text, l.sheet_id::text, l.kind, l.description, l.category,
		       l.effective, l.effective_date, l.amount, l.adjustment_amount,
		       l.fixed_account_id, l.details
		from finance.lines l
		where l.sheet_id = any($1::uuid[])
		order by l.sheet_id, l.position`, sheetIDs)
	if err != nil {
		return nil, err
	}
	lineIDs := make([]string, 0)
	func() {
		defer rows.Close()
		for rows.Next() {
			var l Line
			var sheetID string
			var effDate pgtype.Date
			if err = rows.Scan(&l.ID, &sheetID, &l.Kind, &l.Description, &l.Category,
				&l.Effective, &effDate, &l.Amount, &l.AdjustmentAmount,
				&l.FixedAccountID, &l.Details); err != nil {
				return
			}
			if effDate.Valid {
				l.EffectiveDate = effDate.Time.Format("2006-01-02")
			}
			l.Adjustments = []Adjustment{}
			grp := out[sheetID]
			if grp == nil {
				grp = &lineGroup{entradas: []Line{}, saidas: []Line{}}
				out[sheetID] = grp
			}
			if l.Kind == "saida" {
				grp.saidas = append(grp.saidas, l)
			} else {
				grp.entradas = append(grp.entradas, l)
			}
			lineIDs = append(lineIDs, l.ID)
		}
		err = rows.Err()
	}()
	if err != nil {
		return nil, err
	}
	// adjustments num map por lineID; atribui depois (evita ponteiro para elemento
	// de slice que o append pode realocar).
	adjByLine, err := r.loadAdjustments(ctx, lineIDs)
	if err != nil {
		return nil, err
	}
	for _, grp := range out {
		fillAdjustments(grp.entradas, adjByLine)
		fillAdjustments(grp.saidas, adjByLine)
	}
	return out, nil
}

// fillAdjustments atribui os adjustments carregados a cada linha por id.
func fillAdjustments(lines []Line, adjByLine map[string][]Adjustment) {
	for i := range lines {
		if adjs := adjByLine[lines[i].ID]; adjs != nil {
			lines[i].Adjustments = adjs
		}
	}
}

// loadAdjustments carrega os adjustments das linhas agrupados por lineID.
func (r *PostgresSheetStore) loadAdjustments(ctx context.Context, lineIDs []string) (map[string][]Adjustment, error) {
	out := make(map[string][]Adjustment)
	if len(lineIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		select a.id::text, a.line_id::text, a.amount, a.note, a.date
		from finance.line_adjustments a
		where a.line_id = any($1::uuid[])
		order by a.line_id, a.position`, lineIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var adj Adjustment
		var lineID string
		var date pgtype.Date
		if err := rows.Scan(&adj.ID, &lineID, &adj.Amount, &adj.Note, &date); err != nil {
			return nil, err
		}
		if date.Valid {
			adj.Date = date.Time.Format("2006-01-02")
		}
		out[lineID] = append(out[lineID], adj)
	}
	return out, rows.Err()
}

// Get devolve o detalhe da planilha (404 se fora da account).
func (r *PostgresSheetStore) Get(ctx context.Context, accountID, id string) (SheetDetail, error) {
	var d SheetDetail
	var created, updated time.Time
	err := r.pool.QueryRow(ctx, `
		select s.id::text, s.title, s.period, s.status, s.notes, s.core_tenant_id,
		       coalesce(a.name,''), s.created_at, s.updated_at
		from finance.sheets s `+clientJoin()+`
		where s.id = $1::uuid and s.account_id = $2::uuid`, id, accountID).
		Scan(&d.ID, &d.Title, &d.Period, &d.Status, &d.Notes, &d.CoreTenantID,
			&d.ClientName, &created, &updated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SheetDetail{}, ErrSheetNotFound
		}
		return SheetDetail{}, err
	}
	d.CreatedAt = tsString(created)
	d.UpdatedAt = tsString(updated)
	grp, err := r.loadLinesForSheets(ctx, []string{d.ID})
	if err != nil {
		return SheetDetail{}, err
	}
	if g := grp[d.ID]; g != nil {
		d.Entradas, d.Saidas = g.entradas, g.saidas
	} else {
		d.Entradas, d.Saidas = []Line{}, []Line{}
	}
	d.Summary = computeSummary(d.Entradas, d.Saidas)
	d.Preview = computePreview(d.Summary)
	return d, nil
}
