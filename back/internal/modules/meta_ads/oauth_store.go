package metaads

import (
	"context"
	"time"
)

// CreateOAuthState persiste somente o hash SHA-256 do state. O cleanup e
// oportunista e fica no mesmo statement para nao transformar lixo expirado em
// uma dependencia operacional do fluxo de login.
func (s *Store) CreateOAuthState(
	ctx context.Context,
	accountID string,
	createdByUserID string,
	stateHash []byte,
	redirectURI string,
	expiresAt time.Time,
) error {
	const q = `with cleanup as (
		delete from meta_ads.oauth_states
		where expires_at <= now()
		   or consumed_at < now() - interval '1 day'
	)
	insert into meta_ads.oauth_states
		(account_id, created_by_user_id, state_hash, redirect_uri, expires_at)
	values ($1::uuid, $2::uuid, $3, $4, $5)`
	_, err := s.pool.Exec(ctx, q, accountID, createdByUserID, stateHash, redirectURI, expiresAt)
	return err
}

// ConsumeOAuthState faz a transicao single-use em um unico UPDATE. State
// inexistente, expirado ou ja consumido e indistinguivel (pgx.ErrNoRows).
func (s *Store) ConsumeOAuthState(ctx context.Context, stateHash []byte) (OAuthPendingState, error) {
	const q = `update meta_ads.oauth_states
	set consumed_at = now()
	where state_hash = $1
	  and consumed_at is null
	  and expires_at > now()
	returning account_id, redirect_uri, expires_at`
	var pending OAuthPendingState
	err := s.pool.QueryRow(ctx, q, stateHash).Scan(
		&pending.AccountID,
		&pending.RedirectURI,
		&pending.ExpiresAt,
	)
	return pending, err
}
