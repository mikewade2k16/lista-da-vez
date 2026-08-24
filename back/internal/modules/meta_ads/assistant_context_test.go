package metaads

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFilterAssistantAdAccountsByClientScope(t *testing.T) {
	t.Parallel()
	clientA, clientB := "client-a", "client-b"
	rows := []AdAccount{
		{ID: "a", ClientAccountID: &clientA},
		{ID: "b", ClientAccountID: &clientB},
		{ID: "unassigned"},
	}

	selected := filterAssistantAdAccounts(rows, AssistantContextRequest{
		AccountID: "agency", ClientAccountID: clientA, VisibleClientIDs: []string{clientA, clientB}, IsAgency: true,
	}, "agency")
	assertAdAccountIDs(t, selected, "a")

	allVisible := filterAssistantAdAccounts(rows, AssistantContextRequest{
		AccountID: "agency", VisibleClientIDs: []string{clientA}, IsAgency: true,
	}, "agency")
	assertAdAccountIDs(t, allVisible, "a", "unassigned")

	clientUsingAgencyConnection := filterAssistantAdAccounts(rows, AssistantContextRequest{
		AccountID: clientA, ClientAccountID: clientA, VisibleClientIDs: []string{clientA},
	}, "agency")
	assertAdAccountIDs(t, clientUsingAgencyConnection, "a")
}

func TestFilterAssistantInstagramIdentitiesUsesExactClientMapping(t *testing.T) {
	t.Parallel()
	accounts := []InstagramAccountView{
		{IGUserID: "101", PageID: "201", Username: "client_a"},
		{IGUserID: "102", PageID: "202", Username: "client_b"},
		{IGUserID: "103", PageID: "203", Username: "unmapped"},
	}
	mappings := []InstagramIdentityClientMapping{
		{IGUserID: "101", PageID: "201", ClientAccountID: "client-a"},
		{IGUserID: "102", PageID: "202", ClientAccountID: "client-b"},
		// Mesmo igUserId com Page divergente e stale: nao autoriza client scope.
		{IGUserID: "103", PageID: "999", ClientAccountID: "client-a"},
	}

	selected := filterAssistantInstagramIdentities(accounts, mappings, AssistantContextRequest{
		AccountID: "agency", ClientAccountID: "client-a",
		VisibleClientIDs: []string{"client-a", "client-b"}, IsAgency: true,
	}, "agency")
	assertInstagramIdentityIDs(t, selected, "101")

	invisibleSelection := filterAssistantInstagramIdentities(accounts, mappings, AssistantContextRequest{
		AccountID: "agency", ClientAccountID: "client-a",
		VisibleClientIDs: []string{"client-b"}, IsAgency: true,
	}, "agency")
	assertInstagramIdentityIDs(t, invisibleSelection)

	clientUsingAgency := filterAssistantInstagramIdentities(accounts, mappings, AssistantContextRequest{
		AccountID: "client-a", ClientAccountID: "client-a",
		VisibleClientIDs: []string{"client-a"},
	}, "agency")
	assertInstagramIdentityIDs(t, clientUsingAgency, "101")

	allVisible := filterAssistantInstagramIdentities(accounts, mappings, AssistantContextRequest{
		AccountID: "agency", VisibleClientIDs: []string{"client-a"}, IsAgency: true,
	}, "agency")
	assertInstagramIdentityIDs(t, allVisible, "101", "103")

	ownConnection := filterAssistantInstagramIdentities(accounts, nil, AssistantContextRequest{
		AccountID: "client-a", ClientAccountID: "client-a",
	}, "client-a")
	assertInstagramIdentityIDs(t, ownConnection, "101", "102", "103")
}

