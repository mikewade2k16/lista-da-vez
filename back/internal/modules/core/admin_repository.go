package core

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

// PostgresAdminRepository implementa AdminRepository contra o schema core.
type PostgresAdminRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAdminRepository cria a implementacao Postgres do AdminRepository.
func NewPostgresAdminRepository(pool *pgxpool.Pool) *PostgresAdminRepository {
	return &PostgresAdminRepository{pool: pool}
}

// ============================================================================
// ListAccounts
// ============================================================================

func (r *PostgresAdminRepository) ListAccounts(ctx context.Context, filter AdminListFilter) ([]AccountAdminView, int, error) {
	args := []any{}
	// A conta-agência (is_agency=true) é o workspace da agência, não um cliente:
	// fica fora da LISTAGEM de /v1/admin/accounts. O GET por id (FindAdminAccount)
	// não aplica este filtro de propósito — a agência continua acessível no detalhe.
	conds := []string{"a.is_agency = false"}
	n := 1

	if filter.Q != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(filter.Q)) + "%"
		conds = append(conds, fmt.Sprintf("(lower(a.name) like $%d or lower(a.slug) like $%d)", n, n))
		args = append(args, pattern)
		n++
	}
	switch filter.Status {
	case "active":
		conds = append(conds, "a.is_active = true")
	case "inactive":
		conds = append(conds, "a.is_active = false")
	}
	if filter.OrganizationID != "" {
		conds = append(conds, fmt.Sprintf("a.organization_id = $%d::uuid", n))
		args = append(args, filter.OrganizationID)
		n++
	}

	where := strings.Join(conds, " and ")

	var total int
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("select count(*) from core.accounts a where %s", where),
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
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	args = append(args, perPage, (page-1)*perPage)
	dataSQL := fmt.Sprintf(`
		select a.id, a.organization_id, a.slug, a.name, a.plan_code, a.is_active,
		       a.is_agency,
		       a.billing_mode, a.monthly_payment_amount, a.payment_due_day,
		       a.webhook_enabled, a.contact_phone, a.contact_site, a.contact_address,
		       a.logo_path, a.require_user_store_link, a.require_user_registration,
		       a.created_at, a.updated_at
		from core.accounts a
		where %s
		order by lower(a.name) asc
		limit $%d offset $%d
	`, where, n, n+1)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	accounts := make([]AccountAdminView, 0)
	for rows.Next() {
		a, err := scanAdminAccount(rows)
		if err != nil {
			return nil, 0, err
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if err := r.enrichAccounts(ctx, accounts); err != nil {
		return nil, 0, err
	}
	return accounts, total, nil
}

// ============================================================================
// FindAdminAccount
// ============================================================================

func (r *PostgresAdminRepository) FindAdminAccount(ctx context.Context, accountID string) (AccountAdminView, error) {
	const query = `
		select id, organization_id, slug, name, plan_code, is_active,
		       is_agency,
		       billing_mode, monthly_payment_amount, payment_due_day,
		       webhook_enabled, contact_phone, contact_site, contact_address,
		       logo_path, require_user_store_link, require_user_registration,
		       created_at, updated_at
		from core.accounts
		where id = $1::uuid
	`
	row := r.pool.QueryRow(ctx, query, accountID)
	a, err := scanAdminAccount(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AccountAdminView{}, ErrAccountNotFound
		}
		return AccountAdminView{}, err
	}

	views := []AccountAdminView{a}
	if err := r.enrichAccounts(ctx, views); err != nil {
		return AccountAdminView{}, err
	}
	return views[0], nil
}

// ============================================================================
// CreateAccount (transacao: account + roles + membership + role assignment)
// ============================================================================

