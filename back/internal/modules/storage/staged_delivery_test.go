package storage

import "testing"

func TestParseStagedRange(t *testing.T) {
	tests := []struct {
		name, value      string
		size, start, end int64
	}{
		{name: "complete tail", value: "bytes=10-19", size: 100, start: 10, end: 19},
		{name: "open end", value: "bytes=90-", size: 100, start: 90, end: 99},
		{name: "suffix", value: "bytes=-10", size: 100, start: 90, end: 99},
		{name: "clamped", value: "bytes=90-200", size: 100, start: 90, end: 99},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end, err := parseStagedRange(test.value, test.size)
			if err != nil {
				t.Fatalf("parse range: %v", err)
			}
			if start != test.start || end != test.end {
				t.Fatalf("got %d-%d, want %d-%d", start, end, test.start, test.end)
			}
		})
	}
}

func TestParseStagedRangeRejectsInvalid(t *testing.T) {
	for _, value := range []string{"bytes=-0", "bytes=100-", "bytes=20-10", "other=0-1"} {
		if _, _, err := parseStagedRange(value, 100); err == nil {
			t.Fatalf("expected %q to fail", value)
		}
	}
}