func TestAssistantContextJSONContainsNoCredentialMaterial(t *testing.T) {
	t.Parallel()
	context := AssistantContext{
		Status:     "connected",
		Connection: AssistantConnectionView{Connected: true, Name: "Meta", Status: "active"},
		AdAccounts: []AssistantAdAccountView{{ID: "ad", MetaAdAccountID: "act_1", Name: "Conta"}},
		Campaigns: []AssistantCampaignView{{
			ID: "campaign", MetaCampaignID: "meta-campaign", AdAccountID: "ad", AdAccountName: "Conta",
			Name: "Campanha", SyncedAt: time.Now().UTC().Format(time.RFC3339),
		}},
		Instagram: AssistantInstagramContext{
			Status:   "available",
			Accounts: []InstagramIdentityView{{IGUserID: "101", Username: "brand"}},
			Posts:    []AssistantInstagramPostView{{ID: "post", IGUserID: "101", MediaType: "IMAGE", Permalink: "https://instagram.example/post"}},
		},
	}
	raw, err := json.Marshal(context)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"apikey", "api_key", "encrypted", "credential", "access_token", "tokenexpiresat"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("assistant context leaked forbidden field %q: %s", forbidden, raw)
		}
	}
}

func TestEmptyAssistantContextUsesStableArrays(t *testing.T) {
	t.Parallel()
	context := emptyAssistantContext("not_connected")
	if context.Status != "not_connected" || context.Connection.Connected || context.AdAccounts == nil ||
		context.Campaigns == nil || context.Performance == nil || context.Instagram.Posts == nil {
		t.Fatalf("unexpected empty context: %#v", context)
	}
}

func TestAssistantPerformanceIsolatesAuthorizedAdAccountAndRecomputesAggregates(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	clientA, clientB := "client-a", "client-b"
	accounts := filterAssistantAdAccounts([]AdAccount{
		{ID: "ad-a", AccountID: "agency", Name: "Conta A", Currency: "brl", ClientAccountID: &clientA},
		{ID: "ad-b", AccountID: "agency", Name: "Conta B", Currency: "usd", ClientAccountID: &clientB},
	}, AssistantContextRequest{
		AccountID: "client-a", ClientAccountID: "client-a", VisibleClientIDs: []string{"client-a"},
	}, "agency")
	if len(accounts) != 1 || accounts[0].ID != "ad-a" {
		t.Fatalf("unexpected authorized accounts: %#v", accounts)
	}
	rows := []InsightDaily{
		{AccountID: "agency", AdAccountID: "ad-a", Date: now, Impressions: 100, Clicks: 10, Spend: 25, Reach: 70, Conversions: 3, SyncedAt: now.Add(-time.Hour)},
		{AccountID: "agency", AdAccountID: "ad-b", Date: now, Impressions: 900, Clicks: 90, Spend: 900, Reach: 800, Conversions: 90, SyncedAt: now},
		{AccountID: "other-tenant", AdAccountID: "ad-a", Date: now, Impressions: 800, Clicks: 80, Spend: 800, Reach: 700, Conversions: 80, SyncedAt: now},
		{AccountID: "agency", AdAccountID: "ad-a", MetaCampaignID: "campaign", Date: now, Impressions: 700, Clicks: 70, Spend: 700, Reach: 600, Conversions: 70, SyncedAt: now},
	}

	view := assistantPerformanceForAdAccount(accounts[0], rows, now, 90)
	if view.Status != "fresh" || view.Currency != "BRL" || len(view.Daily) != 1 {
		t.Fatalf("unexpected performance view: %#v", view)
	}
	metrics := view.Last30Days
	if metrics.Spend != 25 || metrics.Impressions != 100 || metrics.Clicks != 10 ||
		metrics.ReachDailySum != 70 || metrics.Conversions != 3 || metrics.CTR != 10 || metrics.CPC != 2.5 {
		t.Fatalf("cross-account rows contaminated aggregate: %#v", metrics)
	}
}

