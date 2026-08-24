package metaads

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MetaClient fala com a Graph/Marketing API da Meta (graph.facebook.com).
// Autenticacao por Authorization Bearer. O token de longa duracao nunca entra
// na URL nem deve aparecer em logs.
type MetaClient struct {
	base string
	http *http.Client
}

// NewMetaClient cria o cliente. graphBase ex.: "https://graph.facebook.com/v24.0".
func NewMetaClient(graphBase string) *MetaClient {
	return &MetaClient{
		base: strings.TrimRight(graphBase, "/"),
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

// ============================================================================
// Structs internas do JSON da Graph API
// ============================================================================

// metaError e o envelope de erro da Graph API (campo "error" no corpo).
type metaError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
	} `json:"error"`
}

type metaPaging struct {
	Next    string `json:"next"`
	Cursors struct {
		After string `json:"after"`
	} `json:"cursors"`
}

type metaPage[T any] struct {
	Data   []T        `json:"data"`
	Paging metaPaging `json:"paging"`
}

const (
	maxMetaPagingCursorLength = 4096
	maxMetaAdAccountPages     = 50
	maxMetaAdAccounts         = 10_000
	maxMetaCampaignPages      = 100
	maxMetaCampaigns          = 50_000
	maxMetaInsightPages       = 250
	maxMetaInsights           = 125_000
)

