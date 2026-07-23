package bi

import (
	"slices"
	"strings"
	"time"
)

const (
	perolaDatasetItemID             = "item"
	perolaDatasetItemImageID        = "imagem-item"
	perolaDatasetPurchasePriceID    = "item-saldo-preco-compra"
	perolaDatasetInvoiceID          = "nota"
	perolaDatasetInvoiceItemID      = "nota-item"
	perolaDatasetInventoryID        = "inventario"
	perolaDatasetDefaultQueryLimit  = 25
	perolaDatasetMaxFilters         = 8
	perolaDatasetMaxPageNumber      = 100_000
	perolaDatasetMaxStringFilterLen = 200
)

type perolaFilterValueType string

const (
	perolaFilterString  perolaFilterValueType = "string"
	perolaFilterInteger perolaFilterValueType = "integer"
	perolaFilterDate    perolaFilterValueType = "date"
	perolaFilterBoolean perolaFilterValueType = "boolean"
)

type perolaDatasetFilterRule struct {
	ValueType perolaFilterValueType
	Operators []string
}

type perolaFilterSelector struct {
	Field    string
	Operator string
}

type perolaDateRangeRule struct {
	Field   string
	MaxDays int
}

type perolaDatasetSpec struct {
	ID                    string
	Label                 string
	Description           string
	Endpoint              string
	DefaultLimit          int
	MaxLimit              int
	DefaultOrderField     string
	DefaultOrderDirection string
	AllowedOrderFields    []string
	Filters               map[string]perolaDatasetFilterRule
	RequiredAlternatives  [][]perolaFilterSelector
	RequiredFilterRule    string
	DateRange             *perolaDateRangeRule
	RequestTimeout        time.Duration
}

