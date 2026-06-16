package metaads

import (
	"context"
	"time"
)

// syncDatePreset e a janela puxada da Graph a cada sync (MVP: ultimos 30 dias).
const syncDatePreset = "last_30d"

// Sync puxa campanhas + insights diarios (nivel conta agregado + por campanha)
// da Graph para o cache local de uma conta de anuncio. Retorna as contagens
// upsertadas. Requer conexao ativa e que a conta de anuncio pertenca a account.
func (s *Service) Sync(ctx context.Context, accountID, adAccountID string) (SyncResult, error) {
	adAccount, err := s.requireAdAccount(ctx, accountID, adAccountID)
	if err != nil {
		return SyncResult{}, err
	}
	token, err := s.store.GetDecryptedToken(ctx, accountID)
	if err != nil {
		return SyncResult{}, err
	}

	campaigns, err := s.syncCampaigns(ctx, accountID, adAccount, token)
	if err != nil {
		return SyncResult{}, err
	}
	insights, err := s.syncInsights(ctx, accountID, adAccount, token)
	if err != nil {
		return SyncResult{}, err
	}

	return SyncResult{
		Campaigns: campaigns,
		Insights:  insights,
		SyncedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// syncCampaigns puxa e cacheia as campanhas da conta de anuncio.
func (s *Service) syncCampaigns(ctx context.Context, accountID string, ad AdAccount, token string) (int, error) {
	remote, err := s.client.ListCampaigns(ctx, token, ad.MetaAdAccountID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, c := range remote {
		row := Campaign{
			AccountID:      accountID,
			AdAccountID:    ad.ID,
			MetaCampaignID: c.ID,
			Name:           c.Name,
			Objective:      c.Objective,
			Status:         c.Status,
			DailyBudget:    budgetCentsToUnits(c.DailyBudget),
			LifetimeBudget: budgetCentsToUnits(c.LifetimeBudget),
		}
		if err := s.store.UpsertCampaign(ctx, row); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// syncInsights puxa e cacheia os insights diarios nos dois niveis: por campanha
// (meta_campaign_id) e agregado da conta (meta_campaign_id = ”).
func (s *Service) syncInsights(ctx context.Context, accountID string, ad AdAccount, token string) (int, error) {
	count := 0

	// Nivel campanha.
	perCampaign, err := s.client.GetInsights(ctx, token, ad.MetaAdAccountID, syncDatePreset, "campaign")
	if err != nil {
		return 0, err
	}
	for _, gi := range perCampaign {
		if err := s.upsertInsight(ctx, accountID, ad.ID, gi.CampaignID, gi); err != nil {
			return count, err
		}
		count++
	}

	// Nivel conta (agregado): meta_campaign_id = accountLevelCampaignID ('').
	accountLevel, err := s.client.GetInsights(ctx, token, ad.MetaAdAccountID, syncDatePreset, "account")
	if err != nil {
		return count, err
	}
	for _, gi := range accountLevel {
		if err := s.upsertInsight(ctx, accountID, ad.ID, accountLevelCampaignID, gi); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// upsertInsight mapeia um GraphInsight -> InsightDaily e persiste. Linhas sem
// data valida sao ignoradas (sem erro).
func (s *Service) upsertInsight(ctx context.Context, accountID, adAccountID, metaCampaignID string, gi GraphInsight) error {
	date, err := time.Parse("2006-01-02", gi.DateStart)
	if err != nil {
		return nil
	}
	row := InsightDaily{
		AccountID:      accountID,
		AdAccountID:    adAccountID,
		MetaCampaignID: metaCampaignID,
		Date:           date,
		Impressions:    parseInt(gi.Impressions),
		Clicks:         parseInt(gi.Clicks),
		Spend:          parseFloat(gi.Spend),
		Reach:          parseInt(gi.Reach),
		CTR:            parseFloat(gi.CTR),
		CPC:            parseFloat(gi.CPC),
		CPM:            parseFloat(gi.CPM),
		Conversions:    conversionsFromActions(gi),
	}
	return s.store.UpsertInsight(ctx, row)
}
