package bi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPerolaDatasetRegistryContainsSixSafeDatasets(t *testing.T) {
	catalog := perolaDatasetCatalog()
	if len(catalog.Datasets) != 6 {
		t.Fatalf("expected six datasets, got %d", len(catalog.Datasets))
	}

	expected := map[string]bool{
		perolaDatasetItemID:          true,
		perolaDatasetItemImageID:     true,
		perolaDatasetPurchasePriceID: true,
		perolaDatasetInvoiceID:       true,
		perolaDatasetInvoiceItemID:   true,
		perolaDatasetInventoryID:     true,
	}
	for _, dataset := range catalog.Datasets {
		if !expected[dataset.ID] {
			t.Errorf("unexpected dataset %q", dataset.ID)
		}
		if dataset.MaxLimit <= 0 || dataset.DefaultLimit > dataset.MaxLimit {
			t.Errorf("invalid limits for dataset %q", dataset.ID)
		}
		if len(dataset.RequiredFilterAlternatives) == 0 || dataset.RequiredFilterRule == "" {
			t.Errorf("dataset %q does not expose its safe query rule", dataset.ID)
		}
		delete(expected, dataset.ID)
	}
	if len(expected) != 0 {
		t.Fatalf("missing datasets: %v", expected)
	}

	encoded, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "/find") || strings.Contains(string(encoded), `"endpoint"`) {
		t.Fatalf("public catalog leaked upstream endpoint: %s", encoded)
	}

	invoice, ok := findCatalogDataset(catalog.Datasets, perolaDatasetInvoiceID)
	if !ok || invoice.DateRange == nil || invoice.DateRange.Field != "dataEmissao" || invoice.DateRange.MaxDays != 31 {
		t.Fatalf("invoice date range not exposed safely: %+v", invoice.DateRange)
	}
}

func findCatalogDataset(
	datasets []PerolaDatasetCatalogItem,
	datasetID string,
) (PerolaDatasetCatalogItem, bool) {
	for _, dataset := range datasets {
		if dataset.ID == datasetID {
			return dataset, true
		}
	}
	return PerolaDatasetCatalogItem{}, false
}

