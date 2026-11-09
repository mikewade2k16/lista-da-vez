package finance

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type storedPortalSession struct {
	CredentialsVersion, Ciphertext string
	ExpiresAt, VerifiedAt          time.Time
}
type portalSessionStore interface {
	readPortalSession(context.Context, string, string) (storedPortalSession, error)
	writePortalSession(context.Context, string, string, storedPortalSession) (storedPortalSession, error)
	dropPortalSession(context.Context, string, string, string) error
}

func (s *PostgresFiscalStore) readPortalSession(ctx context.Context, account, user string) (storedPortalSession, error) {
	var out storedPortalSession
	err := s.pool.QueryRow(ctx, `select credentials_version,ciphertext,expires_at,verified_at from finance.portal_sessions where account_id=$1::uuid and user_id=$2::uuid`, account, user).Scan(&out.CredentialsVersion, &out.Ciphertext, &out.ExpiresAt, &out.VerifiedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrFiscalNotFound
	}
	return out, err
}
func (s *PostgresFiscalStore) writePortalSession(ctx context.Context, account, user string, in storedPortalSession) (storedPortalSession, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return in, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockFiscalScope(ctx, tx, account, user); err != nil {
		return in, err
	}
	p, err := readFiscalProfile(ctx, tx, account, user)
	if err != nil {
		return in, err
	}
	if !p.CredentialsSet || p.CredentialsUpdatedAt != in.CredentialsVersion || p.Profile.PortalLoginMethod == "govbr" || (p.Profile.PortalURL != nationalPortalLogin && p.Profile.PortalURL != "https://www.nfse.gov.br/EmissorNacional") {
		return in, ErrFiscalConflict
	}
	err = tx.QueryRow(ctx, `insert into finance.portal_sessions(account_id,user_id,credentials_version,ciphertext,expires_at) values($1::uuid,$2::uuid,$3,$4,$5) on conflict(account_id,user_id) do update set credentials_version=excluded.credentials_version,ciphertext=excluded.ciphertext,expires_at=excluded.expires_at,verified_at=clock_timestamp() returning verified_at`, account, user, in.CredentialsVersion, in.Ciphertext, in.ExpiresAt).Scan(&in.VerifiedAt)
	if err != nil {
		return in, err
	}
	return in, tx.Commit(ctx)
}
func (s *PostgresFiscalStore) dropPortalSession(ctx context.Context, account, user, version string) error {
	_, err := s.pool.Exec(ctx, `delete from finance.portal_sessions where account_id=$1::uuid and user_id=$2::uuid and credentials_version=$3`, account, user, version)
	return err
}
