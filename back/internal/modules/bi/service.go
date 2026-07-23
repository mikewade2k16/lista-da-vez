package bi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"mime"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	perolaAPIBase          = "https://api.perola.c10.srv.br/api/v1"
	defaultPerolaPageLimit = 100
	// teto de seguranca do fallback; datasets do overview fixam MaxPages:1
	defaultPerolaMaxPages       = 5
	defaultPerolaTokenTTL       = 50 * time.Minute
	defaultPerolaRequestTimeout = 12 * time.Second
	perolaOverviewConcurrency   = 8
)

var perolaAllowedFindEndpoints = map[string]string{
	"/item/find":                 "/item/find",
	"/imagemItem/find":           "/imagemItem/find",
	"/itemSaldoPrecoCompra/find": "/itemSaldoPrecoCompra/find",
	"/nota/find":                 "/nota/find",
	"/notaItem/find":             "/notaItem/find",
	"/inventario/find":           "/inventario/find",
}

type Service struct {
	perola         *PerolaClient
	options        Options
	tokenTTL       time.Duration
	requestTimeout time.Duration
}

type perolaDatasetDefinition struct {
	Key               string
	Label             string
	Endpoint          string
	Description       string
	IncludeInOverview bool
	OrderByField      string
	RequestTimeout    time.Duration
	PageLimit         int
	MaxPages          int
	PreferredColumns  []perolaColumnDefinition
	FilterKeys        []string
}

type perolaColumnDefinition struct {
	Key            string
	Label          string
	Width          string
	Align          string
	DefaultVisible bool
	Description    string
}

type perolaPagePayload struct {
	PageNumber int                       `json:"pageNumber"`
	Limit      int                       `json:"limit"`
	OrderBy    map[string]string         `json:"orderBy"`
	Conditions map[string]map[string]any `json:"conditions"`
}

type perolaDatasetResult struct {
	Definition perolaDatasetDefinition
	Records    []map[string]any
	Source     PerolaSource
	Err        error
}

type perolaPageResult struct {
	Page         int
	Records      []map[string]any
	Source       PerolaSource
	TotalRecords int
	TotalPages   int
	Err          error
}

func NewService(options ...Options) *Service {
	resolvedOptions := Options{}
	if len(options) > 0 {
		resolvedOptions = options[0]
	}

	if resolvedOptions.PageLimit <= 0 {
		resolvedOptions.PageLimit = defaultPerolaPageLimit
	}
	if resolvedOptions.MaxPages <= 0 {
		resolvedOptions.MaxPages = defaultPerolaMaxPages
	}

	tokenTTL := defaultPerolaTokenTTL
	if parsed, err := time.ParseDuration(strings.TrimSpace(resolvedOptions.TokenTTL)); err == nil && parsed > 0 {
		tokenTTL = parsed
	}

	requestTimeout := defaultPerolaRequestTimeout
	if parsed, err := time.ParseDuration(strings.TrimSpace(resolvedOptions.RequestTimeout)); err == nil && parsed > 0 {
		requestTimeout = parsed
	}

	service := &Service{
		options:        resolvedOptions,
		tokenTTL:       tokenTTL,
		requestTimeout: requestTimeout,
	}
	service.perola = newPerolaClient(perolaClientOptions{
		Credentials: perolaCredentials{
			CompanyKey:  resolvedOptions.CompanyKey,
			CNPJEmpresa: resolvedOptions.DefaultCNPJEmpresa,
			Login:       resolvedOptions.Login,
			Pass:        resolvedOptions.Pass,
			StaticToken: resolvedOptions.StaticToken,
		},
		TokenTTL:       tokenTTL,
		RequestTimeout: requestTimeout,
	})
	return service
}

func (service *Service) PerolaLogin(ctx context.Context, input PerolaLoginInput) (PerolaProxyResponse, error) {
	return service.perola.Login(ctx, perolaCredentials{
		CompanyKey:  fallbackString(input.CompanyKey, service.options.CompanyKey),
		CNPJEmpresa: onlyDigits(fallbackString(input.CNPJEmpresa, service.options.DefaultCNPJEmpresa)),
		Login:       fallbackString(input.Login, service.options.Login),
		Pass:        fallbackString(input.Pass, service.options.Pass),
	})
}

func (service *Service) PerolaFind(ctx context.Context, input PerolaFindInput) (PerolaProxyResponse, error) {
	endpoint, ok := normalizeFindEndpoint(input.Endpoint)
	if !ok {
		return PerolaProxyResponse{}, ErrUnsupportedEndpoint
	}

	companyKey := fallbackString(input.CompanyKey, service.options.CompanyKey)
	cnpjEmpresa := onlyDigits(fallbackString(input.CNPJEmpresa, service.options.DefaultCNPJEmpresa))
	body := bytes.TrimSpace(input.Body)
	if len(body) == 0 {
		body = []byte(defaultFindBody())
	}
	if !json.Valid(body) {
		return PerolaProxyResponse{}, ErrValidation
	}

	token := normalizeBearerToken(input.Token)
	if token == "" && strings.TrimSpace(input.CompanyKey) == "" && strings.TrimSpace(input.CNPJEmpresa) == "" {
		return service.perola.Find(ctx, endpoint, body)
	}
	if token == "" {
		resolvedToken, err := service.resolvePerolaToken(ctx, false)
		if err != nil {
			return PerolaProxyResponse{}, err
		}
		token = resolvedToken
	}
	return service.perolaFind(ctx, endpoint, companyKey, cnpjEmpresa, token, body)
}

func (service *Service) PerolaOverview(ctx context.Context, input PerolaOverviewInput) (PerolaOverviewResponse, error) {
	companyKey := fallbackString(input.CompanyKey, service.options.CompanyKey)
	cnpjEmpresa := onlyDigits(fallbackString(input.CNPJEmpresa, service.options.DefaultCNPJEmpresa))
	token := normalizeBearerToken(input.Token)
	if companyKey == "" || cnpjEmpresa == "" {
		return PerolaOverviewResponse{}, ErrConfiguration
	}
	if token == "" {
		resolvedToken, err := service.resolvePerolaToken(ctx, false)
		if err != nil {
			return PerolaOverviewResponse{}, err
		}
		token = resolvedToken
	}

	definitions := perolaDatasetDefinitions()
	activeDefinitions := overviewDatasetDefinitions(definitions, input.IncludeInventory)
	limiter := make(chan struct{}, perolaOverviewConcurrency)
	results := service.fetchPerolaOverviewDatasets(ctx, activeDefinitions, companyKey, cnpjEmpresa, token, limiter)

	if input.Token == "" && service.hasLoginCredentials() && overviewHasUnauthorizedSource(results) {
		if refreshedToken, tokenErr := service.perola.RefreshAfterUnauthorized(ctx, token); tokenErr == nil {
			token = refreshedToken
			results = service.refetchUnauthorizedOverviewDatasets(ctx, results, companyKey, cnpjEmpresa, token, limiter)
		}
	}

	resultsByKey := map[string]perolaDatasetResult{}
	for _, result := range results {
		resultsByKey[result.Definition.Key] = result
	}
	if !input.IncludeInventory {
		for _, definition := range definitions {
			if definition.IncludeInOverview {
				continue
			}
			resultsByKey[definition.Key] = buildDeferredPerolaDatasetResult(definition)
		}
	}

	tables := make([]PerolaDataTable, 0, len(definitions))
	sources := make([]PerolaSource, 0, len(definitions))
	ok := true

	for _, definition := range definitions {
		result, okResult := resultsByKey[definition.Key]
		if !okResult {
			result = buildDeferredPerolaDatasetResult(definition)
		}
		if result.Err != nil {
			result.Source.Error = "Nao foi possivel ler este conjunto na Perola BI."
			ok = false
		}
		if !result.Source.OK && !result.Source.Pending {
			ok = false
		}

		sources = append(sources, result.Source)
		tables = append(tables, buildPerolaTable(result.Definition, result.Records, result.Source))
	}

	return PerolaOverviewResponse{
		OK:          ok,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		CNPJEmpresa: cnpjEmpresa,
		Metrics:     buildPerolaMetrics(tables, sources),
		Sources:     sources,
		Insights:    buildPerolaInsights(tables, sources),
		Sections:    buildPerolaSections(tables, sources),
		Tables:      tables,
	}, nil
}