func (r *PostgresAdminRepository) CreateAccount(ctx context.Context, input AdminCreateAccountInput) (AccountAdminView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AccountAdminView{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var accountID string
	err = tx.QueryRow(ctx, `
		insert into core.accounts (slug, name, plan_code, is_active, billing_mode, created_at, updated_at)
		values ($1, $2, $3, true, 'single', now(), now())
		returning id
	`, input.Slug, input.Name, input.PlanCode).Scan(&accountID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return AccountAdminView{}, ErrAccountSlugConflict
		}
		return AccountAdminView{}, err
	}

	if err := cloneRoleTemplates(ctx, tx, accountID); err != nil {
		return AccountAdminView{}, err
	}

	// Vincula o dono SOMENTE quando um adminEmail foi informado. Vazio = conta de
	// controle interno, sem dono/usuario (permitido por design). O dono pode ser
	// anexado depois via POST /v1/admin/users/{id}/memberships.
	if input.AdminEmail != "" {
		var userID string
		err = tx.QueryRow(ctx,
			`select id from core.users where lower(email) = lower($1) and is_active = true limit 1`,
			input.AdminEmail,
		).Scan(&userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return AccountAdminView{}, ErrAdminUserNotFound
			}
			return AccountAdminView{}, err
		}

		_, err = tx.Exec(ctx, `
			insert into core.account_users (account_id, user_id, is_active, joined_at)
			values ($1::uuid, $2::uuid, true, now())
			on conflict (account_id, user_id) do nothing
		`, accountID, userID)
		if err != nil {
			return AccountAdminView{}, err
		}

		_, err = tx.Exec(ctx, `
			insert into core.user_role_assignments (account_id, user_id, role_id)
			select $1::uuid, $2::uuid, r.id
			from core.roles r
			where r.account_id = $1::uuid and r.code = 'core.owner'
			limit 1
			on conflict (account_id, user_id, role_id) do nothing
		`, accountID, userID)
		if err != nil {
			return AccountAdminView{}, err
		}
	}

	// Seed dos modulos default da conta nova (mesmo set da migration 0124:
	// queue/tasks/crm). Sem isto a account nasce com core.account_modules vazio,
	// o guard de modulos barra TODAS as rotas e o owner nao entra em fluxo util.
	// `m.id in (...)` ignora modulos ausentes do catalogo (JOIN vazio = no-op).
	_, err = tx.Exec(ctx, `
		insert into core.account_modules (account_id, module_id, enabled, config)
		select $1::uuid, m.id, true, '{}'::jsonb
		from core.modules m
		where m.id in ('queue', 'tasks', 'crm')
		on conflict (account_id, module_id) do nothing
	`, accountID)
	if err != nil {
		return AccountAdminView{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AccountAdminView{}, err
	}

	return r.FindAdminAccount(ctx, accountID)
}

