package metaads

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	assistantInstagramPostLimit      = 12
	assistantInstagramIdentityLimit  = 8
	assistantAdAccountLimit          = 12
	assistantCampaignLimit           = 100
	assistantResourceLimit           = 20
	assistantPerformanceAccountLimit = 12
	assistantPerformanceWindowDays   = 90
	assistantPerformancePointLimit   = 360
	assistantPerformanceStaleAfter   = 24 * time.Hour
)

// AssistantContextRequest recebe somente o escopo ja validado pelo motor do
// chat. O provider ainda repete o filtro sobre ad_accounts antes de devolver
// qualquer dado do cache ou consultar o Instagram.
type AssistantContextRequest struct {
	AccountID        string
	ClientAccountID  string
	VisibleClientIDs []string
	IsAgency         bool
}

// AssistantContext e a projecao read-only consumida pelo assistente 360. Nao
// contem token, credential id, chave de cifra nem payload bruto da Graph.
type AssistantContext struct {
	Status               string                     `json:"status"`
	Connection           AssistantConnectionView    `json:"connection"`
	AdAccounts           []AssistantAdAccountView   `json:"adAccounts"`
	AdAccountsTruncated  bool                       `json:"adAccountsTruncated"`
	Campaigns            []AssistantCampaignView    `json:"campaigns"`
	CampaignsTruncated   bool                       `json:"campaignsTruncated"`
	Performance          []AssistantPerformanceView `json:"performance"`
	PerformanceTruncated bool                       `json:"performanceTruncated"`
	Instagram            AssistantInstagramContext  `json:"instagram"`
}

type AssistantConnectionView struct {
	Connected bool   `json:"connected"`
	Name      string `json:"name"`
	Status    string `json:"status"`
}

type AssistantAdAccountView struct {
	ID              string  `json:"id"`
	MetaAdAccountID string  `json:"metaAdAccountId"`
	Name            string  `json:"name"`
	Currency        string  `json:"currency"`
	Status          string  `json:"status"`
	ClientAccountID *string `json:"clientAccountId"`
}

type AssistantCampaignView struct {
	ID             string   `json:"id"`
	MetaCampaignID string   `json:"metaCampaignId"`
	AdAccountID    string   `json:"adAccountId"`
	AdAccountName  string   `json:"adAccountName"`
	Currency       string   `json:"currency"`
	Name           string   `json:"name"`
	Objective      string   `json:"objective"`
	Status         string   `json:"status"`
	DailyBudget    *float64 `json:"dailyBudget"`
	LifetimeBudget *float64 `json:"lifetimeBudget"`
	SyncedAt       string   `json:"syncedAt"`
}

// AssistantPerformanceView e a projecao compacta de insights account-level de
// uma ad account ja autorizada. ReachDailySum e explicitamente a soma do alcance
// diario: o cache nao permite deduplicar pessoas entre dias, portanto nao deve ser
// apresentado como alcance unico do periodo.
type AssistantPerformanceView struct {
	AdAccountID    string                      `json:"adAccountId"`
	AdAccountName  string                      `json:"adAccountName"`
	Currency       string                      `json:"currency"`
	Status         string                      `json:"status"`
	SyncedAt       string                      `json:"syncedAt"`
	Last30Days     AssistantPerformanceMetrics `json:"last30Days"`
	Last7Days      AssistantPerformanceMetrics `json:"last7Days"`
	Previous7Days  AssistantPerformanceMetrics `json:"previous7Days"`
	Daily          []AssistantPerformancePoint `json:"daily"`
	DailyTruncated bool                        `json:"dailyTruncated"`
}

type AssistantPerformanceMetrics struct {
	Spend         float64 `json:"spend"`
	Impressions   int64   `json:"impressions"`
	Clicks        int64   `json:"clicks"`
	ReachDailySum int64   `json:"reachDailySum"`
	CTR           float64 `json:"ctr"`
	CPC           float64 `json:"cpc"`
	Conversions   float64 `json:"conversions"`
}

