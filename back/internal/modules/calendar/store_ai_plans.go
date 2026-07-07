package calendar

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// Planos de IA (Fase 6) — calendar.ai_plans
// ============================================================================

// aiPlanCols e a ordem esperada por scanAIPlan. jsonb com coalesce para shape estavel.
const aiPlanCols = `id::text, account_id::text, month_key,
	coalesce(client_ids, '[]'::jsonb), status, provider, model,
	coalesce(content, '{}'::jsonb), error, created_by, created_at, updated_at`

func scanAIPlan(row rowScanner) (AIPlan, error) {
	var p AIPlan
	var clientIDs, content json.RawMessage
	err := row.Scan(&p.ID, &p.AccountID, &p.Month, &clientIDs, &p.Status,
		&p.Provider, &p.Model, &content, &p.Error, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return p, err
	}
	p.ClientIDs = decodeStringArray(clientIDs)
	p.Content = decodeContent(content)
	return p, nil
}

// CreateAIPlan insere um plano pending na account.
func (s *Store) CreateAIPlan(ctx context.Context, p AIPlan) (AIPlan, error) {
	const q = `
		insert into calendar.ai_plans
			(account_id, month_key, client_ids, status, provider, model, created_by)
		values ($1::uuid, $2, $3::jsonb, $4, $5, $6, $7)
		returning ` + aiPlanCols
	return scanAIPlan(s.pool.QueryRow(ctx, q,
		p.AccountID, p.Month, jsonArray(p.ClientIDs), p.Status, p.Provider, p.Model, p.CreatedBy))
}

// GetAIPlan le um plano no escopo da account (defesa em profundidade). Fora do
// escopo => pgx.ErrNoRows (mapeado para 404 pelo service).
func (s *Store) GetAIPlan(ctx context.Context, accountID, id string) (AIPlan, error) {
	const q = `select ` + aiPlanCols + `
		from calendar.ai_plans where id = $1::uuid and account_id = $2::uuid`
	return scanAIPlan(s.pool.QueryRow(ctx, q, id, accountID))
}

// GetAIPlanByID le um plano so pelo id, sem escopo de account (callback publico
// do n8n, que nao tem contexto de conta). Inexistente => pgx.ErrNoRows.
func (s *Store) GetAIPlanByID(ctx context.Context, id string) (AIPlan, error) {
	const q = `select ` + aiPlanCols + ` from calendar.ai_plans where id = $1::uuid`
	return scanAIPlan(s.pool.QueryRow(ctx, q, id))
}

// ListAIPlans devolve o indice lean dos planos da account (opcional por mes),
// mais recentes primeiro. Nao carrega o content (projecao lean).
func (s *Store) ListAIPlans(ctx context.Context, accountID, month string) ([]AIPlanIndexItem, error) {
	q := `select id::text, month_key, coalesce(client_ids, '[]'::jsonb), status,
		provider, model, created_at
		from calendar.ai_plans where account_id = $1::uuid`
	args := []any{accountID}
	if month != "" {
		args = append(args, month)
		q += " and month_key = $2"
	}
	q += " order by created_at desc"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AIPlanIndexItem, 0)
	for rows.Next() {
		var it AIPlanIndexItem
		var clientIDs json.RawMessage
		if err := rows.Scan(&it.ID, &it.Month, &clientIDs, &it.Status,
			&it.Provider, &it.Model, &it.CreatedAt); err != nil {
			return nil, err
		}
		it.ClientIDs = decodeStringArray(clientIDs)
		out = append(out, it)
	}
	return out, rows.Err()
}

// SetAIPlanResult transiciona pending -> done|error. Guarda a transicao no proprio
// UPDATE (where status = pending): plano ja done/applied => 0 linhas => ErrNoRows
// (mapeado para 404/409 pelo service). accountID vazio (callback publico) => filtra
// so pelo id; preenchido => defesa em profundidade adicional.
func (s *Store) SetAIPlanResult(ctx context.Context, accountID, id, status string, content AIPlanContent, planErr string) (AIPlan, error) {
	q := `
		update calendar.ai_plans set
			status = $2, content = $3::jsonb, error = $4, updated_at = now()
		where id = $1::uuid and status = 'pending'`
	args := []any{id, status, marshalContent(content), planErr}
	if accountID != "" {
		args = append(args, accountID)
		q += " and account_id = $5::uuid"
	}
	q += " returning " + aiPlanCols
	return scanAIPlan(s.pool.QueryRow(ctx, q, args...))
}

// MarkAIPlanApplied transiciona done -> applied no escopo da account. Plano em
// outro estado ou fora do escopo => ErrNoRows (404/409 pelo service).
func (s *Store) MarkAIPlanApplied(ctx context.Context, accountID, id string) (AIPlan, error) {
	const q = `
		update calendar.ai_plans set status = 'applied', updated_at = now()
		where id = $1::uuid and account_id = $2::uuid and status = 'done'
		returning ` + aiPlanCols
	return scanAIPlan(s.pool.QueryRow(ctx, q, id, accountID))
}

// DeleteAIPlan remove um plano no escopo da account. Nada apagado => ErrNoRows.
func (s *Store) DeleteAIPlan(ctx context.Context, accountID, id string) error {
	const q = `delete from calendar.ai_plans where id = $1::uuid and account_id = $2::uuid`
	tag, err := s.pool.Exec(ctx, q, id, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// decodeStringArray desserializa um jsonb de strings; falha/nulo -> lista vazia.
func decodeStringArray(raw json.RawMessage) []string {
	out := []string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	if out == nil {
		out = []string{}
	}
	return out
}
