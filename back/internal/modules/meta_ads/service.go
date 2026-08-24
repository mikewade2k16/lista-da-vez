package metaads

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNotConnected indica que a account ainda nao conectou uma conta Meta.
// Mapeado para 404 not_connected nos handlers que exigem conexao ativa.
var (
	ErrNotConnected         = errors.New("meta_ads: conta Meta nao conectada")
	ErrInvalidClientAccount = errors.New("meta_ads: client account invalida")
)

var metaAdsUUIDRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// Service orquestra a persistencia (meta_ads.*), o cliente da Graph API e o
// cliente do agent-runner (assistente MCP, service_assistant.go).
type Service struct {
	store               *Store
	connectionSnapshots connectionSnapshotRepository
	client              *MetaClient
	runner              *RunnerClient

	// bridgeToken e o bearer de servico do BRIDGE INTERNO (/internal/meta-ads/*)
	// consumido pelo runner Node no HOST. Vazio = bridge nao configurado (503).
	// Injetado via SetBridgeToken no Build (env META_ADS_RUNNER_BRIDGE_TOKEN).
	bridgeToken                    string
	assistantActionSourceValidator AssistantActionSourceValidator
}

// connectionSnapshotRepository mantem o seam de teste do connect restrito ao
// snapshot atomico. O restante do modulo continua usando o Store concreto.
type connectionSnapshotRepository interface {
	HasCryptoKey() bool
	SaveConnectionSnapshot(
		ctx context.Context,
		accountID, metaBusinessID, name, token string,
		tokenExpiresAt *time.Time,
		adAccounts []AdAccount,
	) (Connection, error)
}

// NewService cria o Service. runner pode estar "nao configurado" (baseURL/token
// vazios) — os endpoints do assistente respondem 503 nesse caso.
func NewService(store *Store, client *MetaClient, runner *RunnerClient) *Service {
	return &Service{
		store:               store,
		connectionSnapshots: store,
		client:              client,
		runner:              runner,
	}
}

// SetBridgeToken injeta o bearer de servico do bridge interno do runner. Lido do
// env no Build (META_ADS_RUNNER_BRIDGE_TOKEN); vazio = bridge desligado (503).
func (s *Service) SetBridgeToken(token string) { s.bridgeToken = strings.TrimSpace(token) }

func (s *Service) SetAssistantActionSourceValidator(validator AssistantActionSourceValidator) {
	s.assistantActionSourceValidator = validator
}

// Overview retorna o status da conexao + KPIs agregados da conta de anuncio. Sem
// conexao NAO e erro: devolve OverviewView com Connection.Connected=false para o
// front exibir o card de conectar. adAccountID opcional (filtra os KPIs).
func (s *Service) Overview(ctx context.Context, accountID, adAccountID string) (OverviewView, error) {
	conn, err := s.connectionForViewer(ctx, accountID)
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
	adAccount, err := s.requireAdAccount(ctx, accountID, adAccountID)
	if err != nil {
		return OverviewView{}, err
	}

	// KPIs zerados quando nao ha insights — sem erro.
	since, until := rangeWindow("last_30d")
	insights, err := s.store.ListInsights(ctx, adAccount.AccountID, adAccount.ID, "account", since, until)
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
	if err := s.validateConnectionPrerequisites(token); err != nil {
		return ConnectionView{}, err
	}
	permissions, err := s.client.ListPermissions(ctx, token)
	if err != nil {
		return ConnectionView{}, err
	}
	if err := validateOAuthPermissions(permissions); err != nil {
		return ConnectionView{}, err
	}
	return s.saveConnection(ctx, accountID, token, nil)
}

// SaveOAuthConnection persiste token, expiracao e contas descobertas em um
// unico snapshot. Nao existe janela em que o token novo aponte para o cache da
// conexao anterior.
func (s *Service) SaveOAuthConnection(
	ctx context.Context,
	accountID, token string,
	tokenExpiresAt *time.Time,
) (ConnectionView, error) {
	return s.saveConnection(ctx, accountID, token, tokenExpiresAt)
}

