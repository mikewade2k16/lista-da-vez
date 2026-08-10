package goalperiod

import (
	"testing"
	"time"
)

func TestCount(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		month string
		want  int
	}{{"2026-02", 4}, {"2028-02", 5}, {"2026-07", 5}} {
		month, err := time.Parse("2006-01", test.month)
		if err != nil {
			t.Fatal(err)
		}
		if got := Count(month); got != test.want {
			t.Fatalf("Count(%s) = %d, want %d", test.month, got, test.want)
		}
	}
}