func (service *Service) fetchPerolaOverviewDatasets(
	ctx context.Context,
	definitions []perolaDatasetDefinition,
	companyKey string,
	cnpjEmpresa string,
	token string,
	limiter chan struct{},
) []perolaDatasetResult {
	results := make([]perolaDatasetResult, len(definitions))
	var waitGroup sync.WaitGroup

	for index, definition := range definitions {
		waitGroup.Add(1)
		go func(index int, definition perolaDatasetDefinition) {
			defer waitGroup.Done()
			records, source, err := service.fetchPerolaDataset(ctx, definition, companyKey, cnpjEmpresa, token, limiter)
			results[index] = perolaDatasetResult{
				Definition: definition,
				Records:    records,
				Source:     source,
				Err:        err,
			}
		}(index, definition)
	}

	waitGroup.Wait()
	return results
}

func overviewDatasetDefinitions(definitions []perolaDatasetDefinition, includeInventory bool) []perolaDatasetDefinition {
	active := make([]perolaDatasetDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if definition.IncludeInOverview || (includeInventory && definition.Key == "inventario") {
			active = append(active, definition)
		}
	}
	return active
}

func buildDeferredPerolaDatasetResult(definition perolaDatasetDefinition) perolaDatasetResult {
	source := PerolaSource{
		Key:      definition.Key,
		Label:    definition.Label,
		Endpoint: definition.Endpoint,
		OK:       false,
		Pending:  true,
		Error:    "Carga separada para manter a visao principal rapida.",
	}

	return perolaDatasetResult{
		Definition: definition,
		Records:    []map[string]any{},
		Source:     source,
	}
}

func (service *Service) refetchUnauthorizedOverviewDatasets(
	ctx context.Context,
	results []perolaDatasetResult,
	companyKey string,
	cnpjEmpresa string,
	token string,
	limiter chan struct{},
) []perolaDatasetResult {
	refetched := make([]perolaDatasetResult, len(results))
	copy(refetched, results)

	var waitGroup sync.WaitGroup
	for index, result := range results {
		if result.Source.UpstreamStatus != http.StatusUnauthorized {
			continue
		}

		waitGroup.Add(1)
		go func(index int, definition perolaDatasetDefinition) {
			defer waitGroup.Done()
			records, source, err := service.fetchPerolaDataset(ctx, definition, companyKey, cnpjEmpresa, token, limiter)
			refetched[index] = perolaDatasetResult{
				Definition: definition,
				Records:    records,
				Source:     source,
				Err:        err,
			}
		}(index, result.Definition)
	}

	waitGroup.Wait()
	return refetched
}

func overviewHasUnauthorizedSource(results []perolaDatasetResult) bool {
	for _, result := range results {
		if result.Source.UpstreamStatus == http.StatusUnauthorized {
			return true
		}
	}
	return false
}

func (service *Service) fetchPerolaDataset(
	ctx context.Context,
	definition perolaDatasetDefinition,
	companyKey string,
	cnpjEmpresa string,
	token string,
	limiter chan struct{},
) ([]map[string]any, PerolaSource, error) {
	pageLimit := definition.PageLimit
	if pageLimit <= 0 {
		pageLimit = service.options.PageLimit
	}
	maxPages := definition.MaxPages
	if maxPages <= 0 {
		maxPages = service.options.MaxPages
	}
	source := PerolaSource{
		Key:      definition.Key,
		Label:    definition.Label,
		Endpoint: definition.Endpoint,
		OK:       true,
	}
	records := []map[string]any{}

	firstPage, err := service.fetchPerolaDatasetPage(ctx, definition, companyKey, cnpjEmpresa, token, pageLimit, 1, limiter)
	source = mergePerolaPageSource(source, firstPage.Source)
	if err != nil {
		source.OK = false
		return records, source, err
	}
	if !firstPage.Source.OK {
		source.OK = false
		source.Error = firstPage.Source.Error
		return records, source, nil
	}

	records = append(records, firstPage.Records...)
	source.Fetched = len(records)
	source.Total = firstPage.TotalRecords
	if source.Total == 0 {
		source.Total = source.Fetched
	}

	lastPage := lastPerolaPageToFetch(firstPage, pageLimit, maxPages)
	if lastPage <= 1 || len(firstPage.Records) == 0 {
		source.Truncated = firstPage.TotalPages > 1 && maxPages <= 1
		return records, source, nil
	}

	pageResults := service.fetchRemainingPerolaPages(ctx, definition, companyKey, cnpjEmpresa, token, pageLimit, lastPage, limiter)
	for _, pageResult := range pageResults {
		source = mergePerolaPageSource(source, pageResult.Source)
		if pageResult.Err != nil {
			source.OK = false
			return records, source, pageResult.Err
		}
		if !pageResult.Source.OK {
			source.OK = false
			source.Error = pageResult.Source.Error
			return records, source, nil
		}

		records = append(records, pageResult.Records...)
		source.Fetched = len(records)
		if source.Total == 0 && pageResult.TotalRecords > 0 {
			source.Total = pageResult.TotalRecords
		}
	}

	if source.Total == 0 {
		source.Total = source.Fetched
	}
	source.Truncated = firstPage.TotalPages > maxPages && maxPages > 0
	return records, source, nil
}

func (service *Service) fetchRemainingPerolaPages(
	ctx context.Context,
	definition perolaDatasetDefinition,
	companyKey string,
	cnpjEmpresa string,
	token string,
	pageLimit int,
	lastPage int,
	limiter chan struct{},
) []perolaPageResult {
	results := make([]perolaPageResult, lastPage-1)
	var waitGroup sync.WaitGroup

	for page := 2; page <= lastPage; page++ {
		index := page - 2
		waitGroup.Add(1)
		go func(page int, index int) {
			defer waitGroup.Done()
			results[index], results[index].Err = service.fetchPerolaDatasetPage(
				ctx,
				definition,
				companyKey,
				cnpjEmpresa,
				token,
				pageLimit,
				page,
				limiter,
			)
		}(page, index)
	}

	waitGroup.Wait()
	return results
}

func (service *Service) fetchPerolaDatasetPage(
	ctx context.Context,
	definition perolaDatasetDefinition,
	companyKey string,
	cnpjEmpresa string,
	token string,
	pageLimit int,
	page int,
	limiter chan struct{},
) (perolaPageResult, error) {
	result := perolaPageResult{
		Page: page,
		Source: PerolaSource{
			Key:      definition.Key,
			Label:    definition.Label,
			Endpoint: definition.Endpoint,
			OK:       true,
		},
	}

	body, err := json.Marshal(defaultPerolaPageBody(page, pageLimit, definition.OrderByField))
	if err != nil {
		result.Source.OK = false
		return result, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	if err := acquirePerolaSlot(ctx, limiter); err != nil {
		result.Source.OK = false
		return result, err
	}
	defer releasePerolaSlot(limiter)

	requestCtx, cancel := context.WithTimeout(ctx, service.requestTimeoutFor(definition))
	defer cancel()

	startedAt := time.Now()
	response, err := service.perolaFind(requestCtx, definition.Endpoint, companyKey, cnpjEmpresa, token, body)
	result.Source.DurationMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		result.Source.OK = false
		return result, err
	}

	result.Source.UpstreamStatus = response.UpstreamStatus
	result.Source.DurationMs = response.DurationMs
	result.Source.OK = response.OK
	if !response.OK {
		result.Source.Error = response.UpstreamStatusText
		return result, nil
	}

	result.Records = extractRecords(response.Body)
	result.TotalRecords, result.TotalPages = extractPagination(response.Body)
	result.Source.Fetched = len(result.Records)
	result.Source.Total = result.TotalRecords
	return result, nil
}

