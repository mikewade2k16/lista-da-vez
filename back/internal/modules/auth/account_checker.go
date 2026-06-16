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

// accountAccessibleQuery resolve a visibilidade org-aware da account. $1=accountID,
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
		          select 1 from core.organization_users ou
		          where ou.user_id = $2::uuid
		            and ou.org_role = 'agency_owner'
		            and ou.organization_id = a.organization_id
		      )
		      or exists (
		          select 1 from core.account_users au
		          where au.user_id = $2::uuid and au.account_id = a.id and au.is_active = true
		      )
		  )
		limit 1
	`

// IsMember retorna true se userID PODE acessar a account (org-aware).
// Espelha a visibilidade de core.ListAccountsForUser/FindAccountIfMember para
// o portao do middleware nao divergir do que /v2/me/accounts lista. A account
// e acessivel quando esta ativa E qualquer um vale:
//
//	(a) o user e platform_admin -> acessa todas;
//	(b) o user e agency_owner em core.organization_users da org da account;
//	(c) o user tem membership ativa em core.account_users (caso comum cliente).
//
// Sem isso, um login-agencia veria a conta no switcher mas levaria 403 ao usar
// o modulo dela (ex.: board Tasks movido para a conta-agencia Crow).
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
