package planning

import (
	"testing"
	"time"
)

func TestGenerateEngineScheduleClosesExactWeeklyHours(t *testing.T) {
	weekStart, _ := time.Parse("2006-01-02", "2026-07-27")
	contract := StaffContract{
		ConsultantID: "44444444-4444-4444-4444-444444444444", WeeklyHours: 44,
		MaxDailyHours: 8, TargetWeight: 1,
		AvailableWeekdays: []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
	}
	context, err := buildEngineContext(weekStart, "shopping", testPlanningConfiguration().Configuration, []StaffContract{contract})
	if err != nil {
		t.Fatalf("buildEngineContext() error = %v", err)
	}

	shifts := generateEngineSchedule(context)
	totalMinutes := 0
	for _, shift := range shifts {
		totalMinutes += engineShiftMinutes(shift)
	}
	if totalMinutes != 44*60 {
		t.Fatalf("generated minutes = %d, want %d; shifts = %#v", totalMinutes, 44*60, shifts)
	}
	for _, issue := range validateEngineSchedule(context, shifts) {
		if issue.Severity == "hard" || issue.ID == "weekly-under-"+contract.ConsultantID || issue.ID == "weekly-over-"+contract.ConsultantID {
			t.Fatalf("generated schedule has workload/restriction issue: %#v", issue)
		}
	}
}

func TestCalculateGoalAllocationsPreservesWeeklyTargetToTheCent(t *testing.T) {
	contracts := []StaffContract{
		{ConsultantID: "a", TargetWeight: 1},
		{ConsultantID: "b", TargetWeight: 1},
		{ConsultantID: "c", TargetWeight: 1},
	}
	shifts := []Shift{
		{StaffID: "a", StartsAt: "09:00", EndsAt: "10:00"},
		{StaffID: "b", StartsAt: "09:00", EndsAt: "10:00"},
		{StaffID: "c", StartsAt: "09:00", EndsAt: "10:00"},
	}
	allocations := calculateGoalAllocations(100, contracts, shifts)
	total := 0.0
	for _, allocation := range allocations {
		total += allocation.Target
	}
	if roundTo(total, 2) != 100 {
		t.Fatalf("allocation total = %.2f, want 100.00; allocations = %#v", total, allocations)
	}
}

func TestGenerateEngineScheduleRespectsAbsenceAndSundayRotation(t *testing.T) {
	weekStart, _ := time.Parse("2006-01-02", "2026-07-27")
	staffID := "44444444-4444-4444-4444-444444444444"
	contract := StaffContract{
		ConsultantID: staffID, WeeklyHours: 30, MaxDailyHours: 8, TargetWeight: 1,
		AvailableWeekdays: []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
	}
	context, err := buildEngineContext(weekStart, "shopping", testPlanningConfiguration().Configuration, []StaffContract{contract})
	if err != nil {
		t.Fatalf("buildEngineContext() error = %v", err)
	}
	worksSundays := true
	worksHolidays := true
	_, isoWeek := weekStart.ISOWeek()
	context.Configuration.Staff = append(context.Configuration.Staff, struct {
		ID                   string   `json:"id"`
		WeeklyHours          float64  `json:"weeklyHours"`
		MaxDailyHours        float64  `json:"maxDailyHours"`
		TargetWeight         float64  `json:"targetWeight"`
		AvailableDays        []string `json:"availableDays"`
		WorksSundays         *bool    `json:"worksSundays"`
		AlternateSundays     bool     `json:"alternateSundays"`
		SundayRotationOffset int      `json:"sundayRotationOffset"`
		WorksHolidays        *bool    `json:"worksHolidays"`
	}{ID: staffID, WorksSundays: &worksSundays, AlternateSundays: true, SundayRotationOffset: 1 - isoWeek%2, WorksHolidays: &worksHolidays})
	context.Configuration.Exceptions = []engineStaffException{{
		ID: "absence-1", StaffID: staffID, ISODate: "2026-07-28", Kind: "vacation", AllDay: true,
	}}

	for _, shift := range generateEngineSchedule(context) {
		if shift.ISODate == "2026-07-28" || shift.ISODate == "2026-08-02" {
			t.Fatalf("generated forbidden shift: %#v", shift)
		}
	}
}

func TestValidateEngineScheduleReportsConfiguredMinimumCoverage(t *testing.T) {
	weekStart, _ := time.Parse("2006-01-02", "2026-07-27")
	context, err := buildEngineContext(weekStart, "shopping", testPlanningConfiguration().Configuration, nil)
	if err != nil {
		t.Fatalf("buildEngineContext() error = %v", err)
	}
	context.Configuration.CoverageByLocationType = map[string]engineCoverageRule{
		"shopping": {Enabled: true, OpeningMinimum: 2, PeakMinimum: 4, ClosingMinimum: 3, PeakStartsAt: "14:00", PeakEndsAt: "18:00"},
	}
	issues := validateEngineSchedule(context, []Shift{{
		StaffID: "unknown", ISODate: "2026-07-27", TemplateID: "opening", StartsAt: "10:00", EndsAt: "19:00", BreakMinutes: 60,
	}})
	wanted := map[string]bool{"coverage-opening-2026-07-27": false, "coverage-peak-2026-07-27": false, "coverage-closing-2026-07-27": false}
	for _, issue := range issues {
		if _, exists := wanted[issue.ID]; exists {
			wanted[issue.ID] = true
		}
	}
	for id, found := range wanted {
		if !found {
			t.Fatalf("missing coverage issue %s in %#v", id, issues)
		}
	}
}