func TestAssistantPerformanceBoundsSeriesAndMarksStale(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	account := AdAccount{ID: "ad-a", AccountID: "agency", Name: "Conta A", Currency: "BRL"}
	rows := make([]InsightDaily, 0, assistantPerformanceWindowDays+10)
	for offset := 0; offset < assistantPerformanceWindowDays+10; offset++ {
		rows = append(rows, InsightDaily{
			AccountID: "agency", AdAccountID: "ad-a", Date: now.AddDate(0, 0, -offset),
			Impressions: 100, Clicks: 10, Spend: 1, Reach: 50, Conversions: 1,
			SyncedAt: now.Add(-25 * time.Hour),
		})
	}

	view := assistantPerformanceForAdAccount(account, rows, now, 30)
	if view.Status != "stale" || len(view.Daily) != 30 || !view.DailyTruncated {
		t.Fatalf("expected stale bounded series, got %#v", view)
	}
	if view.Daily[0].Date != "2026-07-20" || view.Daily[len(view.Daily)-1].Date != "2026-08-18" {
		t.Fatalf("series did not retain the newest 30 days: %#v", view.Daily)
	}
	if view.Last30Days.Spend != 30 || view.Last7Days.Spend != 7 || view.Previous7Days.Spend != 7 ||
		view.Last30Days.ReachDailySum != 1500 {
		t.Fatalf("unexpected bounded aggregates: %#v", view)
	}
	if got := assistantPerformanceSeriesLimit(assistantPerformanceAccountLimit); got != 30 {
		t.Fatalf("series quota=%d want=30", got)
	}
	if got := assistantPerformanceSeriesLimit(4); got != assistantPerformanceWindowDays {
		t.Fatalf("series quota for four accounts=%d want=%d", got, assistantPerformanceWindowDays)
	}
}

func TestPopulateAssistantCachedContextBoundsQueriesAndSignalsTruncation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	clientA, clientB := "client-a", "client-b"
	rows := make([]AdAccount, 0, 40)
	for index := 0; index < 20; index++ {
		rows = append(rows,
			AdAccount{
				ID: fmt.Sprintf("ad-a-%02d", index), AccountID: "agency",
				Name: fmt.Sprintf("Conta A %02d", index), Currency: "BRL", ClientAccountID: &clientA,
			},
			AdAccount{
				ID: fmt.Sprintf("ad-b-%02d", index), AccountID: "agency",
				Name: fmt.Sprintf("Conta B %02d", index), Currency: "USD", ClientAccountID: &clientB,
			},
		)
	}
	authorized := filterAssistantAdAccounts(rows, AssistantContextRequest{
		AccountID: "client-a", ClientAccountID: clientA, VisibleClientIDs: []string{clientA},
	}, "agency")
	if len(authorized) != 20 {
		t.Fatalf("authorized accounts=%d want=20", len(authorized))
	}
	authorized = authorized[:assistantAdAccountLimit]
	result := AssistantContext{
		AdAccounts: []AssistantAdAccountView{}, AdAccountsTruncated: true,
		Campaigns: []AssistantCampaignView{}, CampaignsTruncated: true,
		Performance: []AssistantPerformanceView{}, PerformanceTruncated: true,
	}
	campaignCalls := 0
	insightCalls := 0
	err := populateAssistantCachedContext(
		context.Background(),
		"agency",
		authorized,
		&result,
		func(_ context.Context, accountID, adAccountID string, limit int) ([]Campaign, bool, error) {
			campaignCalls++
			if accountID != "agency" || strings.Contains(adAccountID, "ad-b-") {
				t.Fatalf("campaign query escaped client scope: account=%q adAccount=%q", accountID, adAccountID)
			}
			campaigns := make([]Campaign, 0, limit)
			for index := 0; index < limit; index++ {
				campaigns = append(campaigns, Campaign{
					ID: fmt.Sprintf("campaign-%03d", index), AccountID: accountID,
					AdAccountID: adAccountID, MetaCampaignID: fmt.Sprintf("meta-%03d", index),
					Name: "Campanha", SyncedAt: now,
				})
			}
			return campaigns, true, nil
		},
		func(
			_ context.Context,
			accountID string,
			adAccountID string,
			level string,
			_ time.Time,
			_ time.Time,
		) ([]InsightDaily, error) {
			insightCalls++
			if accountID != "agency" || level != "account" || strings.Contains(adAccountID, "ad-b-") {
				t.Fatalf("insight query escaped client scope: account=%q adAccount=%q level=%q", accountID, adAccountID, level)
			}
			insights := make([]InsightDaily, 0, assistantPerformanceWindowDays)
			for offset := 0; offset < assistantPerformanceWindowDays; offset++ {
				insights = append(insights, InsightDaily{
					AccountID: accountID, AdAccountID: adAccountID,
					Date: now.AddDate(0, 0, -offset), Impressions: 100,
					SyncedAt: now,
				})
			}
			return insights, nil
		},
		now,
	)
	if err != nil {
		t.Fatalf("populateAssistantCachedContext: %v", err)
	}
	if campaignCalls != 1 {
		t.Fatalf("campaign queries=%d want=1 after global cap", campaignCalls)
	}
	if insightCalls != assistantPerformanceAccountLimit {
		t.Fatalf("insight queries=%d want=%d", insightCalls, assistantPerformanceAccountLimit)
	}
	if len(result.AdAccounts) != assistantAdAccountLimit || len(result.Campaigns) != assistantCampaignLimit ||
		len(result.Performance) != assistantPerformanceAccountLimit {
		t.Fatalf("unexpected bounded context: ads=%d campaigns=%d performance=%d",
			len(result.AdAccounts), len(result.Campaigns), len(result.Performance))
	}
	pointCount := 0
	for _, performance := range result.Performance {
		pointCount += len(performance.Daily)
		if !performance.DailyTruncated {
			t.Fatalf("90-day series was not marked truncated: %#v", performance)
		}
	}
	if pointCount != assistantPerformancePointLimit {
		t.Fatalf("daily points=%d want=%d", pointCount, assistantPerformancePointLimit)
	}
	if !result.AdAccountsTruncated || !result.CampaignsTruncated || !result.PerformanceTruncated {
		t.Fatalf("missing truncation flags: %#v", result)
	}
}

