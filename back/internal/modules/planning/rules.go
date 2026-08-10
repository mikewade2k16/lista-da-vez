package planning

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/goalperiod"
)

type configurationDocument struct {
	ActivePolicyID               string                           `json:"activePolicyId"`
	OperatingHoursByLocationType map[string][]engineOperatingDay  `json:"operatingHoursByLocationType"`
	ShiftTemplatesByLocationType map[string][]engineShiftTemplate `json:"shiftTemplatesByLocationType"`
	Policies                     []engineLaborPolicy              `json:"policies"`
	CoverageByLocationType       map[string]engineCoverageRule    `json:"coverageByLocationType"`
	Holidays                     []engineHoliday                  `json:"holidays"`
	Exceptions                   []engineStaffException           `json:"exceptions"`
	Staff                        []struct {
		ID                   string   `json:"id"`
		WeeklyHours          float64  `json:"weeklyHours"`
		MaxDailyHours        float64  `json:"maxDailyHours"`
		TargetWeight         float64  `json:"targetWeight"`
		AvailableDays        []string `json:"availableDays"`
		WorksSundays         *bool    `json:"worksSundays"`
		AlternateSundays     bool     `json:"alternateSundays"`
		SundayRotationOffset int      `json:"sundayRotationOffset"`
		WorksHolidays        *bool    `json:"worksHolidays"`
	} `json:"staff"`
}

func contractsFromConfiguration(raw json.RawMessage) ([]StaffContract, error) {
	var document configurationDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, ErrValidation
	}
	contracts := make([]StaffContract, 0, len(document.Staff))
	seen := map[string]struct{}{}
	for _, member := range document.Staff {
		consultantID := strings.TrimSpace(member.ID)
		if consultantID == "" || member.WeeklyHours <= 0 || member.WeeklyHours > 60 ||
			member.MaxDailyHours <= 0 || member.MaxDailyHours > 12 ||
			member.TargetWeight < 0 || member.TargetWeight > 3 || !validWeekdays(member.AvailableDays) {
			return nil, ErrValidation
		}
		if _, duplicated := seen[consultantID]; duplicated {
			return nil, ErrValidation
		}
		seen[consultantID] = struct{}{}
		contracts = append(contracts, StaffContract{
			ConsultantID: consultantID, WeeklyHours: member.WeeklyHours,
			MaxDailyHours: member.MaxDailyHours, TargetWeight: member.TargetWeight,
			AvailableWeekdays: append([]string(nil), member.AvailableDays...),
		})
	}
	sort.Slice(contracts, func(i, j int) bool { return contracts[i].ConsultantID < contracts[j].ConsultantID })
	return contracts, nil
}

func validateConfigurationRules(raw json.RawMessage) error {
	var document configurationDocument
	if json.Unmarshal(raw, &document) != nil {
		return ErrValidation
	}
	for _, locationType := range []string{"street", "shopping"} {
		coverage, ok := document.CoverageByLocationType[locationType]
		if !ok || !coverage.Enabled {
			continue
		}
		if coverage.OpeningMinimum < 0 || coverage.OpeningMinimum > 50 ||
			coverage.PeakMinimum < 0 || coverage.PeakMinimum > 50 ||
			coverage.ClosingMinimum < 0 || coverage.ClosingMinimum > 50 ||
			!validClockRange(coverage.PeakStartsAt, coverage.PeakEndsAt) {
			return ErrValidation
		}
	}
	holidayDates := map[string]struct{}{}
	for _, holiday := range document.Holidays {
		if strings.TrimSpace(holiday.Name) == "" {
			return ErrValidation
		}
		if _, err := time.Parse("2006-01-02", holiday.ISODate); err != nil {
			return ErrValidation
		}
		if _, duplicated := holidayDates[holiday.ISODate]; duplicated {
			return ErrValidation
		}
		holidayDates[holiday.ISODate] = struct{}{}
		if holiday.IsOpen && (holiday.OpensAt != "" || holiday.ClosesAt != "") && !validClockRange(holiday.OpensAt, holiday.ClosesAt) {
			return ErrValidation
		}
	}
	staffIDs := map[string]struct{}{}
	for _, staff := range document.Staff {
		staffIDs[staff.ID] = struct{}{}
		if staff.SundayRotationOffset < 0 || staff.SundayRotationOffset > 1 {
			return ErrValidation
		}
	}
	allowedKinds := map[string]struct{}{
		"vacation": {}, "medical_leave": {}, "training": {}, "meeting": {}, "time_bank": {}, "exceptional_day_off": {},
	}
	exceptionIDs := map[string]struct{}{}
	for _, exception := range document.Exceptions {
		if strings.TrimSpace(exception.ID) == "" {
			return ErrValidation
		}
		if _, duplicated := exceptionIDs[exception.ID]; duplicated {
			return ErrValidation
		}
		exceptionIDs[exception.ID] = struct{}{}
		if _, exists := staffIDs[exception.StaffID]; !exists {
			return ErrValidation
		}
		if _, exists := allowedKinds[exception.Kind]; !exists {
			return ErrValidation
		}
		if _, err := time.Parse("2006-01-02", exception.ISODate); err != nil {
			return ErrValidation
		}
		if !exception.AllDay && !validClockRange(exception.StartsAt, exception.EndsAt) {
			return ErrValidation
		}
	}
	return nil
}

