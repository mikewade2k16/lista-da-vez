package metaads

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ListInstagramIdentityMappings lista somente atribuicoes pertencentes a dona
// da conexao. O teto acompanha o limite seguro da descoberta Graph e impede
// que legado ou corrupcao infle o contexto do assistente.
func (s *Store) ListInstagramIdentityMappings(ctx context.Context, accountID string) ([]InstagramIdentityClientMapping, error) {
	const q = `select id, account_id, connection_id, client_account_id, ig_user_id, page_id,
			created_at, updated_at
		from meta_ads.instagram_identity_client_mappings
		where account_id = $1::uuid
		order by ig_user_id
		limit $2`
	rows, err := s.pool.Query(ctx, q, accountID, instagramIdentityMappingLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]InstagramIdentityClientMapping, 0)
	for rows.Next() {
		mapping, scanErr := scanInstagramIdentityMapping(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, mapping)
	}
	return out, rows.Err()
}

// SetInstagramIdentityClient troca o vinculo de forma atomica. O INSERT repete
// no PostgreSQL as invariantes agencia/cliente ativo/mesma organizacao; se elas
// mudarem entre a validacao do service e a escrita, a transacao inteira fecha.
func (s *Store) SetInstagramIdentityClient(
	ctx context.Context, accountID, clientAccountID, igUserID, pageID string,
) (InstagramIdentityClientMapping, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return InstagramIdentityClientMapping{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockMetaActionConnectionTx(ctx, tx, accountID); err != nil {
		return InstagramIdentityClientMapping{}, err
	}

	const removeStale = `delete from meta_ads.instagram_identity_client_mappings
		where account_id = $1::uuid
		  and (ig_user_id = $2 or page_id = $3)
		  and not (ig_user_id = $2 and page_id = $3)`
	if _, err = tx.Exec(ctx, removeStale, accountID, igUserID, pageID); err != nil {
		return InstagramIdentityClientMapping{}, err
	}

	const insert = `insert into meta_ads.instagram_identity_client_mappings
			(account_id, connection_id, client_account_id, ig_user_id, page_id)
		select agency.id, connection.id, client.id, $3, $4
		from core.accounts agency
		join core.accounts client
		  on client.organization_id = agency.organization_id
		 and client.is_active = true
		 and client.is_agency = false
		join meta_ads.connections connection
		  on connection.account_id = agency.id
		 and connection.status = 'active'
		where agency.id = $1::uuid
		  and agency.is_active = true
		  and agency.is_agency = true
		  and agency.organization_id is not null
		  and client.id = $2::uuid
		on conflict (account_id, ig_user_id) do update
		set connection_id = excluded.connection_id,
		    client_account_id = excluded.client_account_id,
		    page_id = excluded.page_id,
		    updated_at = now()
		returning id, account_id, connection_id, client_account_id, ig_user_id, page_id,
			created_at, updated_at`
	mapping, err := scanInstagramIdentityMapping(
		tx.QueryRow(ctx, insert, accountID, clientAccountID, igUserID, pageID),
	)
	if err != nil {
		return InstagramIdentityClientMapping{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return InstagramIdentityClientMapping{}, err
	}
	return mapping, nil
}

// DeleteInstagramIdentityMapping e idempotente. O service ja revalidou o par
// vivo na Graph; remover por qualquer um dos IDs tambem limpa um mapping stale
// quando a Page trocou de Instagram (ou o inverso).
func (s *Store) DeleteInstagramIdentityMapping(ctx context.Context, accountID, igUserID, pageID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockMetaActionConnectionTx(ctx, tx, accountID); err != nil {
		return err
	}
	const q = `delete from meta_ads.instagram_identity_client_mappings
		where account_id = $1::uuid and (ig_user_id = $2 or page_id = $3)`
	if _, err = tx.Exec(ctx, q, accountID, igUserID, pageID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func scanInstagramIdentityMapping(row rowScanner) (InstagramIdentityClientMapping, error) {
	var mapping InstagramIdentityClientMapping
	err := row.Scan(
		&mapping.ID, &mapping.AccountID, &mapping.ConnectionID, &mapping.ClientAccountID,
		&mapping.IGUserID, &mapping.PageID, &mapping.CreatedAt, &mapping.UpdatedAt,
	)
	return mapping, err
}