func (s *Service) saveConnection(
	ctx context.Context,
	accountID, token string,
	tokenExpiresAt *time.Time,
) (ConnectionView, error) {
	token = strings.TrimSpace(token)
	if err := s.validateConnectionPrerequisites(token); err != nil {
		return ConnectionView{}, err
	}

	// Valida o token chamando a Graph (lista contas acessiveis).
	adAccounts, err := s.client.GetAdAccounts(ctx, token)
	if err != nil {
		return ConnectionView{}, err
	}

	conn, err := s.connectionSnapshots.SaveConnectionSnapshot(
		ctx,
		accountID,
		"",
		"Meta Ads",
		token,
		tokenExpiresAt,
		graphAdAccountsSnapshot(adAccounts),
	)
	if err != nil {
		return ConnectionView{}, err
	}
	return toConnectionView(conn), nil
}

func (s *Service) validateConnectionPrerequisites(token string) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("token vazio")
	}
	if s == nil || s.connectionSnapshots == nil || !s.connectionSnapshots.HasCryptoKey() {
		return ErrCryptoKeyMissing
	}
	if s.client == nil {
		return errors.New("meta graph: cliente nao configurado")
	}
	return nil
}

// DeleteConnection remove a conexao da account (cascade no cache).
func (s *Service) DeleteConnection(ctx context.Context, accountID string) error {
	return s.store.DeleteConnection(ctx, accountID)
}

// ListAdAccounts lista as contas de anuncio do cache. Se o cache estiver vazio
// mas houver conexao, busca ao vivo na Graph e popula o cache.
func (s *Service) ListAdAccounts(ctx context.Context, accountID string) ([]AdAccountView, error) {
	conn, err := s.connectionForViewer(ctx, accountID)
	if noRows(err) {
		return nil, ErrNotConnected
	}
	if err != nil {
		return nil, err
	}

	cached, err := s.store.ListAdAccounts(ctx, conn.AccountID)
	if err != nil {
		return nil, err
	}
	if len(cached) == 0 {
		if cached, err = s.refreshAdAccounts(ctx, conn); err != nil {
			return nil, err
		}
	}
	cached = filterAdAccountsForViewer(cached, accountID, conn.AccountID)

	views := make([]AdAccountView, len(cached))
	for i, a := range cached {
		views[i] = toAdAccountView(a)
	}
	return views, nil
}

// SetAdAccountClient torna explicito o recurso que pertence a cada cliente. A
// operacao so existe na conexao direta da agencia; um cliente usando a conexao
// compartilhada nunca pode remapear contas de anuncio.
func (s *Service) SetAdAccountClient(ctx context.Context, accountID, adAccountID, clientAccountID string) (AdAccountView, error) {
	accountID = strings.TrimSpace(accountID)
	adAccountID = strings.TrimSpace(adAccountID)
	clientAccountID = strings.TrimSpace(clientAccountID)
	if !metaAdsUUIDRe.MatchString(adAccountID) || (clientAccountID != "" && !metaAdsUUIDRe.MatchString(clientAccountID)) {
		return AdAccountView{}, ErrInvalidClientAccount
	}
	if _, err := s.store.GetConnection(ctx, accountID); err != nil {
		if noRows(err) {
			return AdAccountView{}, ErrNotConnected
		}
		return AdAccountView{}, err
	}
	isAgency, err := s.store.AccountIsAgency(ctx, accountID)
	if err != nil {
		return AdAccountView{}, err
	}
	if !isAgency {
		return AdAccountView{}, pgx.ErrNoRows
	}
	if clientAccountID != "" {
		allowed, err := s.store.AgencyCanAssignClient(ctx, accountID, clientAccountID)
		if err != nil {
			return AdAccountView{}, err
		}
		if !allowed {
			return AdAccountView{}, pgx.ErrNoRows
		}
	}
	row, err := s.store.SetAdAccountClient(ctx, accountID, adAccountID, clientAccountID)
	if err != nil {
		return AdAccountView{}, err
	}
	return toAdAccountView(row), nil
}

