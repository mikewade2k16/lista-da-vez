// Package metaads e o modulo de integracao com Meta/Facebook Ads.
//
// O modulo conecta via OAuth first-party ou token manual cifrado, sincroniza contas
// de anuncio, campanhas e insights no cache PostgreSQL e fornece contexto escopado
// ao Assistente 360. Writes confirmados usam proposals e executor first-party;
// o escopo atual e os gaps ficam em ASSISTENTE_360_STATUS_E_ROADMAP.md.
//
// Convencoes: todo metodo de Service recebe accountID como 1o arg apos ctx;
// todo SQL filtra por account_id. accountID vem SEMPRE do Principal/header,
// nunca do body (AGENT_RULES.md / ENGINEERING_PRINCIPLES.md).
package metaads

import "time"

// ============================================================================
// Status / constantes
// ============================================================================

const (
	connectionActive = "active"

	// accountLevelCampaignID e o sentinela para a linha de insight agregada da
	// conta de anuncio (meta_campaign_id = '' no banco).
	accountLevelCampaignID = ""
)

// ============================================================================
// Modelos de dominio (linhas das tabelas meta_ads.*)
// ============================================================================

// Connection e a conexao Meta de uma account. O token NAO vive nesta struct —
// e lido/decifrado sob demanda no Store apenas para chamar a Graph.
type Connection struct {
	ID             string
	AccountID      string
	OrganizationID *string
	MetaBusinessID string
	Name           string
	TokenExpiresAt *time.Time
	Status         string
	Revision       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AdAccount e uma conta de anuncio (act_...) descoberta na conexao.
type AdAccount struct {
	ID              string
	AccountID       string
	ConnectionID    string
	MetaAdAccountID string
	ClientAccountID *string
	Name            string
	Currency        string
	Status          string
	IsCurrent       bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Campaign e uma campanha cacheada (sync da Marketing API).
type Campaign struct {
	ID             string
	AccountID      string
	AdAccountID    string
	MetaCampaignID string
	Name           string
	Objective      string
	Status         string
	DailyBudget    *float64
	LifetimeBudget *float64
	IsCurrent      bool
	SyncedAt       time.Time
}

// InsightDaily e uma linha de metrica diaria cacheada. MetaCampaignID == ""
// representa o agregado da conta de anuncio no dia.
type InsightDaily struct {
	ID             string
	AccountID      string
	AdAccountID    string
	MetaCampaignID string
	Date           time.Time
	Impressions    int64
	Clicks         int64
	Spend          float64
	Reach          int64
	CTR            float64
	CPC            float64
	CPM            float64
	Conversions    float64
	SyncedAt       time.Time
}

// ============================================================================
// Views (shape JSON devolvido ao painel). Sem token, sem dado sensivel.
// ============================================================================

// ConnectionView e o status da conexao para o painel.
type ConnectionView struct {
	Connected      bool    `json:"connected"`
	Name           string  `json:"name"`
	MetaBusinessID string  `json:"metaBusinessId"`
	Status         string  `json:"status"`
	TokenExpiresAt *string `json:"tokenExpiresAt"`
	Revision       string  `json:"revision"`
}

// AdAccountView e uma conta de anuncio para o seletor do painel.
type AdAccountView struct {
	ID              string  `json:"id"`
	MetaAdAccountID string  `json:"metaAdAccountId"`
	Name            string  `json:"name"`
	Currency        string  `json:"currency"`
	Status          string  `json:"status"`
	ClientAccountID *string `json:"clientAccountId"`
}

// CampaignView e uma campanha (projecao lean) para a tabela do painel.
type CampaignView struct {
	ID             string   `json:"id"`
	MetaCampaignID string   `json:"metaCampaignId"`
	Name           string   `json:"name"`
	Objective      string   `json:"objective"`
	Status         string   `json:"status"`
	DailyBudget    *float64 `json:"dailyBudget"`
	LifetimeBudget *float64 `json:"lifetimeBudget"`
}

// InsightPoint e um ponto de serie temporal para os graficos.
type InsightPoint struct {
	Date        string  `json:"date"`
	Impressions int64   `json:"impressions"`
	Clicks      int64   `json:"clicks"`
	Spend       float64 `json:"spend"`
	Reach       int64   `json:"reach"`
	CTR         float64 `json:"ctr"`
	CPC         float64 `json:"cpc"`
	CPM         float64 `json:"cpm"`
	Conversions float64 `json:"conversions"`
}

// OverviewKPIs sao os numeros agregados do topo do painel.
type OverviewKPIs struct {
	Spend       float64 `json:"spend"`
	Impressions int64   `json:"impressions"`
	Clicks      int64   `json:"clicks"`
	CTR         float64 `json:"ctr"`
	CPC         float64 `json:"cpc"`
	Conversions float64 `json:"conversions"`
}

// OverviewView e a resposta de GET /v1/meta-ads/overview.
type OverviewView struct {
	Connection  ConnectionView `json:"connection"`
	KPIs        OverviewKPIs   `json:"kpis"`
	AdAccountID string         `json:"adAccountId"`
}

// SyncResult e a resposta de POST /v1/meta-ads/sync.
type SyncResult struct {
	Campaigns int    `json:"campaigns"`
	Insights  int    `json:"insights"`
	SyncedAt  string `json:"syncedAt"`
}

// ============================================================================
// Mapeadores dominio -> view (definem o contrato JSON, sem comportamento)
// ============================================================================

func toConnectionView(c Connection) ConnectionView {
	v := ConnectionView{
		Connected:      c.Status == connectionActive,
		Name:           c.Name,
		MetaBusinessID: c.MetaBusinessID,
		Status:         c.Status,
		Revision:       c.Revision,
	}
	if c.TokenExpiresAt != nil {
		s := c.TokenExpiresAt.UTC().Format(time.RFC3339)
		v.TokenExpiresAt = &s
	}
	return v
}

func toAdAccountView(a AdAccount) AdAccountView {
	return AdAccountView{
		ID:              a.ID,
		MetaAdAccountID: a.MetaAdAccountID,
		Name:            a.Name,
		Currency:        a.Currency,
		Status:          a.Status,
		ClientAccountID: a.ClientAccountID,
	}
}

func toCampaignView(c Campaign) CampaignView {
	return CampaignView{
		ID:             c.ID,
		MetaCampaignID: c.MetaCampaignID,
		Name:           c.Name,
		Objective:      c.Objective,
		Status:         c.Status,
		DailyBudget:    c.DailyBudget,
		LifetimeBudget: c.LifetimeBudget,
	}
}

func toInsightPoint(i InsightDaily) InsightPoint {
	return InsightPoint{
		Date:        i.Date.UTC().Format("2006-01-02"),
		Impressions: i.Impressions,
		Clicks:      i.Clicks,
		Spend:       i.Spend,
		Reach:       i.Reach,
		CTR:         i.CTR,
		CPC:         i.CPC,
		CPM:         i.CPM,
		Conversions: i.Conversions,
	}
}