type AssistantPerformancePoint struct {
	Date        string  `json:"date"`
	Spend       float64 `json:"spend"`
	Impressions int64   `json:"impressions"`
	Clicks      int64   `json:"clicks"`
	Reach       int64   `json:"reach"`
	CTR         float64 `json:"ctr"`
	CPC         float64 `json:"cpc"`
	Conversions float64 `json:"conversions"`
}

type AssistantInstagramContext struct {
	Status   string                       `json:"status"`
	Accounts []InstagramIdentityView      `json:"accounts"`
	Posts    []AssistantInstagramPostView `json:"posts"`
}

// AssistantInstagramPostView atribui cada midia a sua identidade de origem.
// Sem isso, um feed multi-Page poderia exibir o username da primeira conta em
// posts de outra marca.
type AssistantInstagramPostView struct {
	ID              string  `json:"id"`
	Caption         string  `json:"caption"`
	MediaType       string  `json:"mediaType"`
	MediaURL        string  `json:"mediaUrl"`
	ThumbnailURL    string  `json:"thumbnailUrl"`
	Permalink       string  `json:"permalink"`
	Timestamp       string  `json:"timestamp"`
	IGUserID        string  `json:"igUserId"`
	Username        string  `json:"username"`
	PageID          string  `json:"pageId"`
	PageName        string  `json:"pageName"`
	ClientAccountID *string `json:"clientAccountId"`
}

// AssistantResource e o DTO owner-owned que a composition root converte para
// o contrato neutro do chat. O ID prefixado e a unica selecao aceita do LLM.
type AssistantResource struct {
	ID        string
	Kind      string
	Title     string
	Subtitle  string
	Status    string
	ImageURL  string
	Permalink string
	Metadata  map[string]string
}

// AssistantContextBundle separa o contexto consultavel do registry seguro de
// cards. Nenhum campo deste DTO contem token ou payload Graph bruto.
type AssistantContextBundle struct {
	Context   AssistantContext
	Resources []AssistantResource
}

func emptyAssistantContext(status string) AssistantContext {
	return AssistantContext{
		Status: status,
		Connection: AssistantConnectionView{
			Connected: false,
			Status:    status,
		},
		AdAccounts:  []AssistantAdAccountView{},
		Campaigns:   []AssistantCampaignView{},
		Performance: []AssistantPerformanceView{},
		Instagram: AssistantInstagramContext{
			Status:   status,
			Accounts: []InstagramIdentityView{},
			Posts:    []AssistantInstagramPostView{},
		},
	}
}

