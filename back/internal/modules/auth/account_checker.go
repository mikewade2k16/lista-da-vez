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

// IsMember retorna true se userID tem membership ativa na account.
// Retorna false (sem erro) quando o registro nao existe — o caller
// deve tratar como 403, nao como 500.
func (c *PostgresAccountMemberChecker) IsMember(ctx context.Context, accountID, userID string) (bool, error) {
	const query = `
		select 1
		from core.account_users
		where account_id = $1::uuid
		  and user_id    = $2::uuid
		  and is_active  = true
		limit 1
	`

	var dummy int
	err := c.pool.QueryRow(ctx, query, accountID, userID).Scan(&dummy)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return false, err
}
