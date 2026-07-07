package analytics

import (
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, 7, 2, 12, 0, 0, 0, analyticsLocation)
}

func TestHistorySinceMillisDefaultsTo90Days(t *testing.T) {
	now := fixedNow()
	expected := now.AddDate(0, 0, -defaultHistoryWindowDays).UnixMilli()

	if got := historySinceMillis("", now); got != expected {
		t.Fatalf("expected default window %d, got %d", expected, got)
	}
}

func TestHistorySinceMillisRespectsOlderDateFrom(t *testing.T) {
	now := fixedNow()
	older := now.AddDate(0, 0, -200)
	dateFrom := older.Format("2006-01-02")

	expected, err := time.ParseInLocation("2006-01-02", dateFrom, analyticsLocation)
	if err != nil {
		t.Fatalf("parse dateFrom: %v", err)
	}

	if got := historySinceMillis(dateFrom, now); got != expected.UnixMilli() {
		t.Fatalf("expected explicit older dateFrom %d, got %d", expected.UnixMilli(), got)
	}
}

func TestHistorySinceMillisIgnoresRecentDateFrom(t *testing.T) {
	now := fixedNow()
	recent := now.AddDate(0, 0, -10).Format("2006-01-02")
	expected := now.AddDate(0, 0, -defaultHistoryWindowDays).UnixMilli()

	if got := historySinceMillis(recent, now); got != expected {
		t.Fatalf("expected default window %d for recent dateFrom, got %d", expected, got)
	}
}
