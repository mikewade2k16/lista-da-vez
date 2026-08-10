package planning

import (
	"encoding/json"
	"math"
	"sort"
	"time"
)

var engineWeekdays = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

func buildEngineContext(
	weekStart time.Time,
	storeType string,
	raw json.RawMessage,
	contracts []StaffContract,
) (engineContext, error) {
	var configuration configurationDocument
	if len(raw) == 0 || json.Unmarshal(raw, &configuration) != nil || validateConfigurationRules(raw) != nil {
		return engineContext{}, ErrValidation
	}
	policy, ok := activeEnginePolicy(configuration)
	if !ok || policy.MaxDailyHours <= 0 || policy.MaxDailyHours > 12 ||
		policy.MaxConsecutiveDays < 1 || policy.MaxConsecutiveDays > 7 ||
		policy.MinDaysOff < 0 || policy.MinDaysOff > 6 ||
		policy.BreakAfterHours <= 0 || policy.BreakAfterHours > 12 ||
		policy.MinBreakMinutes < 0 || policy.MinBreakMinutes > 240 {
		return engineContext{}, ErrValidation
	}
	locationType := "street"
	if storeType == "shopping" {
		locationType = "shopping"
	}
	if len(configuration.OperatingHoursByLocationType[locationType]) == 0 ||
		len(configuration.ShiftTemplatesByLocationType[locationType]) == 0 {
		return engineContext{}, ErrValidation
	}
	return engineContext{
		WeekStart: weekStart, LocationType: locationType, Configuration: configuration,
		Policy: policy, Contracts: contracts,
	}, nil
}

func activeEnginePolicy(configuration configurationDocument) (engineLaborPolicy, bool) {
	for _, policy := range configuration.Policies {
		if policy.ID == configuration.ActivePolicyID {
			return policy, true
		}
	}
	return engineLaborPolicy{}, false
}

func engineWeek(context engineContext) []engineDate {
	configured := make(map[string]engineOperatingDay, 7)
	for _, day := range context.Configuration.OperatingHoursByLocationType[context.LocationType] {
		configured[day.Weekday] = day
	}
	dates := make([]engineDate, 0, 7)
	holidays := make(map[string]engineHoliday, len(context.Configuration.Holidays))
	for _, holiday := range context.Configuration.Holidays {
		holidays[holiday.ISODate] = holiday
	}
	for offset := 0; offset < 7; offset++ {
		date := context.WeekStart.AddDate(0, 0, offset)
		weekday := engineWeekdays[int(date.Weekday())]
		day := configured[weekday]
		isOpen := day.IsOpen && validClockRange(day.OpensAt, day.ClosesAt)
		holiday, isHoliday := holidays[date.Format("2006-01-02")]
		if isHoliday {
			isOpen = holiday.IsOpen
			if validClockRange(holiday.OpensAt, holiday.ClosesAt) {
				day.OpensAt, day.ClosesAt = holiday.OpensAt, holiday.ClosesAt
			}
		}
		dates = append(dates, engineDate{
			Date: date, ISODate: date.Format("2006-01-02"), Weekday: weekday,
			IsOpen: isOpen, OpensAt: day.OpensAt, ClosesAt: day.ClosesAt, IsHoliday: isHoliday,
		})
	}
	return dates
}

