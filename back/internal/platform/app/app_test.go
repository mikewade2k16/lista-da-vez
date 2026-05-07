package app

import (
	"testing"
	"time"
)

func TestNextERPScheduledRunUsesDailyHourForDailyIntervals(t *testing.T) {
	now := time.Date(2026, 5, 5, 6, 30, 0, 0, time.UTC)
	next := nextERPScheduledRun(now, 24*time.Hour, 4)
	expected := time.Date(2026, 5, 6, 4, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}
}

func TestNextERPScheduledRunUsesIntervalBelowOneDay(t *testing.T) {
	now := time.Date(2026, 5, 5, 6, 30, 0, 0, time.UTC)
	next := nextERPScheduledRun(now, 6*time.Hour, 4)
	expected := time.Date(2026, 5, 5, 12, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}
}
