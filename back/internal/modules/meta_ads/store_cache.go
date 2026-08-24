package metaads

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// ad_accounts
// ============================================================================

// ListAdAccounts retorna as contas de anuncio cacheadas da account.
func (s *Store) ListAdAccounts(ctx context.Context, accountID string) ([]AdAccount, error) {
	const q = `select aa.id, aa.account_id, aa.connection_id, aa.meta_ad_account_id,
			aa.client_account_id, aa.name, aa.currency, aa.status, aa.is_current,
			aa.created_at, aa.updated_at
		from meta_ads.ad_accounts aa
		join meta_ads.connections c on c.id = aa.connection_id and c.account_id = aa.account_id
		where aa.account_id = $1 and aa.is_current and c.status = 'active'
		order by aa.name, aa.meta_ad_account_id`
	rows, err := s.pool.Query(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdAccount
	for rows.Next() {
		a, scanErr := scanAdAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAdAccount retorna uma conta de anuncio pelo id da linha, escopada na account
// (pgx.ErrNoRows se nao existe OU pertence a outra account).
func (s *Store) GetAdAccount(ctx context.Context, accountID, adAccountID string) (AdAccount, error) {
	const q = `select aa.id, aa.account_id, aa.connection_id, aa.meta_ad_account_id,
			aa.client_account_id, aa.name, aa.currency, aa.status, aa.is_current,
			aa.created_at, aa.updated_at
		from meta_ads.ad_accounts aa
		join meta_ads.connections c on c.id = aa.connection_id and c.account_id = aa.account_id
		where aa.account_id = $1 and aa.id = $2 and aa.is_current and c.status = 'active'`
	return scanAdAccount(s.pool.QueryRow(ctx, q, accountID, adAccountID))
}

// SetAdAccountClient vincula (ou desvincula, clientAccountID vazio) uma conta de
// anuncio a um cliente. O filtro por account_id impede alterar uma linha de outra
// conexao/agencia.
func (s *Store) SetAdAccountClient(ctx context.Context, accountID, adAccountID, clientAccountID string) (AdAccount, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AdAccount{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockMetaActionConnectionTx(ctx, tx, accountID); err != nil {
		return AdAccount{}, err
	}
	const q = `update meta_ads.ad_accounts
		set client_account_id = nullif($3, '')::uuid,
		    updated_at = now()
		where account_id = $1::uuid and id = $2::uuid and is_current
		returning id, account_id, connection_id, meta_ad_account_id,
			client_account_id, name, currency, status, is_current, created_at, updated_at`
	adAccount, err := scanAdAccount(tx.QueryRow(ctx, q, accountID, adAccountID, clientAccountID))
	if err != nil {
		return AdAccount{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AdAccount{}, err
	}
	return adAccount, nil
}

func scanAdAccount(row rowScanner) (AdAccount, error) {
	var a AdAccount
	err := row.Scan(
		&a.ID, &a.AccountID, &a.ConnectionID, &a.MetaAdAccountID,
		&a.ClientAccountID, &a.Name, &a.Currency, &a.Status, &a.IsCurrent, &a.CreatedAt, &a.UpdatedAt,
	)
	return a, err
}

// ============================================================================
// campaigns
// ============================================================================

// ListCampaigns retorna as campanhas cacheadas de uma conta de anuncio (escopadas).
func (s *Store) ListCampaigns(ctx context.Context, accountID, adAccountID string) ([]Campaign, error) {
	const q = `select campaign.id, campaign.account_id, campaign.ad_account_id,
			campaign.meta_campaign_id, campaign.name, campaign.objective,
			campaign.status, campaign.daily_budget, campaign.lifetime_budget,
			campaign.is_current, campaign.synced_at
		from meta_ads.campaigns campaign
		join meta_ads.ad_accounts aa
		  on aa.id = campaign.ad_account_id and aa.account_id = campaign.account_id
		join meta_ads.connections connection
		  on connection.id = aa.connection_id and connection.account_id = aa.account_id
		where campaign.account_id = $1 and campaign.ad_account_id = $2
		  and campaign.is_current and aa.is_current and connection.status = 'active'
		order by campaign.name, campaign.meta_campaign_id`
	rows, err := s.pool.Query(ctx, q, accountID, adAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Campaign
	for rows.Next() {
		c, scanErr := scanCampaign(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanCampaign(row rowScanner) (Campaign, error) {
	var c Campaign
	err := row.Scan(
		&c.ID, &c.AccountID, &c.AdAccountID, &c.MetaCampaignID, &c.Name,
		&c.Objective, &c.Status, &c.DailyBudget, &c.LifetimeBudget, &c.IsCurrent, &c.SyncedAt,
	)
	return c, err
}

// ============================================================================
// insights_daily
// ============================================================================

// ListInsights retorna os insights cacheados de uma conta de anuncio num nivel
// e janela de datas. level "account" => so a linha agregada (meta_campaign_id =
// ”); level "campaign" => so as linhas por campanha.
func (s *Store) ListInsights(ctx context.Context, accountID, adAccountID, level string, since, until time.Time) ([]InsightDaily, error) {
	const q = `select i.id, i.account_id, i.ad_account_id, i.meta_campaign_id, i.date,
			i.impressions, i.clicks, i.spend, i.reach, i.ctr, i.cpc, i.cpm,
			i.conversions, i.synced_at
		from meta_ads.insights_daily i
		join meta_ads.ad_accounts aa on aa.id = i.ad_account_id and aa.account_id = i.account_id
		join meta_ads.connections connection
		  on connection.id = aa.connection_id and connection.account_id = aa.account_id
		where i.account_id = $1 and i.ad_account_id = $2 and aa.is_current
		  and connection.status = 'active'
		  and i.date >= $3 and i.date <= $4
		  and ($5 = '' or ($5 = 'account' and i.meta_campaign_id = '')
		                or ($5 = 'campaign' and i.meta_campaign_id <> ''))
		order by i.date, i.meta_campaign_id`
	rows, err := s.pool.Query(ctx, q, accountID, adAccountID, since, until, level)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InsightDaily
	for rows.Next() {
		i, scanErr := scanInsight(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func scanInsight(row rowScanner) (InsightDaily, error) {
	var i InsightDaily
	err := row.Scan(
		&i.ID, &i.AccountID, &i.AdAccountID, &i.MetaCampaignID, &i.Date,
		&i.Impressions, &i.Clicks, &i.Spend, &i.Reach, &i.CTR, &i.CPC, &i.CPM,
		&i.Conversions, &i.SyncedAt,
	)
	return i, err
}
