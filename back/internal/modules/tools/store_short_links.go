package tools

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresShortLinkStore persiste links curtos em tools.short_links.
type PostgresShortLinkStore struct {
	pool *pgxpool.Pool
}

// NewPostgresShortLinkStore cria o store.
func NewPostgresShortLinkStore(pool *pgxpool.Pool) *PostgresShortLinkStore {
	return &PostgresShortLinkStore{pool: pool}
}

// ---------------------------------------------------------------------------
// Helpers compartilhados dos stores do modulo (usados tambem por qr_codes).
// ---------------------------------------------------------------------------

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isUUID reporta se s ja e um uuid canonico (evita erro de cast ::uuid no SQL).
func isUUID(s string) bool { return uuidPattern.MatchString(strings.TrimSpace(s)) }

// tsString formata timestamptz para RFC3339 (contrato do front).
func tsString(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// ptrTimeString formata um timestamptz nullable; nil vira "".
func ptrTimeString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// isUniqueViolation reporta se o erro e uma violacao de unique (slug duplicado).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isForeignKeyViolation reporta violacao de FK (account_id inexistente no create
// cross-conta do admin) — mapeada para 400 no handler.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// ---------------------------------------------------------------------------
// Short links
// ---------------------------------------------------------------------------

// shortReturning e a projecao de uma linha (com clientName via subquery).
const shortReturning = `id::text, slug, target_url, hits, account_id::text, created_at,
	coalesce((select name from core.accounts where id = account_id), '')`

// scanShortRow le uma linha na ordem de shortReturning.
func scanShortRow(row pgx.Row) (ShortLinkItem, error) {
	var it ShortLinkItem
	var created time.Time
	if err := row.Scan(&it.ID, &it.Slug, &it.TargetURL, &it.Hits, &it.AccountID, &created,
		&it.ClientName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ShortLinkItem{}, ErrShortLinkNotFound
		}
		return ShortLinkItem{}, err
	}
	it.CreatedAt = tsString(created)
	return it, nil
}

// List devolve os links da account (accountID "" = todas as contas) com filtro.
func (r *PostgresShortLinkStore) List(ctx context.Context, accountID string, f ListFilter) ([]ShortLinkItem, int, error) {
	args := []any{}
	conds := []string{}
	n := 1
	if accountID != "" {
		conds = append(conds, fmt.Sprintf("s.account_id = $%d::uuid", n))
		args = append(args, accountID)
		n++
	}
	if f.Q != "" {
		pattern := "%" + strings.ToLower(f.Q) + "%"
		conds = append(conds, fmt.Sprintf(
			"(lower(s.slug) like $%d or lower(s.target_url) like $%d or lower(coalesce(a.name,'')) like $%d)", n, n, n))
		args = append(args, pattern)
		n++
	}
	where := "true"
	if len(conds) > 0 {
		where = strings.Join(conds, " and ")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`select count(*) from tools.short_links s join core.accounts a on a.id = s.account_id where `+where,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	q := fmt.Sprintf(`
		select s.id::text, s.slug, s.target_url, s.hits, s.account_id::text,
		       coalesce(a.name,''), s.created_at
		from tools.short_links s join core.accounts a on a.id = s.account_id
		where %s
		order by s.created_at desc
		limit $%d offset $%d`, where, n, n+1)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]ShortLinkItem, 0)
	for rows.Next() {
		var it ShortLinkItem
		var created time.Time
		if err := rows.Scan(&it.ID, &it.Slug, &it.TargetURL, &it.Hits, &it.AccountID,
			&it.ClientName, &created); err != nil {
			return nil, 0, err
		}
		it.CreatedAt = tsString(created)
		items = append(items, it)
	}
	return items, total, rows.Err()
}