func validWeekdays(values []string) bool {
	allowed := map[string]struct{}{"mon": {}, "tue": {}, "wed": {}, "thu": {}, "fri": {}, "sat": {}, "sun": {}}
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return false
		}
		if _, duplicated := seen[value]; duplicated {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func normalizeGoalPeriod(weekStart time.Time, rawMonth string, week int) (time.Time, int, error) {
	targetMonth, err := time.Parse("2006-01", strings.TrimSpace(rawMonth))
	if err != nil || week < 1 || week > goalperiod.Count(targetMonth) {
		return time.Time{}, 0, ErrValidation
	}
	expectedWeekStart := goalPeriodWeekStart(targetMonth, week)
	if !weekStart.Equal(expectedWeekStart) {
		return time.Time{}, 0, ErrValidation
	}
	return time.Date(targetMonth.Year(), targetMonth.Month(), 1, 0, 0, 0, 0, time.UTC), week, nil
}

func goalPeriodWeekStart(targetMonth time.Time, week int) time.Time {
	periodAnchor := time.Date(
		targetMonth.Year(),
		targetMonth.Month(),
		1+(week-1)*7,
		0,
		0,
		0,
		0,
		time.UTC,
	)
	distanceFromMonday := (int(periodAnchor.Weekday()) + 6) % 7
	return periodAnchor.AddDate(0, 0, -distanceFromMonday)
}

func calculateGoalAllocations(target float64, contracts []StaffContract, shifts []Shift) []GoalAllocation {
	contractByID := make(map[string]StaffContract, len(contracts))
	for _, contract := range contracts {
		contractByID[contract.ConsultantID] = contract
	}
	hoursByID := map[string]float64{}
	for _, shift := range shifts {
		start, startOK := clockMinutes(shift.StartsAt)
		end, endOK := clockMinutes(shift.EndsAt)
		if !startOK || !endOK || end <= start {
			continue
		}
		hoursByID[shift.StaffID] += math.Max(0, float64(end-start-shift.BreakMinutes)/60)
	}
	totalWeightedHours := 0.0
	for consultantID, hours := range hoursByID {
		totalWeightedHours += hours * contractByID[consultantID].TargetWeight
	}
	allocations := make([]GoalAllocation, 0, len(contracts))
	lastEligible := -1
	for _, contract := range contracts {
		hours := hoursByID[contract.ConsultantID]
		weightedHours := hours * contract.TargetWeight
		share := 0.0
		if totalWeightedHours > 0 {
			share = weightedHours / totalWeightedHours
		}
		allocations = append(allocations, GoalAllocation{
			ConsultantID:   contract.ConsultantID,
			ScheduledHours: roundTo(hours, 2), WeightedHours: roundTo(weightedHours, 2),
			Share: roundTo(share, 6), Target: roundTo(math.Max(0, target)*share, 2),
		})
		if weightedHours > 0 {
			lastEligible = len(allocations) - 1
		}
	}
	if lastEligible >= 0 {
		allocated := 0.0
		for index, allocation := range allocations {
			if index != lastEligible {
				allocated += allocation.Target
			}
		}
		allocations[lastEligible].Target = roundTo(math.Max(0, target)-allocated, 2)
	}
	return allocations
}

func scaleGoalAllocations(target float64, source []GoalAllocation) []GoalAllocation {
	allocations := make([]GoalAllocation, len(source))
	copy(allocations, source)
	lastEligible := -1
	allocated := 0.0
	for index := range allocations {
		allocations[index].Target = roundTo(math.Max(0, target)*math.Max(0, allocations[index].Share), 2)
		allocated += allocations[index].Target
		if allocations[index].Share > 0 {
			lastEligible = index
		}
	}
	if lastEligible >= 0 {
		allocations[lastEligible].Target = roundTo(
			allocations[lastEligible].Target+math.Max(0, target)-allocated,
			2,
		)
	}
	return allocations
}

func clockMinutes(value string) (int, bool) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, false
	}
	return parsed.Hour()*60 + parsed.Minute(), true
}

func roundTo(value float64, decimals int) float64 {
	factor := math.Pow10(decimals)
	return math.Round(value*factor) / factor
}
