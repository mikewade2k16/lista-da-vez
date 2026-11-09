package finance

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type portalAccessStore interface {
	portalCiphertext(context.Context, string, string) (string, error)
	portalLastCheck(context.Context, string, string) (PortalCheck, error)
	savePortalCheck(context.Context, string, string, PortalCheck) (PortalCheck, error)
}

func (s *PostgresFiscalStore) portalCiphertext(ctx context.Context, account, user string) (string, error) {
	var cipher string
	err := s.pool.QueryRow(ctx, `select ciphertext from finance.user_fiscal_credentials where account_id=$1::uuid and user_id=$2::uuid`, account, user).Scan(&cipher)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrFiscalNotFound
	}
	return cipher, err
}
func (s *PostgresFiscalStore) portalLastCheck(ctx context.Context, account, user string) (PortalCheck, error) {
	var out PortalCheck
	var checked time.Time
	err := s.pool.QueryRow(ctx, `select status,profile_version,checked_at from finance.portal_access_checks where account_id=$1::uuid and user_id=$2::uuid`, account, user).Scan(&out.Status, &out.ProfileVersion, &checked)
	if errors.Is(err, pgx.ErrNoRows) {
		return PortalCheck{Status: "not_tested", Message: "Acesso ainda não testado."}, nil
	}
	if err != nil {
		return out, err
	}
	out.Message = portalCheck(out.Status).Message
	out.CheckedAt = checked.UTC().Format(time.RFC3339Nano)
	return out, nil
}
func (s *PostgresFiscalStore) savePortalCheck(ctx context.Context, account, user string, in PortalCheck) (PortalCheck, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return in, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockFiscalScope(ctx, tx, account, user); err != nil {
		return in, err
	}
	current, err := readFiscalProfile(ctx, tx, account, user)
	if err != nil {
		return in, err
	}
	if current.UpdatedAt != in.ProfileVersion {
		return in, ErrFiscalConflict
	}
	var checked time.Time
	err = tx.QueryRow(ctx, `insert into finance.portal_access_checks(account_id,user_id,profile_version,status) values($1::uuid,$2::uuid,$3,$4) on conflict(account_id,user_id) do update set profile_version=excluded.profile_version,status=excluded.status,checked_at=clock_timestamp() returning checked_at`, account, user, in.ProfileVersion, in.Status).Scan(&checked)
	if err != nil {
		return in, err
	}
	in.CheckedAt = checked.UTC().Format(time.RFC3339Nano)
	return in, tx.Commit(ctx)
}
func (s *FiscalService) PortalStatus(ctx context.Context, account, user string) (PortalCheck, error) {
	store, ok := s.store.(portalAccessStore)
	if !ok {
		return PortalCheck{}, ErrFiscalVaultUnavailable
	}
	profile, err := s.GetProfile(ctx, account, user)
	if err != nil {
		return PortalCheck{}, err
	}
	out, err := store.portalLastCheck(ctx, account, user)
	if err != nil {
		return out, err
	}
	if out.ProfileVersion != profile.UpdatedAt {
		return PortalCheck{Status: "not_tested", Message: "Teste o acesso com o cadastro atual."}, nil
	}
	return out, nil
}
func (s *FiscalService) TestPortalAccess(ctx context.Context, account, user, expected string) (PortalCheck, error) {
	store, ok := s.store.(portalAccessStore)
	if !ok {
		return PortalCheck{}, ErrFiscalVaultUnavailable
	}
	profile, err := s.GetProfile(ctx, account, user)
	if err != nil {
		return PortalCheck{}, err
	}
	if profile.UpdatedAt == "" || profile.UpdatedAt != expected {
		return PortalCheck{}, ErrFiscalConflict
	}
	select {
	case s.portalSlots <- struct{}{}:
		defer func() { <-s.portalSlots }()
	default:
		return PortalCheck{}, invalidFiscal("Teste ocupado. Aguarde antes de tentar novamente.")
	}
	previous, err := s.PortalStatus(ctx, account, user)
	if err != nil {
		return previous, err
	}
	if when, e := time.Parse(time.RFC3339Nano, previous.CheckedAt); e == nil && time.Since(when) < 10*time.Second {
		return previous, nil
	}
	out := portalCheck("unsupported")
	switch {
	case profile.Profile.PortalLoginMethod == "govbr":
		out = portalCheck("manual_required")
	case profile.Profile.PortalURL == nationalPortalLogin || profile.Profile.PortalURL == "https://www.nfse.gov.br/EmissorNacional":
		if !profile.CredentialsSet {
			return PortalCheck{}, invalidFiscal("Cadastre CPF/CNPJ e senha próprios do Emissor Nacional para testar.")
		}
		if s.box == nil {
			return PortalCheck{}, ErrFiscalVaultUnavailable
		}
		cipher, e := store.portalCiphertext(ctx, account, user)
		if e != nil {
			return out, e
		}
		raw, e := s.box.Decrypt(cipher)
		if e != nil {
			return out, ErrFiscalVaultUnavailable
		}
		var credentials portalCredentials
		if e = json.Unmarshal([]byte(raw), &credentials); e != nil {
			return out, ErrFiscalVaultUnavailable
		}
		out = s.portalProbe(ctx, credentials)
	}
	out.ProfileVersion = profile.UpdatedAt
	return store.savePortalCheck(ctx, account, user, out)
}