// AssistantContextForScope devolve somente leituras: conexao, cache local de
// contas/campanhas e no maximo 12 posts reais. Ausencia de conexao e estado de
// produto (not_connected), nao falha do chat.
func (s *Service) AssistantContextForScope(ctx context.Context, req AssistantContextRequest) (AssistantContext, error) {
	accountID := strings.TrimSpace(req.AccountID)
	conn, err := s.connectionForViewer(ctx, accountID)
	if noRows(err) || (err == nil && conn.Status != connectionActive) {
		return emptyAssistantContext("not_connected"), nil
	}
	if err != nil {
		return AssistantContext{}, err
	}

	sourceAccountID := conn.AccountID
	req.AccountID = accountID
	req.ClientAccountID = strings.TrimSpace(req.ClientAccountID)
	req.VisibleClientIDs = normalizedAssistantClientIDs(req.VisibleClientIDs)
	rows, adAccountsTruncated, err := s.store.ListAssistantAdAccounts(
		ctx,
		sourceAccountID,
		req.AccountID,
		req.ClientAccountID,
		req.VisibleClientIDs,
		req.IsAgency,
		assistantAdAccountLimit,
	)
	if err != nil {
		return AssistantContext{}, err
	}
	// Defesa em profundidade: o repository filtra e limita no SQL; o service
	// repete ownership e teto antes de emitir qualquer consulta derivada.
	rows = filterAssistantAdAccounts(rows, req, sourceAccountID)
	if len(rows) > assistantAdAccountLimit {
		rows = rows[:assistantAdAccountLimit]
		adAccountsTruncated = true
	}

	result := AssistantContext{
		Status: "connected",
		Connection: AssistantConnectionView{
			Connected: true,
			Name:      conn.Name,
			Status:    conn.Status,
		},
		AdAccounts:           make([]AssistantAdAccountView, 0, len(rows)),
		AdAccountsTruncated:  adAccountsTruncated,
		Campaigns:            []AssistantCampaignView{},
		CampaignsTruncated:   adAccountsTruncated,
		Performance:          []AssistantPerformanceView{},
		PerformanceTruncated: adAccountsTruncated,
		Instagram: AssistantInstagramContext{
			Status:   "scope_unavailable",
			Accounts: []InstagramIdentityView{},
			Posts:    []AssistantInstagramPostView{},
		},
	}
	performanceNow := time.Now().UTC()
	if err := populateAssistantCachedContext(
		ctx,
		sourceAccountID,
		rows,
		&result,
		s.store.ListAssistantCampaigns,
		s.store.ListInsights,
		performanceNow,
	); err != nil {
		return AssistantContext{}, err
	}

	mappings, err := s.store.ListInstagramIdentityMappings(ctx, sourceAccountID)
	if err != nil {
		return AssistantContext{}, err
	}
	accounts, graphErr := s.InstagramAccounts(ctx, sourceAccountID)
	if graphErr != nil {
		result.Instagram.Status = assistantInstagramStatus(graphErr)
		return result, nil
	}
	identities := filterAssistantInstagramIdentities(accounts, mappings, req, sourceAccountID)
	if len(identities) > assistantInstagramIdentityLimit {
		identities = identities[:assistantInstagramIdentityLimit]
	}
	result.Instagram.Accounts = identities
	if len(identities) == 0 {
		if strings.TrimSpace(req.ClientAccountID) == "" || sourceAccountID == strings.TrimSpace(req.ClientAccountID) {
			result.Instagram.Status = "no_account"
		}
		return result, nil
	}
	postGroups := make([][]AssistantInstagramPostView, 0, len(identities))
	for _, identity := range identities {
		media, mediaErr := s.InstagramMedia(
			ctx, sourceAccountID, identity.IGUserID, assistantInstagramPostLimit,
		)
		if mediaErr != nil {
			result.Instagram.Status = assistantInstagramStatus(mediaErr)
			result.Instagram.Posts = []AssistantInstagramPostView{}
			return result, nil
		}
		posts := make([]AssistantInstagramPostView, 0, len(media))
		for _, post := range media {
			posts = append(posts, toAssistantInstagramPost(identity, post))
		}
		postGroups = append(postGroups, posts)
	}
	result.Instagram.Status = "available"
	result.Instagram.Posts = roundRobinAssistantInstagramPosts(postGroups, assistantInstagramPostLimit)
	return result, nil
}

type assistantCampaignQuery func(
	ctx context.Context,
	accountID string,
	adAccountID string,
	limit int,
) ([]Campaign, bool, error)

type assistantInsightQuery func(
	ctx context.Context,
	accountID string,
	adAccountID string,
	level string,
	since time.Time,
	until time.Time,
) ([]InsightDaily, error)