func (service *Service) requestTimeoutFor(definition perolaDatasetDefinition) time.Duration {
	if definition.RequestTimeout > 0 {
		return definition.RequestTimeout
	}
	return service.requestTimeout
}

func mergePerolaPageSource(source PerolaSource, pageSource PerolaSource) PerolaSource {
	if pageSource.UpstreamStatus != 0 {
		source.UpstreamStatus = pageSource.UpstreamStatus
	}
	source.DurationMs += pageSource.DurationMs
	if pageSource.Total > 0 {
		source.Total = pageSource.Total
	}
	if pageSource.Error != "" {
		source.Error = pageSource.Error
	}
	if !pageSource.OK {
		source.OK = false
	}
	return source
}

func lastPerolaPageToFetch(firstPage perolaPageResult, pageLimit int, maxPages int) int {
	if maxPages <= 1 || len(firstPage.Records) == 0 {
		return 1
	}

	if firstPage.TotalPages > 0 {
		if firstPage.TotalPages < maxPages {
			return firstPage.TotalPages
		}
		return maxPages
	}

	if len(firstPage.Records) < pageLimit {
		return 1
	}
	return maxPages
}

func acquirePerolaSlot(ctx context.Context, limiter chan struct{}) error {
	if limiter == nil {
		return nil
	}

	select {
	case limiter <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrUpstream, ctx.Err())
	}
}

func releasePerolaSlot(limiter chan struct{}) {
	if limiter == nil {
		return
	}
	<-limiter
}

func (service *Service) perolaFind(ctx context.Context, endpoint string, companyKey string, cnpjEmpresa string, token string, body []byte) (PerolaProxyResponse, error) {
	return service.perola.FindWithToken(ctx, endpoint, body, perolaCredentials{
		CompanyKey:  companyKey,
		CNPJEmpresa: cnpjEmpresa,
	}, token)
}

func (service *Service) resolvePerolaToken(ctx context.Context, forceRefresh bool) (string, error) {
	return service.perola.EnsureToken(ctx, forceRefresh)
}

func (service *Service) hasLoginCredentials() bool {
	return service.perola.hasLoginCredentials()
}

func perolaDatasetDefinitions() []perolaDatasetDefinition {
	return []perolaDatasetDefinition{
		{
			Key:               "items",
			Label:             "Itens",
			Endpoint:          "/item/find",
			Description:       "Cadastro de produtos e atributos comerciais retornados pela Perola BI.",
			IncludeInOverview: true,
			OrderByField:      "created",
			MaxPages:          1,
			PreferredColumns: []perolaColumnDefinition{
				{Key: "id", Label: "ID", Width: "92px", Align: "center", DefaultVisible: true},
				{Key: "itemId", Label: "Item", Width: "96px", Align: "center", DefaultVisible: true},
				{Key: "departamento", Label: "Departamento", Width: "150px", DefaultVisible: true},
				{Key: "tipo", Label: "Tipo", Width: "110px", DefaultVisible: true},
				{Key: "subtipo", Label: "Subtipo", Width: "130px", DefaultVisible: true},
				{Key: "classe", Label: "Classe", Width: "150px", DefaultVisible: true},
				{Key: "marca", Label: "Marca", Width: "120px", DefaultVisible: true},
				{Key: "colecao", Label: "Colecao", Width: "120px", DefaultVisible: true},
				{Key: "material", Label: "Material", Width: "120px", DefaultVisible: true},
				{Key: "referencia", Label: "Referencia", Width: "140px", DefaultVisible: true},
				{Key: "cor", Label: "Cor", Width: "120px", DefaultVisible: true},
				{Key: "tamanho", Label: "Tamanho", Width: "110px", Align: "center", DefaultVisible: true},
				{Key: "estilo", Label: "Estilo", Width: "130px", DefaultVisible: false},
			},
			FilterKeys: []string{"departamento", "tipo", "subtipo", "classe", "marca", "colecao", "material", "cor", "tamanho"},
		},
		{
			Key:               "itemImages",
			Label:             "Imagens dos itens",
			Endpoint:          "/imagemItem/find",
			Description:       "Metadados de imagens associados ao cadastro de Item.",
			IncludeInOverview: false,
			OrderByField:      "id",
			PageLimit:         25,
			MaxPages:          1,
			PreferredColumns: []perolaColumnDefinition{
				{Key: "id", Label: "ID", Width: "92px", Align: "center", DefaultVisible: true},
				{Key: "itemId", Label: "Item", Width: "110px", Align: "center", DefaultVisible: true},
				{Key: "filename", Label: "Arquivo", Width: "minmax(220px, 1fr)", DefaultVisible: true},
				{Key: "ordem", Label: "Ordem", Width: "90px", Align: "center", DefaultVisible: true},
			},
			FilterKeys: []string{"itemId", "filename"},
		},
		{
			Key:               "itemPurchasePrices",
			Label:             "Saldo e preco de compra",
			Endpoint:          "/itemSaldoPrecoCompra/find",
			Description:       "Historico de custo, entrada e preco medio por saldo do item.",
			IncludeInOverview: false,
			OrderByField:      "data",
			PageLimit:         25,
			MaxPages:          1,
			PreferredColumns: []perolaColumnDefinition{
				{Key: "id", Label: "ID", Width: "92px", Align: "center", DefaultVisible: true},
				{Key: "itemSaldoId", Label: "Item saldo", Width: "110px", Align: "center", DefaultVisible: true},
				{Key: "empresaId", Label: "Empresa", Width: "100px", Align: "center", DefaultVisible: true},
				{Key: "data", Label: "Data", Width: "150px", DefaultVisible: true},
				{Key: "precoCusto", Label: "Custo", Width: "120px", Align: "right", DefaultVisible: true},
				{Key: "precoEntrada", Label: "Entrada", Width: "120px", Align: "right", DefaultVisible: true},
				{Key: "precoMedio", Label: "Preco medio", Width: "120px", Align: "right", DefaultVisible: true},
			},
			FilterKeys: []string{"itemSaldoId", "empresaId"},
		},
		{
			Key:               "notas",
			Label:             "Notas",
			Endpoint:          "/nota/find",
			Description:       "Cabecalhos de notas para analisar volume, periodos, clientes e valores.",
			IncludeInOverview: true,
			OrderByField:      "dataEmissao",
			MaxPages:          1,
			PreferredColumns: []perolaColumnDefinition{
				{Key: "id", Label: "ID", Width: "92px", Align: "center", DefaultVisible: true},
				{Key: "serie", Label: "Serie", Width: "90px", Align: "center", DefaultVisible: true},
				{Key: "empresaFantasia", Label: "Loja", Width: "170px", DefaultVisible: true},
				{Key: "colaboradorNome", Label: "Vendedor", Width: "170px", DefaultVisible: true},
				{Key: "pessoaNomeRazaoSocial", Label: "Cliente", Width: "minmax(200px, 1fr)", DefaultVisible: true},
				{Key: "pessoaCpfCnpj", Label: "CPF/CNPJ", Width: "150px", DefaultVisible: false},
				{Key: "pessoaCidade", Label: "Cidade", Width: "140px", DefaultVisible: true},
				{Key: "pessoaUf", Label: "UF", Width: "80px", Align: "center", DefaultVisible: true},
				{Key: "pessoaDatNascimento", Label: "Nascimento", Width: "130px", DefaultVisible: false},
				{Key: "dataEmissao", Label: "Emissao", Width: "170px", DefaultVisible: true},
				{Key: "tipoNota", Label: "Tipo nota", Width: "120px", DefaultVisible: true},
				{Key: "numDocumento", Label: "Documento", Width: "120px", DefaultVisible: false},
				{Key: "valorTotal", Label: "Valor total", Width: "130px", Align: "right", DefaultVisible: true},
				{Key: "valorDesconto", Label: "Desconto", Width: "120px", Align: "right", DefaultVisible: false},
			},
			FilterKeys: []string{"empresaFantasia", "colaboradorNome", "pessoaCidade", "pessoaUf", "tipoNota", "serie"},
		},
		{
			Key:               "notaItems",
			Label:             "Itens da nota",
			Endpoint:          "/notaItem/find",
			Description:       "Itens vendidos por nota, base para ranking de produtos, quantidade e ticket por item.",
			IncludeInOverview: true,
			OrderByField:      "id",
			MaxPages:          1,
			PreferredColumns: []perolaColumnDefinition{
				{Key: "id", Label: "ID", Width: "92px", Align: "center", DefaultVisible: true},
				{Key: "notaId", Label: "Nota", Width: "96px", Align: "center", DefaultVisible: true},
				{Key: "itemSaldoId", Label: "Item saldo", Width: "110px", Align: "center", DefaultVisible: true},
				{Key: "colaboradorNome", Label: "Vendedor", Width: "170px", DefaultVisible: true},
				{Key: "quantidade", Label: "Qtd", Width: "95px", Align: "right", DefaultVisible: true},
				{Key: "precoUnitario", Label: "Preco unitario", Width: "120px", Align: "right", DefaultVisible: true},
				{Key: "precoTotal", Label: "Preco total", Width: "120px", Align: "right", DefaultVisible: true},
				{Key: "valorDesconto", Label: "Desconto", Width: "120px", Align: "right", DefaultVisible: false},
				{Key: "quantidadeDevolvida", Label: "Qtd devolvida", Width: "120px", Align: "right", DefaultVisible: false},
				{Key: "estoqueOperacao", Label: "Operacao", Width: "100px", Align: "center", DefaultVisible: false},
			},
			FilterKeys: []string{"notaId", "itemSaldoId", "colaboradorNome", "estoqueOperacao"},
		},
		{
			Key:               "inventario",
			Label:             "Inventario",
			Endpoint:          "/inventario/find",
			Description:       "Movimentos e correcoes de inventario, consultados somente sob demanda por itemSaldoId.",
			IncludeInOverview: false,
			OrderByField:      "data",
			RequestTimeout:    30 * time.Second,
			PageLimit:         25,
			MaxPages:          1,
			PreferredColumns: []perolaColumnDefinition{
				{Key: "id", Label: "ID", Width: "92px", Align: "center", DefaultVisible: true},
				{Key: "itemSaldoId", Label: "Item saldo", Width: "110px", Align: "center", DefaultVisible: true},
				{Key: "empresaFantasia", Label: "Loja", Width: "170px", DefaultVisible: true},
				{Key: "empresaSigla", Label: "Sigla", Width: "90px", Align: "center", DefaultVisible: true},
				{Key: "empresaCnpj", Label: "CNPJ", Width: "150px", DefaultVisible: false},
				{Key: "tipoInventario", Label: "Tipo", Width: "150px", DefaultVisible: true},
				{Key: "tipoInventarioSigla", Label: "Tipo sigla", Width: "110px", Align: "center", DefaultVisible: false},
				{Key: "data", Label: "Data", Width: "170px", DefaultVisible: true},
				{Key: "quantidade", Label: "Qtd", Width: "95px", Align: "right", DefaultVisible: true},
			},
			FilterKeys: []string{"empresaFantasia", "empresaSigla", "tipoInventario", "tipoInventarioSigla"},
		},
	}
}

