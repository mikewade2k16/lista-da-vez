package metaads

import "context"

// ListAssistantAdAccounts aplica o escopo client/all dentro do PostgreSQL e
// le somente limit+1 linhas. A linha extra informa truncamento sem carregar a
// lista completa para a memoria do chat.
func (s *Store) ListAssistantAdAccounts(
	ctx context.Context,
	sourceAccountID string,
	requestAccountID string,
	clientAccountID string,
	visibleClientIDs []string,
	isAgency bool,
	limit int,
) ([]AdAccount, bool, error) {
	if limit <= 0 {
		return []AdAccount{}, false, nil
	}
	const query = `select aa.id, aa.account_id, aa.connection_id, aa.meta_ad_account_id,
			aa.client_account_id, aa.name, aa.currency, aa.status, aa.is_current,
			aa.created_at, aa.updated_at
		from meta_ads.ad_accounts aa
		join meta_ads.connections connection
		  on connection.id = aa.connection_id and connection.account_id = aa.account_id
		where aa.account_id = $1::uuid and aa.is_current and connection.status = 'active'
		  and (connection.token_expires_at is null or connection.token_expires_at > now())
		  and (
			(nullif($3, '') is not null and (
				aa.client_account_id::text = $3
				or (aa.client_account_id is null and $1::text = $3)
			))
			or (nullif($3, '') is null and (
				(aa.client_account_id is null and ($5::boolean or $1::text = $2))
				or aa.client_account_id::text = any($4::text[])
			))
		  )
		order by aa.name, aa.meta_ad_account_id
		limit $6`
	rows, err := s.pool.Query(
		ctx,
		query,
		sourceAccountID,
		requestAccountID,
		clientAccountID,
		visibleClientIDs,
		isAgency,
		limit+1,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := make([]AdAccount, 0, limit+1)
	for rows.Next() {
		row, scanErr := scanAdAccount(rows)
		if scanErr != nil {
			return nil, false, scanErr
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(out) > limit
	if truncated {
		out = out[:limit]
	}
	return out, truncated, nil
}

// ListAssistantCampaigns repete ownership/current-state e limita cada consulta
// ao saldo global restante. Assim uma unica ad account com milhares de
// campanhas nao infla o prompt nem o heap do processo.
func (s *Store) ListAssistantCampaigns(
	ctx context.Context,
	accountID string,
	adAccountID string,
	limit int,
) ([]Campaign, bool, error) {
	if limit <= 0 {
		return []Campaign{}, false, nil
	}
	const query = `select campaign.id, campaign.account_id, campaign.ad_account_id,
			campaign.meta_campaign_id, campaign.name, campaign.objective,
			campaign.status, campaign.daily_budget, campaign.lifetime_budget,
			campaign.is_current, campaign.synced_at
		from meta_ads.campaigns campaign
		join meta_ads.ad_accounts aa
		  on aa.id = campaign.ad_account_id and aa.account_id = campaign.account_id
		join meta_ads.connections connection
		  on connection.id = aa.connection_id and connection.account_id = aa.account_id
		where campaign.account_id = $1::uuid and campaign.ad_account_id = $2::uuid
		  and campaign.is_current and aa.is_current and connection.status = 'active'
		  and (connection.token_expires_at is null or connection.token_expires_at > now())
		order by campaign.name, campaign.meta_campaign_id
		limit $3`
	rows, err := s.pool.Query(ctx, query, accountID, adAccountID, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := make([]Campaign, 0, limit+1)
	for rows.Next() {
		row, scanErr := scanCampaign(rows)
		if scanErr != nil {
			return nil, false, scanErr
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(out) > limit
	if truncated {
		out = out[:limit]
	}
	return out, truncated, nil
}
