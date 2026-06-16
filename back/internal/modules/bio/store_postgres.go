package bio

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store e a persistencia do modulo (schema bio.*).
type Store struct {
	pool *pgxpool.Pool
}

// NewStore cria o Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type rowScanner interface {
	Scan(dest ...any) error
}

// ============================================================================
// bio.bios
// ============================================================================

// List retorna a projecao lean das bios. accountID vazio = todas (admin); caso
// contrario filtra a account. status e q sao filtros opcionais.
func (s *Store) List(ctx context.Context, f ListFilter) ([]BioSummary, error) {
	const base = `
		select b.id, b.account_id, coalesce(a.name, ''), b.slug, b.name,
		       b.status, b.updated_at, b.published_at
		from bio.bios b
		join core.accounts a on a.id = b.account_id`

	conds := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if strings.TrimSpace(f.AccountID) != "" {
		args = append(args, f.AccountID)
		conds = append(conds, "b.account_id = $1::uuid")
	}
	if strings.TrimSpace(f.Status) != "" {
		args = append(args, f.Status)
		conds = append(conds, paramRef("b.status = ", len(args)))
	}
	if strings.TrimSpace(f.Q) != "" {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(f.Q))+"%")
		conds = append(conds, paramLike(len(args)))
	}

	query := base
	if len(conds) > 0 {
		query += " where " + strings.Join(conds, " and ")
	}
	query += " order by b.updated_at desc"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BioSummary
	for rows.Next() {
		var b Bio
		if err := rows.Scan(&b.ID, &b.AccountID, &b.AccountName, &b.Slug, &b.Name,
			&b.Status, &b.UpdatedAt, &b.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, b.summary())
	}
	return out, rows.Err()
}

// paramRef monta "<prefix>$N" para clausulas simples.
func paramRef(prefix string, n int) string {
	return prefix + "$" + strconv.Itoa(n)
}

// paramLike monta a clausula de busca por nome/slug com o parametro $N.
func paramLike(n int) string {
	p := "$" + strconv.Itoa(n)
	return "(lower(b.name) like " + p + " or lower(b.slug) like " + p + ")"
}

// GetByID retorna a bio completa (com jsonb) + nome da account. accountID vazio
// = sem filtro de escopo (admin). Filtra account_id quando informado para
// defesa em profundidade.
func (s *Store) GetByID(ctx context.Context, id, accountID string) (Bio, error) {
	query := `
		select b.id, b.account_id, coalesce(a.name, ''), b.slug, b.name, b.status,
		       b.data_draft, b.data_published, b.published_at, b.created_at, b.updated_at
		from bio.bios b
		join core.accounts a on a.id = b.account_id
		where b.id = $1::uuid`
	args := []any{id}
	if strings.TrimSpace(accountID) != "" {
		args = append(args, accountID)
		query += " and b.account_id = $2::uuid"
	}
	return scanBio(s.pool.QueryRow(ctx, query, args...))
}

// Create insere uma nova bio em estado draft (data_draft vazio).
func (s *Store) Create(ctx context.Context, accountID, slug, name string) (Bio, error) {
	return s.CreateWithDraft(ctx, accountID, slug, name, json.RawMessage("{}"))
}

// CreateWithDraft insere uma nova bio em estado draft ja com o data_draft
// informado (usado na duplicacao, que copia o draft da origem). draft vazio/nil
// vira "{}".
func (s *Store) CreateWithDraft(ctx context.Context, accountID, slug, name string, draft json.RawMessage) (Bio, error) {
	const q = `
		insert into bio.bios (account_id, slug, name, status, data_draft)
		values ($1::uuid, $2, $3, 'draft', $4::jsonb)
		returning id, account_id, '', slug, name, status,
		          data_draft, data_published, published_at, created_at, updated_at`
	return scanBio(s.pool.QueryRow(ctx, q, accountID, slug, name, []byte(normalizeRaw(draft))))
}

// AccountExists informa se a account existe (validacao do destino ao mover uma
// bio de account). A FK ja protege na escrita; este check da um erro 404 limpo
// antes do update.
func (s *Store) AccountExists(ctx context.Context, accountID string) (bool, error) {
	const q = `select 1 from core.accounts where id = $1::uuid limit 1`
	var dummy int
	err := s.pool.QueryRow(ctx, q, accountID).Scan(&dummy)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	default:
		return false, err
	}
}

// EnsureBioModuleEnabled garante que a account tenha o modulo `bio` habilitado
// em core.account_modules. Criar uma bio para uma account = essa account passa a
// ter o modulo bio (senao o endpoint publico, que exige o modulo habilitado,
// devolveria 404 mesmo com a bio publicada). Idempotente.
func (s *Store) EnsureBioModuleEnabled(ctx context.Context, accountID string) error {
	const q = `
		insert into core.account_modules (account_id, module_id, enabled, config)
		select $1::uuid, 'bio', true, '{}'::jsonb
		where exists (select 1 from core.modules where id = 'bio')
		on conflict (account_id, module_id) do update set enabled = true`
	_, err := s.pool.Exec(ctx, q, accountID)
	return err
}

// Patch atualiza nome, slug, draft e/ou account_id. Campos nil sao preservados
// via coalesce. accountID (mover de account) so e passado nao-nil pelo service
// quando o requisitante e platform_admin.
func (s *Store) Patch(ctx context.Context, id string, name, slug, accountID *string, draft *json.RawMessage) (Bio, error) {
	var draftArg any
	if draft != nil {
		draftArg = []byte(*draft)
	}
	const q = `
		update bio.bios
		set name = coalesce($2, name),
		    slug = coalesce($3, slug),
		    data_draft = coalesce($4::jsonb, data_draft),
		    account_id = coalesce($5::uuid, account_id),
		    updated_at = now()
		where id = $1::uuid
		returning id, account_id, '', slug, name, status,
		          data_draft, data_published, published_at, created_at, updated_at`
	return scanBio(s.pool.QueryRow(ctx, q, id, name, slug, draftArg, accountID))
}