func buildPerolaTable(definition perolaDatasetDefinition, records []map[string]any, source PerolaSource) PerolaDataTable {
	rows := make([]map[string]any, 0, len(records))
	for index, record := range records {
		rows = append(rows, normalizePerolaRow(record, index))
	}

	columns := resolvePerolaColumns(definition, rows)

	return PerolaDataTable{
		Key:         definition.Key,
		Label:       definition.Label,
		Description: definition.Description,
		Pending:     source.Pending,
		Total:       maxInt(source.Total, len(rows)),
		Fetched:     len(rows),
		Columns:     columns,
		Filters:     resolvePerolaFilters(definition, rows, columns),
		Rows:        rows,
	}
}

func resolvePerolaColumns(definition perolaDatasetDefinition, rows []map[string]any) []PerolaTableColumn {
	seen := map[string]bool{"__rowId": true}
	columns := []PerolaTableColumn{}

	hasKey := func(key string) bool {
		if len(rows) == 0 {
			return true
		}
		for _, row := range rows {
			if isNonEmptyScalar(row[key]) {
				return true
			}
		}
		return false
	}

	for _, preferred := range definition.PreferredColumns {
		if seen[preferred.Key] || !hasKey(preferred.Key) {
			continue
		}
		seen[preferred.Key] = true
		columns = append(columns, PerolaTableColumn{
			ID:             preferred.Key,
			Label:          preferred.Label,
			Width:          preferred.Width,
			Align:          fallbackString(preferred.Align, "start"),
			DefaultVisible: preferred.DefaultVisible,
			Description:    preferred.Description,
		})
	}

	discovered := map[string]int{}
	for _, row := range rows {
		for key, value := range row {
			if seen[key] || !isNonEmptyScalar(value) {
				continue
			}
			discovered[key]++
		}
	}

	keys := make([]string, 0, len(discovered))
	for key := range discovered {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if discovered[keys[i]] == discovered[keys[j]] {
			return keys[i] < keys[j]
		}
		return discovered[keys[i]] > discovered[keys[j]]
	})

	for _, key := range keys {
		if len(columns) >= 18 {
			break
		}
		seen[key] = true
		columns = append(columns, PerolaTableColumn{
			ID:             key,
			Label:          humanizeKey(key),
			Width:          "minmax(130px, 1fr)",
			Align:          inferColumnAlign(key),
			DefaultVisible: len(columns) < 12,
		})
	}

	return columns
}

func resolvePerolaFilters(definition perolaDatasetDefinition, rows []map[string]any, columns []PerolaTableColumn) []PerolaTableFilter {
	columnSet := map[string]bool{}
	for _, column := range columns {
		columnSet[column.ID] = true
	}

	labelByKey := map[string]string{}
	for _, column := range definition.PreferredColumns {
		labelByKey[column.Key] = column.Label
	}

	filters := []PerolaTableFilter{}
	for _, key := range definition.FilterKeys {
		if !columnSet[key] {
			continue
		}
		options := topFilterOptions(rows, key, 50)
		if len(options) < 2 {
			continue
		}
		filters = append(filters, PerolaTableFilter{
			Key:         key,
			Label:       fallbackString(labelByKey[key], humanizeKey(key)),
			Placeholder: "Todos",
			Options:     options,
		})
	}

	return filters
}

func buildPerolaMetrics(tables []PerolaDataTable, sources []PerolaSource) []PerolaMetric {
	tableByKey := map[string]PerolaDataTable{}
	for _, table := range tables {
		tableByKey[table.Key] = table
	}

	sourceByKey := map[string]PerolaSource{}
	for _, source := range sources {
		sourceByKey[source.Key] = source
	}

	okSources := 0
	pendingSources := 0
	for _, source := range sources {
		if source.OK {
			okSources++
			continue
		}
		if source.Pending {
			pendingSources++
		}
	}

	items := tableByKey["items"]
	notas := tableByKey["notas"]
	notaItems := tableByKey["notaItems"]
	inventario := tableByKey["inventario"]
	connectedSources := okSources + pendingSources

	metrics := []PerolaMetric{
		{
			Key:    "sources",
			Label:  "Fontes ativas",
			Value:  fmt.Sprintf("%d/%d", connectedSources, len(sources)),
			Detail: sourcesMetricDetail(okSources, pendingSources, len(sources)),
			Tone:   sourcesMetricTone(okSources, pendingSources, len(sources)),
		},
		{
			Key:    "items",
			Label:  "Itens",
			Value:  formatInt(items.Fetched),
			Detail: sourceTotalDetail(sourceByKey["items"], "itens cadastrados", "Amostra rapida para analise de mix"),
		},
		{
			Key:    "notas",
			Label:  "Notas",
			Value:  formatInt(notas.Fetched),
			Detail: sourceTotalDetail(sourceByKey["notas"], "notas no historico", "Amostra rapida para volume, cliente e periodo"),
		},
		{
			Key:    "notaItems",
			Label:  "Itens vendidos",
			Value:  formatInt(notaItems.Fetched),
			Detail: sourceTotalDetail(sourceByKey["notaItems"], "linhas vendidas no historico", "Amostra rapida para ranking e ticket por item"),
		},
		{
			Key:    "inventario",
			Label:  "Inventario",
			Value:  inventoryMetricValue(inventario),
			Detail: inventoryMetricDetail(inventario, sourceByKey["inventario"]),
			Tone:   inventoryMetricTone(inventario),
		},
		{
			Key:    "mix",
			Label:  "Mix identificado",
			Value:  formatInt(uniqueCount(items.Rows, "tipo", "classe", "marca")),
			Detail: "Combinacoes de tipo, classe e marca",
		},
	}

	return metrics
}

