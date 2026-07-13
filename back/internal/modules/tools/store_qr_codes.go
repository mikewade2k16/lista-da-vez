package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresQrCodeStore persiste QR codes em tools.qr_codes.
type PostgresQrCodeStore struct {
	pool *pgxpool.Pool
}

// NewPostgresQrCodeStore cria o store.
func NewPostgresQrCodeStore(pool *pgxpool.Pool) *PostgresQrCodeStore {
	return &PostgresQrCodeStore{pool: pool}
}

// qrReturning e a projecao completa de uma linha (com clientName via subquery).
const qrReturning = `id::text, slug, target_url, fill_color, back_color, size,
	is_active, scan_count, last_scanned_at, account_id::text, created_at,
	coalesce((select name from core.accounts where id = account_id), '')`

// scanQrRow le uma linha na ordem de qrReturning.
func scanQrRow(row pgx.Row) (QrCodeItem, error) {
	var it QrCodeItem
	var created time.Time
	var lastScanned *time.Time
	if err := row.Scan(&it.ID, &it.Slug, &it.TargetURL, &it.FillColor, &it.BackColor, &it.Size,
		&it.IsActive, &it.ScanCount, &lastScanned, &it.AccountID, &created, &it.ClientName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return QrCodeItem{}, ErrQrCodeNotFound
		}
		return QrCodeItem{}, err
	}
	it.CreatedAt = tsString(created)
	it.LastScannedAt = ptrTimeString(lastScanned)
	return it, nil
}

// List devolve os QR da account (accountID "" = todas) com filtro de busca/status.
func (r *PostgresQrCodeStore) List(ctx context.Context, accountID string, f ListFilter) ([]QrCodeItem, int, error) {
	args := []any{}
	conds := []string{}
	n := 1
	if accountID != "" {
		conds = append(conds, fmt.Sprintf("q.account_id = $%d::uuid", n))
		args = append(args, accountID)
		n++
	}
	switch f.Status {
	case "active":
		conds = append(conds, "q.is_active = true")
	case "inactive":
		conds = append(conds, "q.is_active = false")
	}
	if f.Q != "" {
		pattern := "%" + strings.ToLower(f.Q) + "%"
		conds = append(conds, fmt.Sprintf(
			"(lower(q.slug) like $%d or lower(q.target_url) like $%d or lower(coalesce(a.name,'')) like $%d)", n, n, n))
		args = append(args, pattern)
		n++
	}
	where := "true"
	if len(conds) > 0 {
		where = strings.Join(conds, " and ")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`select count(*) from tools.qr_codes q join core.accounts a on a.id = q.account_id where `+where,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	q := fmt.Sprintf(`
		select q.id::text, q.slug, q.target_url, q.fill_color, q.back_color, q.size,
		       q.is_active, q.scan_count, q.last_scanned_at, q.account_id::text,
		       q.created_at, coalesce(a.name,'')
		from tools.qr_codes q join core.accounts a on a.id = q.account_id
		where %s
		order by q.created_at desc
		limit $%d offset $%d`, where, n, n+1)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]QrCodeItem, 0)
	for rows.Next() {
		var it QrCodeItem
		var created time.Time
		var lastScanned *time.Time
		if err := rows.Scan(&it.ID, &it.Slug, &it.TargetURL, &it.FillColor, &it.BackColor, &it.Size,
			&it.IsActive, &it.ScanCount, &lastScanned, &it.AccountID, &created, &it.ClientName); err != nil {
			return nil, 0, err
		}
		it.CreatedAt = tsString(created)
		it.LastScannedAt = ptrTimeString(lastScanned)
		items = append(items, it)
	}
	return items, total, rows.Err()
}

// Create insere o QR garantindo slug unico (sufixo -2/-3 em colisao).
func (r *PostgresQrCodeStore) Create(ctx context.Context, accountID string, rec QrCodeRecord) (QrCodeItem, error) {
	base := rec.Slug
	if base == "" {
		base = randomCode(7)
	}
	candidate := base
	const maxAttempts = 60
	for attempt := 0; attempt < maxAttempts; attempt++ {
		it, err := r.insert(ctx, accountID, candidate, rec)
		if err == nil {
			return it, nil
		}
		if isUniqueViolation(err) {
			candidate = fmt.Sprintf("%s-%d", base, attempt+2)
			continue
		}
		return QrCodeItem{}, err
	}
	return r.insert(ctx, accountID, randomCode(10), rec)
}