// Publish copia o jsonb mesclado para data_published e marca status/published_at.
func (s *Store) Publish(ctx context.Context, id string, published json.RawMessage) (Bio, error) {
	const q = `
		update bio.bios
		set data_published = $2::jsonb,
		    status = 'published',
		    published_at = now(),
		    updated_at = now()
		where id = $1::uuid
		returning id, account_id, '', slug, name, status,
		          data_draft, data_published, published_at, created_at, updated_at`
	return scanBio(s.pool.QueryRow(ctx, q, id, []byte(published)))
}

// Unpublish volta a bio para draft (o publico passa a 404). data_published e
// preservado para reuso futuro.
func (s *Store) Unpublish(ctx context.Context, id string) (Bio, error) {
	const q = `
		update bio.bios
		set status = 'draft', published_at = null, updated_at = now()
		where id = $1::uuid
		returning id, account_id, '', slug, name, status,
		          data_draft, data_published, published_at, created_at, updated_at`
	return scanBio(s.pool.QueryRow(ctx, q, id))
}

// Delete remove a bio. Retorna pgx.ErrNoRows quando nada foi apagado.
func (s *Store) Delete(ctx context.Context, id, accountID string) error {
	query := `delete from bio.bios where id = $1::uuid`
	args := []any{id}
	if strings.TrimSpace(accountID) != "" {
		args = append(args, accountID)
		query += " and account_id = $2::uuid"
	}
	tag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// SlugExists verifica se o slug ja existe (case-insensitive). excludeID permite
// ignorar a propria bio em updates.
func (s *Store) SlugExists(ctx context.Context, slug, excludeID string) (bool, error) {
	query := `select 1 from bio.bios where lower(slug) = lower($1)`
	args := []any{slug}
	if strings.TrimSpace(excludeID) != "" {
		args = append(args, excludeID)
		query += " and id <> $2::uuid"
	}
	query += " limit 1"
	var dummy int
	err := s.pool.QueryRow(ctx, query, args...).Scan(&dummy)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	default:
		return false, err
	}
}

// ============================================================================
// Endpoint publico
// ============================================================================

// PublicLookup busca o data_published (+ account_id) de uma bio published cuja
// account esteja ativa e com o modulo `bio` habilitado. 1 query com joins. O
// account_id volta para o service resolver as fontes de produto (B7).
// pgx.ErrNoRows quando qualquer condicao falha (nao vaza existencia).
func (s *Store) PublicLookup(ctx context.Context, slug string) (json.RawMessage, string, error) {
	const q = `
		select b.data_published, b.account_id
		from bio.bios b
		join core.accounts a on a.id = b.account_id
		join core.account_modules m on m.account_id = b.account_id
		where lower(b.slug) = lower($1)
		  and b.status = 'published'
		  and b.data_published is not null
		  and a.is_active = true
		  and m.module_id = 'bio'
		  and m.enabled = true
		limit 1`
	var data json.RawMessage
	var accountID string
	if err := s.pool.QueryRow(ctx, q, slug).Scan(&data, &accountID); err != nil {
		return nil, "", err
	}
	return data, accountID, nil
}

// ============================================================================
// bio.defaults
// ============================================================================

// GetDefaults retorna a linha global bio.defaults.
func (s *Store) GetDefaults(ctx context.Context) (BioDefaults, error) {
	const q = `select data, updated_at from bio.defaults where id = 'global'`
	var d BioDefaults
	err := s.pool.QueryRow(ctx, q).Scan(&d.Data, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return BioDefaults{Data: json.RawMessage("{}"), UpdatedAt: time.Now()}, nil
	}
	return d, err
}

// PutDefaults faz upsert do data global.
func (s *Store) PutDefaults(ctx context.Context, data json.RawMessage) (BioDefaults, error) {
	const q = `
		insert into bio.defaults (id, data, updated_at)
		values ('global', $1::jsonb, now())
		on conflict (id) do update
		set data = excluded.data, updated_at = now()
		returning data, updated_at`
	var d BioDefaults
	err := s.pool.QueryRow(ctx, q, []byte(data)).Scan(&d.Data, &d.UpdatedAt)
	return d, err
}

// ============================================================================
// bio.media
// ============================================================================

// InsertMedia registra o metadado de um arquivo enviado.
func (s *Store) InsertMedia(ctx context.Context, accountID, bioID, kind, path, mime string, size int64) (Media, error) {
	var bioArg *string
	if strings.TrimSpace(bioID) != "" {
		bioArg = &bioID
	}
	var mimeArg *string
	if strings.TrimSpace(mime) != "" {
		mimeArg = &mime
	}
	const q = `
		insert into bio.media (account_id, bio_id, kind, path, mime, size_bytes)
		values ($1::uuid, $2::uuid, $3, $4, $5, $6)
		returning id, account_id, bio_id, kind, path, mime, size_bytes, created_at`
	var m Media
	err := s.pool.QueryRow(ctx, q, accountID, bioArg, kind, path, mimeArg, size).
		Scan(&m.ID, &m.AccountID, &m.BioID, &m.Kind, &m.Path, &m.Mime, &m.SizeBytes, &m.CreatedAt)
	return m, err
}

// ============================================================================
// Scan helpers
// ============================================================================

func scanBio(row rowScanner) (Bio, error) {
	var b Bio
	err := row.Scan(&b.ID, &b.AccountID, &b.AccountName, &b.Slug, &b.Name, &b.Status,
		&b.DataDraft, &b.DataPublished, &b.PublishedAt, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}