func buildPerolaInsights(tables []PerolaDataTable, sources []PerolaSource) []PerolaInsight {
	tableByKey := map[string]PerolaDataTable{}
	for _, table := range tables {
		tableByKey[table.Key] = table
	}

	sourceByKey := map[string]PerolaSource{}
	for _, source := range sources {
		sourceByKey[source.Key] = source
	}

	items := tableByKey["items"]
	inventario := tableByKey["inventario"]
	truncated := false
	for _, source := range sources {
		if source.Truncated {
			truncated = true
			break
		}
	}

	insights := []PerolaInsight{
		{
			Title: "Escala da base",
			Body: fmt.Sprintf(
				"A Perola BI informa %s itens cadastrados, %s notas, %s linhas vendidas e %s. A tela carrega uma amostra recente para continuar rapida, mas ja conhece o tamanho real da base para BI comercial.",
				sourceTotalValue(sourceByKey["items"]),
				sourceTotalValue(sourceByKey["notas"]),
				sourceTotalValue(sourceByKey["notaItems"]),
				sourceTotalSentence(sourceByKey["inventario"], "movimentos de inventario", "inventario em carga separada"),
			),
			Tone: "info",
		},
		{
			Title: "CRM acionavel",
			Body:  "Notas ja trazem cliente, CPF/CNPJ, cidade, UF, nascimento, vendedor, loja e valor. Isso sustenta aniversario, reativacao, carteira VIP, mapa regional e performance comercial sem expor payload tecnico no navegador.",
			Tone:  "success",
		},
		{
			Title: "Compra e sortimento",
			Body: fmt.Sprintf(
				"No cadastro de itens, os cortes mais uteis ja estao disponiveis: departamento, tipo, subtipo, classe, marca, colecao, material, cor e tamanho. O maior agrupamento atual e %s, o que ajuda a enxergar concentracao e lacuna de grade.",
				topDescriptor(items.Rows, []string{"departamento", "tipo", "marca"}),
			),
			Tone: "info",
		},
		{
			Title: "Mais vendido e comportamento",
			Body:  "Itens da nota ja entregam itemSaldoId, quantidade, preco, desconto e devolucao. Isso permite ranking do que mais vendeu por itemSaldoId, ticket medio, disciplina de desconto e leitura de devolucao por linha vendida.",
			Tone:  "info",
		},
		{
			Title: "Imagem e amarracao do produto",
			Body:  "A API atual nao traz foto real do produto e tambem nao entrega um vinculo claro entre itemSaldoId e o cadastro do item na mesma resposta. Por isso ja mostramos mix, ranking e estoque operacional, mas nao um catalogo visual fiel do mais vendido.",
			Tone:  "warning",
		},
		{
			Title: "Estoque inteligente",
			Body:  inventoryInsightBody(inventario),
			Tone:  "warning",
		},
	}

	if truncated {
		insights = append(insights, PerolaInsight{
			Title: "Carga parcial",
			Body:  "Pelo menos uma fonte bateu o limite de paginas configurado. Para buscar mais historico, aumente PEROLA_BI_MAX_PAGES no backend.",
			Tone:  "warning",
		})
	}

	return insights
}