func TestPopulateAssistantCachedContextKeepsCompleteSmallScopeUntruncated(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	rows := []AdAccount{
		{ID: "ad-1", AccountID: "agency", Name: "Conta 1"},
		{ID: "ad-2", AccountID: "agency", Name: "Conta 2"},
	}
	result := AssistantContext{
		AdAccounts:  []AssistantAdAccountView{},
		Campaigns:   []AssistantCampaignView{},
		Performance: []AssistantPerformanceView{},
	}
	campaignCalls := 0
	insightCalls := 0
	err := populateAssistantCachedContext(
		context.Background(), "agency", rows, &result,
		func(_ context.Context, accountID, adAccountID string, _ int) ([]Campaign, bool, error) {
			campaignCalls++
			return []Campaign{{
				ID: "campaign-" + adAccountID, AccountID: accountID,
				AdAccountID: adAccountID, SyncedAt: now,
			}}, false, nil
		},
		func(
			_ context.Context,
			accountID string,
			adAccountID string,
			_ string,
			_ time.Time,
			_ time.Time,
		) ([]InsightDaily, error) {
			insightCalls++
			return []InsightDaily{{
				AccountID: accountID, AdAccountID: adAccountID, Date: now, SyncedAt: now,
			}}, nil
		},
		now,
	)
	if err != nil {
		t.Fatalf("populateAssistantCachedContext: %v", err)
	}
	if campaignCalls != 2 || insightCalls != 2 || result.AdAccountsTruncated ||
		result.CampaignsTruncated || result.PerformanceTruncated {
		t.Fatalf("small scope unexpectedly truncated: campaigns=%d insights=%d result=%#v",
			campaignCalls, insightCalls, result)
	}
}

func TestAssistantPerformanceEmptyAndOutOfWindowRowsStayEmpty(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	account := AdAccount{ID: "ad-a", AccountID: "agency", Name: "Conta A", Currency: "BRL"}
	view := assistantPerformanceForAdAccount(account, []InsightDaily{{
		AccountID: "agency", AdAccountID: "ad-a",
		Date: now.AddDate(0, 0, -assistantPerformanceWindowDays), SyncedAt: now,
	}}, now, assistantPerformanceWindowDays)
	if view.Status != "empty" || view.SyncedAt != "" || view.Daily == nil || len(view.Daily) != 0 ||
		view.Last30Days != (AssistantPerformanceMetrics{}) {
		t.Fatalf("unexpected empty performance view: %#v", view)
	}
}