// populateAssistantCachedContext recebe somente as ad accounts ja filtradas e
// limitadas. O saldo de campanhas e passado ao repository em cada consulta; ao
// esgotar o teto global, nenhuma conta seguinte gera ListCampaigns. Insights
// executa no maximo uma consulta para cada uma das 12 contas autorizadas.
func populateAssistantCachedContext(
	ctx context.Context,
	sourceAccountID string,
	adAccounts []AdAccount,
	result *AssistantContext,
	listCampaigns assistantCampaignQuery,
	listInsights assistantInsightQuery,
	now time.Time,
) error {
	performanceAccountCount := min(len(adAccounts), assistantPerformanceAccountLimit)
	performanceSeriesLimit := assistantPerformanceSeriesLimit(performanceAccountCount)
	performanceSince, performanceUntil := assistantPerformanceRange(now)
	for index, adAccount := range adAccounts {
		result.AdAccounts = append(result.AdAccounts, AssistantAdAccountView{
			ID: adAccount.ID, MetaAdAccountID: adAccount.MetaAdAccountID,
			Name: adAccount.Name, Currency: adAccount.Currency, Status: adAccount.Status,
			ClientAccountID: adAccount.ClientAccountID,
		})

		campaignCapacity := assistantCampaignLimit - len(result.Campaigns)
		if campaignCapacity > 0 {
			campaigns, truncated, err := listCampaigns(
				ctx, sourceAccountID, adAccount.ID, campaignCapacity,
			)
			if err != nil {
				return err
			}
			if len(campaigns) > campaignCapacity {
				campaigns = campaigns[:campaignCapacity]
				truncated = true
			}
			for _, campaign := range campaigns {
				result.Campaigns = append(result.Campaigns, AssistantCampaignView{
					ID: campaign.ID, MetaCampaignID: campaign.MetaCampaignID,
					AdAccountID: adAccount.ID, AdAccountName: adAccount.Name, Currency: adAccount.Currency,
					Name: campaign.Name, Objective: campaign.Objective, Status: campaign.Status,
					DailyBudget: campaign.DailyBudget, LifetimeBudget: campaign.LifetimeBudget,
					SyncedAt: campaign.SyncedAt.UTC().Format(time.RFC3339),
				})
			}
			result.CampaignsTruncated = result.CampaignsTruncated || truncated
		} else {
			result.CampaignsTruncated = true
		}
		if len(result.Campaigns) >= assistantCampaignLimit && index < len(adAccounts)-1 {
			// As contas restantes nao sao consultadas. Mesmo que eventualmente
			// estejam vazias, a lista completa nao foi comprovada.
			result.CampaignsTruncated = true
		}

		if index >= performanceAccountCount {
			result.PerformanceTruncated = true
			continue
		}
		insights, err := listInsights(
			ctx, sourceAccountID, adAccount.ID, "account", performanceSince, performanceUntil,
		)
		if err != nil {
			return err
		}
		view := assistantPerformanceForAdAccount(
			adAccount, insights, now, performanceSeriesLimit,
		)
		result.PerformanceTruncated = result.PerformanceTruncated || view.DailyTruncated
		result.Performance = append(result.Performance, view)
	}
	return nil
}

func normalizedAssistantClientIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func assistantPerformanceRange(now time.Time) (time.Time, time.Time) {
	until := now.UTC().Truncate(24 * time.Hour)
	return until.AddDate(0, 0, -(assistantPerformanceWindowDays - 1)), until
}

func assistantPerformanceSeriesLimit(accountCount int) int {
	if accountCount <= 0 {
		return 0
	}
	limit := assistantPerformancePointLimit / accountCount
	if limit < 1 {
		return 1
	}
	return min(limit, assistantPerformanceWindowDays)
}

