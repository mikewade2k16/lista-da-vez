package metaads

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ErrNotConnected indica que a account ainda nao conectou uma conta Meta.
// Mapeado para 404 not_connected nos handlers que exigem conexao ativa.
var ErrNotConnected = errors.New("meta_ads: conta Meta nao conectada")

// Service orquestra a persistencia (meta_ads.*), o cliente da Graph API e o
// cliente do agent-runner (assistente MCP, service_assistant.go).
type Service struct {
	store  *Store
	client *MetaClient
	runner *RunnerClient

	// bridgeToken e o bearer de servico do BRIDGE INTERNO (/internal/meta-ads/*)
	// consumido pelo runner Node no HOST. Vazio = bridge nao configurado (503).
	// Injetado via SetBridgeToken no Build (env META_ADS_RUNNER_BRIDGE_TOKEN).
	bridgeToken string
}

// NewService cria o Service. runner pode estar "nao configurado" (baseURL/token
// vazios) — os endpoints do assistente respondem 503 nesse caso.
func NewService(store *Store, client *MetaClient, runner *RunnerClient) *Service {
	return &Service{store: store, client: client, runner: runner}
}

// SetBridgeToken injeta o bearer de servico do bridge interno do runner. Lido do
// env no Build (META_ADS_RUNNER_BRIDGE_TOKEN); vazio = bridge desligado (503).
func (s *Service) SetBridgeToken(token string) { s.bridgeToken = strings.TrimSpace(token) }

// Overview retorna o status da conexao + KPIs agregados da conta de anuncio. Sem
// conexao NAO e erro: devolve OverviewView com Connection.Connected=false para o
// front exibir o card de conectar. adAccountID opcional (filtra os KPIs).
func (s *Service) Overview(ctx context.Context, accountID, adAccountID string) (OverviewView, error) {
	conn, err := s.store.GetConnection(ctx, accountID)
	if noRows(err) {
		return OverviewView{Connection: ConnectionView{Connected: false}}, nil
	}
	if err != nil {
		return OverviewView{}, err
	}

	view := OverviewView{Connection: toConnectionView(conn), AdAccountID: adAccountID}
	if adAccountID == "" {
		return view, nil
	}

	// KPIs zerados quando nao ha insights — sem erro.
	since, until := rangeWindow("last_30d")
	insights, err := s.store.ListInsights(ctx, accountID, adAccountID, "account", since, until)
	if err != nil {
		return OverviewView{}, err
	}
	view.KPIs = aggregateKPIs(insights)
	return view, nil
}

// SaveConnection valida o System User token na Graph, cifra e persiste. Tambem
// descobre e cacheia as contas de anuncio acessiveis. Falha rapido sem chave de
// cifra. NUNCA loga o token.
func (s *Service) SaveConnection(ctx context.Context, accountID, token string) (ConnectionView, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return ConnectionView{}, errors.New("token vazio")
	}
	if !s.store.HasCryptoKey() {
		return ConnectionView{}, ErrCryptoKeyMissing
	}

	// Valida o token chamando a Graph (lista contas acessiveis).
	adAccounts, err := s.client.GetAdAccounts(ctx, token)
	if err != nil {
		return ConnectionView{}, err
	}

	conn, err := s.store.UpsertConnection(ctx, accountID, "", "Meta Ads", token)
	if err != nil {
		return ConnectionView{}, err
	}

	// Cacheia as contas descobertas (best-effort dentro da transacao logica).
	for _, a := range adAccounts {
		if _, upErr := s.store.UpsertAdAccount(ctx, accountID, conn.ID,
			a.AccountID, a.Name, a.Currency, accountStatusLabel(a.AccountStatus)); upErr != nil {
			return ConnectionView{}, upErr
		}
	}
	return toConnectionView(conn), nil
}

// DeleteConnection remove a conexao da account (cascade no cache).
func (s *Service) DeleteConnection(ctx context.Context, accountID string) error {
	return s.store.DeleteConnection(ctx, accountID)
}

// ListAdAccounts lista as contas de anuncio do cache. Se o cache estiver vazio
// mas houver conexao, busca ao vivo na Graph e popula o cache.
func (s *Service) ListAdAccounts(ctx context.Context, accountID string) ([]AdAccountView, error) {
	conn, err := s.store.GetConnection(ctx, accountID)
	if noRows(err) {
		return nil, ErrNotConnected
	}
	if err != nil {
		return nil, err
	}

	cached, err := s.store.ListAdAccounts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if len(cached) == 0 {
		if cached, err = s.refreshAdAccounts(ctx, accountID, conn.ID); err != nil {
			return nil, err
		}
	}

	views := make([]AdAccountView, len(cached))
	for i, a := range cached {
		views[i] = toAdAccountView(a)
	}
	return views, nil
}