func buildPerolaSections(tables []PerolaDataTable, sources []PerolaSource) []PerolaIntelligenceSection {
	tableByKey := map[string]PerolaDataTable{}
	for _, table := range tables {
		tableByKey[table.Key] = table
	}

	sourceByKey := map[string]PerolaSource{}
	for _, source := range sources {
		sourceByKey[source.Key] = source
	}

	notas := tableByKey["notas"]
	notaItems := tableByKey["notaItems"]
	items := tableByKey["items"]
	inventario := tableByKey["inventario"]
	inventorySource := sourceByKey["inventario"]

	customerCount := uniqueCount(notas.Rows, "pessoaCpfCnpj")
	customerLeader, customerLeaderSpend := topStringBySum(notas.Rows, "pessoaNomeRazaoSocial", "valorTotal")
	sellerLeader, sellerLeaderValue := topStringBySum(notas.Rows, "colaboradorNome", "valorTotal")
	cityLeader, cityLeaderCount := topStringByCount(notas.Rows, "pessoaCidade")
	storeLeader, storeLeaderValue := topStringBySum(notas.Rows, "empresaFantasia", "valorTotal")
	itemLeaderQty, itemLeaderQtyValue := topStringBySum(notaItems.Rows, "itemSaldoId", "quantidade")
	itemLeaderRevenue, itemLeaderRevenueValue := topStringBySum(notaItems.Rows, "itemSaldoId", "precoTotal")
	mixDepartment, mixDepartmentCount := topStringByCount(items.Rows, "departamento")
	mixBrand, mixBrandCount := topStringByCount(items.Rows, "marca")
	mixMaterial, mixMaterialCount := topStringByCount(items.Rows, "material")
	mixColor, mixColorCount := topStringByCount(items.Rows, "cor")
	discountTotal := sumFloat(notas.Rows, "valorDesconto")
	salesTotal := sumFloat(notas.Rows, "valorTotal")
	discountRate := ratioFloat(discountTotal, salesTotal)
	returnedQty := sumFloat(notaItems.Rows, "quantidadeDevolvida")
	soldQty := sumFloat(notaItems.Rows, "quantidade")
	returnRate := ratioFloat(returnedQty, soldQty)
	activeStores := uniqueCount(notas.Rows, "empresaFantasia")
	activeSellers := uniqueCount(notas.Rows, "colaboradorNome")

	sections := []PerolaIntelligenceSection{
		{
			Key:     "potencial",
			Title:   "Potencial da API",
			Summary: "Mesmo carregando uma amostra rapida no front, a propria paginacao da Perola ja revela o tamanho do historico disponivel para BI.",
			Tone:    "info",
			Items: []PerolaIntelligenceItem{
				{Label: "Cadastro de itens", Value: sourceTotalValue(sourceByKey["items"]), Detail: "Base total informada pela Perola BI."},
				{Label: "Historico de notas", Value: sourceTotalValue(sourceByKey["notas"]), Detail: "Cabecalhos para CRM, vendedor, loja e ticket."},
				{Label: "Linhas vendidas", Value: sourceTotalValue(sourceByKey["notaItems"]), Detail: "Base para ranking, desconto e item saldo."},
				{Label: "Movimentos de inventario", Value: sourceTotalValue(sourceByKey["inventario"]), Detail: sourceTotalDetail(sourceByKey["inventario"], "movimentos no historico", "Carregando em segundo plano para manter a tela rapida")},
				{Label: "Imagem real do produto", Value: "Nao", Detail: "Os endpoints atuais nao trazem URL, foto ou midia pronta para catalogo."},
			},
		},
		{
			Key:     "crm",
			Title:   "CRM",
			Summary: "Clientes, cidades, aniversarios e vendedores ja aparecem em nota, entao a API pode alimentar relacionamento e recorrencia.",
			Tone:    "info",
			Items: []PerolaIntelligenceItem{
				{Label: "Clientes unicos", Value: formatInt(customerCount), Detail: "Base observada nesta amostra de notas."},
				{Label: "Clientes recorrentes", Value: formatInt(recurrentCount(notas.Rows, "pessoaCpfCnpj")), Detail: "Quem apareceu em mais de uma nota."},
				{Label: "Cidade lider", Value: fallbackString(cityLeader, "-"), Detail: countDetail(cityLeaderCount, "compras registradas")},
				{Label: "Maior cliente", Value: fallbackString(customerLeader, "-"), Detail: moneyDetail(customerLeaderSpend)},
				{Label: "Valor por cliente", Value: formatMoneyBRL(ratioFloat(salesTotal, float64(customerCount))), Detail: "Media observada por cliente unico."},
				{Label: "Aniversarios no mes", Value: formatInt(countBirthdaysInMonth(notas.Rows, "pessoaDatNascimento", time.Now().UTC().Month())), Detail: "Gatilho para campanha de relacionamento."},
			},
		},
		{
			Key:     "vendas",
			Title:   "Vendas",
			Summary: "Notas e itens da nota ja sustentam ticket, vendedor, loja, item mais forte e comportamento de desconto.",
			Tone:    "success",
			Items: []PerolaIntelligenceItem{
				{Label: "Ticket medio", Value: formatMoneyBRL(avgFloat(notas.Rows, "valorTotal")), Detail: "Media por nota observada."},
				{Label: "Valor observado", Value: formatMoneyBRL(salesTotal), Detail: "Soma das notas carregadas na amostra."},
				{Label: "Vendedor lider", Value: fallbackString(sellerLeader, "-"), Detail: moneyDetail(sellerLeaderValue)},
				{Label: "Loja lider", Value: fallbackString(storeLeader, "-"), Detail: moneyDetail(storeLeaderValue)},
				{Label: "Mais vendido", Value: fallbackString(itemLeaderQty, "-"), Detail: quantityDetail(itemLeaderQtyValue, "unidades na amostra")},
				{Label: "Maior faturamento", Value: fallbackString(itemLeaderRevenue, "-"), Detail: moneyDetail(itemLeaderRevenueValue)},
			},
		},
		{
			Key:     "comportamento",
			Title:   "Comportamento",
			Summary: "Mesmo sem foto real, ja temos sinais de desconto, devolucao, preco medio e cobertura comercial por loja e vendedor.",
			Tone:    "info",
			Items: []PerolaIntelligenceItem{
				{Label: "Desconto observado", Value: formatMoneyBRL(discountTotal), Detail: fmt.Sprintf("%s do valor vendido na amostra", formatPercent(discountRate))},
				{Label: "Devolucao observada", Value: quantityValue(returnedQty), Detail: fmt.Sprintf("%s das unidades observadas", formatPercent(returnRate))},
				{Label: "Preco medio por linha", Value: formatMoneyBRL(avgFloat(notaItems.Rows, "precoUnitario")), Detail: "Media por item vendido na amostra."},
				{Label: "Lojas ativas", Value: formatInt(activeStores), Detail: "Lojas com nota nesta leitura."},
				{Label: "Equipe ativa", Value: formatInt(activeSellers), Detail: "Vendedores identificados na amostra."},
			},
		},
		{
			Key:     "mix",
			Title:   "Compra e Mix",
			Summary: "Itens trazem marca, colecao, material, tipo, classe e cor. Isso ajuda compra, profundidade de grade e leitura de sortimento.",
			Tone:    "info",
			Items: []PerolaIntelligenceItem{
				{Label: "Departamento lider", Value: fallbackString(mixDepartment, "-"), Detail: countDetail(mixDepartmentCount, "itens nesta amostra")},
				{Label: "Marca lider", Value: fallbackString(mixBrand, "-"), Detail: countDetail(mixBrandCount, "itens nesta amostra")},
				{Label: "Material lider", Value: fallbackString(mixMaterial, "-"), Detail: countDetail(mixMaterialCount, "itens nesta amostra")},
				{Label: "Cor lider", Value: fallbackString(mixColor, "-"), Detail: countDetail(mixColorCount, "itens nesta amostra")},
				{Label: "Classes mapeadas", Value: formatInt(uniqueCount(items.Rows, "classe")), Detail: "Base para cluster de compra e ruptura."},
				{Label: "Tipos mapeados", Value: formatInt(uniqueCount(items.Rows, "tipo")), Detail: "Leitura inicial do mix comercial."},
			},
		},
	}

	sections = append(sections, buildInventorySection(inventario, inventorySource))
	sections = append(sections, PerolaIntelligenceSection{
		Key:     "limites",
		Title:   "O que falta para avancar",
		Summary: "A documentacao e os dados atuais ja permitem muita inteligencia, mas ainda existem lacunas para chegar em um BI de produto e reposicao mais completo.",
		Tone:    "warning",
		Items: []PerolaIntelligenceItem{
			{Label: "Imagem real do produto", Value: "Nao disponivel", Detail: "Os endpoints atuais nao trazem URL, foto ou midia do item."},
			{Label: "Produto no inventario", Value: "Parcial", Detail: "Inventario chega por itemSaldoId; falta um vinculo direto com nome e foto do item."},
			{Label: "Cobertura completa", Value: "Depende de mais carga", Detail: "Para giro, cobertura e reposicao mais confiaveis, vale ampliar historico ou expor saldo consolidado."},
			{Label: "Mais vendido por nome", Value: "Depende de mapeamento", Detail: "Hoje o ranking confiavel sai por itemSaldoId; para nome/foto precisamos de mais amarracao."},
			{Label: "Compra assistida", Value: "Ja possivel", Detail: "Departamento, marca, material, cor, desconto e preco ja sustentam decisao de compra e mix."},
		},
	})

	return sections
}

func buildInventorySection(inventario PerolaDataTable, source PerolaSource) PerolaIntelligenceSection {
	if source.Pending {
		return PerolaIntelligenceSection{
			Key:     "estoque",
			Title:   "Estoque",
			Summary: "Inventario e o endpoint mais pesado da Perola. A tela sobe primeiro e a leitura de estoque entra em segundo plano.",
			Tone:    "warning",
			Items: []PerolaIntelligenceItem{
				{Label: "Status", Value: "Em carga separada", Detail: "A API de inventario respondeu mais lenta que as demais."},
				{Label: "Tempo esperado", Value: "Ate 30s", Detail: "Timeout maior reservado so para esta fonte."},
				{Label: "Chave disponivel", Value: "itemSaldoId", Detail: "Ja da para cruzar inventario com itens da nota."},
				{Label: "Leitura possivel", Value: "Correcao e loja", Detail: "Tipo de inventario, data, quantidade e empresa ja chegam nesse endpoint."},
			},
		}
	}

	if !source.OK || len(inventario.Rows) == 0 {
		return PerolaIntelligenceSection{
			Key:     "estoque",
			Title:   "Estoque",
			Summary: "A fonte de inventario existe, mas a leitura ainda nao ficou utilizavel nesta tentativa.",
			Tone:    "warning",
			Items: []PerolaIntelligenceItem{
				{Label: "Status", Value: "Sem leitura valida", Detail: fallbackString(source.Error, "A Perola nao retornou dados no tempo esperado.")},
				{Label: "Impacto", Value: "Sem cobertura real", Detail: "Sem inventario carregado, ruptura e excesso ficam indicativos."},
				{Label: "Ainda temos", Value: "Venda e CRM", Detail: "Item, nota e item da nota continuam alimentando mix, ticket e comportamento."},
			},
		}
	}

	storeLeader, storeCount := topStringByCount(inventario.Rows, "empresaFantasia")
	typeLeader, typeCount := topStringByCount(inventario.Rows, "tipoInventario")
	activeInventoryStores := uniqueCount(inventario.Rows, "empresaFantasia")
	return PerolaIntelligenceSection{
		Key:     "estoque",
		Title:   "Estoque",
		Summary: "Quando o inventario responde, ele vira base para conferencia, correcao recente por loja e cruzamento com venda pelo itemSaldoId.",
		Tone:    "success",
		Items: []PerolaIntelligenceItem{
			{Label: "Lojas com movimento", Value: formatInt(activeInventoryStores), Detail: "Lojas observadas na amostra de inventario."},
			{Label: "Loja lider", Value: fallbackString(storeLeader, "-"), Detail: countDetail(storeCount, "movimentos observados")},
			{Label: "Tipo lider", Value: fallbackString(typeLeader, "-"), Detail: countDetail(typeCount, "movimentos observados")},
			{Label: "Ultima data", Value: maxDateLabel(inventario.Rows, "data"), Detail: "Ultimo movimento encontrado na amostra."},
			{Label: "Quantidade amostrada", Value: quantityValue(sumFloat(inventario.Rows, "quantidade")), Detail: "Soma de quantidade da leitura atual."},
		},
	}
}

