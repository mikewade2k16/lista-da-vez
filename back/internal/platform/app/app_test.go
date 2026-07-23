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

func TestMissedERPScheduledRunReturnsTodayWhenDailyHourAlreadyPassed(t *testing.T) {
	now := time.Date(2026, 5, 5, 6, 30, 0, 0, time.UTC)
	missedFor, ok := missedERPScheduledRun(now, 24*time.Hour, 4)
	expected := time.Date(2026, 5, 5, 4, 0, 0, 0, time.UTC)
	if !ok {
		t.Fatal("expected missed scheduled run")
	}
	if !missedFor.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, missedFor)
	}
}

func TestMissedERPScheduledRunIgnoresFutureDailyHour(t *testing.T) {
	now := time.Date(2026, 5, 5, 3, 30, 0, 0, time.UTC)
	if missedFor, ok := missedERPScheduledRun(now, 24*time.Hour, 4); ok {
		t.Fatalf("expected no missed run, got %v", missedFor)
	}
}

func TestMissedERPScheduledRunIgnoresShortIntervals(t *testing.T) {
	now := time.Date(2026, 5, 5, 6, 30, 0, 0, time.UTC)
	if missedFor, ok := missedERPScheduledRun(now, 6*time.Hour, 4); ok {
		t.Fatalf("expected no missed run for short interval, got %v", missedFor)
	}
}

func TestModuleGatingRulesIncludeSocialPublishing(t *testing.T) {
	t.Parallel()

	for _, rule := range moduleGatingRules() {
		if rule.Prefix == "/v1/social-publishing" {
			if rule.ModuleID != "social_publishing" {
				t.Fatalf("unexpected module id %q", rule.ModuleID)
			}
			return
		}
	}
	t.Fatal("social publishing API must be protected by the module gate")
}
