package metaads

import (
	"context"
	"time"
)

// ============================================================================
// ad_accounts
// ============================================================================

// ListAdAccounts retorna as contas de anuncio cacheadas da account.
func (s *Store) ListAdAccounts(ctx context.Context, accountID string) ([]AdAccount, error) {
	const q = `select id, account_id, connection_id, meta_ad_account_id,
			client_account_id, name, currency, status, created_at, updated_at
		from meta_ads.ad_accounts
		where account_id = $1
		order by name, meta_ad_account_id`
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
	const q = `select id, account_id, connection_id, meta_ad_account_id,
			client_account_id, name, currency, status, created_at, updated_at
		from meta_ads.ad_accounts
		where account_id = $1 and id = $2`
	return scanAdAccount(s.pool.QueryRow(ctx, q, accountID, adAccountID))
}

// UpsertAdAccount insere/atualiza uma conta de anuncio (UNIQUE account+meta id).
func (s *Store) UpsertAdAccount(ctx context.Context, accountID, connectionID, metaAdAccountID, name, currency, status string) (AdAccount, error) {
	const q = `insert into meta_ads.ad_accounts
			(account_id, connection_id, meta_ad_account_id, name, currency, status, updated_at)
		values ($1, $2, $3, $4, $5, $6, now())
		on conflict (account_id, meta_ad_account_id) do update
			set connection_id = excluded.connection_id,
			    name = excluded.name,
			    currency = excluded.currency,
			    status = excluded.status,
			    updated_at = now()
		returning id, account_id, connection_id, meta_ad_account_id,
			client_account_id, name, currency, status, created_at, updated_at`
	return scanAdAccount(s.pool.QueryRow(ctx, q, accountID, connectionID, metaAdAccountID, name, currency, status))
}

func scanAdAccount(row rowScanner) (AdAccount, error) {
	var a AdAccount
	err := row.Scan(
		&a.ID, &a.AccountID, &a.ConnectionID, &a.MetaAdAccountID,
		&a.ClientAccountID, &a.Name, &a.Currency, &a.Status, &a.CreatedAt, &a.UpdatedAt,
	)
	return a, err
}

// ============================================================================
// campaigns
// ============================================================================

// ListCampaigns retorna as campanhas cacheadas de uma conta de anuncio (escopadas).
func (s *Store) ListCampaigns(ctx context.Context, accountID, adAccountID string) ([]Campaign, error) {
	const q = `select id, account_id, ad_account_id, meta_campaign_id, name,
			objective, status, daily_budget, lifetime_budget, synced_at
		from meta_ads.campaigns
		where account_id = $1 and ad_account_id = $2
		order by name, meta_campaign_id`
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

// UpsertCampaign insere/atualiza uma campanha cacheada (UNIQUE ad_account+meta id).
func (s *Store) UpsertCampaign(ctx context.Context, c Campaign) error {
	const q = `insert into meta_ads.campaigns
			(account_id, ad_account_id, meta_campaign_id, name, objective, status,
			 daily_budget, lifetime_budget, synced_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, now())
		on conflict (ad_account_id, meta_campaign_id) do update
			set name = excluded.name,
			    objective = excluded.objective,
			    status = excluded.status,
			    daily_budget = excluded.daily_budget,
			    lifetime_budget = excluded.lifetime_budget,
			    synced_at = now()`
	_, err := s.pool.Exec(ctx, q,
		c.AccountID, c.AdAccountID, c.MetaCampaignID, c.Name, c.Objective, c.Status,
		c.DailyBudget, c.LifetimeBudget,
	)
	return err
}

func scanCampaign(row rowScanner) (Campaign, error) {
	var c Campaign
	err := row.Scan(
		&c.ID, &c.AccountID, &c.AdAccountID, &c.MetaCampaignID, &c.Name,
		&c.Objective, &c.Status, &c.DailyBudget, &c.LifetimeBudget, &c.SyncedAt,
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
	const q = `select id, account_id, ad_account_id, meta_campaign_id, date,
			impressions, clicks, spend, reach, ctr, cpc, cpm, conversions, synced_at
		from meta_ads.insights_daily
		where account_id = $1 and ad_account_id = $2
		  and date >= $3 and date <= $4
		  and ($5 = '' or ($5 = 'account' and meta_campaign_id = '')
		                or ($5 = 'campaign' and meta_campaign_id <> ''))
		order by date, meta_campaign_id`
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

// UpsertInsight insere/atualiza uma linha de insight diario (UNIQUE
// ad_account+meta_campaign+date). meta_campaign_id = ” = agregado da conta.
func (s *Store) UpsertInsight(ctx context.Context, i InsightDaily) error {
	const q = `insert into meta_ads.insights_daily
			(account_id, ad_account_id, meta_campaign_id, date, impressions, clicks,
			 spend, reach, ctr, cpc, cpm, conversions, synced_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
		on conflict (ad_account_id, meta_campaign_id, date) do update
			set impressions = excluded.impressions,
			    clicks = excluded.clicks,
			    spend = excluded.spend,
			    reach = excluded.reach,
			    ctr = excluded.ctr,
			    cpc = excluded.cpc,
			    cpm = excluded.cpm,
			    conversions = excluded.conversions,
			    synced_at = now()`
	_, err := s.pool.Exec(ctx, q,
		i.AccountID, i.AdAccountID, i.MetaCampaignID, i.Date, i.Impressions, i.Clicks,
		i.Spend, i.Reach, i.CTR, i.CPC, i.CPM, i.Conversions,
	)
	return err
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
