package performancefeedback

import (
	"testing"
	"time"
)

func TestResolvePeriodUsesFixedMonthWeeks(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	tests := []struct {
		week     int
		dateFrom string
		dateTo   string
	}{
		{week: 0, dateFrom: "2026-02-01", dateTo: "2026-02-28"},
		{week: 1, dateFrom: "2026-02-01", dateTo: "2026-02-07"},
		{week: 2, dateFrom: "2026-02-08", dateTo: "2026-02-14"},
		{week: 3, dateFrom: "2026-02-15", dateTo: "2026-02-21"},
		{week: 4, dateFrom: "2026-02-22", dateTo: "2026-02-28"},
	}

	for _, test := range tests {
		period, err := resolvePeriod("2026-02", test.week, now)
		if err != nil {
			t.Fatalf("resolvePeriod(week=%d): %v", test.week, err)
		}
		if period.DateFrom != test.dateFrom || period.DateTo != test.dateTo {
			t.Fatalf("resolvePeriod(week=%d) = %s..%s, want %s..%s", test.week, period.DateFrom, period.DateTo, test.dateFrom, test.dateTo)
		}
	}
}

func TestResolvePeriodRejectsInvalidWeek(t *testing.T) {
	t.Parallel()

	if _, err := resolvePeriod("2026-08", 6, time.Now()); err != ErrValidation {
		t.Fatalf("resolvePeriod() error = %v, want ErrValidation", err)
	}
}
