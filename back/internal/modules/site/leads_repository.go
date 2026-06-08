package site

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresLeadRepository implementa LeadRepository contra site.leads.
type PostgresLeadRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresLeadRepository cria o repo.
func NewPostgresLeadRepository(pool *pgxpool.Pool) *PostgresLeadRepository {
	return &PostgresLeadRepository{pool: pool}
}

// ============================================================================
// List
// ============================================================================

func (r *PostgresLeadRepository) List(ctx context.Context, filter LeadListFilter) ([]LeadView, int, error) {
	args := []any{filter.AccountID}
	conds := []string{"l.account_id = $1::uuid", "l.is_active = true"}
	n := 2

	if filter.Q != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(filter.Q)) + "%"
		conds = append(conds, fmt.Sprintf(
			"(lower(l.nome) like $%d or lower(l.email) like $%d or lower(l.telefone) like $%d or lower(l.cupom) like $%d)",
			n, n, n, n,
		))
		args = append(args, pattern)
		n++
	}
	if filter.Status != "" {
		conds = append(conds, fmt.Sprintf("l.status = $%d", n))
		args = append(args, filter.Status)
		n++
	}
	if filter.SourceID != "" {
		conds = append(conds, fmt.Sprintf("l.source_id = $%d::uuid", n))
		args = append(args, filter.SourceID)
		n++
	}

	where := strings.Join(conds, " and ")

	var total int
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("select count(*) from site.leads l where %s", where),
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}

	args = append(args, perPage, (page-1)*perPage)
	dataSQL := fmt.Sprintf(`
		select l.id, l.account_id, l.source_id, l.source_label,
		       l.nome, l.email, l.telefone, l.page, l.cupom,
		       l.consent, l.consent_label, l.tracking_data::text, l.payload_raw::text,
		       l.status, l.notes, l.created_at, l.updated_at
		from site.leads l
		where %s
		order by l.created_at desc
		limit $%d offset $%d
	`, where, n, n+1)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	leads := make([]LeadView, 0)
	for rows.Next() {
		v, err := scanLead(rows)
		if err != nil {
			return nil, 0, err
		}
		leads = append(leads, v)
	}
	return leads, total, rows.Err()
}

// ============================================================================
// Find
// ============================================================================

func (r *PostgresLeadRepository) Find(ctx context.Context, accountID, leadID string) (LeadView, error) {
	const query = `
		select l.id, l.account_id, l.source_id, l.source_label,
		       l.nome, l.email, l.telefone, l.page, l.cupom,
		       l.consent, l.consent_label, l.tracking_data::text, l.payload_raw::text,
		       l.status, l.notes, l.created_at, l.updated_at
		from site.leads l
		where l.account_id = $1::uuid and l.id = $2::uuid and l.is_active = true
	`
	row := r.pool.QueryRow(ctx, query, accountID, leadID)
	v, err := scanLead(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LeadView{}, ErrLeadNotFound
		}
		return LeadView{}, err
	}
	return v, nil
}

// ============================================================================
// Create (manual)
// ============================================================================

