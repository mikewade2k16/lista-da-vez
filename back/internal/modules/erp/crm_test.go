package erp

import (
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
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

func TestNormalizeCRMOverviewQueryKeepsEmptyRangeAsAllData(t *testing.T) {
	normalized, err := normalizeCRMOverviewQuery(CRMOverviewQuery{})
	if err != nil {
		t.Fatalf("normalizeCRMOverviewQuery() error = %v", err)
	}
	if !normalized.DateFrom.IsZero() || !normalized.DateTo.IsZero() {
		t.Fatalf("expected empty range for all data, got %s - %s", normalized.DateFrom.Format(time.DateOnly), normalized.DateTo.Format(time.DateOnly))
	}
}

func TestNormalizeCRMOverviewQueryPreservesExplicitTime(t *testing.T) {
	dateFrom := time.Date(2026, 1, 10, 9, 30, 0, 0, time.FixedZone("BRT", -3*60*60))
	dateTo := time.Date(2026, 1, 10, 18, 0, 0, 0, time.FixedZone("BRT", -3*60*60))

	normalized, err := normalizeCRMOverviewQuery(CRMOverviewQuery{
		DateFrom:        dateFrom,
		DateTo:          dateTo,
		DateFromHasTime: true,
		DateToHasTime:   true,
	})
	if err != nil {
		t.Fatalf("normalizeCRMOverviewQuery() error = %v", err)
	}
	if normalized.DateFrom.Hour() != 12 || normalized.DateFrom.Minute() != 30 {
		t.Fatalf("expected UTC-preserved from time, got %s", normalized.DateFrom.Format(time.RFC3339))
	}
	if normalized.DateTo.Hour() != 21 || normalized.DateTo.Minute() != 0 {
		t.Fatalf("expected UTC-preserved to time, got %s", normalized.DateTo.Format(time.RFC3339))
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

func TestERPAdminDetailsRequirePlatformAdmin(t *testing.T) {
	ownerWithERPEdit := auth.Principal{
		Role:                auth.RoleOwner,
		PermissionsResolved: true,
		Permissions:         []string{access.PermissionERPView, access.PermissionERPEdit},
	}
	if canEditERP(ownerWithERPEdit) || canViewERPAdminDetails(ownerWithERPEdit) {
		t.Fatal("owner with ERP edit permission must not access technical ERP admin details")
	}
	if !canViewERP(ownerWithERPEdit) {
		t.Fatal("owner with ERP view permission should still read allowed ERP tables")
	}

	platformAdmin := auth.Principal{Role: auth.RolePlatformAdmin}
	if !canEditERP(platformAdmin) || !canViewERPAdminDetails(platformAdmin) {
		t.Fatal("platform_admin must access ERP admin details")
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

func TestBuildQueueStatsMapsOperationalStoresToCRMSlugs(t *testing.T) {
	stats := buildQueueStats(
		[]crmQueueStoreStat{
			{StoreID: "store-rio", StoreCode: "RIO", StoreName: "Perola Riomar", Attendances: 5, Conversions: 2, QueueCancellations: 1},
			{StoreID: "store-jar", StoreCode: "JAR", StoreName: "Perola Jardins", Attendances: 4, Conversions: 1, QueueCancellations: 0},
		},
		[]crmQueueConsultantStat{
			{PersonID: "consultant-1", PersonName: "Rayane", StoreID: "store-rio", StoreCode: "RIO", StoreName: "Perola Riomar", Attendances: 3, Conversions: 1, QueueCancellations: 1},
			{PersonID: "consultant-1", PersonName: "Rayane", StoreID: "store-rio-2", StoreCode: "PJ-RIO", StoreName: "Perola Riomar", Attendances: 2, Conversions: 1, QueueCancellations: 0},
		},
	)

	if stats.TotalAttendances != 9 || stats.TotalConversions != 3 || stats.TotalCancellations != 1 {
		t.Fatalf("totals = %#v", stats)
	}
	if len(stats.ByStore) != 2 {
		t.Fatalf("ByStore len = %d, want 2", len(stats.ByStore))
	}
	if stats.ByStore[1].StoreSlug != "riomar" {
		t.Fatalf("expected riomar slug in aggregated store stats, got %#v", stats.ByStore)
	}
	if len(stats.ByConsultant) != 1 {
		t.Fatalf("ByConsultant len = %d, want 1", len(stats.ByConsultant))
	}
	consultant := stats.ByConsultant[0]
	if consultant.StoreSlug != "riomar" || consultant.Attendances != 5 || consultant.Conversions != 2 {
		t.Fatalf("aggregated consultant queue stats = %#v", consultant)
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

func TestResolveCRMConsultantLink(t *testing.T) {
	profiles := []crmConsultantLinkProfile{
		{
			ConsultantID:   "consultant-1",
			ConsultantName: "Diana Nicory Gomes",
			StoreID:        "store-garcia",
			StoreCode:      "GAR",
			StoreName:      "Perola Garcia",
			EmployeeCode:   "321",
		},
		{
			ConsultantID:   "consultant-2",
			ConsultantName: "Rayane Tavares Santos Araujo",
			StoreID:        "store-riomar",
			StoreCode:      "RIO",
			StoreName:      "Perola Riomar",
			EmployeeCode:   "231",
		},
	}
	manualLinks := map[string]crmConsultantManualLink{
		crmManualConsultantLinkKey("56173889000163", "231"): {
			ERPEmployeeID: "231",
			ERPStoreCode:  "56173889000163",
			Profile: crmConsultantLinkProfile{
				ConsultantID:   "manual-consultant",
				ConsultantName: "Manual Match",
				StoreID:        "store-manual",
			},
		},
	}

	manual := resolveCRMConsultantLink("56173889000163", "231", "Diana Nicory Gomes", manualLinks, profiles)
	if manual.Status != crmConsultantLinkStatusManual || manual.Profile.ConsultantID != "manual-consultant" {
		t.Fatalf("manual link = %#v, want manual-consultant", manual)
	}

	employeeCode := resolveCRMConsultantLink("53578278000107", "321", "Nome divergente", nil, profiles)
	if employeeCode.Status != crmConsultantLinkStatusEmployeeCode || employeeCode.Profile.ConsultantID != "consultant-1" {
		t.Fatalf("employee code link = %#v, want consultant-1", employeeCode)
	}

	name := resolveCRMConsultantLink("53578278000107", "999", "  diana   nicory-gomes ", nil, profiles)
	if name.Status != crmConsultantLinkStatusNameExact || name.Profile.ConsultantID != "consultant-1" {
		t.Fatalf("name link = %#v, want consultant-1", name)
	}

	unmatched := resolveCRMConsultantLink("", "999", "Sem Cadastro", nil, profiles)
	if unmatched.Status != crmConsultantLinkStatusUnmatched {
		t.Fatalf("unmatched link = %#v, want unmatched", unmatched)
	}
}

func TestResolveCRMConsultantLinkKeepsAutoLinkStatusFromInternalNote(t *testing.T) {
	profiles := []crmConsultantLinkProfile{{
		ConsultantID:   "consultant-1",
		ConsultantName: "Rayane Tavares Santos Araujo",
		StoreID:        "store-riomar",
	}}

	manualLinks := map[string]crmConsultantManualLink{
		crmManualConsultantLinkKey("12583959000186", "231"): {
			ERPEmployeeID: "231",
			ERPStoreCode:  "12583959000186",
			Note:          crmConsultantLinkNoteAutoName,
			Profile:       profiles[0],
		},
	}

	resolved := resolveCRMConsultantLink("12583959000186", "231", "RAYANE TAVARES SANTOS ARAUJO", manualLinks, profiles)
	if resolved.Status != crmConsultantLinkStatusNameExact {
		t.Fatalf("resolved auto note status = %q, want %q", resolved.Status, crmConsultantLinkStatusNameExact)
	}
}

func TestBuildAutoConsultantERPLinkInputs(t *testing.T) {
	profiles := []crmConsultantLinkProfile{
		{
			ConsultantID:   "consultant-1",
			ConsultantName: "Rayane Tavares Santos Araujo",
			StoreID:        "store-riomar",
			StoreCode:      "RIO",
			StoreName:      "Perola Riomar",
			EmployeeCode:   "231",
		},
		{
			ConsultantID:   "consultant-2",
			ConsultantName: "Daniella de Morais Oliveira",
			StoreID:        "store-riomar",
			StoreCode:      "RIO",
			StoreName:      "Perola Riomar",
		},
		{
			ConsultantID:   "consultant-3",
			ConsultantName: "Maria Silva",
			StoreID:        "store-jardins",
			StoreCode:      "JAR",
			StoreName:      "Perola Jardins",
		},
		{
			ConsultantID:   "consultant-4",
			ConsultantName: "Maria Silva",
			StoreID:        "store-riomar-2",
			StoreCode:      "RIO",
			StoreName:      "Perola Riomar 2",
		},
	}

	manualLinks := map[string]crmConsultantManualLink{
		crmManualConsultantLinkKey("12583959000186", "555"): {
			ERPEmployeeID: "555",
			ERPStoreCode:  "12583959000186",
			Profile:       profiles[2],
		},
	}

	employees := []crmERPEmployeeLinkCandidate{
		{
			ERPEmployeeID:   "231",
			ERPEmployeeName: "Nome divergente",
			ERPStoreCode:    "12583959000186",
		},
		{
			ERPEmployeeID:   "999",
			ERPEmployeeName: "Daniella de Morais Oliveira",
			ERPStoreCode:    "12583959000186",
		},
		{
			ERPEmployeeID:   "555",
			ERPEmployeeName: "Maria Silva",
			ERPStoreCode:    "12583959000186",
		},
		{
			ERPEmployeeID:   "888",
			ERPEmployeeName: "Maria Silva",
			ERPStoreCode:    "",
		},
		{
			ERPEmployeeID:   "777",
			ERPEmployeeName: "Sem Match",
			ERPStoreCode:    "12583959000186",
		},
	}

	inputs := buildAutoConsultantERPLinkInputs(manualLinks, profiles, employees)
	if len(inputs) != 2 {
		t.Fatalf("buildAutoConsultantERPLinkInputs() len = %d, want 2", len(inputs))
	}

	if inputs[0].ConsultantID != "consultant-1" || inputs[0].Note != crmConsultantLinkNoteAutoEmployee {
		t.Fatalf("first auto input = %#v, want employee-code link to consultant-1", inputs[0])
	}
	if inputs[1].ConsultantID != "consultant-2" || inputs[1].Note != crmConsultantLinkNoteAutoName {
		t.Fatalf("second auto input = %#v, want exact-name link to consultant-2", inputs[1])
	}
}

func TestResolveCRMConsultantLinkAmbiguousName(t *testing.T) {
	profiles := []crmConsultantLinkProfile{
		{ConsultantID: "consultant-1", ConsultantName: "Maria Silva"},
		{ConsultantID: "consultant-2", ConsultantName: "Maria  Silva"},
	}

	link := resolveCRMConsultantLink("", "", "Maria-Silva", nil, profiles)
	if link.Status != crmConsultantLinkStatusAmbiguous || link.Candidates != 2 {
		t.Fatalf("ambiguous link = %#v, want 2 candidates", link)
	}
}

func TestResolveCRMConsultantLinkUsesStoreForAmbiguousName(t *testing.T) {
	profiles := []crmConsultantLinkProfile{
		{ConsultantID: "consultant-garcia", ConsultantName: "Maria Silva", StoreCode: "GAR", StoreName: "Perola Garcia"},
		{ConsultantID: "consultant-riomar", ConsultantName: "Maria Silva", StoreCode: "RIO", StoreName: "Perola Riomar"},
	}

	link := resolveCRMConsultantLink("53578278000107", "", "Maria-Silva", nil, profiles)
	if link.Status != crmConsultantLinkStatusNameExact || link.Profile.ConsultantID != "consultant-garcia" || link.Candidates != 2 {
		t.Fatalf("store-disambiguated link = %#v, want consultant-garcia with 2 candidates", link)
	}
}
