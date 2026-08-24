package metaads

import (
	"testing"
	"time"
)

func TestGraphAdAccountsSnapshotStartsOnlyFetchedRowsAsCurrent(t *testing.T) {
	rows := graphAdAccountsSnapshot([]GraphAdAccount{
		{AccountID: "123", Name: "Conta", Currency: "BRL", AccountStatus: 1},
	})
	if len(rows) != 1 || rows[0].MetaAdAccountID != "123" || !rows[0].IsCurrent {
		t.Fatalf("snapshot = %#v", rows)
	}
	if got := graphAdAccountsSnapshot(nil); len(got) != 0 {
		t.Fatalf("empty snapshot = %#v", got)
	}
}

func TestReportingSnapshotRowsAreCompleteAndDropInvalidDates(t *testing.T) {
	campaigns := campaignSnapshotRows("account", "ad-account", []GraphCampaign{
		{ID: "campaign", Name: "Campanha", DailyBudget: "12345"},
	})
	if len(campaigns) != 1 || !campaigns[0].IsCurrent || campaigns[0].DailyBudget == nil ||
		*campaigns[0].DailyBudget != 123.45 {
		t.Fatalf("campaign snapshot = %#v", campaigns)
	}

	perCampaign := []GraphInsight{
		{CampaignID: "campaign", DateStart: "2026-08-18", Spend: "12.50"},
		{CampaignID: "invalid", DateStart: "not-a-date", Spend: "999"},
	}
	accountLevel := []GraphInsight{{DateStart: "2026-08-18", Spend: "12.50"}}
	insights := insightSnapshotRows("account", "ad-account", perCampaign, accountLevel)
	if len(insights) != 2 {
		t.Fatalf("insight snapshot = %#v", insights)
	}
	if insights[0].MetaCampaignID != "campaign" || insights[1].MetaCampaignID != accountLevelCampaignID {
		t.Fatalf("levels = %#v", insights)
	}
	if !insights[0].Date.Equal(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date = %s", insights[0].Date)
	}
}