func generateEngineSchedule(context engineContext) []Shift {
	dates := engineWeek(context)
	templates := validEngineTemplates(context)
	if len(templates) == 0 {
		return []Shift{}
	}
	generated := make([]Shift, 0, len(context.Contracts)*6)
	for staffIndex, contract := range context.Contracts {
		candidates := eligibleEngineDates(context, dates, contract, staffIndex)
		maxWorkingDays := min(len(candidates), 7-context.Policy.MinDaysOff)
		remainingMinutes := int(math.Round(contract.WeeklyHours * 60))
		assigned := make([]time.Time, 0, maxWorkingDays)
		for _, date := range candidates {
			if remainingMinutes <= 0 || len(assigned) >= maxWorkingDays {
				break
			}
			if engineConsecutiveDays(append(assigned, date.Date)) > context.Policy.MaxConsecutiveDays {
				continue
			}
			dailyLimit := int(math.Round(math.Min(contract.MaxDailyHours, context.Policy.MaxDailyHours) * 60))
			desiredMinutes := min(remainingMinutes, dailyLimit, engineDayCapacity(date, context.Policy))
			if desiredMinutes <= 0 {
				continue
			}
			templateIndex := (staffIndex + int(date.Date.Weekday())) % len(templates)
			var shift Shift
			built := false
			for attempt := 0; attempt < len(templates); attempt++ {
				template := templates[(templateIndex+attempt)%len(templates)]
				candidate, ok := buildEngineShift(contract.ConsultantID, date, template, desiredMinutes, context.Policy)
				if ok && !engineShiftConflictsWithException(context, candidate) {
					shift, built = candidate, true
					break
				}
			}
			if !built {
				continue
			}
			generated = append(generated, shift)
			assigned = append(assigned, date.Date)
			remainingMinutes -= engineShiftMinutes(shift)
		}
	}
	return normalizeEngineCoverage(context, generated, dates)
}

func validEngineTemplates(context engineContext) []engineShiftTemplate {
	templates := make([]engineShiftTemplate, 0, 3)
	for _, template := range context.Configuration.ShiftTemplatesByLocationType[context.LocationType] {
		if (template.ID == "opening" || template.ID == "middle" || template.ID == "closing") &&
			validClockRange(template.StartsAt, template.EndsAt) {
			templates = append(templates, template)
		}
	}
	return templates
}

func eligibleEngineDates(context engineContext, dates []engineDate, contract StaffContract, offset int) []engineDate {
	available := make(map[string]struct{}, len(contract.AvailableWeekdays))
	for _, weekday := range contract.AvailableWeekdays {
		available[weekday] = struct{}{}
	}
	candidates := make([]engineDate, 0, 7)
	rule := engineStaffRuleFor(context, contract.ConsultantID)
	_, isoWeek := context.WeekStart.ISOWeek()
	for _, date := range dates {
		_, availableOnWeekday := available[date.Weekday]
		allowedSunday := date.Weekday != "sun" || (engineRuleEnabled(rule.WorksSundays) && (!rule.AlternateSundays || isoWeek%2 == rule.SundayRotationOffset))
		allowedHoliday := !date.IsHoliday || engineRuleEnabled(rule.WorksHolidays)
		if date.IsOpen && availableOnWeekday && allowedSunday && allowedHoliday && !engineHasAllDayException(context, contract.ConsultantID, date.ISODate) {
			candidates = append(candidates, date)
		}
	}
	if len(candidates) == 0 {
		return candidates
	}
	rotation := offset % len(candidates)
	return append(append([]engineDate{}, candidates[rotation:]...), candidates[:rotation]...)
}

func engineDayCapacity(date engineDate, policy engineLaborPolicy) int {
	open, openOK := clockMinutes(date.OpensAt)
	close, closeOK := clockMinutes(date.ClosesAt)
	if !openOK || !closeOK || close <= open {
		return 0
	}
	gross := close - open
	threshold := int(math.Round(policy.BreakAfterHours * 60))
	if gross >= threshold+policy.MinBreakMinutes {
		return gross - policy.MinBreakMinutes
	}
	return gross
}

