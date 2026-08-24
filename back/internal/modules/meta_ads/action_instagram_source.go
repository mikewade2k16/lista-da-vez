package metaads

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"
)

const actionInstagramMediaLookupLimit = 20

// ValidatePromotableInstagramPost revalida a selecao na fonte viva antes de
// persistir e novamente antes do claim. O payload contem IDs derivados pelo
// backend do registry do chat; nenhum URL ou legenda decide autoridade.
func (s *Service) ValidatePromotableInstagramPost(
	ctx context.Context,
	viewerAccountID, adAccountID string,
	raw json.RawMessage,
) error {
	var payload promoteInstagramPostActionPayload
	if err := decodeStrictActionJSON(raw, &payload); err != nil {
		return ErrActionValidation
	}
	adAccount, err := s.requireAdAccount(ctx, viewerAccountID, adAccountID)
	if err != nil {
		return err
	}
	if err := s.store.ValidateActionInstagramIdentityScope(
		ctx, adAccount.AccountID, viewerAccountID, adAccount.ID,
		payload.IGUserID, payload.PageID, payload.ClientAccountID,
	); err != nil {
		return err
	}
	accounts, err := s.InstagramAccounts(ctx, adAccount.AccountID)
	if err != nil {
		return err
	}
	foundIdentity := false
	for _, account := range accounts {
		if account.IGUserID == payload.IGUserID && account.PageID == payload.PageID {
			foundIdentity = true
			break
		}
	}
	if !foundIdentity {
		return pgx.ErrNoRows
	}
	media, err := s.InstagramMedia(
		ctx, adAccount.AccountID, payload.IGUserID, actionInstagramMediaLookupLimit,
	)
	if err != nil {
		return err
	}
	for _, post := range media {
		if strings.TrimSpace(post.ID) == payload.InstagramPostID {
			return nil
		}
	}
	return pgx.ErrNoRows
}

// ValidateActionInstagramIdentityScope cruza ad account e Page/IG sob a mesma
// conexao atual. Uma identidade atribuida a outro cliente nunca pode ser usada
// mesmo que o token da agencia consiga le-la na Graph.
func (s *Store) ValidateActionInstagramIdentityScope(
	ctx context.Context,
	resourceAccountID, viewerAccountID, adAccountID, igUserID, pageID, payloadClientID string,
) error {
	const query = `select 1
		from meta_ads.ad_accounts aa
		join meta_ads.connections connection
		  on connection.id = aa.connection_id and connection.account_id = aa.account_id
		join core.accounts viewer on viewer.id = $2::uuid and viewer.is_active
		left join meta_ads.instagram_identity_client_mappings mapping
		  on mapping.account_id = aa.account_id
		 and mapping.connection_id = aa.connection_id
		 and mapping.ig_user_id = $4
		 and mapping.page_id = $5
		where aa.account_id = $1::uuid and aa.id = $3::uuid
		  and aa.is_current and connection.status = 'active'
		  and (connection.token_expires_at is null or connection.token_expires_at > now())
		  and (
		    (aa.client_account_id is not null
		      and aa.client_account_id::text = $6
		      and mapping.client_account_id = aa.client_account_id)
		    or (aa.client_account_id is null
		      and aa.account_id = viewer.id
		      and mapping.id is null
		      and ($6 = '' or ($6 = viewer.id::text and not viewer.is_agency)))
		  )`
	var exists int
	return s.pool.QueryRow(ctx, query,
		resourceAccountID, viewerAccountID, adAccountID,
		igUserID, pageID, payloadClientID,
	).Scan(&exists)
}