func TestAssistantResourcesUsePrefixedAuthoritativeIDsAndBoundedRoundRobin(t *testing.T) {
	t.Parallel()
	clientID := "client-a"
	context := AssistantContext{
		AdAccounts: []AssistantAdAccountView{{
			ID: "ad-1", MetaAdAccountID: "act_1", Name: "Conta principal", Currency: "BRL",
			Status: "active", ClientAccountID: &clientID,
		}},
		Instagram: AssistantInstagramContext{
			Accounts: []InstagramIdentityView{
				{IGUserID: "101", PageID: "201", Username: "marca"},
				{IGUserID: "102", PageID: "202", Username: "outra"},
			},
			Posts: []AssistantInstagramPostView{{
				ID: "post-1", Caption: "Legenda real", MediaType: "IMAGE",
				MediaURL: "https://cdn.example/post.jpg", Permalink: "https://instagram.com/p/post-1",
				IGUserID: "101", PageID: "201", Username: "marca",
			}, {
				ID: "post-2", Caption: "Outra legenda", MediaType: "VIDEO",
				ThumbnailURL: "https://cdn.example/post-2.jpg", Permalink: "https://instagram.com/p/post-2",
				IGUserID: "102", PageID: "202", Username: "outra",
			}},
		},
	}
	for index := 0; index < assistantResourceLimit+5; index++ {
		context.Campaigns = append(context.Campaigns, AssistantCampaignView{
			ID: "campaign-" + string(rune('a'+index)), MetaCampaignID: "meta-campaign",
			AdAccountID: "ad-1", AdAccountName: "Conta principal", Currency: "BRL",
			Name: "Campanha", Status: "PAUSED",
		})
	}

	resources := assistantResourcesFromContext(context)
	if len(resources) != assistantResourceLimit {
		t.Fatalf("resources=%d want=%d", len(resources), assistantResourceLimit)
	}
	wantKinds := []string{"instagram_post", "meta_campaign", "meta_ad_account"}
	for index, kind := range wantKinds {
		if resources[index].Kind != kind || !strings.HasPrefix(resources[index].ID, kind+":") {
			t.Fatalf("resource[%d] sem kind/id autoritativo: %#v", index, resources[index])
		}
	}
	if resources[0].ImageURL != "https://cdn.example/post.jpg" || resources[0].Metadata["username"] != "marca" {
		t.Fatalf("post resource incompleto: %#v", resources[0])
	}
	if resources[1].Metadata["currency"] != "BRL" {
		t.Fatalf("campaign resource sem moeda da conta: %#v", resources[1])
	}
	var secondPost *AssistantResource
	for index := range resources {
		if resources[index].ID == "instagram_post:post-2" {
			secondPost = &resources[index]
			break
		}
	}
	if secondPost == nil || secondPost.Metadata["username"] != "outra" ||
		secondPost.Metadata["igUserId"] != "102" || secondPost.Subtitle != "@outra - VIDEO" {
		t.Fatalf("post da segunda identidade perdeu atribuicao: %#v", secondPost)
	}
}

func TestRoundRobinAssistantInstagramPostsIsGloballyBounded(t *testing.T) {
	t.Parallel()
	groups := [][]AssistantInstagramPostView{
		{{ID: "a1"}, {ID: "a2"}, {ID: "a3"}},
		{{ID: "b1"}, {ID: "b2"}},
	}
	got := roundRobinAssistantInstagramPosts(groups, 4)
	want := []string{"a1", "b1", "a2", "b2"}
	if len(got) != len(want) {
		t.Fatalf("posts=%d want=%d", len(got), len(want))
	}
	for index, id := range want {
		if got[index].ID != id {
			t.Fatalf("posts[%d]=%q want=%q", index, got[index].ID, id)
		}
	}
}

func assertAdAccountIDs(t *testing.T, rows []AdAccount, want ...string) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("ids length=%d want=%d rows=%#v", len(rows), len(want), rows)
	}
	for index, id := range want {
		if rows[index].ID != id {
			t.Fatalf("rows[%d].ID=%q want=%q", index, rows[index].ID, id)
		}
	}
}

func assertInstagramIdentityIDs(t *testing.T, rows []InstagramIdentityView, want ...string) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("identity ids length=%d want=%d rows=%#v", len(rows), len(want), rows)
	}
	for index, id := range want {
		if rows[index].IGUserID != id {
			t.Fatalf("rows[%d].IGUserID=%q want=%q", index, rows[index].IGUserID, id)
		}
	}
}
