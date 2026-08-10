package planning

import "testing"

func TestCalculateMonthlyConsultantGoalsScalesGeneratedWeeksToFullMonth(t *testing.T) {
	goals := calculateMonthlyConsultantGoals(125_000, map[string]float64{
		"consultant-a": 20_000,
		"consultant-b": 10_000,
	})
	if len(goals) != 2 {
		t.Fatalf("goals count = %d, want 2", len(goals))
	}
	if goals[0].ConsultantID != "consultant-a" || goals[0].Target != 83_333.33 {
		t.Fatalf("first goal = %#v", goals[0])
	}
	if goals[1].ConsultantID != "consultant-b" || goals[1].Target != 41_666.67 {
		t.Fatalf("second goal = %#v", goals[1])
	}
	if goals[0].Target+goals[1].Target != 125_000 {
		t.Fatalf("monthly total = %.2f, want 125000.00", goals[0].Target+goals[1].Target)
	}
}

func TestScaleGoalAllocationsClosesWeeklyTarget(t *testing.T) {
	t.Parallel()

	source := []GoalAllocation{
		{ConsultantID: "a", Share: 0.333333},
		{ConsultantID: "b", Share: 0.333333},
		{ConsultantID: "c", Share: 0.333334},
	}
	got := scaleGoalAllocations(31_250, source)
	total := 0.0
	for _, allocation := range got {
		total += allocation.Target
	}
	if total != 31_250 {
		t.Fatalf("scaled total = %.2f, want 31250.00; allocations = %#v", total, got)
	}
	if source[0].Target != 0 {
		t.Fatalf("source allocation was mutated: %#v", source)
	}
}

func TestCalculateMonthlyConsultantGoalsKeepsZeroHourConsultant(t *testing.T) {
	goals := calculateMonthlyConsultantGoals(100_000, map[string]float64{
		"consultant-a": 25_000,
		"consultant-b": 0,
	})
	if len(goals) != 2 || goals[0].Target != 100_000 || goals[1].Target != 0 {
		t.Fatalf("unexpected goals: %#v", goals)
	}
}
