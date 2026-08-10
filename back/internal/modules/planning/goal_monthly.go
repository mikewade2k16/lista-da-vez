package planning

import (
	"math"
	"sort"
)

type consultantGoalTarget struct {
	ConsultantID string
	Target       float64
}

func calculateMonthlyConsultantGoals(monthlyTarget float64, weeklyTotals map[string]float64) []consultantGoalTarget {
	consultantIDs := make([]string, 0, len(weeklyTotals))
	total := 0.0
	for consultantID, value := range weeklyTotals {
		consultantIDs = append(consultantIDs, consultantID)
		total += math.Max(0, value)
	}
	sort.Strings(consultantIDs)

	goals := make([]consultantGoalTarget, 0, len(consultantIDs))
	allocated := 0.0
	lastEligible := -1
	for _, consultantID := range consultantIDs {
		value := math.Max(0, weeklyTotals[consultantID])
		target := 0.0
		if total > 0 {
			target = roundTo(math.Max(0, monthlyTarget)*value/total, 2)
			if value > 0 {
				lastEligible = len(goals)
			}
		}
		goals = append(goals, consultantGoalTarget{ConsultantID: consultantID, Target: target})
		allocated += target
	}
	if lastEligible >= 0 {
		goals[lastEligible].Target = roundTo(goals[lastEligible].Target+math.Max(0, monthlyTarget)-allocated, 2)
	}
	return goals
}
