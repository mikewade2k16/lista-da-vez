package metaads

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrCryptoKeyMissing sinaliza que META_ADS_CRYPTO_KEY nao foi configurada.
// Sem ela nao da para cifrar o token, entao conectar deve falhar rapido.
var ErrCryptoKeyMissing = errors.New("META_ADS_CRYPTO_KEY nao configurado")

// Store e a persistencia do modulo (schema meta_ads.*). O token e cifrado
// at-rest via pgcrypto (pgp_sym_encrypt com cryptoKey); nunca trafega em claro
// fora deste processo e nunca e logado.
type Store struct {
	pool      *pgxpool.Pool
	cryptoKey string
}

// NewStore cria o Store. cryptoKey e a chave simetrica do pgcrypto.
func NewStore(pool *pgxpool.Pool, cryptoKey string) *Store {
	return &Store{pool: pool, cryptoKey: cryptoKey}
}

// HasCryptoKey informa se a chave de cifra esta configurada.
func (s *Store) HasCryptoKey() bool { return s.cryptoKey != "" }

type rowScanner interface {
	Scan(dest ...any) error
}

// ============================================================================
// connections
// ============================================================================

// GetConnection retorna a conexao da account (ou pgx.ErrNoRows). Nunca traz o
// token: ele e lido a parte via GetDecryptedToken.
func (s *Store) GetConnection(ctx context.Context, accountID string) (Connection, error) {
	const q = `select id, account_id, organization_id, meta_business_id, name,
			token_expires_at, status, created_at, updated_at
		from meta_ads.connections
		where account_id = $1`
	return scanConnection(s.pool.QueryRow(ctx, q, accountID))
}

// UpsertConnection cifra e persiste o token (1 conexao por account). Em conflito
// (account_id) atualiza o token e os metadados, reativando a conexao.
func (s *Store) UpsertConnection(ctx context.Context, accountID, metaBusinessID, name, token string) (Connection, error) {
	if s.cryptoKey == "" {
		return Connection{}, ErrCryptoKeyMissing
	}
	const q = `insert into meta_ads.connections
			(account_id, meta_business_id, name, encrypted_token, status, updated_at)
		values ($1, $2, $3, pgp_sym_encrypt($4, $5), 'active', now())
		on conflict (account_id) do update
			set meta_business_id = excluded.meta_business_id,
			    name = excluded.name,
			    encrypted_token = excluded.encrypted_token,
			    status = 'active',
			    updated_at = now()
		returning id, account_id, organization_id, meta_business_id, name,
			token_expires_at, status, created_at, updated_at`
	return scanConnection(s.pool.QueryRow(ctx, q, accountID, metaBusinessID, name, token, s.cryptoKey))
}

// DeleteConnection remove a conexao da account (cascade nas tabelas filhas).
// Idempotente: ausencia de linha nao e erro.
func (s *Store) DeleteConnection(ctx context.Context, accountID string) error {
	const q = `delete from meta_ads.connections where account_id = $1`
	_, err := s.pool.Exec(ctx, q, accountID)
	return err
}

// GetDecryptedToken decifra e retorna o token da conexao da account. Usado
// apenas no momento de chamar a Graph; o valor nunca e logado.
func (s *Store) GetDecryptedToken(ctx context.Context, accountID string) (string, error) {
	if s.cryptoKey == "" {
		return "", ErrCryptoKeyMissing
	}
	const q = `select pgp_sym_decrypt(encrypted_token, $2)
		from meta_ads.connections
		where account_id = $1`
	var token string
	if err := s.pool.QueryRow(ctx, q, accountID, s.cryptoKey).Scan(&token); err != nil {
		return "", err
	}
	return token, nil
}

func scanConnection(row rowScanner) (Connection, error) {
	var c Connection
	err := row.Scan(
		&c.ID, &c.AccountID, &c.OrganizationID, &c.MetaBusinessID, &c.Name,
		&c.TokenExpiresAt, &c.Status, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

// noRows e um atalho para checar o sentinel do pgx.
func noRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
