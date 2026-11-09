package finance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresFiscalStore struct{ pool *pgxpool.Pool }

func NewPostgresFiscalStore(pool *pgxpool.Pool) *PostgresFiscalStore {
	return &PostgresFiscalStore{pool: pool}
}

type fiscalQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readFiscalProfile(ctx context.Context, q fiscalQuerier, account, user string) (FiscalProfileView, error) {
	var out FiscalProfileView
	var raw []byte
	var updated time.Time
	var credentialUpdated *time.Time
	err := q.QueryRow(ctx, `select p.data,p.updated_at,c.updated_at from finance.user_fiscal_profiles p
 left join finance.user_fiscal_credentials c on c.account_id=p.account_id and c.user_id=p.user_id
 where p.account_id=$1::uuid and p.user_id=$2::uuid`, account, user).Scan(&raw, &updated, &credentialUpdated)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(raw, &out.Profile); err != nil {
		return out, err
	}
	out.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	out.CredentialsSet = credentialUpdated != nil
	if credentialUpdated != nil {
		out.CredentialsUpdatedAt = credentialUpdated.UTC().Format(time.RFC3339Nano)
	}
	return out, nil
}
func (s *PostgresFiscalStore) GetProfile(ctx context.Context, account, user string) (FiscalProfileView, error) {
	return readFiscalProfile(ctx, s.pool, account, user)
}
func lockFiscalScope(ctx context.Context, tx pgx.Tx, account, user string) error {
	_, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, "finance:fiscal:"+account+":"+user)
	return err
}
func (s *PostgresFiscalStore) SaveProfile(ctx context.Context, account, user string, in FiscalProfileInput) (FiscalProfileView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FiscalProfileView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockFiscalScope(ctx, tx, account, user); err != nil {
		return FiscalProfileView{}, err
	}
	current, err := readFiscalProfile(ctx, tx, account, user)
	if err != nil {
		return FiscalProfileView{}, err
	}
	if current.UpdatedAt != in.ExpectedUpdatedAt {
		return FiscalProfileView{}, ErrFiscalConflict
	}
	if current.Profile.PortalURL != in.Profile.PortalURL || current.Profile.PortalLoginMethod != in.Profile.PortalLoginMethod {
		if _, err = tx.Exec(ctx, `delete from finance.portal_sessions where account_id=$1::uuid and user_id=$2::uuid`, account, user); err != nil {
			return FiscalProfileView{}, err
		}
		if _, err = tx.Exec(ctx, `delete from finance.user_fiscal_credentials where account_id=$1::uuid and user_id=$2::uuid`, account, user); err != nil {
			return FiscalProfileView{}, err
		}
	}
	raw, err := json.Marshal(in.Profile)
	if err != nil {
		return FiscalProfileView{}, err
	}
	_, err = tx.Exec(ctx, `insert into finance.user_fiscal_profiles(account_id,user_id,data) values($1::uuid,$2::uuid,$3::jsonb)
 on conflict(account_id,user_id) do update set data=excluded.data,updated_at=clock_timestamp()`, account, user, raw)
	if err != nil {
		return FiscalProfileView{}, err
	}
	out, err := readFiscalProfile(ctx, tx, account, user)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (s *PostgresFiscalStore) mutateCredentials(ctx context.Context, account, user, expected string, ciphertext *string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockFiscalScope(ctx, tx, account, user); err != nil {
		return err
	}
	current, err := readFiscalProfile(ctx, tx, account, user)
	if err != nil {
		return err
	}
	if current.UpdatedAt == "" || current.UpdatedAt != expected {
		return ErrFiscalConflict
	}
	if _, err = tx.Exec(ctx, `delete from finance.portal_sessions where account_id=$1::uuid and user_id=$2::uuid`, account, user); err != nil {
		return err
	}
	if ciphertext != nil {
		if current.Profile.PortalLoginMethod == "govbr" {
			return invalidFiscal("O acesso gov.br deve ser concluído pelo usuário no navegador. Não cadastre a senha gov.br aqui.")
		}
		if current.Profile.PortalURL == "" {
			return invalidFiscal("Salve o endereço do portal no perfil antes do acesso.")
		}
		_, err = tx.Exec(ctx, `insert into finance.user_fiscal_credentials(account_id,user_id,ciphertext) values($1::uuid,$2::uuid,$3)
  on conflict(account_id,user_id) do update set ciphertext=excluded.ciphertext,updated_at=clock_timestamp()`, account, user, *ciphertext)
	} else {
		_, err = tx.Exec(ctx, `delete from finance.user_fiscal_credentials where account_id=$1::uuid and user_id=$2::uuid`, account, user)
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `update finance.user_fiscal_profiles set updated_at=clock_timestamp() where account_id=$1::uuid and user_id=$2::uuid`, account, user); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *PostgresFiscalStore) SaveCredentials(ctx context.Context, account, user, expected, ciphertext string) error {
	return s.mutateCredentials(ctx, account, user, expected, &ciphertext)
}
func (s *PostgresFiscalStore) DeleteCredentials(ctx context.Context, account, user, expected string) error {
	return s.mutateCredentials(ctx, account, user, expected, nil)
}
