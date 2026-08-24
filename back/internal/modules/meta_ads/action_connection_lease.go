package metaads

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A chave de duas partes reserva um namespace somente para a conexao usada por
// mutacoes Meta. O segundo componente e deterministico por resource account.
// Colisoes de hash apenas serializam duas contas; nunca liberam concorrencia.
const (
	metaActionConnectionLockNamespace int32 = 1296122179
	metaActionConnectionLockPrefix          = "meta_ads.action-connection.v1:"
	actionConnectionUnlockTimeout           = 5 * time.Second
)

var errActionConnectionLeaseRelease = errors.New("meta_ads: falha ao liberar lease da conexao")

func actionConnectionLockKey(accountID string) string {
	return metaActionConnectionLockPrefix + strings.ToLower(strings.TrimSpace(accountID))
}

func lockMetaActionConnectionTx(ctx context.Context, tx pgx.Tx, accountID string) error {
	_, err := tx.Exec(ctx, `select pg_advisory_xact_lock($1::integer, hashtext($2::text))`,
		metaActionConnectionLockNamespace, actionConnectionLockKey(accountID))
	return err
}

// WithDecryptedTokenAtRevision mantem um advisory lock de sessao desde a
// leitura da revisao/token ate o retorno da chamada Graph. Nao ha transacao de
// banco aberta durante I/O externo. Rotacao e exclusao usam o mesmo lock em
// modo xact e, portanto, aguardam o fim desta janela.
func (s *Store) WithDecryptedTokenAtRevision(
	ctx context.Context,
	accountID, connectionID, revision string,
	use func(string) error,
) (returnErr error) {
	if s.cryptoKey == "" {
		return ErrCryptoKeyMissing
	}
	if use == nil {
		return errors.New("meta_ads: callback de token ausente")
	}

	connection, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	locked := false
	defer func() {
		if !locked {
			connection.Release()
			return
		}
		if releaseErr := releaseMetaActionConnectionLease(connection, accountID); releaseErr != nil {
			returnErr = errors.Join(returnErr, releaseErr)
		}
	}()

	// Apos enviar pg_advisory_lock, ate um erro/cancelamento e ambiguo: o
	// servidor pode ter concedido o lock antes de a resposta chegar. Marcamos a
	// sessao para cleanup antes do Exec; unlock=false tambem a remove do pool.
	locked = true
	if _, err := connection.Exec(ctx,
		`select pg_advisory_lock($1::integer, hashtext($2::text))`,
		metaActionConnectionLockNamespace, actionConnectionLockKey(accountID),
	); err != nil {
		return err
	}

	const query = `select pgp_sym_decrypt(encrypted_token, $4)
		from meta_ads.connections
		where account_id = $1::uuid and id = $2::uuid and revision = $3::uuid
		  and status = 'active'
		  and (token_expires_at is null or token_expires_at > now())`
	var token string
	if err := connection.QueryRow(ctx, query,
		accountID, connectionID, revision, s.cryptoKey,
	).Scan(&token); errors.Is(err, pgx.ErrNoRows) {
		return ErrConnectionChanged
	} else if err != nil {
		return err
	}
	return use(token)
}

func releaseMetaActionConnectionLease(connection *pgxpool.Conn, accountID string) error {
	unlockCtx, cancel := context.WithTimeout(context.Background(), actionConnectionUnlockTimeout)
	defer cancel()

	var unlocked bool
	err := connection.QueryRow(unlockCtx,
		`select pg_advisory_unlock($1::integer, hashtext($2::text))`,
		metaActionConnectionLockNamespace, actionConnectionLockKey(accountID),
	).Scan(&unlocked)
	if err == nil && unlocked {
		connection.Release()
		return nil
	}

	// Uma sessao que pode continuar segurando o lock nunca volta ao pool. Fechar
	// a conexao PostgreSQL tambem libera qualquer advisory lock remanescente.
	rawConnection := connection.Hijack()
	closeCtx, closeCancel := context.WithTimeout(context.Background(), actionConnectionUnlockTimeout)
	defer closeCancel()
	closeErr := rawConnection.Close(closeCtx)
	if err != nil {
		return errors.Join(errActionConnectionLeaseRelease, err, closeErr)
	}
	if !unlocked {
		return errors.Join(errActionConnectionLeaseRelease,
			fmt.Errorf("meta_ads: pg_advisory_unlock retornou false"), closeErr)
	}
	return errors.Join(errActionConnectionLeaseRelease, closeErr)
}