// cloneRoleTemplates clona role_templates em core.roles para a account e
// popula core.role_permissions a partir de core.role_template_permissions.
// Marca is_default=true nos papeis clonados — assim a conta NOVA nasce com os
// papeis-padrao sinalizados, igual ao que a migration 0176 fez nas contas ja
// existentes (paridade entre conta nova e legada).
func cloneRoleTemplates(ctx context.Context, tx pgx.Tx, accountID string) error {
	_, err := tx.Exec(ctx, `
		insert into core.roles (account_id, cloned_from_template_id, code, label, description, is_default, is_locked)
		select $1::uuid, rt.id, rt.id, rt.label, rt.description, true, rt.is_locked
		from core.role_templates rt
		on conflict (account_id, code) do nothing
	`, accountID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		insert into core.role_permissions (role_id, permission_key)
		select r.id, rtp.permission_key
		from core.roles r
		join core.role_template_permissions rtp on rtp.role_template_id = r.cloned_from_template_id
		where r.account_id = $1::uuid and r.cloned_from_template_id is not null
		on conflict do nothing
	`, accountID)
	return err
}

// ============================================================================
// UpdateAccount (patch semantico via SET dinamico)
// ============================================================================

func (r *PostgresAdminRepository) UpdateAccount(ctx context.Context, accountID string, input AdminUpdateAccountInput) (AccountAdminView, error) {
	sets := []string{}
	args := []any{accountID}
	n := 2

	addSet := func(column string, value any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", column, n))
		args = append(args, value)
		n++
	}

	if input.Active != nil {
		addSet("is_active", *input.Active)
	}
	if input.Name != nil {
		addSet("name", *input.Name)
	}
	if input.Slug != nil {
		addSet("slug", *input.Slug)
	}
	if input.PlanCode != nil {
		addSet("plan_code", *input.PlanCode)
	}
	if input.OrganizationID != nil {
		// String vazia → NULL (desvincula da organization).
		trimmed := strings.TrimSpace(*input.OrganizationID)
		if trimmed == "" {
			sets = append(sets, "organization_id = NULL")
		} else {
			sets = append(sets, fmt.Sprintf("organization_id = $%d::uuid", n))
			args = append(args, trimmed)
			n++
		}
	}
	if input.BillingMode != nil {
		addSet("billing_mode", *input.BillingMode)
	}
	if input.MonthlyPaymentAmount != nil {
		addSet("monthly_payment_amount", *input.MonthlyPaymentAmount)
	}
	if input.PaymentDueDay != nil {
		addSet("payment_due_day", *input.PaymentDueDay)
	}
	if input.WebhookEnabled != nil {
		addSet("webhook_enabled", *input.WebhookEnabled)
	}
	if input.ContactPhone != nil {
		addSet("contact_phone", *input.ContactPhone)
	}
	if input.ContactSite != nil {
		addSet("contact_site", *input.ContactSite)
	}
	if input.ContactAddress != nil {
		addSet("contact_address", *input.ContactAddress)
	}
	if input.LogoPath != nil {
		addSet("logo_path", *input.LogoPath)
	}
	if input.RequireUserStoreLink != nil {
		addSet("require_user_store_link", *input.RequireUserStoreLink)
	}
	if input.RequireUserRegistration != nil {
		addSet("require_user_registration", *input.RequireUserRegistration)
	}

	if len(sets) == 0 {
		return r.FindAdminAccount(ctx, accountID)
	}

	sets = append(sets, fmt.Sprintf("updated_at = $%d", n))
	args = append(args, time.Now())

	query := fmt.Sprintf(`
		update core.accounts
		set %s
		where id = $1::uuid
		returning id, organization_id, slug, name, plan_code, is_active,
		          is_agency,
		          billing_mode, monthly_payment_amount, payment_due_day,
		          webhook_enabled, contact_phone, contact_site, contact_address,
		          logo_path, require_user_store_link, require_user_registration,
		          created_at, updated_at
	`, strings.Join(sets, ", "))

	row := r.pool.QueryRow(ctx, query, args...)
	a, err := scanAdminAccount(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AccountAdminView{}, ErrAccountNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return AccountAdminView{}, ErrAccountSlugConflict
		}
		return AccountAdminView{}, err
	}

	views := []AccountAdminView{a}
	if err := r.enrichAccounts(ctx, views); err != nil {
		return AccountAdminView{}, err
	}
	return views[0], nil
}

// ============================================================================
// SoftDeleteAccount
// ============================================================================

func (r *PostgresAdminRepository) SoftDeleteAccount(ctx context.Context, accountID string) error {
	tag, err := r.pool.Exec(ctx,
		`update core.accounts set is_active = false, updated_at = now() where id = $1::uuid`,
		accountID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAccountNotFound
	}
	return nil
}

// ============================================================================
// Scanner
// ============================================================================

func scanAdminAccount(row scannable) (AccountAdminView, error) {
	var a AccountAdminView
	var orgID *string
	var paymentDueDay *int
	var contactPhone, contactSite, contactAddress, logoPath *string

	if err := row.Scan(
		&a.ID, &orgID, &a.Slug, &a.Name, &a.PlanCode, &a.Active,
		&a.IsAgency,
		&a.BillingMode, &a.MonthlyPaymentAmount, &paymentDueDay,
		&a.WebhookEnabled, &contactPhone, &contactSite, &contactAddress,
		&logoPath, &a.RequireUserStoreLink, &a.RequireUserRegistration,
		&a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return AccountAdminView{}, err
	}

	if orgID != nil {
		a.OrganizationID = *orgID
	}
	a.PaymentDueDay = paymentDueDay
	if contactPhone != nil {
		a.ContactPhone = *contactPhone
	}
	if contactSite != nil {
		a.ContactSite = *contactSite
	}
	if contactAddress != nil {
		a.ContactAddress = *contactAddress
	}
	if logoPath != nil {
		a.LogoPath = *logoPath
	}
	return a, nil
}