func perolaDatasetSpecs() []perolaDatasetSpec {
	return []perolaDatasetSpec{
		{
			ID:                    perolaDatasetItemID,
			Label:                 "Item",
			Description:           "Cadastro e classificacao comercial dos produtos.",
			Endpoint:              "/item/find",
			DefaultLimit:          25,
			MaxLimit:              50,
			DefaultOrderField:     "modified",
			DefaultOrderDirection: "DESC",
			AllowedOrderFields:    []string{"id", "itemId", "created", "modified", "referencia"},
			Filters: map[string]perolaDatasetFilterRule{
				"id":             integerFilter("eq"),
				"itemId":         integerFilter("eq"),
				"referencia":     stringFilter("eq", "startsWith"),
				"marcaId":        integerFilter("eq"),
				"departamentoId": integerFilter("eq"),
				"tipoId":         integerFilter("eq"),
				"subTipoId":      integerFilter("eq"),
				"classeId":       integerFilter("eq"),
				"corId":          integerFilter("eq"),
				"modified":       dateFilter("gte", "lte"),
			},
			RequiredAlternatives: [][]perolaFilterSelector{
				selectorSet("id", "eq"),
				selectorSet("itemId", "eq"),
				selectorSet("referencia", "eq"),
				selectorSet("marcaId", "eq"),
				selectorSet("departamentoId", "eq"),
				selectorSet("tipoId", "eq"),
				selectorSet("modified", "gte", "modified", "lte"),
			},
			RequiredFilterRule: "Informe um identificador/classificacao exata ou um periodo fechado de modified.",
		},
		{
			ID:                    perolaDatasetItemImageID,
			Label:                 "Imagem do item",
			Description:           "Metadados de imagem associados ao Item.",
			Endpoint:              "/imagemItem/find",
			DefaultLimit:          25,
			MaxLimit:              100,
			DefaultOrderField:     "id",
			DefaultOrderDirection: "DESC",
			AllowedOrderFields:    []string{"id", "itemId", "ordem"},
			Filters: map[string]perolaDatasetFilterRule{
				"id":     integerFilter("eq"),
				"itemId": integerFilter("eq"),
			},
			RequiredAlternatives: [][]perolaFilterSelector{
				selectorSet("id", "eq"),
				selectorSet("itemId", "eq"),
			},
			RequiredFilterRule: "Informe id ou itemId.",
		},
		{
			ID:                    perolaDatasetPurchasePriceID,
			Label:                 "Saldo e preco de compra",
			Description:           "Historico de custo, entrada e preco medio por saldo do item.",
			Endpoint:              "/itemSaldoPrecoCompra/find",
			DefaultLimit:          25,
			MaxLimit:              50,
			DefaultOrderField:     "data",
			DefaultOrderDirection: "DESC",
			AllowedOrderFields:    []string{"id", "itemSaldoId", "data"},
			Filters: map[string]perolaDatasetFilterRule{
				"id":          integerFilter("eq"),
				"itemSaldoId": integerFilter("eq"),
				"empresaId":   integerFilter("eq"),
				"data":        dateFilter("gte", "lte"),
			},
			RequiredAlternatives: [][]perolaFilterSelector{
				selectorSet("id", "eq"),
				selectorSet("itemSaldoId", "eq"),
				selectorSet("empresaId", "eq", "data", "gte", "data", "lte"),
			},
			RequiredFilterRule: "Informe id, itemSaldoId ou empresaId com periodo fechado de data.",
			DateRange:          &perolaDateRangeRule{Field: "data", MaxDays: 31},
		},
		{
			ID:                    perolaDatasetInvoiceID,
			Label:                 "Nota",
			Description:           "Cabecalho fiscal, empresa, colaborador, cliente, impostos e totais.",
			Endpoint:              "/nota/find",
			DefaultLimit:          15,
			MaxLimit:              25,
			DefaultOrderField:     "dataEmissao",
			DefaultOrderDirection: "DESC",
			AllowedOrderFields:    []string{"id", "dataEmissao", "numDocumento", "valorTotal"},
			Filters: map[string]perolaDatasetFilterRule{
				"id":            integerFilter("eq"),
				"numDocumento":  stringFilter("eq"),
				"empresaId":     integerFilter("eq"),
				"colaboradorId": integerFilter("eq"),
				"dataEmissao":   dateFilter("gte", "lte"),
				"tipoNota":      stringFilter("eq"),
				"excluido":      booleanFilter("eq"),
			},
			RequiredAlternatives: [][]perolaFilterSelector{
				selectorSet("id", "eq"),
				selectorSet("numDocumento", "eq"),
				selectorSet("dataEmissao", "gte", "dataEmissao", "lte"),
			},
			RequiredFilterRule: "Informe id, numDocumento ou periodo fechado de dataEmissao (maximo 31 dias).",
			DateRange:          &perolaDateRangeRule{Field: "dataEmissao", MaxDays: 31},
			RequestTimeout:     15 * time.Second,
		},
		{
			ID:                    perolaDatasetInvoiceItemID,
			Label:                 "Item da nota",
			Description:           "Linhas fiscais vendidas ou devolvidas, com quantidade, preco e custo.",
			Endpoint:              "/notaItem/find",
			DefaultLimit:          25,
			MaxLimit:              50,
			DefaultOrderField:     "id",
			DefaultOrderDirection: "DESC",
			AllowedOrderFields:    []string{"id", "notaId", "itemSaldoId"},
			Filters: map[string]perolaDatasetFilterRule{
				"id":            integerFilter("eq"),
				"notaId":        integerFilter("eq"),
				"itemSaldoId":   integerFilter("eq"),
				"colaboradorId": integerFilter("eq"),
				"excluido":      booleanFilter("eq"),
			},
			RequiredAlternatives: [][]perolaFilterSelector{
				selectorSet("id", "eq"),
				selectorSet("notaId", "eq"),
				selectorSet("itemSaldoId", "eq"),
			},
			RequiredFilterRule: "Informe id, notaId ou itemSaldoId.",
		},
		{
			ID:                    perolaDatasetInventoryID,
			Label:                 "Inventario",
			Description:           "Movimentos e ajustes de quantidade por saldo do item.",
			Endpoint:              "/inventario/find",
			DefaultLimit:          10,
			MaxLimit:              25,
			DefaultOrderField:     "data",
			DefaultOrderDirection: "DESC",
			AllowedOrderFields:    []string{"id", "itemSaldoId", "data"},
			Filters: map[string]perolaDatasetFilterRule{
				"id":                  integerFilter("eq"),
				"itemSaldoId":         integerFilter("eq"),
				"empresaId":           integerFilter("eq"),
				"data":                dateFilter("gte", "lte"),
				"tipoInventarioSigla": stringFilter("eq"),
			},
			RequiredAlternatives: [][]perolaFilterSelector{
				selectorSet("itemSaldoId", "eq"),
			},
			RequiredFilterRule: "itemSaldoId exato e obrigatorio; consulta aberta e proibida.",
			DateRange:          &perolaDateRangeRule{Field: "data", MaxDays: 31},
			RequestTimeout:     30 * time.Second,
		},
	}
}

