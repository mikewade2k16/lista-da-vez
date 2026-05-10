package erp

import (
	"testing"
	"time"
)

func TestResolveCRMStoreAlias(t *testing.T) {
	tests := []struct {
		cnpj         string
		expectSlug   string
		expectLabel  string
		expectMapped bool
	}{
		{cnpj: crmStoreKeyManagementMultiStore, expectSlug: crmStoreKeyManagementMultiStore, expectLabel: "Gerencia / Multi-loja", expectMapped: true},
		{cnpj: "12583959000186", expectSlug: "riomar", expectLabel: "Riomar", expectMapped: true},
		{cnpj: "56173889000163", expectSlug: "jardins", expectLabel: "Jardins", expectMapped: true},
		{cnpj: "43068099000257", expectSlug: "treze", expectLabel: "Treze", expectMapped: true},
		{cnpj: "99999999999999", expectMapped: false},
	}

	for _, test := range tests {
		alias, ok := resolveCRMStoreAlias(test.cnpj)
		if ok != test.expectMapped {
			t.Fatalf("resolveCRMStoreAlias(%q) mapped = %v, want %v", test.cnpj, ok, test.expectMapped)
		}
		if !test.expectMapped {
			continue
		}
		if alias.Slug != test.expectSlug || alias.Label != test.expectLabel {
			t.Fatalf("resolveCRMStoreAlias(%q) = %#v, want slug=%q label=%q", test.cnpj, alias, test.expectSlug, test.expectLabel)
		}
	}
}

func TestNormalizeCRMOverviewQueryDefaultsCurrentMonth(t *testing.T) {
	normalized, err := normalizeCRMOverviewQuery(CRMOverviewQuery{})
	if err != nil {
		t.Fatalf("normalizeCRMOverviewQuery() error = %v", err)
	}
	if normalized.DateFrom.IsZero() || normalized.DateTo.IsZero() {
		t.Fatal("expected default current-month range")
	}
	if normalized.DateFrom.Day() != 1 {
		t.Fatalf("expected first day of month, got %s", normalized.DateFrom.Format(time.DateOnly))
	}
	if normalized.DateTo.Before(normalized.DateFrom) {
		t.Fatalf("expected dateTo >= dateFrom, got %s < %s", normalized.DateTo.Format(time.DateOnly), normalized.DateFrom.Format(time.DateOnly))
	}
	if normalized.DateFrom.Location() != time.UTC || normalized.DateTo.Location() != time.UTC {
		t.Fatal("expected UTC-normalized dates")
	}
}

func TestBuildCRMMetricValues(t *testing.T) {
	ticketAverage, valuePerProduct, paScore := buildCRMMetricValues(4, 10, 200000, 150000)
	if ticketAverage != 50000 {
		t.Fatalf("ticketAverage = %d, want 50000", ticketAverage)
	}
	if valuePerProduct != 15000 {
		t.Fatalf("valuePerProduct = %d, want 15000", valuePerProduct)
	}
	if paScore != 2.5 {
		t.Fatalf("paScore = %v, want 2.5", paScore)
	}
}

func TestCRMStoreKeyFromOperationalStore(t *testing.T) {
	tests := []struct {
		code      string
		name      string
		expectKey string
	}{
		{code: "RIO", name: "Perola Riomar", expectKey: "12583959000186"},
		{code: "JAR", name: "Perola Jardins", expectKey: "56173889000163"},
		{code: "PJ-GARCIA", name: "Perola Garcia", expectKey: "53578278000107"},
		{code: "TRE", name: "Perola Treze", expectKey: "43068099000176"},
		{code: "", name: "Loja sem mapeamento", expectKey: ""},
	}

	for _, test := range tests {
		if got := crmStoreKeyFromOperationalStore(test.code, test.name); got != test.expectKey {
			t.Fatalf("crmStoreKeyFromOperationalStore(%q, %q) = %q, want %q", test.code, test.name, got, test.expectKey)
		}
	}
}

func TestResolveCRMOrderStoreKey(t *testing.T) {
	employeeStoreFallbacks := map[string]string{
		"259": "53578278000107",
		"888": "53578278000107",
	}
	employeeDominantStoreKeys := map[string]string{
		"301": "56173889000163",
		"888": "56173889000163",
	}

	if got := resolveCRMOrderStoreKey("43068099000176", "12583959000186", "259", employeeStoreFallbacks, employeeDominantStoreKeys); got != "43068099000176" {
		t.Fatalf("expected explicit store key to win, got %q", got)
	}

	if got := resolveCRMOrderStoreKey("", "12583959000186", "16", employeeStoreFallbacks, employeeDominantStoreKeys); got != crmStoreKeyManagementMultiStore {
		t.Fatalf("expected management multi-store key for employee 16, got %q", got)
	}

	if got := resolveCRMOrderStoreKey("", "12583959000186", "888", employeeStoreFallbacks, employeeDominantStoreKeys); got != "53578278000107" {
		t.Fatalf("expected current internal fallback to win over dominant ERP store, got %q", got)
	}

	if got := resolveCRMOrderStoreKey("", "12583959000186", "259", employeeStoreFallbacks, employeeDominantStoreKeys); got != "53578278000107" {
		t.Fatalf("expected employee fallback key, got %q", got)
	}

	if got := resolveCRMOrderStoreKey("", "12583959000186", "301", employeeStoreFallbacks, employeeDominantStoreKeys); got != "56173889000163" {
		t.Fatalf("expected dominant ERP store key, got %q", got)
	}

	if got := resolveCRMOrderStoreKey("", "12583959000186", "sem-mapeamento", employeeStoreFallbacks, employeeDominantStoreKeys); got != "12583959000186" {
		t.Fatalf("expected fallback store CNPJ, got %q", got)
	}

	if got := resolveCRMOrderStoreKey("", "", "sem-mapeamento", employeeStoreFallbacks, employeeDominantStoreKeys); got != "" {
		t.Fatalf("expected empty store key, got %q", got)
	}
}