func normalizeFindEndpoint(value string) (string, bool) {
	endpoint := strings.TrimSpace(value)
	if endpoint == "" {
		endpoint = "/item/find"
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	resolved, ok := perolaAllowedFindEndpoints[endpoint]
	return resolved, ok
}

func normalizeBearerToken(value string) string {
	token := strings.TrimSpace(value)
	parts := strings.Fields(token)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		token = parts[1]
	}
	return strings.TrimSpace(token)
}

func onlyDigits(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func defaultFindBody() string {
	body, _ := json.Marshal(defaultPerolaPageBody(1, defaultPerolaPageLimit, "created"))
	return string(body)
}

func defaultPerolaPageBody(page int, limit int, orderByField string) perolaPagePayload {
	resolvedOrderField := strings.TrimSpace(orderByField)
	if resolvedOrderField == "" {
		resolvedOrderField = "created"
	}

	return perolaPagePayload{
		PageNumber: page,
		Limit:      limit,
		OrderBy:    map[string]string{resolvedOrderField: "DESC"},
		Conditions: map[string]map[string]any{
			"startWith":       {},
			"endWith":         {},
			"content":         {},
			"equalsTo":        {},
			"differentTo":     {},
			"greaterThan":     {},
			"greaterMoreThan": {},
			"lessThan":        {},
			"lessMoreThan":    {},
		},
	}
}

func parseUpstreamBody(contentType string, rawBody []byte) (any, string) {
	rawText := string(rawBody)
	if len(bytes.TrimSpace(rawBody)) == 0 {
		return nil, ""
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if (err == nil && strings.EqualFold(mediaType, "application/json")) || json.Valid(rawBody) {
		var parsed any
		if err := json.Unmarshal(rawBody, &parsed); err == nil {
			return parsed, rawText
		}
	}

	return rawText, rawText
}

func selectedHeaders(headers http.Header) map[string]any {
	selected := map[string]any{}
	for _, key := range []string{
		"Content-Type",
		"Content-Length",
		"Date",
		"Server",
		"Cache-Control",
	} {
		if value := headers.Values(key); len(value) > 0 {
			selected[key] = value
		}
	}
	return selected
}

func extractRecords(value any) []map[string]any {
	body, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	rawRecords, ok := body["registros"].([]any)
	if !ok {
		return nil
	}

	records := make([]map[string]any, 0, len(rawRecords))
	for _, rawRecord := range rawRecords {
		record, ok := rawRecord.(map[string]any)
		if !ok {
			continue
		}
		records = append(records, record)
	}

	return records
}

func extractPagination(value any) (totalRecords int, totalPages int) {
	body, ok := value.(map[string]any)
	if !ok {
		return 0, 0
	}
	pagination, ok := body["paginacao"].(map[string]any)
	if !ok {
		return 0, 0
	}

	return intFromAny(pagination["totalRegistros"]), intFromAny(pagination["totalPaginas"])
}

func normalizePerolaRow(record map[string]any, index int) map[string]any {
	row := map[string]any{
		"__rowId": fmt.Sprintf("perola-row-%d", index+1),
	}

	for key, value := range record {
		normalizedKey := normalizePerolaKey(key)
		if normalizedKey == "" {
			continue
		}
		scalar, ok := scalarValue(value)
		if !ok {
			continue
		}
		row[normalizedKey] = scalar
	}

	return row
}

func normalizePerolaKey(key string) string {
	normalized := strings.TrimSpace(key)
	if normalized == "" {
		return ""
	}
	switch normalized {
	case "subTipo", "subTipoId":
		return strings.Replace(normalized, "T", "t", 1)
	default:
		return normalized
	}
}

func scalarValue(value any) (any, bool) {
	switch typed := value.(type) {
	case nil:
		return "", true
	case string:
		return strings.TrimSpace(typed), true
	case bool:
		return typed, true
	case float64:
		if math.Trunc(typed) == typed {
			return int64(typed), true
		}
		return typed, true
	case float32:
		return float64(typed), true
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return typed, true
	default:
		return nil, false
	}
}

func isNonEmptyScalar(value any) bool {
	scalar, ok := scalarValue(value)
	if !ok {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(scalar)) != ""
}

func topFilterOptions(rows []map[string]any, key string, limit int) []string {
	counts := map[string]int{}
	for _, row := range rows {
		value := strings.TrimSpace(fmt.Sprint(row[key]))
		if value == "" || value == "<nil>" {
			continue
		}
		counts[value]++
	}

	values := make([]string, 0, len(counts))
	for value := range counts {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		if counts[values[i]] == counts[values[j]] {
			return values[i] < values[j]
		}
		return counts[values[i]] > counts[values[j]]
	})

	if limit > 0 && len(values) > limit {
		values = values[:limit]
	}

	return values
}

func extractBearerToken(value any) string {
	switch typed := value.(type) {
	case string:
		return jwtCandidate(typed)
	case map[string]any:
		for _, key := range []string{"token", "access_token", "accessToken", "bearer", "bearerToken", "jwt"} {
			if raw, ok := typed[key]; ok {
				if token := extractBearerToken(raw); token != "" {
					return token
				}
			}
		}
		for _, nested := range typed {
			if token := extractBearerToken(nested); token != "" {
				return token
			}
		}
	case []any:
		for _, nested := range typed {
			if token := extractBearerToken(nested); token != "" {
				return token
			}
		}
	}

	return ""
}

var jwtPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)