func findPerolaDatasetSpec(datasetID string) (perolaDatasetSpec, bool) {
	resolvedID := strings.ToLower(strings.TrimSpace(datasetID))
	for _, spec := range perolaDatasetSpecs() {
		if spec.ID == resolvedID {
			return spec, true
		}
	}
	return perolaDatasetSpec{}, false
}

func perolaDatasetCatalog() PerolaDatasetCatalogResponse {
	specs := perolaDatasetSpecs()
	items := make([]PerolaDatasetCatalogItem, 0, len(specs))
	for _, spec := range specs {
		filterFields := make([]string, 0, len(spec.Filters))
		for field := range spec.Filters {
			filterFields = append(filterFields, field)
		}
		slices.Sort(filterFields)

		filters := make([]PerolaDatasetFilterCatalog, 0, len(filterFields))
		for _, field := range filterFields {
			rule := spec.Filters[field]
			filters = append(filters, PerolaDatasetFilterCatalog{
				Field:     field,
				ValueType: string(rule.ValueType),
				Operators: slices.Clone(rule.Operators),
			})
		}

		items = append(items, PerolaDatasetCatalogItem{
			ID:                 spec.ID,
			Label:              spec.Label,
			Description:        spec.Description,
			Endpoint:           spec.Endpoint,
			DefaultLimit:       spec.DefaultLimit,
			MaxLimit:           spec.MaxLimit,
			DefaultOrderBy:     PerolaDatasetOrderInput{Field: spec.DefaultOrderField, Direction: spec.DefaultOrderDirection},
			AllowedOrderFields: slices.Clone(spec.AllowedOrderFields),
			Filters:            filters,
			RequiredFilterRule: spec.RequiredFilterRule,
		})
	}
	return PerolaDatasetCatalogResponse{Datasets: items}
}

func stringFilter(operators ...string) perolaDatasetFilterRule {
	return perolaDatasetFilterRule{ValueType: perolaFilterString, Operators: operators}
}

func integerFilter(operators ...string) perolaDatasetFilterRule {
	return perolaDatasetFilterRule{ValueType: perolaFilterInteger, Operators: operators}
}

func dateFilter(operators ...string) perolaDatasetFilterRule {
	return perolaDatasetFilterRule{ValueType: perolaFilterDate, Operators: operators}
}

func booleanFilter(operators ...string) perolaDatasetFilterRule {
	return perolaDatasetFilterRule{ValueType: perolaFilterBoolean, Operators: operators}
}

func selectorSet(values ...string) []perolaFilterSelector {
	selectors := make([]perolaFilterSelector, 0, len(values)/2)
	for index := 0; index+1 < len(values); index += 2 {
		selectors = append(selectors, perolaFilterSelector{Field: values[index], Operator: values[index+1]})
	}
	return selectors
}
