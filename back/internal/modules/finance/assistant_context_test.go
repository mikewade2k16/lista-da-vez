package finance

import (
	"strings"
	"testing"
)

func TestAssistantContextBoundsKeepStoredLinesAndAmounts(t *testing.T) {
	rows := make([]Line, 60)
	for i := range rows {
		rows[i] = Line{Description: strings.Repeat("á", 600), Details: strings.Repeat("x", 2000), Amount: 100, AdjustmentAmount: 24, Adjustments: make([]Adjustment, 12)}
		for j := range rows[i].Adjustments {
			rows[i].Adjustments[j] = Adjustment{Amount: 2, Note: strings.Repeat("n", 600)}
		}
	}
	got := assistantLines(rows)
	if len(got) != 50 || len(got[0].Adjustments) != 10 || len([]rune(got[0].Description)) != 501 || got[0].AdjustmentAmount != 24 || got[0].Amount != 100 {
		t.Fatal("context limit or authoritative amount changed")
	}
	if len(rows) != 60 || len(rows[0].Adjustments) != 12 || len(rows[0].Adjustments[0].Note) != 600 || len(rows[0].Details) != 2000 {
		t.Fatal("stored lines mutated while creating context")
	}
}