func assistantPerformanceForAdAccount(
	adAccount AdAccount,
	insights []InsightDaily,
	now time.Time,
	seriesLimit int,
) AssistantPerformanceView {
	view := AssistantPerformanceView{
		AdAccountID: adAccount.ID, AdAccountName: adAccount.Name,
		Currency: strings.ToUpper(strings.TrimSpace(adAccount.Currency)),
		Status:   "empty", Daily: []AssistantPerformancePoint{},
	}
	since, until := assistantPerformanceRange(now)
	valid := make([]InsightDaily, 0, min(len(insights), assistantPerformanceWindowDays))
	var latestSync time.Time
	for _, insight := range insights {
		date := insight.Date.UTC().Truncate(24 * time.Hour)
		if insight.AccountID != adAccount.AccountID || insight.AdAccountID != adAccount.ID ||
			insight.MetaCampaignID != accountLevelCampaignID || date.Before(since) || date.After(until) {
			continue
		}
		insight.Date = date
		valid = append(valid, insight)
		if insight.SyncedAt.After(latestSync) {
			latestSync = insight.SyncedAt.UTC()
		}
	}
	sort.SliceStable(valid, func(left, right int) bool {
		return valid[left].Date.Before(valid[right].Date)
	})
	if len(valid) == 0 {
		return view
	}
	view.Status = "fresh"
	if latestSync.IsZero() || now.UTC().Sub(latestSync) > assistantPerformanceStaleAfter {
		view.Status = "stale"
	}
	if !latestSync.IsZero() {
		view.SyncedAt = latestSync.Format(time.RFC3339)
	}
	last30Start := until.AddDate(0, 0, -29)
	last7Start := until.AddDate(0, 0, -6)
	previous7Start := until.AddDate(0, 0, -13)
	previous7End := until.AddDate(0, 0, -7)
	view.Last30Days = assistantPerformanceMetricsForRange(valid, last30Start, until)
	view.Last7Days = assistantPerformanceMetricsForRange(valid, last7Start, until)
	view.Previous7Days = assistantPerformanceMetricsForRange(valid, previous7Start, previous7End)
	if seriesLimit <= 0 {
		view.DailyTruncated = len(valid) > 0
		return view
	}
	if len(valid) > seriesLimit {
		view.DailyTruncated = true
		valid = valid[len(valid)-seriesLimit:]
	}
	view.Daily = make([]AssistantPerformancePoint, 0, len(valid))
	for _, insight := range valid {
		metrics := assistantPerformanceMetrics([]InsightDaily{insight})
		view.Daily = append(view.Daily, AssistantPerformancePoint{
			Date: insight.Date.Format("2006-01-02"), Spend: metrics.Spend,
			Impressions: metrics.Impressions, Clicks: metrics.Clicks,
			Reach: metrics.ReachDailySum, CTR: metrics.CTR, CPC: metrics.CPC,
			Conversions: metrics.Conversions,
		})
	}
	return view
}

func assistantPerformanceMetricsForRange(
	insights []InsightDaily, since, until time.Time,
) AssistantPerformanceMetrics {
	selected := make([]InsightDaily, 0, len(insights))
	for _, insight := range insights {
		if !insight.Date.Before(since) && !insight.Date.After(until) {
			selected = append(selected, insight)
		}
	}
	return assistantPerformanceMetrics(selected)
}

func assistantPerformanceMetrics(insights []InsightDaily) AssistantPerformanceMetrics {
	var result AssistantPerformanceMetrics
	for _, insight := range insights {
		result.Spend += insight.Spend
		result.Impressions += insight.Impressions
		result.Clicks += insight.Clicks
		result.ReachDailySum += insight.Reach
		result.Conversions += insight.Conversions
	}
	if result.Impressions > 0 {
		result.CTR = float64(result.Clicks) / float64(result.Impressions) * 100
	}
	if result.Clicks > 0 {
		result.CPC = result.Spend / float64(result.Clicks)
	}
	return result
}

// AssistantContextBundleForScope monta o registry a partir do contexto ja
// filtrado por account/client. O round-robin preserva representacao das tres
// categorias e o teto impede inflar prompt, resposta e persistencia.
func (s *Service) AssistantContextBundleForScope(ctx context.Context, req AssistantContextRequest) (AssistantContextBundle, error) {
	result, err := s.AssistantContextForScope(ctx, req)
	if err != nil {
		return AssistantContextBundle{}, err
	}
	return AssistantContextBundle{
		Context:   result,
		Resources: assistantResourcesFromContext(result),
	}, nil
}

