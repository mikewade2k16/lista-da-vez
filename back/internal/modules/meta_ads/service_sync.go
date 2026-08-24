package metaads

import (
	"context"
	"time"
)

// syncDatePreset cobre a maior janela oferecida pelo painel. Assim selecionar
// 90 dias nunca promete uma série que o sincronizador deliberadamente deixou
// pela metade; ranges menores continuam sendo recortes do mesmo cache.
const syncDatePreset = "last_90d"

// Sync puxa campanhas + insights diarios (nivel conta agregado + por campanha)
// da Graph para o cache local de uma conta de anuncio. Retorna as contagens
// upsertadas. Requer conexao ativa e que a conta de anuncio pertenca a account.
func (s *Service) Sync(ctx context.Context, accountID, adAccountID string) (SyncResult, error) {
	adAccount, err := s.requireAdAccount(ctx, accountID, adAccountID)
	if err != nil {
		return SyncResult{}, err
	}
	sourceAccountID := adAccount.AccountID
	connection, err := s.store.GetConnection(ctx, sourceAccountID)
	if err != nil {
		return SyncResult{}, err
	}
	if connection.ID != adAccount.ConnectionID {
		return SyncResult{}, ErrConnectionChanged
	}
	token, err := s.store.GetDecryptedTokenAtRevision(ctx, sourceAccountID, connection.Revision)
	if err != nil {
		return SyncResult{}, err
	}

	// Todas as paginas dos tres endpoints sao obtidas antes de abrir a transacao
	// de publicacao. Qualquer erro deixa o snapshot anterior integralmente ativo.
	remoteCampaigns, err := s.client.ListCampaigns(ctx, token, adAccount.MetaAdAccountID)
	if err != nil {
		return SyncResult{}, err
	}
	perCampaign, err := s.client.GetInsights(ctx, token, adAccount.MetaAdAccountID, syncDatePreset, "campaign")
	if err != nil {
		return SyncResult{}, err
	}
	accountLevel, err := s.client.GetInsights(ctx, token, adAccount.MetaAdAccountID, syncDatePreset, "account")
	if err != nil {
		return SyncResult{}, err
	}

	campaigns := campaignSnapshotRows(sourceAccountID, adAccount.ID, remoteCampaigns)
	insights := insightSnapshotRows(sourceAccountID, adAccount.ID, perCampaign, accountLevel)
	since, until := rangeWindow(syncDatePreset)
	if err := s.store.ReplaceReportingSnapshotAtRevision(
		ctx,
		sourceAccountID,
		connection.ID,
		adAccount.ID,
		connection.Revision,
		campaigns,
		insights,
		since,
		until,
	); err != nil {
		return SyncResult{}, err
	}

	return SyncResult{
		Campaigns: len(campaigns),
		Insights:  len(insights),
		SyncedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func campaignSnapshotRows(accountID, adAccountID string, remote []GraphCampaign) []Campaign {
	rows := make([]Campaign, 0, len(remote))
	for _, c := range remote {
		rows = append(rows, Campaign{
			AccountID:      accountID,
			AdAccountID:    adAccountID,
			MetaCampaignID: c.ID,
			Name:           c.Name,
			Objective:      c.Objective,
			Status:         c.Status,
			DailyBudget:    budgetCentsToUnits(c.DailyBudget),
			LifetimeBudget: budgetCentsToUnits(c.LifetimeBudget),
			IsCurrent:      true,
		})
	}
	return rows
}

func insightSnapshotRows(
	accountID, adAccountID string,
	perCampaign, accountLevel []GraphInsight,
) []InsightDaily {
	rows := make([]InsightDaily, 0, len(perCampaign)+len(accountLevel))
	for _, gi := range perCampaign {
		if row, ok := graphInsightSnapshotRow(accountID, adAccountID, gi.CampaignID, gi); ok {
			rows = append(rows, row)
		}
	}
	for _, gi := range accountLevel {
		if row, ok := graphInsightSnapshotRow(accountID, adAccountID, accountLevelCampaignID, gi); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func graphInsightSnapshotRow(
	accountID, adAccountID, metaCampaignID string,
	gi GraphInsight,
) (InsightDaily, bool) {
	date, err := time.Parse("2006-01-02", gi.DateStart)
	if err != nil {
		return InsightDaily{}, false
	}
	return InsightDaily{
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
	}, true
}
