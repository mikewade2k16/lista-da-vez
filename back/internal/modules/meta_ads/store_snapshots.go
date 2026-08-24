package metaads

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrConnectionChanged indica que o token/conexao mudou enquanto uma leitura
// externa estava em andamento. O snapshot obtido com a revisao antiga jamais e
// persistido sobre a conexao nova.
var ErrConnectionChanged = errors.New("meta_ads: conexao alterada durante sincronizacao")

// SaveConnectionSnapshot grava token, expiracao e a lista completa de contas
// de anuncio na mesma transacao. Reconexoes concorrentes ficam serializadas pelo
// conflito em connections.account_id; a ultima transacao completa vence inteira.
func (s *Store) SaveConnectionSnapshot(
	ctx context.Context,
	accountID, metaBusinessID, name, token string,
	tokenExpiresAt *time.Time,
	adAccounts []AdAccount,
) (Connection, error) {
	if s.cryptoKey == "" {
		return Connection{}, ErrCryptoKeyMissing
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Connection{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockMetaActionConnectionTx(ctx, tx, accountID); err != nil {
		return Connection{}, err
	}

	const connectionQuery = `insert into meta_ads.connections
		(account_id, meta_business_id, name, encrypted_token, token_expires_at,
		 status, revision, updated_at)
	values ($1::uuid, $2, $3, pgp_sym_encrypt($4, $5), $6, 'active', gen_random_uuid(), now())
	 on conflict (account_id) do update
	 set meta_business_id = excluded.meta_business_id,
	     name = excluded.name,
	     encrypted_token = excluded.encrypted_token,
	     token_expires_at = excluded.token_expires_at,
	     status = 'active',
	     revision = gen_random_uuid(),
	     updated_at = now()
	 returning id::text, account_id::text, organization_id::text,
		meta_business_id, name, token_expires_at, status, revision::text,
		created_at, updated_at`
	connection, err := scanConnection(tx.QueryRow(
		ctx, connectionQuery, accountID, metaBusinessID, name, token, s.cryptoKey, tokenExpiresAt,
	))
	if err != nil {
		return Connection{}, err
	}
	if err := replaceAdAccountsSnapshotTx(ctx, tx, accountID, connection.ID, adAccounts); err != nil {
		return Connection{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Connection{}, err
	}
	return connection, nil
}

// ReplaceAdAccountsSnapshotAtRevision troca o conjunto corrente sem remover as
// linhas historicas. A revisao e checada sob row lock no mesmo commit.
func (s *Store) ReplaceAdAccountsSnapshotAtRevision(
	ctx context.Context,
	accountID, connectionID, expectedRevision string,
	adAccounts []AdAccount,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockConnectionRevision(ctx, tx, accountID, connectionID, expectedRevision); err != nil {
		return err
	}
	if err := replaceAdAccountsSnapshotTx(ctx, tx, accountID, connectionID, adAccounts); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replaceAdAccountsSnapshotTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID, connectionID string,
	adAccounts []AdAccount,
) error {
	// Uma nova descoberta (inclusive reconexao/token novo) invalida todo dado de
	// relatorio produzido sob o conjunto anterior de grants. As linhas de
	// campanha permanecem para auditoria/IDs, mas nao voltam a ficar visiveis so
	// porque a mesma ad account reapareceu; ela exige um Sync novo.
	if _, err := tx.Exec(ctx, `update meta_ads.campaigns c
		set is_current = false
		from meta_ads.ad_accounts aa
		where aa.id = c.ad_account_id and aa.account_id = c.account_id
		  and aa.account_id = $1::uuid and aa.connection_id = $2::uuid
		  and c.is_current`, accountID, connectionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from meta_ads.insights_daily i
		using meta_ads.ad_accounts aa
		where aa.id = i.ad_account_id and aa.account_id = i.account_id
		  and aa.account_id = $1::uuid and aa.connection_id = $2::uuid`,
		accountID, connectionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update meta_ads.ad_accounts
		set is_current = false, updated_at = now()
		where account_id = $1::uuid and connection_id = $2::uuid and is_current`,
		accountID, connectionID); err != nil {
		return err
	}
	const upsert = `insert into meta_ads.ad_accounts
		(account_id, connection_id, meta_ad_account_id, name, currency, status,
		 is_current, updated_at)
	values ($1::uuid, $2::uuid, $3, $4, $5, $6, true, now())
	 on conflict (account_id, meta_ad_account_id) do update
	 set connection_id = excluded.connection_id,
	     name = excluded.name,
	     currency = excluded.currency,
	     status = excluded.status,
	     is_current = true,
	     updated_at = now()`
	for _, account := range adAccounts {
		if _, err := tx.Exec(ctx, upsert, accountID, connectionID,
			account.MetaAdAccountID, account.Name, account.Currency, account.Status); err != nil {
			return err
		}
	}
	return nil
}

// ReplaceReportingSnapshotAtRevision publica campanhas e insights somente
// depois que todas as paginas dos tres endpoints Graph foram obtidas. O cache
// antigo continua intacto em qualquer erro anterior ou durante a transacao.
func (s *Store) ReplaceReportingSnapshotAtRevision(
	ctx context.Context,
	accountID, connectionID, adAccountID, expectedRevision string,
	campaigns []Campaign,
	insights []InsightDaily,
	insightsSince, insightsUntil time.Time,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockConnectionRevision(ctx, tx, accountID, connectionID, expectedRevision); err != nil {
		return err
	}
	var lockedAdAccountID string
	err = tx.QueryRow(ctx, `select id::text
		from meta_ads.ad_accounts
		where account_id = $1::uuid and connection_id = $2::uuid
		  and id = $3::uuid and is_current
		for update`, accountID, connectionID, adAccountID).Scan(&lockedAdAccountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConnectionChanged
	}
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `update meta_ads.campaigns
		set is_current = false
		where account_id = $1::uuid and ad_account_id = $2::uuid and is_current`,
		accountID, adAccountID); err != nil {
		return err
	}
	const campaignUpsert = `insert into meta_ads.campaigns
		(account_id, ad_account_id, meta_campaign_id, name, objective, status,
		 daily_budget, lifetime_budget, is_current, synced_at)
	values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, true, now())
	 on conflict (ad_account_id, meta_campaign_id) do update
	 set name = excluded.name,
	     objective = excluded.objective,
	     status = excluded.status,
	     daily_budget = excluded.daily_budget,
	     lifetime_budget = excluded.lifetime_budget,
	     is_current = true,
	     synced_at = now()`
	for _, campaign := range campaigns {
		if _, err := tx.Exec(ctx, campaignUpsert, accountID, adAccountID,
			campaign.MetaCampaignID, campaign.Name, campaign.Objective, campaign.Status,
			campaign.DailyBudget, campaign.LifetimeBudget); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `delete from meta_ads.insights_daily
		where account_id = $1::uuid and ad_account_id = $2::uuid
		  and date >= $3::date and date <= $4::date`,
		accountID, adAccountID, insightsSince, insightsUntil); err != nil {
		return err
	}
	const insightInsert = `insert into meta_ads.insights_daily
		(account_id, ad_account_id, meta_campaign_id, date, impressions, clicks,
		 spend, reach, ctr, cpc, cpm, conversions, synced_at)
	values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
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
	for _, insight := range insights {
		if _, err := tx.Exec(ctx, insightInsert, accountID, adAccountID,
			insight.MetaCampaignID, insight.Date, insight.Impressions, insight.Clicks,
			insight.Spend, insight.Reach, insight.CTR, insight.CPC, insight.CPM,
			insight.Conversions); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func lockConnectionRevision(
	ctx context.Context,
	tx pgx.Tx,
	accountID, connectionID, expectedRevision string,
) error {
	var lockedConnectionID string
	err := tx.QueryRow(ctx, `select id::text
		from meta_ads.connections
		where account_id = $1::uuid and id = $2::uuid and revision = $3::uuid
		  and status = 'active'
		  and (token_expires_at is null or token_expires_at > now())
		for update`, accountID, connectionID, expectedRevision).Scan(&lockedConnectionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConnectionChanged
	}
	return err
}