func (r *PostgresLeadRepository) Create(ctx context.Context, accountID string, input LeadCreateInput) (LeadView, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		insert into site.leads (
		    account_id, source_label, nome, email, telefone, page, cupom,
		    consent, consent_label, notes, status, is_active, created_at, updated_at
		) values ($1::uuid, $2, $3, lower($4), $5, $6, $7, $8, $9, $10, 'new', true, now(), now())
		returning id
	`, accountID, input.SourceLabel, input.Nome, input.Email, input.Telefone,
		input.Page, input.Cupom, input.Consent, input.ConsentLabel, input.Notes).Scan(&id)
	if err != nil {
		return LeadView{}, err
	}
	return r.Find(ctx, accountID, id)
}

// ============================================================================
// CreateFromWebhook
// ============================================================================

func (r *PostgresLeadRepository) CreateFromWebhook(ctx context.Context, accountID, sourceID, sourceLabel string, fields map[string]any, raw string) (LeadView, error) {
	getStr := func(key string) string {
		if v, ok := fields[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
	consent := false
	if v, ok := fields["consent"]; ok {
		if b, ok := v.(bool); ok {
			consent = b
		}
	}
	trackingJSON := ""
	if v, ok := fields["tracking_data"]; ok {
		if b, err := json.Marshal(v); err == nil {
			trackingJSON = string(b)
		}
	}

	var id string
	err := r.pool.QueryRow(ctx, `
		insert into site.leads (
		    account_id, source_id, source_label,
		    nome, email, telefone, page, cupom, consent, consent_label,
		    tracking_data, payload_raw, status, is_active, created_at, updated_at
		) values (
		    $1::uuid, $2::uuid, $3,
		    $4, lower($5), $6, $7, $8, $9, $10,
		    nullif($11, '')::jsonb, nullif($12, '')::jsonb, 'new', true, now(), now()
		)
		returning id
	`, accountID, sourceID, sourceLabel,
		getStr("nome"), getStr("email"), getStr("telefone"),
		getStr("page"), getStr("cupom"), consent, getStr("consent_label"),
		trackingJSON, raw).Scan(&id)
	if err != nil {
		return LeadView{}, err
	}
	return r.Find(ctx, accountID, id)
}

// ============================================================================
// Update
// ============================================================================

func (r *PostgresLeadRepository) Update(ctx context.Context, accountID, leadID string, input LeadUpdateInput) (LeadView, error) {
	sets := []string{}
	args := []any{accountID, leadID}
	n := 3

	addSet := func(column string, value any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", column, n))
		args = append(args, value)
		n++
	}

	if input.Nome != nil {
		addSet("nome", *input.Nome)
	}
	if input.Email != nil {
		addSet("email", strings.ToLower(strings.TrimSpace(*input.Email)))
	}
	if input.Telefone != nil {
		addSet("telefone", *input.Telefone)
	}
	if input.Status != nil {
		addSet("status", *input.Status)
	}
	if input.Notes != nil {
		addSet("notes", *input.Notes)
	}

	if len(sets) == 0 {
		return r.Find(ctx, accountID, leadID)
	}

	sets = append(sets, fmt.Sprintf("updated_at = $%d", n))
	args = append(args, time.Now())

	query := fmt.Sprintf(`
		update site.leads set %s
		where account_id = $1::uuid and id = $2::uuid and is_active = true
	`, strings.Join(sets, ", "))

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return LeadView{}, err
	}
	if tag.RowsAffected() == 0 {
		return LeadView{}, ErrLeadNotFound
	}
	return r.Find(ctx, accountID, leadID)
}

// ============================================================================
// SoftDelete
// ============================================================================

func (r *PostgresLeadRepository) SoftDelete(ctx context.Context, accountID, leadID string) error {
	tag, err := r.pool.Exec(ctx,
		`update site.leads set is_active = false, updated_at = now() where account_id = $1::uuid and id = $2::uuid`,
		accountID, leadID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLeadNotFound
	}
	return nil
}

// ============================================================================
// Scanner
// ============================================================================

type scannable interface {
	Scan(dest ...any) error
}

func scanLead(row scannable) (LeadView, error) {
	var v LeadView
	var sourceID, tracking, payload *string
	if err := row.Scan(
		&v.ID, &v.AccountID, &sourceID, &v.SourceLabel,
		&v.Nome, &v.Email, &v.Telefone, &v.Page, &v.Cupom,
		&v.Consent, &v.ConsentLabel, &tracking, &payload,
		&v.Status, &v.Notes, &v.CreatedAt, &v.UpdatedAt,
	); err != nil {
		return LeadView{}, err
	}
	if sourceID != nil {
		v.SourceID = *sourceID
	}
	if tracking != nil {
		v.TrackingData = *tracking
	}
	if payload != nil {
		v.PayloadRaw = *payload
	}
	return v, nil
}