func jwtCandidate(value string) string {
	token := normalizeBearerToken(value)
	if !jwtPattern.MatchString(token) {
		return ""
	}
	return token
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		value, err := typed.Int64()
		if err != nil {
			return 0
		}
		return int(value)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func humanizeKey(key string) string {
	replacer := strings.NewReplacer("_", " ", "-", " ")
	text := replacer.Replace(strings.TrimSpace(key))
	var words []string
	var current strings.Builder
	for index, r := range text {
		if index > 0 && r >= 'A' && r <= 'Z' {
			words = append(words, current.String())
			current.Reset()
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	for index, word := range words {
		if word == "" {
			continue
		}
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func inferColumnAlign(key string) string {
	normalized := strings.ToLower(key)
	for _, token := range []string{"id", "qtd", "quant", "valor", "total", "preco", "price", "amount", "estoque"} {
		if strings.Contains(normalized, token) {
			return "end"
		}
	}
	return "start"
}

func formatInt(value int) string {
	text := strconv.Itoa(value)
	if len(text) <= 3 {
		return text
	}
	var parts []string
	for len(text) > 3 {
		parts = append([]string{text[len(text)-3:]}, parts...)
		text = text[:len(text)-3]
	}
	parts = append([]string{text}, parts...)
	return strings.Join(parts, ".")
}

func uniqueCount(rows []map[string]any, keys ...string) int {
	values := map[string]bool{}
	for _, row := range rows {
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, strings.TrimSpace(fmt.Sprint(row[key])))
		}
		value := strings.TrimSpace(strings.Join(parts, "|"))
		if value == "" || value == "||" {
			continue
		}
		values[value] = true
	}
	return len(values)
}

func topDescriptor(rows []map[string]any, keys []string) string {
	counts := map[string]int{}
	for _, row := range rows {
		parts := []string{}
		for _, key := range keys {
			value := strings.TrimSpace(fmt.Sprint(row[key]))
			if value != "" && value != "<nil>" {
				parts = append(parts, value)
			}
		}
		if len(parts) == 0 {
			continue
		}
		counts[strings.Join(parts, " / ")]++
	}
	if len(counts) == 0 {
		return "aguardando mais dados"
	}

	type entry struct {
		value string
		count int
	}
	entries := make([]entry, 0, len(counts))
	for value, count := range counts {
		entries = append(entries, entry{value: value, count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count == entries[j].count {
			return entries[i].value < entries[j].value
		}
		return entries[i].count > entries[j].count
	})

	return fmt.Sprintf("%s (%s registros)", entries[0].value, formatInt(entries[0].count))
}

func topStringByCount(rows []map[string]any, key string) (string, int) {
	counts := map[string]int{}
	for _, row := range rows {
		value := strings.TrimSpace(fmt.Sprint(row[key]))
		if value == "" || value == "<nil>" {
			continue
		}
		counts[value]++
	}

	bestValue := ""
	bestCount := 0
	for value, count := range counts {
		if count > bestCount || (count == bestCount && value < bestValue) {
			bestValue = value
			bestCount = count
		}
	}

	return bestValue, bestCount
}

func topStringBySum(rows []map[string]any, key string, metricKey string) (string, float64) {
	sums := map[string]float64{}
	for _, row := range rows {
		value := strings.TrimSpace(fmt.Sprint(row[key]))
		if value == "" || value == "<nil>" {
			continue
		}
		sums[value] += floatFromAny(row[metricKey])
	}

	bestValue := ""
	bestSum := 0.0
	for value, sum := range sums {
		if sum > bestSum || (sum == bestSum && value < bestValue) {
			bestValue = value
			bestSum = sum
		}
	}

	return bestValue, bestSum
}

func recurrentCount(rows []map[string]any, key string) int {
	counts := map[string]int{}
	for _, row := range rows {
		value := strings.TrimSpace(fmt.Sprint(row[key]))
		if value == "" || value == "<nil>" {
			continue
		}
		counts[value]++
	}

	total := 0
	for _, count := range counts {
		if count > 1 {
			total++
		}
	}
	return total
}

func countBirthdaysInMonth(rows []map[string]any, key string, month time.Month) int {
	total := 0
	for _, row := range rows {
		if monthFromAny(row[key]) == month {
			total++
		}
	}
	return total
}

func sumFloat(rows []map[string]any, key string) float64 {
	total := 0.0
	for _, row := range rows {
		total += floatFromAny(row[key])
	}
	return total
}

func avgFloat(rows []map[string]any, key string) float64 {
	if len(rows) == 0 {
		return 0
	}
	return sumFloat(rows, key) / float64(len(rows))
}

func floatFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case uint:
		return float64(typed)
	case string:
		normalized := strings.TrimSpace(typed)
		if normalized == "" {
			return 0
		}
		if strings.Contains(normalized, ",") {
			normalized = strings.ReplaceAll(normalized, ".", "")
			normalized = strings.ReplaceAll(normalized, ",", ".")
		}
		value, err := strconv.ParseFloat(normalized, 64)
		if err != nil {
			return 0
		}
		return value
	default:
		return 0
	}
}

func monthFromAny(value any) time.Month {
	raw := strings.TrimSpace(fmt.Sprint(value))
	if raw == "" || raw == "<nil>" {
		return 0
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return 0
	}
	return parsed.Month()
}

func maxDateLabel(rows []map[string]any, key string) string {
	var latest time.Time
	for _, row := range rows {
		raw := strings.TrimSpace(fmt.Sprint(row[key]))
		if raw == "" || raw == "<nil>" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			continue
		}
		if parsed.After(latest) {
			latest = parsed
		}
	}

	if latest.IsZero() {
		return "-"
	}
	return latest.Format("02/01/2006")
}

func formatMoneyBRL(value float64) string {
	if value <= 0 {
		return "R$ 0,00"
	}

	cents := int(math.Round(value * 100))
	whole := cents / 100
	decimal := cents % 100
	return fmt.Sprintf("R$ %s,%02d", formatInt(whole), decimal)
}

func countDetail(count int, noun string) string {
	if count <= 0 {
		return ""
	}
	return fmt.Sprintf("%s %s", formatInt(count), noun)
}

func moneyDetail(value float64) string {
	if value <= 0 {
		return ""
	}
	return formatMoneyBRL(value)
}

func quantityValue(value float64) string {
	return formatInt(int(math.Round(value)))
}

func quantityDetail(value float64, noun string) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%s %s", quantityValue(value), noun)
}

func ratioFloat(numerator float64, denominator float64) float64 {
	if numerator <= 0 || denominator <= 0 {
		return 0
	}
	return numerator / denominator
}

func formatPercent(value float64) string {
	if value <= 0 {
		return "0,0%"
	}
	return strings.Replace(fmt.Sprintf("%.1f%%", value*100), ".", ",", 1)
}

func sourceTotalValue(source PerolaSource) string {
	if source.Pending {
		return "Em carga"
	}
	if source.Total <= 0 {
		return "-"
	}
	return formatInt(source.Total)
}

func sourceTotalSentence(source PerolaSource, noun string, pendingFallback string) string {
	if source.Pending {
		return pendingFallback
	}
	if source.Total <= 0 {
		return noun
	}
	return fmt.Sprintf("%s %s", formatInt(source.Total), noun)
}

func sourceTotalDetail(source PerolaSource, noun string, fallback string) string {
	if source.Pending {
		return fallback
	}
	if source.Total <= 0 {
		return fallback
	}
	return fmt.Sprintf("%s %s na base Perola BI", formatInt(source.Total), noun)
}

func inventoryMetricValue(inventario PerolaDataTable) string {
	if inventario.Pending {
		return "..."
	}
	return formatInt(inventario.Fetched)
}

func inventoryMetricDetail(inventario PerolaDataTable, source PerolaSource) string {
	if inventario.Pending {
		return "Carga sob demanda para nao travar a tela"
	}
	if source.Total > 0 {
		return fmt.Sprintf("%s movimentos no historico da Perola BI", formatInt(source.Total))
	}
	return "Base para cobertura e ruptura"
}

func inventoryMetricTone(inventario PerolaDataTable) string {
	if inventario.Pending {
		return "info"
	}
	if inventario.Fetched > 0 {
		return "success"
	}
	return ""
}

func inventoryInsightBody(inventario PerolaDataTable) string {
	if inventario.Pending {
		return "Inventario e a fonte mais pesada da Perola. Para a tela ficar rapida, essa leitura entra em segundo plano e volta assim que a API termina de responder."
	}
	if inventario.Fetched == 0 {
		return "Sem inventario carregado, ainda da para ler venda, CRM e mix. Estoque, cobertura e ruptura ficam dependentes da conclusao dessa fonte."
	}
	return "Com inventario carregado, o painel passa a enxergar correcao por loja, volume amostrado, tipo de inventario e cruzamento pelo itemSaldoId com os itens vendidos."
}

func sourcesMetricDetail(okSources int, pendingSources int, totalSources int) string {
	if pendingSources > 0 {
		return fmt.Sprintf("%d prontas e %d em carga na Perola BI", okSources, pendingSources)
	}
	return fmt.Sprintf("%d fontes respondendo na Perola BI", totalSources)
}

func sourcesMetricTone(okSources int, pendingSources int, totalSources int) string {
	if okSources == totalSources {
		return "success"
	}
	if pendingSources > 0 {
		return "info"
	}
	return "warning"
}

func fallbackString(value string, fallback string) string {
	normalized := strings.TrimSpace(value)
	if normalized != "" {
		return normalized
	}
	return strings.TrimSpace(fallback)
}

func maxInt(values ...int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}
