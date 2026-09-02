package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAccountMemberChecker implementa AccountMemberChecker consultando
// core.account_users. Injetado em Middleware.SetAccountChecker.
type PostgresAccountMemberChecker struct {
	pool *pgxpool.Pool
}

func NewPostgresAccountMemberChecker(pool *pgxpool.Pool) *PostgresAccountMemberChecker {
	return &PostgresAccountMemberChecker{pool: pool}
}

// accountAccessibleQuery resolve a visibilidade autoritativa da account. $1=accountID,
// $2=userID. Mantido em const de pacote para teste de contrato (account_checker_test).
const accountAccessibleQuery = `
		select 1
		from core.accounts a
		where a.id = $1::uuid
		  and a.is_active = true
		  and (
		      exists (
		          select 1 from core.users u
		          where u.id = $2::uuid and u.is_active = true and u.is_platform_admin = true
		      )
		      or exists (
		          select 1 from core.account_users au
		          where au.user_id = $2::uuid and au.account_id = a.id and au.is_active = true
		      )
		  )
		limit 1
	`

// accountPermissionsQuery espelha a RBAC efetiva por account do core: grants
// dos papeis + overrides allow, removendo overrides deny. O middleware so chama
// esta query depois de accountAccessibleQuery confirmar o acesso à account.
const accountPermissionsQuery = `
	select permission_key
	from (
		select rp.permission_key
		from core.user_role_assignments ura
		join core.role_permissions rp on rp.role_id = ura.role_id
		join core.permissions p on p.key = rp.permission_key and p.deprecated_at is null
		where ura.account_id = $1::uuid and ura.user_id = $2::uuid

		union

		select upo.permission_key
		from core.user_permission_overrides upo
		join core.permissions p on p.key = upo.permission_key and p.deprecated_at is null
		where upo.account_id = $1::uuid and upo.user_id = $2::uuid
		  and upo.effect = 'allow' and upo.is_active = true

		except

		select upo.permission_key
		from core.user_permission_overrides upo
		where upo.account_id = $1::uuid and upo.user_id = $2::uuid
		  and upo.effect = 'deny' and upo.is_active = true
	) effective
	order by permission_key
`

// IsMember retorna true se userID PODE acessar a account.
// Espelha a visibilidade de core.ListAccountsForUser/FindAccountIfMember para
// o portao do middleware nao divergir do que /v2/me/accounts lista. A account
// e acessivel quando esta ativa E um dos caminhos vale:
//
//	(a) o user e platform_admin -> acessa todas;
//	(b) o user tem membership ativa em core.account_users.
//
// Membership de organizacao, inclusive agency_owner, nao concede contexto de cliente.
// A agencia precisa de account_users explicito na conta que vai operar.
// Retorna false (sem erro) quando nao acessivel — o caller trata como 403.
func (c *PostgresAccountMemberChecker) IsMember(ctx context.Context, accountID, userID string) (bool, error) {
	var dummy int
	err := c.pool.QueryRow(ctx, accountAccessibleQuery, accountID, userID).Scan(&dummy)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return false, err
}

// ResolveAccountPermissions devolve a matriz CUSTOM efetiva da account. Lista
// vazia e um resultado valido e permanece fail-closed nos handlers downstream.
func (c *PostgresAccountMemberChecker) ResolveAccountPermissions(ctx context.Context, accountID, userID string) ([]string, error) {
	rows, err := c.pool.Query(ctx, accountPermissionsQuery, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make([]string, 0)
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return permissions, nil
}