// ListCampaigns retorna as campanhas cacheadas de uma conta de anuncio.
func (s *Service) ListCampaigns(ctx context.Context, accountID, adAccountID string) ([]CampaignView, error) {
	if _, err := s.requireAdAccount(ctx, accountID, adAccountID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListCampaigns(ctx, accountID, adAccountID)
	if err != nil {
		return nil, err
	}
	views := make([]CampaignView, len(rows))
	for i, c := range rows {
		views[i] = toCampaignView(c)
	}
	return views, nil
}

// Insights retorna a serie temporal cacheada para os graficos. rangeKey ex.:
// "last_7d"/"last_30d"; level "account" (default) ou "campaign".
func (s *Service) Insights(ctx context.Context, accountID, adAccountID, rangeKey, level string) ([]InsightPoint, error) {
	if _, err := s.requireAdAccount(ctx, accountID, adAccountID); err != nil {
		return nil, err
	}
	if level != "campaign" {
		level = "account"
	}
	since, until := rangeWindow(rangeKey)
	rows, err := s.store.ListInsights(ctx, accountID, adAccountID, level, since, until)
	if err != nil {
		return nil, err
	}
	points := make([]InsightPoint, len(rows))
	for i, r := range rows {
		points[i] = toInsightPoint(r)
	}
	return points, nil
}

// requireAdAccount garante que existe conexao e que a conta de anuncio pertence
// a esta account. Retorna ErrNotConnected (sem conexao) ou pgx.ErrNoRows (conta
// de outra account / inexistente).
func (s *Service) requireAdAccount(ctx context.Context, accountID, adAccountID string) (AdAccount, error) {
	if _, err := s.store.GetConnection(ctx, accountID); err != nil {
		if noRows(err) {
			return AdAccount{}, ErrNotConnected
		}
		return AdAccount{}, err
	}
	return s.store.GetAdAccount(ctx, accountID, adAccountID)
}

// refreshAdAccounts busca as contas de anuncio ao vivo e popula o cache.
func (s *Service) refreshAdAccounts(ctx context.Context, accountID, connectionID string) ([]AdAccount, error) {
	token, err := s.store.GetDecryptedToken(ctx, accountID)
	if err != nil {
		return nil, err
	}
	remote, err := s.client.GetAdAccounts(ctx, token)
	if err != nil {
		return nil, err
	}
	for _, a := range remote {
		if _, upErr := s.store.UpsertAdAccount(ctx, accountID, connectionID,
			a.AccountID, a.Name, a.Currency, accountStatusLabel(a.AccountStatus)); upErr != nil {
			return nil, upErr
		}
	}
	return s.store.ListAdAccounts(ctx, accountID)
}

// rangeWindow traduz uma chave de range em [since, until] (datas UTC, inclusivo).
// Default e last_30d. until = hoje.
func rangeWindow(rangeKey string) (since, until time.Time) {
	until = time.Now().UTC().Truncate(24 * time.Hour)
	days := 30
	switch rangeKey {
	case "last_7d":
		days = 7
	case "last_14d":
		days = 14
	case "last_90d":
		days = 90
	case "last_30d", "":
		days = 30
	}
	since = until.AddDate(0, 0, -(days - 1))
	return since, until
}

// aggregateKPIs soma os insights agregados num conjunto de KPIs (CTR/CPC
// derivados do total para refletir o periodo inteiro, nao a media de medias).
func aggregateKPIs(rows []InsightDaily) OverviewKPIs {
	var k OverviewKPIs
	for _, r := range rows {
		k.Spend += r.Spend
		k.Impressions += r.Impressions
		k.Clicks += r.Clicks
		k.Conversions += r.Conversions
	}
	if k.Impressions > 0 {
		k.CTR = float64(k.Clicks) / float64(k.Impressions) * 100
	}
	if k.Clicks > 0 {
		k.CPC = k.Spend / float64(k.Clicks)
	}
	return k
}

// accountStatusLabel traduz o account_status numerico da Graph num rotulo curto.
func accountStatusLabel(status int) string {
	switch status {
	case 1:
		return "active"
	case 2:
		return "disabled"
	case 3:
		return "unsettled"
	case 7:
		return "pending_risk_review"
	case 9:
		return "in_grace_period"
	case 101:
		return "closed"
	default:
		return "unknown"
	}
}
