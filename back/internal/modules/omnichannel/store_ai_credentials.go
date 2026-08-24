package omnichannel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const aiCredentialColumns = `id::text,name,provider,secret_ciphertext,secret_last4,created_at,updated_at` // #nosec G101 -- SQL column names only; no credential value is hardcoded.

// #nosec G101 -- the query names encrypted credential columns; it embeds no secret value.
const assistantCredentialCatalogQuery = `select credential.id::text,credential.name,
		credential.provider,credential.secret_ciphertext,credential.secret_last4,
		credential.created_at,credential.updated_at,
		(owner.id=consumer.id) as owned_by_account,owner.name,
		(owner.id<>consumer.id) as read_only
	from core.accounts consumer
	join messaging.ai_credentials credential on true
	join core.accounts owner on owner.id=credential.account_id and owner.is_active
	where consumer.id=$1::uuid
	  and consumer.is_active
	  and (
		owner.id=consumer.id
		or (
			consumer.is_agency=false
			and owner.id=(
				select agency.id
				from core.accounts agency
				where agency.organization_id=consumer.organization_id
				  and agency.is_agency=true
				  and agency.is_active
				order by agency.created_at,agency.id
				limit 1
			)
		)
	  )
	order by (owner.id=consumer.id) desc,lower(credential.name),credential.id`

// #nosec G101 -- the query names encrypted credential columns; it embeds no secret value.
const sharedRuntimeAICredentialQuery = `select credential.id::text,credential.name,
		credential.provider,credential.secret_ciphertext,credential.secret_last4,
		credential.created_at,credential.updated_at
	from messaging.ai_credentials credential
	join core.accounts owner on owner.id=credential.account_id and owner.is_active
	join core.accounts consumer on consumer.id=$1::uuid and consumer.is_active
	where credential.id=$2::uuid
	  and (
		owner.id=consumer.id
		or (
			consumer.is_agency=false
			and owner.id=(
				select agency.id
				from core.accounts agency
				where agency.organization_id=consumer.organization_id
				  and agency.is_agency=true
				  and agency.is_active
				order by agency.created_at,agency.id
				limit 1
			)
		)
	  )`

func scanAICredential(row rowScanner) (aiCredentialRow, error) {
	var out aiCredentialRow
	err := row.Scan(&out.ID, &out.Name, &out.Provider, &out.SecretCiphertext, &out.Last4,
		&out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func scanAssistantAICredential(row rowScanner) (aiCredentialRow, error) {
	var out aiCredentialRow
	err := row.Scan(&out.ID, &out.Name, &out.Provider, &out.SecretCiphertext, &out.Last4,
		&out.CreatedAt, &out.UpdatedAt, &out.OwnedByAccount, &out.OwnerName, &out.ReadOnly)
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

// ListAssistantAICredentials projeta o catalogo utilizavel pelo Assistente 360:
// credenciais da conta ativa e, para cliente, da conta-agencia canonica da org.
// Nenhum segredo e retornado pela camada de servico.
func (s *Store) ListAssistantAICredentials(ctx context.Context, accountID string) ([]aiCredentialRow, error) {
	rows, err := s.pool.Query(ctx, assistantCredentialCatalogQuery, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]aiCredentialRow, 0)
	for rows.Next() {
		row, scanErr := scanAssistantAICredential(rows)
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

func (s *Store) GetSharedRuntimeAICredential(ctx context.Context, consumerAccountID, credentialID string) (aiCredentialRow, error) {
	row, err := scanAICredential(s.pool.QueryRow(ctx, sharedRuntimeAICredentialQuery, consumerAccountID, credentialID))
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrConflict
		}
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
