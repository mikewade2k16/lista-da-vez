package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// securityFinding é um achado da auditoria como item de fila de trabalho: além do
// que vem da AUDITORIA, carrega responsável, prazo e nota editados no painel.
type securityFinding struct {
	ID        string    `json:"id"`
	Severity  string    `json:"severity"`
	Title     string    `json:"title"`
	Area      string    `json:"area"`
	Location  string    `json:"location"`
	Status    string    `json:"status"`
	Owner     string    `json:"owner"`
	DueDate   *string   `json:"dueDate"`
	Note      string    `json:"note"`
	UpdatedBy string    `json:"updatedBy"`
	UpdatedAt time.Time `json:"updatedAt"`
	Overdue   bool      `json:"overdue"`
}

const findingColumns = `id, severity, title, area, location, status, owner,
	to_char(due_date, 'YYYY-MM-DD'), note, updated_by, updated_at`

func scanFinding(row pgx.Row) (securityFinding, error) {
	var f securityFinding
	err := row.Scan(&f.ID, &f.Severity, &f.Title, &f.Area, &f.Location, &f.Status, &f.Owner,
		&f.DueDate, &f.Note, &f.UpdatedBy, &f.UpdatedAt)
	return f, err
}

// SecurityFindings lista os achados (abertos primeiro, por severidade). `today` é a
// data local (YYYY-MM-DD) usada para marcar os vencidos.
func (s *Store) SecurityFindings(ctx context.Context, today string) ([]securityFinding, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select `+findingColumns+` from obs.findings
		order by (status = 'open') desc,
		         case severity when 'critical' then 4 when 'high' then 3 when 'medium' then 2 else 1 end desc, id`)
	if err != nil {
		return nil, fmt.Errorf("security findings: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (securityFinding, error) {
		return scanFinding(row)
	})
	if items == nil {
		items = []securityFinding{}
	}
	for i := range items {
		items[i].Overdue = findingOverdue(items[i].Status, items[i].DueDate, today)
	}
	return items, err
}

// UpdateFinding aplica só os campos informados e grava quem alterou. ErrNotFound se
// o achado não existir. A coluna status e as demais da AUDITORIA seguem a regra do
// sync (observability/runner/security-db.mjs): o status editado aqui nunca é desfeito.
func (s *Store) UpdateFinding(ctx context.Context, id string, patch findingPatch, actor alertActor, today string) (securityFinding, error) {
	var dueSet bool
	var due *string
	if patch.DueDate.Set {
		dueSet, due = true, patch.DueDate.Value
	}
	row := s.pool.QueryRow(quiet(ctx), `
		update obs.findings set
		  status = coalesce($2, status),
		  owner = coalesce($3, owner),
		  due_date = case when $4::boolean then $5::date else due_date end,
		  note = coalesce($6, note),
		  updated_by = coalesce((select u.display_name from core.users u where u.id = nullif($7, '')::uuid), $8),
		  updated_at = now()
		where id = $1
		returning `+findingColumns,
		id, patch.Status, patch.Owner, dueSet, due, patch.Note, actor.UserID, actor.Name)
	f, err := scanFinding(row)
	if err == pgx.ErrNoRows {
		return securityFinding{}, ErrNotFound
	}
	if err != nil {
		return securityFinding{}, fmt.Errorf("update finding: %w", err)
	}
	f.Overdue = findingOverdue(f.Status, f.DueDate, today)
	return f, nil
}