// ListCampaigns retorna as campanhas cacheadas de uma conta de anuncio.
func (s *Service) ListCampaigns(ctx context.Context, accountID, adAccountID string) ([]CampaignView, error) {
	adAccount, err := s.requireAdAccount(ctx, accountID, adAccountID)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListCampaigns(ctx, adAccount.AccountID, adAccount.ID)
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
	adAccount, err := s.requireAdAccount(ctx, accountID, adAccountID)
	if err != nil {
		return nil, err
	}
	if level != "campaign" {
		level = "account"
	}
	since, until := rangeWindow(rangeKey)
	rows, err := s.store.ListInsights(ctx, adAccount.AccountID, adAccount.ID, level, since, until)
	if err != nil {
		return nil, err
	}
	points := make([]InsightPoint, len(rows))
	for i, r := range rows {
		points[i] = toInsightPoint(r)
	}
	return points, nil
}

// connectionForViewer resolve a conexao direta da account. Para uma account de
// cliente sem conexao propria, pode reutilizar a conexao central da agencia da
// mesma organizacao; os recursos continuam sujeitos ao vinculo client_account_id.
func (s *Service) connectionForViewer(ctx context.Context, accountID string) (Connection, error) {
	conn, err := s.store.GetConnection(ctx, accountID)
	if noRows(err) {
		return s.store.FindAgencyConnectionForClient(ctx, accountID)
	}
	return conn, err
}

// requireAdAccount garante que a conta de anuncio esta visivel para a account
// ativa. Na conexao compartilhada da agencia, somente linhas explicitamente
// vinculadas ao client_account_id do viewer sao aceitas; sem vinculo = 404.
func (s *Service) requireAdAccount(ctx context.Context, accountID, adAccountID string) (AdAccount, error) {
	conn, err := s.connectionForViewer(ctx, accountID)
	if err != nil {
		if noRows(err) {
			return AdAccount{}, ErrNotConnected
		}
		return AdAccount{}, err
	}
	adAccount, err := s.store.GetAdAccount(ctx, conn.AccountID, adAccountID)
	if err != nil {
		return AdAccount{}, err
	}
	if conn.AccountID != accountID && !adAccountBelongsToClient(adAccount, accountID) {
		return AdAccount{}, pgx.ErrNoRows
	}
	return adAccount, nil
}

func filterAdAccountsForViewer(rows []AdAccount, viewerAccountID, sourceAccountID string) []AdAccount {
	if strings.TrimSpace(viewerAccountID) == strings.TrimSpace(sourceAccountID) {
		return rows
	}
	out := make([]AdAccount, 0, len(rows))
	for _, row := range rows {
		if adAccountBelongsToClient(row, viewerAccountID) {
			out = append(out, row)
		}
	}
	return out
}

func adAccountBelongsToClient(adAccount AdAccount, clientAccountID string) bool {
	return adAccount.ClientAccountID != nil &&
		strings.TrimSpace(*adAccount.ClientAccountID) == strings.TrimSpace(clientAccountID)
}

// refreshAdAccounts busca as contas de anuncio ao vivo e popula o cache.
func (s *Service) refreshAdAccounts(ctx context.Context, connection Connection) ([]AdAccount, error) {
	token, err := s.store.GetDecryptedTokenAtRevision(ctx, connection.AccountID, connection.Revision)
	if err != nil {
		return nil, err
	}
	remote, err := s.client.GetAdAccounts(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := s.store.ReplaceAdAccountsSnapshotAtRevision(
		ctx,
		connection.AccountID,
		connection.ID,
		connection.Revision,
		graphAdAccountsSnapshot(remote),
	); err != nil {
		return nil, err
	}
	return s.store.ListAdAccounts(ctx, connection.AccountID)
}

func graphAdAccountsSnapshot(remote []GraphAdAccount) []AdAccount {
	rows := make([]AdAccount, 0, len(remote))
	for _, account := range remote {
		rows = append(rows, AdAccount{
			MetaAdAccountID: account.AccountID,
			Name:            account.Name,
			Currency:        account.Currency,
			Status:          accountStatusLabel(account.AccountStatus),
			IsCurrent:       true,
		})
	}
	return rows
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