func assistantResourcesFromContext(context AssistantContext) []AssistantResource {
	posts := make([]AssistantResource, 0, len(context.Instagram.Posts))
	for _, post := range context.Instagram.Posts {
		imageURL := strings.TrimSpace(post.ThumbnailURL)
		if imageURL == "" {
			imageURL = strings.TrimSpace(post.MediaURL)
		}
		subtitle := strings.TrimSpace(post.MediaType)
		username := strings.TrimSpace(post.Username)
		if username != "" {
			subtitle = "@" + username + " - " + subtitle
		}
		metadata := map[string]string{
			"mediaType": post.MediaType,
			"timestamp": post.Timestamp,
			"igUserId":  post.IGUserID,
			"pageId":    post.PageID,
			"pageName":  post.PageName,
		}
		if username != "" {
			metadata["username"] = username
		}
		if post.ClientAccountID != nil {
			metadata["clientAccountId"] = *post.ClientAccountID
		}
		posts = append(posts, AssistantResource{
			ID:        "instagram_post:" + strings.TrimSpace(post.ID),
			Kind:      "instagram_post",
			Title:     assistantPostTitle(post.Caption),
			Subtitle:  subtitle,
			Status:    "published",
			ImageURL:  imageURL,
			Permalink: post.Permalink,
			Metadata:  metadata,
		})
	}

	campaigns := make([]AssistantResource, 0, len(context.Campaigns))
	for _, campaign := range context.Campaigns {
		metadata := map[string]string{
			"adAccountId":    campaign.AdAccountID,
			"adAccountName":  campaign.AdAccountName,
			"currency":       campaign.Currency,
			"metaCampaignId": campaign.MetaCampaignID,
			"objective":      campaign.Objective,
			"syncedAt":       campaign.SyncedAt,
		}
		if campaign.DailyBudget != nil {
			metadata["dailyBudget"] = strconv.FormatFloat(*campaign.DailyBudget, 'f', 2, 64)
		}
		if campaign.LifetimeBudget != nil {
			metadata["lifetimeBudget"] = strconv.FormatFloat(*campaign.LifetimeBudget, 'f', 2, 64)
		}
		campaigns = append(campaigns, AssistantResource{
			ID:       "meta_campaign:" + strings.TrimSpace(campaign.ID),
			Kind:     "meta_campaign",
			Title:    campaign.Name,
			Subtitle: campaign.AdAccountName,
			Status:   campaign.Status,
			Metadata: metadata,
		})
	}

	adAccounts := make([]AssistantResource, 0, len(context.AdAccounts))
	for _, adAccount := range context.AdAccounts {
		metadata := map[string]string{
			"currency":        adAccount.Currency,
			"metaAdAccountId": adAccount.MetaAdAccountID,
		}
		if adAccount.ClientAccountID != nil {
			metadata["clientAccountId"] = *adAccount.ClientAccountID
		}
		adAccounts = append(adAccounts, AssistantResource{
			ID:       "meta_ad_account:" + strings.TrimSpace(adAccount.ID),
			Kind:     "meta_ad_account",
			Title:    adAccount.Name,
			Subtitle: adAccount.MetaAdAccountID,
			Status:   adAccount.Status,
			Metadata: metadata,
		})
	}

	groups := [][]AssistantResource{posts, campaigns, adAccounts}
	out := make([]AssistantResource, 0, assistantResourceLimit)
	for index := 0; len(out) < assistantResourceLimit; index++ {
		appended := false
		for _, group := range groups {
			if index >= len(group) {
				continue
			}
			out = append(out, group[index])
			appended = true
			if len(out) >= assistantResourceLimit {
				break
			}
		}
		if !appended {
			break
		}
	}
	return out
}

func assistantPostTitle(caption string) string {
	title := strings.Join(strings.Fields(strings.TrimSpace(caption)), " ")
	if title == "" {
		return "Post do Instagram"
	}
	return title
}

