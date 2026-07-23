package omnichannel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const aiCredentialColumns = `id::text,name,provider,secret_ciphertext,secret_last4,created_at,updated_at` // #nosec G101 -- SQL column names only; no credential value is hardcoded.

func scanAICredential(row rowScanner) (aiCredentialRow, error) {
	var out aiCredentialRow
	err := row.Scan(&out.ID, &out.Name, &out.Provider, &out.SecretCiphertext, &out.Last4,
		&out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (s *Store) ListAICredentials(ctx context.Context, accountID string) ([]aiCredentialRow, error) {
	rows, err := s.pool.Query(ctx, `select `+aiCredentialColumns+` from messaging.ai_credentials
		where account_id=$1::uuid order by lower(name),id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]aiCredentialRow, 0)
	for rows.Next() {
		row, scanErr := scanAICredential(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) GetAICredential(ctx context.Context, accountID, credentialID string) (aiCredentialRow, error) {
	row, err := scanAICredential(s.pool.QueryRow(ctx, `select `+aiCredentialColumns+`
		from messaging.ai_credentials where account_id=$1::uuid and id=$2::uuid`, accountID, credentialID))
	if errors.Is(err, pgx.ErrNoRows) {
		return aiCredentialRow{}, ErrNotFound
	}
	return row, err
}

func (s *Store) CreateAICredential(ctx context.Context, accountID, userID, name, provider, ciphertext, last4 string) (aiCredentialRow, error) {
	return scanAICredential(s.pool.QueryRow(ctx, `insert into messaging.ai_credentials
		(account_id,name,provider,secret_ciphertext,secret_last4,created_by)
		values ($1::uuid,$2,$3,$4,$5,nullif($6,'')::uuid)
		returning `+aiCredentialColumns, accountID, name, provider, ciphertext, last4, userID))
}

func (s *Store) UpdateAICredential(ctx context.Context, accountID, credentialID string, name, ciphertext, last4 *string) (aiCredentialRow, error) {
	row, err := scanAICredential(s.pool.QueryRow(ctx, `update messaging.ai_credentials set
		name=coalesce($3,name),secret_ciphertext=coalesce($4,secret_ciphertext),
		secret_last4=coalesce($5,secret_last4),updated_at=now()
		where account_id=$1::uuid and id=$2::uuid returning `+aiCredentialColumns,
		accountID, credentialID, name, ciphertext, last4))
	if errors.Is(err, pgx.ErrNoRows) {
		return aiCredentialRow{}, ErrNotFound
	}
	return row, err
}

func (s *Store) DeleteAICredential(ctx context.Context, accountID, credentialID string) error {
	result, err := s.pool.Exec(ctx, `delete from messaging.ai_credentials
		where account_id=$1::uuid and id=$2::uuid
		  and not exists (select 1 from messaging.ai_agent_versions version
			where version.account_id=$1::uuid and version.response_credential_id=$2::uuid)
		  and not exists (select 1 from messaging.media_analyses analysis
			where analysis.account_id=$1::uuid and analysis.credential_id=$2::uuid)`, accountID, credentialID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 1 {
		return nil
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `select exists(select 1 from messaging.ai_credentials
		where account_id=$1::uuid and id=$2::uuid)`, accountID, credentialID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrConflict
	}
	return ErrNotFound
}

func (s *Store) AICredentialNameExists(ctx context.Context, accountID, name string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `select exists(select 1 from messaging.ai_credentials
		where account_id=$1::uuid and lower(btrim(name))=lower(btrim($2)))`, accountID, strings.TrimSpace(name)).Scan(&exists)
	return exists, err
}
