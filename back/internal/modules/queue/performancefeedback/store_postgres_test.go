package performancefeedback

import (
	"encoding/json"
	"testing"
)

func TestExtractTranscriptionScoreNormalizesKnownScales(t *testing.T) {
	t.Parallel()

	tests := []struct {
		report string
		want   float64
	}{
		{report: `{"overallScore":0.84}`, want: 8.4},
		{report: `{"score":8.7}`, want: 8.7},
		{report: `{"evaluation":{"nota":92}}`, want: 9.2},
	}

	for _, test := range tests {
		got, ok := extractTranscriptionScore(json.RawMessage(test.report))
		if !ok || got != test.want {
			t.Fatalf("extractTranscriptionScore(%s) = %v, %v; want %v, true", test.report, got, ok, test.want)
		}
	}
}

func TestExtractTranscriptionScoreIgnoresUnknownReport(t *testing.T) {
	t.Parallel()

	if _, ok := extractTranscriptionScore(json.RawMessage(`{"summary":"sem nota"}`)); ok {
		t.Fatal("extractTranscriptionScore() accepted report without score")
	}
}
