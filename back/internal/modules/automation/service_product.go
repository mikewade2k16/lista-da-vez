package automation

import "context"

// ProductSource e a fonte de catalogo plugavel da tool de produto (M5). Hoje a unica
// implementacao e o site (site.products); ERP/catalog entram como fontes futuras sem
// mexer no handler/runtime — basta outra implementacao desta interface. A busca JA
// recebe o accountID resolvido pela sessao (escopo multi-tenant garantido na borda).
type ProductSource interface {
	Search(ctx context.Context, accountID, query string, limit int) ([]ProductHit, error)
}

// siteProductSource consulta site.products (modulo site) via o Store deste modulo
// (apenas SELECT escopado por account_id; o modulo site nao e tocado).
type siteProductSource struct {
	store *Store
}

func (s siteProductSource) Search(ctx context.Context, accountID, query string, limit int) ([]ProductHit, error) {
	return s.store.SearchSiteProducts(ctx, accountID, query, limit)
}

// catalogToolLimit e o teto da busca estreita (resultado enxuto para o bot).
const catalogToolLimit = 5

// Sources retorna a config de fontes de produto da automacao default da account.
func (s *Service) Sources(ctx context.Context, accountID string) (SourcesView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return SourcesView{}, err
	}
	return s.store.GetSources(ctx, a.ID)
}

// SetSources grava a config de fontes no settings jsonb da automacao default.
func (s *Service) SetSources(ctx context.Context, accountID string, catalogEnabled bool, siteURLs []string) (SourcesView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return SourcesView{}, err
	}
	if siteURLs == nil {
		siteURLs = []string{}
	}
	return s.store.SetSources(ctx, a.ID, SourcesView{CatalogEnabled: catalogEnabled, SiteURLs: siteURLs})
}

// SearchCatalog (M5) e a tool runtime consumida pelo n8n. Resolve a sessao ->
// automacao -> account (account_id NUNCA vem do query). So busca quando
// catalogEnabled e q nao-vazio; caso contrario devolve lista vazia (sem erro).
func (s *Service) SearchCatalog(ctx context.Context, session, query string) ([]ProductHit, error) {
	ch, err := s.store.GetChannelBySession(ctx, session)
	if err != nil {
		return nil, err
	}
	sources, err := s.store.GetSources(ctx, ch.AutomationID)
	if err != nil {
		return nil, err
	}
	if !sources.CatalogEnabled || query == "" {
		return []ProductHit{}, nil
	}
	src := s.productSource()
	hits, err := src.Search(ctx, ch.AccountID, query, catalogToolLimit)
	if err != nil {
		return nil, err
	}
	return hits, nil
}

// SearchCatalogByAccount (Omni Chat / Fase 2) e a tool de catalogo do chat
// interno. O accountID vem do context token assinado (ContextScope.AccountID),
// NUNCA do query/body do n8n — mesmo escopo multi-tenant da tool do WhatsApp,
// mas resolvido pelo token em vez da sessao WAHA. Reusa productSource().Search
// (SELECT escopado por account_id em site.products), sem reimplementar a query.
//
// Diferenca deliberada vs. SearchCatalog (WhatsApp): aqui NAO consultamos o
// toggle CatalogEnabled das sources. Decisao: o chat interno (operadores do
// painel) sempre pode buscar o catalogo da propria account — o toggle existe
// para controlar o que o bot publico do WhatsApp expoe a clientes externos, nao
// o uso interno. q vazio => lista vazia (igual o WhatsApp; sem busca degenerada).
func (s *Service) SearchCatalogByAccount(ctx context.Context, accountID, query string) ([]ProductHit, error) {
	if accountID == "" || query == "" {
		return []ProductHit{}, nil
	}
	return s.productSource().Search(ctx, accountID, query, catalogToolLimit)
}

// productSource escolhe a fonte de catalogo. Hoje fixa no site; ponto unico de
// extensao para ERP/catalog futuros (sem hardcode espalhado pelos handlers).
func (s *Service) productSource() ProductSource {
	return siteProductSource{store: s.store}
}