func (r *PostgresQrCodeStore) insert(ctx context.Context, accountID, slug string, rec QrCodeRecord) (QrCodeItem, error) {
	const q = `
		insert into tools.qr_codes (account_id, slug, target_url, fill_color, back_color, size, is_active)
		values ($1::uuid, $2, $3, $4, $5, $6, $7)
		returning ` + qrReturning
	return scanQrRow(r.pool.QueryRow(ctx, q, accountID, slug, rec.TargetURL,
		rec.FillColor, rec.BackColor, rec.Size, rec.IsActive))
}

// Update aplica o patch parcial. Se o slug muda, garante unicidade (sufixo).
// accountID "" = por id (admin); senao id + account_id (isolamento).
func (r *PostgresQrCodeStore) Update(ctx context.Context, id, accountID string, p QrCodePatch) (QrCodeItem, error) {
	if !isUUID(id) {
		return QrCodeItem{}, ErrQrCodeNotFound
	}
	set := []string{"updated_at = now()"}
	args := []any{id}
	n := 2
	addStr := func(col string, v string) {
		set = append(set, fmt.Sprintf("%s = $%d", col, n))
		args = append(args, v)
		n++
	}
	if p.TargetURL != nil {
		addStr("target_url", *p.TargetURL)
	}
	if p.FillColor != nil {
		addStr("fill_color", *p.FillColor)
	}
	if p.BackColor != nil {
		addStr("back_color", *p.BackColor)
	}
	if p.Size != nil {
		set = append(set, fmt.Sprintf("size = $%d", n))
		args = append(args, *p.Size)
		n++
	}
	if p.IsActive != nil {
		set = append(set, fmt.Sprintf("is_active = $%d", n))
		args = append(args, *p.IsActive)
		n++
	}

	// slug: pode precisar de retry por unicidade — guarda a posicao no args.
	base := ""
	slugIdx := -1
	if p.Slug != nil {
		base = *p.Slug
		if base == "" {
			base = randomCode(7)
		}
		set = append(set, fmt.Sprintf("slug = $%d", n))
		args = append(args, base)
		slugIdx = len(args) - 1
		n++
	}

	scope := ""
	if accountID != "" {
		scope = fmt.Sprintf(" and account_id = $%d::uuid", n)
		args = append(args, accountID)
		n++
	}

	query := fmt.Sprintf(
		`update tools.qr_codes set %s where id = $1::uuid%s returning %s`,
		strings.Join(set, ", "), scope, qrReturning)

	if slugIdx < 0 {
		return scanQrRow(r.pool.QueryRow(ctx, query, args...))
	}

	candidate := base
	const maxAttempts = 60
	for attempt := 0; attempt < maxAttempts; attempt++ {
		args[slugIdx] = candidate
		it, err := scanQrRow(r.pool.QueryRow(ctx, query, args...))
		if err == nil {
			return it, nil
		}
		if isUniqueViolation(err) {
			candidate = fmt.Sprintf("%s-%d", base, attempt+2)
			continue
		}
		return QrCodeItem{}, err
	}
	args[slugIdx] = randomCode(10)
	return scanQrRow(r.pool.QueryRow(ctx, query, args...))
}

// Delete remove o QR. accountID "" = por id (admin); senao id + account_id.
func (r *PostgresQrCodeStore) Delete(ctx context.Context, id, accountID string) error {
	if !isUUID(id) {
		return ErrQrCodeNotFound
	}
	var tag pgconn.CommandTag
	var err error
	if accountID == "" {
		tag, err = r.pool.Exec(ctx, `delete from tools.qr_codes where id = $1::uuid`, id)
	} else {
		tag, err = r.pool.Exec(ctx,
			`delete from tools.qr_codes where id = $1::uuid and account_id = $2::uuid`, id, accountID)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrQrCodeNotFound
	}
	return nil
}

// Resolve incrementa scan_count/last_scanned_at e devolve o destino, apenas se o
// QR esta ativo e a conta dona ativa com o modulo tools habilitado. pgx.ErrNoRows
// quando qualquer condicao falha (nao vaza existencia).
func (r *PostgresQrCodeStore) Resolve(ctx context.Context, slug string) (string, error) {
	const q = `
		update tools.qr_codes q
		set scan_count = scan_count + 1, last_scanned_at = now()
		from core.accounts a, core.account_modules m
		where q.slug = $1
		  and q.is_active = true
		  and a.id = q.account_id
		  and m.account_id = q.account_id
		  and a.is_active = true
		  and m.module_id = 'tools'
		  and m.enabled = true
		returning q.target_url`
	var target string
	if err := r.pool.QueryRow(ctx, q, slug).Scan(&target); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrQrCodeNotFound
		}
		return "", err
	}
	return target, nil
}