func TestNormalizePerolaDatasetQueryClampsLimitAndBuildsAllowlistedPayload(t *testing.T) {
	spec, ok := findPerolaDatasetSpec(perolaDatasetInvoiceID)
	if !ok {
		t.Fatal("invoice dataset not found")
	}

	query, err := normalizePerolaDatasetQuery(spec, PerolaDatasetQueryInput{
		PageNumber: 2,
		Limit:      500,
		OrderBy:    PerolaDatasetOrderInput{Field: "dataEmissao", Direction: "asc"},
		Filters: []PerolaDatasetFilterInput{
			{Field: "dataEmissao", Operator: "gte", Value: "2026-07-01"},
			{Field: "dataEmissao", Operator: "lte", Value: "2026-07-31"},
			{Field: "empresaId", Operator: "eq", Value: float64(7)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if query.Limit != 25 {
		t.Fatalf("expected limit capped at 25, got %d", query.Limit)
	}
	if query.OrderBy.Direction != "ASC" {
		t.Fatalf("expected normalized ASC direction, got %q", query.OrderBy.Direction)
	}

	var payload perolaPagePayload
	if err := json.Unmarshal(query.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.PageNumber != 2 || payload.Limit != 25 {
		t.Fatalf("unexpected page payload: %+v", payload)
	}
	if payload.Conditions["greaterMoreThan"]["dataEmissao"] != "2026-07-01" {
		t.Fatalf("missing lower date boundary: %+v", payload.Conditions)
	}
	if payload.Conditions["lessMoreThan"]["dataEmissao"] != "2026-07-31" {
		t.Fatalf("missing upper date boundary: %+v", payload.Conditions)
	}
}

func TestNormalizePerolaDatasetQueryRequiresSelectiveInventoryFilter(t *testing.T) {
	spec, ok := findPerolaDatasetSpec(perolaDatasetInventoryID)
	if !ok {
		t.Fatal("inventory dataset not found")
	}

	_, err := normalizePerolaDatasetQuery(spec, PerolaDatasetQueryInput{
		Filters: []PerolaDatasetFilterInput{{Field: "id", Operator: "eq", Value: 1}},
	})
	if !errors.Is(err, ErrFilterRequired) {
		t.Fatalf("expected required itemSaldoId error, got %v", err)
	}

	query, err := normalizePerolaDatasetQuery(spec, PerolaDatasetQueryInput{
		Filters: []PerolaDatasetFilterInput{{Field: "itemSaldoId", Operator: "eq", Value: 123}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.Limit != 10 {
		t.Fatalf("expected inventory default limit 10, got %d", query.Limit)
	}

	_, err = normalizePerolaDatasetQuery(spec, PerolaDatasetQueryInput{
		Filters: []PerolaDatasetFilterInput{{Field: "itemSaldoId", Operator: "eq", Value: 0}},
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected zero itemSaldoId to be rejected, got %v", err)
	}
}

func TestNormalizePerolaDatasetQueryRequiresBoundedInvoicePeriod(t *testing.T) {
	spec, ok := findPerolaDatasetSpec(perolaDatasetInvoiceID)
	if !ok {
		t.Fatal("invoice dataset not found")
	}

	testCases := []struct {
		name    string
		filters []PerolaDatasetFilterInput
	}{
		{
			name: "open ended",
			filters: []PerolaDatasetFilterInput{
				{Field: "dataEmissao", Operator: "gte", Value: "2026-07-01"},
			},
		},
		{
			name: "more than 31 days",
			filters: []PerolaDatasetFilterInput{
				{Field: "dataEmissao", Operator: "gte", Value: "2026-06-01"},
				{Field: "dataEmissao", Operator: "lte", Value: "2026-07-31"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := normalizePerolaDatasetQuery(spec, PerolaDatasetQueryInput{Filters: testCase.filters})
			if err == nil {
				t.Fatal("expected date range validation error")
			}
		})
	}
}

func TestNormalizePerolaDatasetQueryRejectsUnknownFieldsAndOperators(t *testing.T) {
	spec, ok := findPerolaDatasetSpec(perolaDatasetItemID)
	if !ok {
		t.Fatal("item dataset not found")
	}

	_, err := normalizePerolaDatasetQuery(spec, PerolaDatasetQueryInput{
		OrderBy: PerolaDatasetOrderInput{Field: "dropTable", Direction: "DESC"},
		Filters: []PerolaDatasetFilterInput{{Field: "itemId", Operator: "eq", Value: 1}},
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected invalid order field, got %v", err)
	}

	_, err = normalizePerolaDatasetQuery(spec, PerolaDatasetQueryInput{
		Filters: []PerolaDatasetFilterInput{{Field: "rawBody", Operator: "contains", Value: "x"}},
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected invalid filter field, got %v", err)
	}
}

func TestQueryPerolaDatasetReturnsOnlyStructuredPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/sessoes":
			_, _ = w.Write([]byte(`{"token":"aaa.bbb.ccc"}`))
		case "/nota/find":
			var payload perolaPagePayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode query: %v", err)
			}
			if payload.Limit != 25 {
				t.Errorf("expected server-side cap 25, got %d", payload.Limit)
			}
			_, _ = w.Write([]byte(`{
				"paginacao":{"totalRegistros":40,"totalPaginas":2},
				"registros":[{"id":9,"numDocumento":" 123 ","nested":{"secret":"omitted"},"nullable":null}]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	service := NewService()
	service.perola = newPerolaClient(perolaClientOptions{
		BaseURL: server.URL,
		Credentials: perolaCredentials{
			CompanyKey:  "company",
			CNPJEmpresa: "12345678000199",
			Login:       "user",
			Pass:        "pass",
		},
		TokenTTL:       time.Minute,
		RequestTimeout: time.Second,
	})

	response, err := service.QueryPerolaDataset(context.Background(), perolaDatasetInvoiceID, PerolaDatasetQueryInput{
		Limit:   200,
		Filters: []PerolaDatasetFilterInput{{Field: "id", Operator: "eq", Value: 9}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if response.TotalRecords != 40 || response.TotalPages != 2 || !response.HasMore {
		t.Fatalf("unexpected pagination: %+v", response)
	}
	if len(response.Records) != 1 || response.Records[0]["numDocumento"] != "123" {
		t.Fatalf("unexpected records: %+v", response.Records)
	}
	if _, leaked := response.Records[0]["nested"]; leaked {
		t.Fatal("nested raw upstream data must not be exposed")
	}
	if value, exists := response.Records[0]["nullable"]; !exists || value != nil {
		t.Fatalf("expected null to be preserved, got exists=%v value=%v", exists, value)
	}
}

func TestOverviewDatasetDefinitionsOnlyOptInInventory(t *testing.T) {
	definitions := perolaDatasetDefinitions()
	active := overviewDatasetDefinitions(definitions, true)

	for _, definition := range active {
		if !definition.IncludeInOverview && definition.Key != "inventario" {
			t.Fatalf("unexpected background dataset %q", definition.Key)
		}
	}
	if len(active) != 4 {
		t.Fatalf("expected 3 overview datasets plus inventory, got %d", len(active))
	}
}
