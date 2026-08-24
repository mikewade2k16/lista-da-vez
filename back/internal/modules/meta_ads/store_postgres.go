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
			token_expires_at, status, revision::text, created_at, updated_at
		from meta_ads.connections
		where account_id = $1 and status = 'active'
		  and (token_expires_at is null or token_expires_at > now())`
	return scanConnection(s.pool.QueryRow(ctx, q, accountID))
}

// FindAgencyConnectionForClient localiza a conexao central da conta-agencia da
// mesma organizacao do cliente. A query exige contas ativas e nunca cruza orgs.
func (s *Store) FindAgencyConnectionForClient(ctx context.Context, clientAccountID string) (Connection, error) {
	const q = `select c.id, c.account_id, c.organization_id, c.meta_business_id, c.name,
			c.token_expires_at, c.status, c.revision::text, c.created_at, c.updated_at
		from core.accounts client
		join core.accounts agency
		  on agency.organization_id = client.organization_id
		 and agency.is_agency = true
		 and agency.is_active = true
		join meta_ads.connections c on c.account_id = agency.id
		where client.id = $1::uuid
		  and client.is_agency = false
		  and client.is_active = true
		  and client.organization_id is not null
		  and c.status = 'active'
		  and (c.token_expires_at is null or c.token_expires_at > now())
		order by c.updated_at desc
		limit 1`
	return scanConnection(s.pool.QueryRow(ctx, q, clientAccountID))
}

// AgencyCanAssignClient valida o vinculo no core sem revelar dados do cliente.
// Somente uma account-agencia ativa pode vincular outra account ativa da mesma
// organization.
func (s *Store) AgencyCanAssignClient(ctx context.Context, agencyAccountID, clientAccountID string) (bool, error) {
	const q = `select exists (
		select 1
		from core.accounts agency
		join core.accounts client
		  on client.organization_id = agency.organization_id
		 and client.is_active = true
		 and client.is_agency = false
		where agency.id = $1::uuid
		  and agency.is_active = true
		  and agency.is_agency = true
		  and agency.organization_id is not null
		  and client.id = $2::uuid
	)`
	var allowed bool
	err := s.pool.QueryRow(ctx, q, agencyAccountID, clientAccountID).Scan(&allowed)
	return allowed, err
}

func (s *Store) AccountIsAgency(ctx context.Context, accountID string) (bool, error) {
	const q = `select exists (
		select 1 from core.accounts
		where id = $1::uuid and is_active = true and is_agency = true
	)`
	var isAgency bool
	err := s.pool.QueryRow(ctx, q, accountID).Scan(&isAgency)
	return isAgency, err
}

// DeleteConnection remove a conexao da account (cascade nas tabelas filhas).
// Idempotente: ausencia de linha nao e erro.
func (s *Store) DeleteConnection(ctx context.Context, accountID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockMetaActionConnectionTx(ctx, tx, accountID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`delete from meta_ads.connections where account_id = $1::uuid`, accountID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetDecryptedToken decifra e retorna o token da conexao da account. Usado
// apenas no momento de chamar a Graph; o valor nunca e logado.
func (s *Store) GetDecryptedToken(ctx context.Context, accountID string) (string, error) {
	if s.cryptoKey == "" {
		return "", ErrCryptoKeyMissing
	}
	const q = `select pgp_sym_decrypt(encrypted_token, $2)
		from meta_ads.connections
		where account_id = $1 and status = 'active'
		  and (token_expires_at is null or token_expires_at > now())`
	var token string
	if err := s.pool.QueryRow(ctx, q, accountID, s.cryptoKey).Scan(&token); err != nil {
		return "", err
	}
	return token, nil
}

// GetDecryptedTokenAtRevision evita que uma sincronizacao iniciada com uma
// conexao antiga continue depois de uma rotacao concorrente do token.
func (s *Store) GetDecryptedTokenAtRevision(ctx context.Context, accountID, revision string) (string, error) {
	if s.cryptoKey == "" {
		return "", ErrCryptoKeyMissing
	}
	const q = `select pgp_sym_decrypt(encrypted_token, $3)
		from meta_ads.connections
		where account_id = $1::uuid and revision = $2::uuid and status = 'active'
		  and (token_expires_at is null or token_expires_at > now())`
	var token string
	if err := s.pool.QueryRow(ctx, q, accountID, revision, s.cryptoKey).Scan(&token); errors.Is(err, pgx.ErrNoRows) {
		return "", ErrConnectionChanged
	} else if err != nil {
		return "", err
	}
	return token, nil
}

func scanConnection(row rowScanner) (Connection, error) {
	var c Connection
	err := row.Scan(
		&c.ID, &c.AccountID, &c.OrganizationID, &c.MetaBusinessID, &c.Name,
		&c.TokenExpiresAt, &c.Status, &c.Revision, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

// noRows e um atalho para checar o sentinel do pgx.
func noRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