// GraphAdAccount e uma conta de anuncio em /me/adaccounts.
type GraphAdAccount struct {
	AccountID     string `json:"account_id"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	AccountStatus int    `json:"account_status"`
}

// GraphCampaign e uma campanha em /act_{id}/campaigns. Orcamentos vem como
// string em centavos da moeda da conta.
type GraphCampaign struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Objective      string `json:"objective"`
	Status         string `json:"status"`
	DailyBudget    string `json:"daily_budget"`
	LifetimeBudget string `json:"lifetime_budget"`
}

// GraphInsight e uma linha de insight em /act_{id}/insights (time_increment=1).
// Campos numericos vem como string; conversoes derivam de "actions".
type GraphInsight struct {
	CampaignID  string `json:"campaign_id"`
	Impressions string `json:"impressions"`
	Clicks      string `json:"clicks"`
	Spend       string `json:"spend"`
	Reach       string `json:"reach"`
	CTR         string `json:"ctr"`
	CPC         string `json:"cpc"`
	CPM         string `json:"cpm"`
	DateStart   string `json:"date_start"`
	Actions     []struct {
		ActionType string `json:"action_type"`
		Value      string `json:"value"`
	} `json:"actions"`
}

// ============================================================================
// Endpoints
// ============================================================================

// GetAdAccounts lista as contas de anuncio acessiveis pelo token. Tambem serve
// como validacao do token (uma chamada que falha = token invalido).
func (c *MetaClient) GetAdAccounts(ctx context.Context, token string) ([]GraphAdAccount, error) {
	q := url.Values{}
	q.Set("fields", "account_id,name,currency,account_status")
	q.Set("limit", "200")
	return graphPageData[GraphAdAccount](
		ctx, c, "/me/adaccounts", token, q, maxMetaAdAccountPages, maxMetaAdAccounts,
	)
}

// ListPermissions usa a mesma leitura sanitizada de grants do Facebook Login.
// O connect manual exige a mesma allowlist antes de descobrir ou persistir
// qualquer conta de anuncio.
func (c *MetaClient) ListPermissions(ctx context.Context, token string) ([]OAuthPermission, error) {
	return listMetaPermissions(ctx, c.base, c.http, token)
}

// ListCampaigns lista as campanhas de uma conta de anuncio. metaAdAccountID pode
// vir com ou sem o prefixo "act_".
func (c *MetaClient) ListCampaigns(ctx context.Context, token, metaAdAccountID string) ([]GraphCampaign, error) {
	q := url.Values{}
	q.Set("fields", "id,name,objective,status,daily_budget,lifetime_budget")
	q.Set("limit", "200")
	path := "/" + actPrefixed(metaAdAccountID) + "/campaigns"
	return graphPageData[GraphCampaign](
		ctx, c, path, token, q, maxMetaCampaignPages, maxMetaCampaigns,
	)
}

// GetInsights busca insights diarios (time_increment=1) de uma conta de anuncio.
// datePreset ex.: "last_30d". level: "account" ou "campaign".
func (c *MetaClient) GetInsights(ctx context.Context, token, metaAdAccountID, datePreset, level string) ([]GraphInsight, error) {
	q := url.Values{}
	q.Set("level", level)
	q.Set("date_preset", datePreset)
	q.Set("time_increment", "1")
	q.Set("fields", "impressions,clicks,spend,reach,ctr,cpc,cpm,date_start,actions")
	q.Set("limit", "500")
	path := "/" + actPrefixed(metaAdAccountID) + "/insights"
	return graphPageData[GraphInsight](
		ctx, c, path, token, q, maxMetaInsightPages, maxMetaInsights,
	)
}

// ============================================================================
// Helpers de transporte e parsing
// ============================================================================

// graphPageData percorre apenas cursores opacos devolvidos pela Graph e sempre
// recompõe a próxima chamada sobre o mesmo host/path server-side. O campo
// paging.next nunca é seguido diretamente: além de poder carregar token na URL,
// confiar nessa URL permitiria SSRF se o provider/proxy fosse comprometido.
func graphPageData[T any](
	ctx context.Context,
	client *MetaClient,
	path, token string,
	query url.Values,
	maxPages, maxItems int,
) ([]T, error) {
	if client == nil || maxPages <= 0 || maxItems <= 0 {
		return nil, fmt.Errorf("meta graph: paginacao invalida")
	}
	pageQuery := cloneURLValues(query)
	items := make([]T, 0)
	seenCursors := make(map[string]struct{})
	for pageNumber := 0; pageNumber < maxPages; pageNumber++ {
		var page metaPage[T]
		if err := client.getJSON(ctx, path, token, pageQuery, &page); err != nil {
			return nil, err
		}
		if len(page.Data) > maxItems-len(items) {
			return nil, fmt.Errorf("meta graph: limite seguro de itens excedido")
		}
		items = append(items, page.Data...)
		if strings.TrimSpace(page.Paging.Next) == "" {
			return items, nil
		}
		after := strings.TrimSpace(page.Paging.Cursors.After)
		if after == "" || len(after) > maxMetaPagingCursorLength {
			return nil, fmt.Errorf("meta graph: cursor de paginacao invalido")
		}
		if _, repeated := seenCursors[after]; repeated {
			return nil, fmt.Errorf("meta graph: cursor de paginacao repetido")
		}
		seenCursors[after] = struct{}{}
		pageQuery.Set("after", after)
	}
	return nil, fmt.Errorf("meta graph: limite seguro de paginas excedido")
}

func cloneURLValues(input url.Values) url.Values {
	out := make(url.Values, len(input))
	for key, values := range input {
		out[key] = append([]string(nil), values...)
	}
	return out
}

// getJSON executa GET {base}{path}?{q} e decodifica em dst. O token segue no
// header Authorization para nao vazar em URL, proxy ou access log.
func (c *MetaClient) getJSON(ctx context.Context, path, token string, q url.Values, dst any) error {
	endpoint := c.base + path + "?" + q.Encode()
	// G704 (gosec): falso-positivo. O host vem de c.base (META_ADS_GRAPH_BASE ou
	// o default constante), nunca de input do usuario; path/query sao fixos no
	// modulo + url-encoded. Sem SSRF.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil) //nolint:gosec // host de config confiavel, nao de input
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req) //nolint:gosec // host de config confiavel, nao de input
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return graphError(resp.StatusCode, raw)
	}
	return json.Unmarshal(raw, dst)
}

// graphError monta um erro legivel a partir do envelope "error" da Graph (ou do
// status HTTP, se o corpo nao seguir o padrao). Nunca inclui o token.
func graphError(status int, raw []byte) error {
	var me metaError
	if err := json.Unmarshal(raw, &me); err == nil && me.Error.Message != "" {
		return fmt.Errorf("meta graph: http %d: %s (code %d)", status, me.Error.Message, me.Error.Code)
	}
	return fmt.Errorf("meta graph: http %d", status)
}

// actPrefixed garante o prefixo "act_" exigido pelos edges da conta de anuncio.
func actPrefixed(id string) string {
	if strings.HasPrefix(id, "act_") {
		return id
	}
	return "act_" + id
}

// parseFloat converte uma string da Graph em float64 (vazio/invalido => 0).
func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseInt converte uma string da Graph em int64 (vazio/invalido => 0).
func parseInt(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// budgetCentsToUnits converte um orcamento da Graph (string em centavos) para a
// unidade da moeda (reais/dolares). Vazio => nil (sem orcamento desse tipo).
func budgetCentsToUnits(s string) *float64 {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	cents, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	v := cents / 100
	return &v
}

// conversionsFromActions soma as acoes que contam como conversao. MVP: usa
// "purchase" / "*_purchase" / "lead" / "complete_registration"; ausente => 0.
func conversionsFromActions(ins GraphInsight) float64 {
	var total float64
	for _, a := range ins.Actions {
		if isConversionAction(a.ActionType) {
			total += parseFloat(a.Value)
		}
	}
	return total
}

func isConversionAction(actionType string) bool {
	switch actionType {
	case "purchase",
		"omni_purchase",
		"offsite_conversion.fb_pixel_purchase",
		"lead",
		"onsite_conversion.lead_grouped",
		"complete_registration",
		"offsite_conversion.fb_pixel_complete_registration":
		return true
	default:
		return false
	}
}