func filterAssistantAdAccounts(rows []AdAccount, req AssistantContextRequest, sourceAccountID string) []AdAccount {
	clientID := strings.TrimSpace(req.ClientAccountID)
	visible := make(map[string]struct{}, len(req.VisibleClientIDs))
	for _, id := range req.VisibleClientIDs {
		if id = strings.TrimSpace(id); id != "" {
			visible[id] = struct{}{}
		}
	}
	out := make([]AdAccount, 0, len(rows))
	for _, row := range rows {
		mappedClientID := ""
		if row.ClientAccountID != nil {
			mappedClientID = strings.TrimSpace(*row.ClientAccountID)
		}
		allowed := false
		switch {
		case clientID != "":
			allowed = mappedClientID == clientID || (mappedClientID == "" && sourceAccountID == clientID)
		case mappedClientID == "":
			allowed = req.IsAgency || sourceAccountID == req.AccountID
		default:
			_, allowed = visible[mappedClientID]
		}
		if allowed {
			out = append(out, row)
		}
	}
	return out
}

func assistantInstagramStatus(err error) string {
	if noRows(err) || err == ErrNotConnected {
		return "not_connected"
	}
	return "unavailable"
}

func filterAssistantInstagramIdentities(
	accounts []InstagramAccountView,
	mappings []InstagramIdentityClientMapping,
	req AssistantContextRequest,
	sourceAccountID string,
) []InstagramIdentityView {
	clientID := strings.TrimSpace(req.ClientAccountID)
	requestAccountID := strings.TrimSpace(req.AccountID)
	visibleClients := make(map[string]struct{}, len(req.VisibleClientIDs))
	for _, id := range req.VisibleClientIDs {
		if id = strings.TrimSpace(id); id != "" {
			visibleClients[id] = struct{}{}
		}
	}
	mappingByIdentity := instagramMappingByIdentity(mappings)
	out := make([]InstagramIdentityView, 0, len(accounts))
	for _, account := range accounts {
		mapping, mapped := mappingByIdentity[instagramIdentityKey(account.IGUserID, account.PageID)]
		var attributedClientID *string
		allowed := false
		switch {
		case clientID != "" && sourceAccountID == clientID:
			allowed = true
			clientIDCopy := clientID
			attributedClientID = &clientIDCopy
		case clientID != "":
			selectedVisible := clientID == requestAccountID
			if !selectedVisible {
				_, selectedVisible = visibleClients[clientID]
			}
			allowed = selectedVisible && mapped && strings.TrimSpace(mapping.ClientAccountID) == clientID
			if allowed {
				clientIDCopy := mapping.ClientAccountID
				attributedClientID = &clientIDCopy
			}
		case req.IsAgency && sourceAccountID == requestAccountID && !mapped:
			allowed = true
		case req.IsAgency && sourceAccountID == requestAccountID:
			_, allowed = visibleClients[strings.TrimSpace(mapping.ClientAccountID)]
			if allowed {
				clientIDCopy := mapping.ClientAccountID
				attributedClientID = &clientIDCopy
			}
		}
		if allowed {
			out = append(out, toInstagramIdentityView(account, attributedClientID))
		}
	}
	return out
}

func toAssistantInstagramPost(
	identity InstagramIdentityView, post InstagramMediaView,
) AssistantInstagramPostView {
	return AssistantInstagramPostView{
		ID: post.ID, Caption: post.Caption, MediaType: post.MediaType,
		MediaURL: post.MediaURL, ThumbnailURL: post.ThumbnailURL,
		Permalink: post.Permalink, Timestamp: post.Timestamp,
		IGUserID: identity.IGUserID, Username: identity.Username,
		PageID: identity.PageID, PageName: identity.PageName,
		ClientAccountID: identity.ClientAccountID,
	}
}

func roundRobinAssistantInstagramPosts(
	groups [][]AssistantInstagramPostView, limit int,
) []AssistantInstagramPostView {
	if limit <= 0 {
		return []AssistantInstagramPostView{}
	}
	out := make([]AssistantInstagramPostView, 0, limit)
	for index := 0; len(out) < limit; index++ {
		appended := false
		for _, group := range groups {
			if index >= len(group) {
				continue
			}
			out = append(out, group[index])
			appended = true
			if len(out) >= limit {
				break
			}
		}
		if !appended {
			break
		}
	}
	return out
}