func buildEngineShift(
	staffID string,
	date engineDate,
	template engineShiftTemplate,
	netMinutes int,
	policy engineLaborPolicy,
) (Shift, bool) {
	open, openOK := clockMinutes(date.OpensAt)
	close, closeOK := clockMinutes(date.ClosesAt)
	templateStart, startOK := clockMinutes(template.StartsAt)
	templateEnd, endOK := clockMinutes(template.EndsAt)
	if !openOK || !closeOK || !startOK || !endOK {
		return Shift{}, false
	}
	breakMinutes := 0
	if netMinutes >= int(math.Round(policy.BreakAfterHours*60)) {
		breakMinutes = policy.MinBreakMinutes
	}
	grossMinutes := netMinutes + breakMinutes
	if grossMinutes > close-open {
		return Shift{}, false
	}
	start := max(open, templateStart)
	switch template.ID {
	case "closing":
		end := min(close, templateEnd)
		start = end - grossMinutes
	case "middle":
		center := (max(open, templateStart) + min(close, templateEnd)) / 2
		start = center - grossMinutes/2
		start = max(open, min(start, close-grossMinutes))
	default:
		start = min(start, close-grossMinutes)
	}
	end := start + grossMinutes
	if start < open || end > close {
		return Shift{}, false
	}
	return Shift{
		StaffID: staffID, ISODate: date.ISODate, TemplateID: template.ID,
		StartsAt: engineClock(start), EndsAt: engineClock(end), BreakMinutes: breakMinutes,
	}, true
}

func normalizeEngineCoverage(context engineContext, shifts []Shift, dates []engineDate) []Shift {
	normalized := append([]Shift(nil), shifts...)
	for _, date := range dates {
		indices := make([]int, 0)
		for index, shift := range normalized {
			if shift.ISODate == date.ISODate {
				indices = append(indices, index)
			}
		}
		if !date.IsOpen || len(indices) == 0 {
			continue
		}
		sort.SliceStable(indices, func(i, j int) bool {
			return normalized[indices[i]].StartsAt < normalized[indices[j]].StartsAt
		})
		coverage := context.Configuration.CoverageByLocationType[context.LocationType]
		openingMinimum, closingMinimum := 1, 1
		if coverage.Enabled {
			openingMinimum = max(1, coverage.OpeningMinimum)
			closingMinimum = max(1, coverage.ClosingMinimum)
		}
		openingCount := min(openingMinimum, len(indices))
		protected := make(map[int]struct{}, len(indices))
		for position := 0; position < openingCount; position++ {
			anchorEngineShift(&normalized[indices[position]], date.OpensAt, true)
			protected[indices[position]] = struct{}{}
		}
		closingStart := max(openingCount, len(indices)-closingMinimum)
		for position := closingStart; position < len(indices); position++ {
			anchorEngineShift(&normalized[indices[position]], date.ClosesAt, false)
			protected[indices[position]] = struct{}{}
		}
		if coverage.Enabled && coverage.PeakMinimum > 0 {
			peakCount := enginePeakCoverageCount(normalized, indices, coverage)
			for _, index := range indices {
				if peakCount >= coverage.PeakMinimum {
					break
				}
				if _, keepBoundary := protected[index]; keepBoundary {
					continue
				}
				if engineShiftCoversPeak(normalized[index], coverage) {
					continue
				}
				if anchorEnginePeakShift(&normalized[index], date, coverage) {
					peakCount++
				}
			}
		}
	}
	return normalized
}

func anchorEngineShift(shift *Shift, boundary string, opening bool) {
	duration := engineShiftMinutes(*shift) + shift.BreakMinutes
	boundaryMinutes, ok := clockMinutes(boundary)
	if !ok {
		return
	}
	if opening {
		shift.StartsAt = boundary
		shift.EndsAt = engineClock(boundaryMinutes + duration)
		shift.TemplateID = "opening"
		return
	}
	shift.StartsAt = engineClock(boundaryMinutes - duration)
	shift.EndsAt = boundary
	shift.TemplateID = "closing"
}

func enginePeakCoverageCount(shifts []Shift, indices []int, coverage engineCoverageRule) int {
	count := 0
	for _, index := range indices {
		if engineShiftCoversPeak(shifts[index], coverage) {
			count++
		}
	}
	return count
}

