package planning

import (
	"testing"
	"time"
)

func TestNormalizeGoalPeriodUsesTheCommercialWeekCalendar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		month     string
		week      int
		weekStart string
	}{
		{name: "month starting on Wednesday", month: "2026-07", week: 1, weekStart: "2026-06-29"},
		{name: "month starting on Saturday", month: "2026-08", week: 1, weekStart: "2026-07-27"},
		{name: "second August period", month: "2026-08", week: 2, weekStart: "2026-08-03"},
		{name: "fourth August period", month: "2026-08", week: 4, weekStart: "2026-08-17"},
		{name: "fifth July period", month: "2026-07", week: 5, weekStart: "2026-07-27"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			weekStart, err := time.Parse("2006-01-02", test.weekStart)
			if err != nil {
				t.Fatal(err)
			}
			month, week, err := normalizeGoalPeriod(weekStart, test.month, test.week)
			if err != nil {
				t.Fatalf("normalizeGoalPeriod() error = %v", err)
			}
			if got := month.Format("2006-01"); got != test.month || week != test.week {
				t.Fatalf("normalizeGoalPeriod() = %s, %d; want %s, %d", got, week, test.month, test.week)
			}
		})
	}
}

func TestNormalizeGoalPeriodRejectsAWeekFromAnotherPeriod(t *testing.T) {
	t.Parallel()

	weekStart, err := time.Parse("2006-01-02", "2026-07-27")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = normalizeGoalPeriod(weekStart, "2026-07", 4); err != ErrValidation {
		t.Fatalf("normalizeGoalPeriod() error = %v, want ErrValidation", err)
	}
}