// Create insere o link garantindo slug unico (sufixo -2/-3 em colisao).
func (r *PostgresShortLinkStore) Create(ctx context.Context, accountID, slug, targetURL string) (ShortLinkItem, error) {
	base := slug
	if base == "" {
		base = randomCode(7)
	}
	candidate := base
	const maxAttempts = 60
	for attempt := 0; attempt < maxAttempts; attempt++ {
		it, err := r.insert(ctx, accountID, candidate, targetURL)
		if err == nil {
			return it, nil
		}
		if isUniqueViolation(err) {
			candidate = fmt.Sprintf("%s-%d", base, attempt+2)
			continue
		}
		return ShortLinkItem{}, err
	}
	// Esgotou os sufixos: um codigo aleatorio novo praticamente nao colide.
	return r.insert(ctx, accountID, randomCode(10), targetURL)
}

func (r *PostgresShortLinkStore) insert(ctx context.Context, accountID, slug, targetURL string) (ShortLinkItem, error) {
	const q = `
		insert into tools.short_links (account_id, slug, target_url)
		values ($1::uuid, $2, $3)
		returning ` + shortReturning
	return scanShortRow(r.pool.QueryRow(ctx, q, accountID, slug, targetURL))
}

// Update aplica o patch parcial. Se o slug muda, garante unicidade (sufixo).
// accountID "" = por id (admin); senao id + account_id (isolamento).
func (r *PostgresShortLinkStore) Update(ctx context.Context, id, accountID string, p ShortLinkPatch) (ShortLinkItem, error) {
	if !isUUID(id) {
		return ShortLinkItem{}, ErrShortLinkNotFound
	}
	set := []string{"updated_at = now()"}
	args := []any{id}
	n := 2
	if p.TargetURL != nil {
		set = append(set, fmt.Sprintf("target_url = $%d", n))
		args = append(args, *p.TargetURL)
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
	}

	query := fmt.Sprintf(
		`update tools.short_links set %s where id = $1::uuid%s returning `+shortReturning,
		strings.Join(set, ", "), scope)

	if slugIdx < 0 {
		return scanShortRow(r.pool.QueryRow(ctx, query, args...))
	}

	candidate := base
	const maxAttempts = 60
	for attempt := 0; attempt < maxAttempts; attempt++ {
		args[slugIdx] = candidate
		it, err := scanShortRow(r.pool.QueryRow(ctx, query, args...))
		if err == nil {
			return it, nil
		}
		if isUniqueViolation(err) {
			candidate = fmt.Sprintf("%s-%d", base, attempt+2)
			continue
		}
		return ShortLinkItem{}, err
	}
	args[slugIdx] = randomCode(10)
	return scanShortRow(r.pool.QueryRow(ctx, query, args...))
}

// Delete remove o link. accountID "" = por id (admin); senao id + account_id.
func (r *PostgresShortLinkStore) Delete(ctx context.Context, id, accountID string) error {
	if !isUUID(id) {
		return ErrShortLinkNotFound
	}
	var tag pgconn.CommandTag
	var err error
	if accountID == "" {
		tag, err = r.pool.Exec(ctx, `delete from tools.short_links where id = $1::uuid`, id)
	} else {
		tag, err = r.pool.Exec(ctx,
			`delete from tools.short_links where id = $1::uuid and account_id = $2::uuid`, id, accountID)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrShortLinkNotFound
	}
	return nil
}

// Resolve incrementa hits e devolve o destino, apenas se a conta dona esta ativa
// e com o modulo tools habilitado. pgx.ErrNoRows quando qualquer condicao falha
// (nao vaza existencia).
func (r *PostgresShortLinkStore) Resolve(ctx context.Context, slug string) (string, error) {
	const q = `
		update tools.short_links s
		set hits = hits + 1, updated_at = now()
		from core.accounts a, core.account_modules m
		where s.slug = $1
		  and a.id = s.account_id
		  and m.account_id = s.account_id
		  and a.is_active = true
		  and m.module_id = 'tools'
		  and m.enabled = true
		returning s.target_url`
	var target string
	if err := r.pool.QueryRow(ctx, q, slug).Scan(&target); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrShortLinkNotFound
		}
		return "", err
	}
	return target, nil
}