func engineShiftCoversPeak(shift Shift, coverage engineCoverageRule) bool {
	peakStart, startOK := clockMinutes(coverage.PeakStartsAt)
	peakEnd, endOK := clockMinutes(coverage.PeakEndsAt)
	start, shiftStartOK := clockMinutes(shift.StartsAt)
	end, shiftEndOK := clockMinutes(shift.EndsAt)
	return startOK && endOK && shiftStartOK && shiftEndOK && start <= peakStart && end >= peakEnd
}

func anchorEnginePeakShift(shift *Shift, date engineDate, coverage engineCoverageRule) bool {
	open, openOK := clockMinutes(date.OpensAt)
	close, closeOK := clockMinutes(date.ClosesAt)
	peakStart, startOK := clockMinutes(coverage.PeakStartsAt)
	peakEnd, endOK := clockMinutes(coverage.PeakEndsAt)
	duration := engineShiftMinutes(*shift) + shift.BreakMinutes
	if !openOK || !closeOK || !startOK || !endOK || duration < peakEnd-peakStart {
		return false
	}
	start := max(open, min(peakStart, peakEnd-duration))
	start = min(start, close-duration)
	if start > peakStart || start+duration < peakEnd {
		return false
	}
	shift.StartsAt = engineClock(start)
	shift.EndsAt = engineClock(start + duration)
	shift.TemplateID = "middle"
	return true
}

func engineConsecutiveDays(values []time.Time) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Time(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
	longest, current := 1, 1
	for index := 1; index < len(sorted); index++ {
		if int(sorted[index].Sub(sorted[index-1]).Hours()) == 24 {
			current++
		} else {
			current = 1
		}
		longest = max(longest, current)
	}
	return longest
}

func engineShiftMinutes(shift Shift) int {
	start, startOK := clockMinutes(shift.StartsAt)
	end, endOK := clockMinutes(shift.EndsAt)
	if !startOK || !endOK {
		return 0
	}
	return max(0, end-start-shift.BreakMinutes)
}

func engineClock(minutes int) string {
	minutes = max(0, min(minutes, 24*60-1))
	return time.Date(2000, 1, 1, minutes/60, minutes%60, 0, 0, time.UTC).Format("15:04")
}

func validClockRange(startsAt, endsAt string) bool {
	start, startOK := clockMinutes(startsAt)
	end, endOK := clockMinutes(endsAt)
	return startOK && endOK && end > start
}

func engineStaffRuleFor(context engineContext, staffID string) engineStaffRule {
	for _, staff := range context.Configuration.Staff {
		if staff.ID == staffID {
			return engineStaffRule{
				WorksSundays: staff.WorksSundays, AlternateSundays: staff.AlternateSundays,
				SundayRotationOffset: max(0, min(1, staff.SundayRotationOffset)), WorksHolidays: staff.WorksHolidays,
			}
		}
	}
	return engineStaffRule{}
}

func engineRuleEnabled(value *bool) bool {
	return value == nil || *value
}

func engineHasAllDayException(context engineContext, staffID, isoDate string) bool {
	for _, exception := range context.Configuration.Exceptions {
		if exception.StaffID == staffID && exception.ISODate == isoDate && exception.AllDay {
			return true
		}
	}
	return false
}

func engineShiftConflictsWithException(context engineContext, shift Shift) bool {
	shiftStart, _ := clockMinutes(shift.StartsAt)
	shiftEnd, _ := clockMinutes(shift.EndsAt)
	for _, exception := range context.Configuration.Exceptions {
		if exception.StaffID != shift.StaffID || exception.ISODate != shift.ISODate {
			continue
		}
		if exception.AllDay {
			return true
		}
		start, startOK := clockMinutes(exception.StartsAt)
		end, endOK := clockMinutes(exception.EndsAt)
		if startOK && endOK && start < shiftEnd && end > shiftStart {
			return true
		}
	}
	return false
}
