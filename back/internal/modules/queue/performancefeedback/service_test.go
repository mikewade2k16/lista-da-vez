package performancefeedback

import (
	"math"
	"strings"
	"testing"
)

func TestNormalizeFeedbackSectionsTrimsAndPreservesOrder(t *testing.T) {
	t.Parallel()

	sections, ok := normalizeFeedbackSections([]FeedbackSection{
		{ID: " strengths ", Title: " Pontos fortes ", ContentHTML: " <p>Bom atendimento</p> "},
		{ID: "next-steps", Title: "Proximos passos", ContentHTML: ""},
	})
	if !ok {
		t.Fatal("normalizeFeedbackSections() rejected valid sections")
	}
	if len(sections) != 2 || sections[0].ID != "strengths" || sections[0].Title != "Pontos fortes" {
		t.Fatalf("normalizeFeedbackSections() = %#v", sections)
	}
	if sections[0].ContentHTML != "<p>Bom atendimento</p>" || sections[1].ID != "next-steps" {
		t.Fatalf("normalizeFeedbackSections() did not preserve normalized content and order: %#v", sections)
	}
}

func TestNormalizeSettingsRequiresCadenceAndAtLeastOneSection(t *testing.T) {
	t.Parallel()

	settings, ok := normalizeSettings(Settings{
		TenantID: " tenant ",
		Cadence:  CadenceWeekly,
		DefaultSections: []FeedbackSection{
			{ID: "summary", Title: "Resumo", ContentHTML: "conteudo nao deve virar template"},
		},
	})
	if !ok {
		t.Fatal("normalizeSettings() rejected valid settings")
	}
	if settings.TenantID != "tenant" || settings.DefaultSections[0].ContentHTML != "" {
		t.Fatalf("normalizeSettings() = %#v", settings)
	}
	if _, ok := normalizeSettings(Settings{TenantID: "tenant", Cadence: "daily"}); ok {
		t.Fatal("normalizeSettings() accepted invalid cadence")
	}
}

func TestNormalizeMetricsSnapshotRejectsInvalidNumbers(t *testing.T) {
	t.Parallel()

	if _, ok := normalizeMetricsSnapshot(Metrics{SoldValue: math.NaN()}); ok {
		t.Fatal("normalizeMetricsSnapshot() accepted NaN")
	}
	if _, ok := normalizeMetricsSnapshot(Metrics{Attendances: -1}); ok {
		t.Fatal("normalizeMetricsSnapshot() accepted negative attendance")
	}
	metrics, ok := normalizeMetricsSnapshot(Metrics{SoldValue: 14725, ERPOrders: 6})
	if !ok || metrics.SoldValue != 14725 || metrics.ERPOrders != 6 {
		t.Fatalf("normalizeMetricsSnapshot() = %#v, %v", metrics, ok)
	}
}

func TestNormalizeFeedbackSectionsRejectsInvalidStructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sections []FeedbackSection
	}{
		{name: "missing title", sections: []FeedbackSection{{ID: "one"}}},
		{name: "duplicate id", sections: []FeedbackSection{{ID: "one", Title: "A"}, {ID: "one", Title: "B"}}},
		{name: "title too long", sections: []FeedbackSection{{ID: "one", Title: strings.Repeat("a", maxSectionTitleLength+1)}}},
		{name: "content too long", sections: []FeedbackSection{{ID: "one", Title: "A", ContentHTML: strings.Repeat("a", maxRichTextLength+1)}}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, ok := normalizeFeedbackSections(test.sections); ok {
				t.Fatalf("normalizeFeedbackSections() accepted %s", test.name)
			}
		})
	}
}
