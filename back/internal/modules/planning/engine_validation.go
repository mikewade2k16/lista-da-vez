package planning

import (
	"fmt"
	"math"
	"sort"
	"time"
)

func validateEngineSchedule(context engineContext, shifts []Shift) []PlanningIssue {
	issues := make([]PlanningIssue, 0)
	dates := engineWeek(context)
	dateByISO := make(map[string]engineDate, len(dates))
	for _, date := range dates {
		dateByISO[date.ISODate] = date
	}
	contractByID := make(map[string]StaffContract, len(context.Contracts))
	for _, contract := range context.Contracts {
		contractByID[contract.ConsultantID] = contract
	}

	for _, shift := range shifts {
		contract, staffOK := contractByID[shift.StaffID]
		date, dateOK := dateByISO[shift.ISODate]
		if !staffOK || !dateOK {
			continue
		}
		if !date.IsOpen {
			issues = append(issues, engineIssue("closed", "hard", "Funcionário escalado em dia fechado.", shift))
			continue
		}
		start, _ := clockMinutes(shift.StartsAt)
		end, _ := clockMinutes(shift.EndsAt)
		open, _ := clockMinutes(date.OpensAt)
		close, _ := clockMinutes(date.ClosesAt)
		if start < open || end > close {
			issues = append(issues, engineIssue("outside-hours", "hard", "Turno fora do horário de funcionamento da loja.", shift))
		}
		if !containsEngineWeekday(contract.AvailableWeekdays, date.Weekday) {
			issues = append(issues, engineIssue("unavailable", "hard", "Funcionário escalado fora da disponibilidade informada.", shift))
		}
		rule := engineStaffRuleFor(context, shift.StaffID)
		_, isoWeek := context.WeekStart.ISOWeek()
		if date.Weekday == "sun" && (!engineRuleEnabled(rule.WorksSundays) || (rule.AlternateSundays && isoWeek%2 != rule.SundayRotationOffset)) {
			issues = append(issues, engineIssue("sunday-rotation", "hard", "Funcionário escalado fora do revezamento de domingos.", shift))
		}
		if date.IsHoliday && !engineRuleEnabled(rule.WorksHolidays) {
			issues = append(issues, engineIssue("holiday", "hard", "Funcionário não está habilitado para trabalhar em feriados.", shift))
		}
		if engineShiftConflictsWithException(context, shift) {
			issues = append(issues, engineIssue("exception", "hard", "Turno conflita com férias, ausência ou compromisso cadastrado.", shift))
		}
		minutes := engineShiftMinutes(shift)
		dailyLimit := int(math.Round(math.Min(contract.MaxDailyHours, context.Policy.MaxDailyHours) * 60))
		if minutes > dailyLimit {
			issues = append(issues, engineIssue("daily-limit", "hard", "Turno ultrapassa o limite diário configurado.", shift))
		}
		if minutes >= int(math.Round(context.Policy.BreakAfterHours*60)) && shift.BreakMinutes < context.Policy.MinBreakMinutes {
			issues = append(issues, engineIssue("break", "hard", "Turno não possui o intervalo mínimo configurado.", shift))
		}
	}

	for _, contract := range context.Contracts {
		staffShifts := engineStaffShifts(shifts, contract.ConsultantID)
		totalMinutes := 0
		workedDates := make([]string, 0, len(staffShifts))
		for _, shift := range staffShifts {
			totalMinutes += engineShiftMinutes(shift)
			workedDates = append(workedDates, shift.ISODate)
		}
		contractMinutes := int(math.Round(contract.WeeklyHours * 60))
		balance := totalMinutes - contractMinutes
		if balance < 0 {
			issues = append(issues, PlanningIssue{
				ID: "weekly-under-" + contract.ConsultantID, Severity: "warning", StaffID: contract.ConsultantID,
				Message: fmt.Sprintf("Funcionário está com %.1fh de %.1fh contratadas; faltam %.1fh.", float64(totalMinutes)/60, contract.WeeklyHours, float64(-balance)/60),
			})
		} else if balance > 0 {
			issues = append(issues, PlanningIssue{
				ID: "weekly-over-" + contract.ConsultantID, Severity: "hard", StaffID: contract.ConsultantID,
				Message: fmt.Sprintf("Funcionário está com %.1fh de %.1fh contratadas; excede %.1fh.", float64(totalMinutes)/60, contract.WeeklyHours, float64(balance)/60),
			})
		}
		if len(staffShifts) > 7-context.Policy.MinDaysOff {
			issues = append(issues, PlanningIssue{ID: "days-off-" + contract.ConsultantID, Severity: "hard", StaffID: contract.ConsultantID, Message: "Funcionário não possui as folgas mínimas configuradas."})
		}
		if engineConsecutiveISODays(workedDates) > context.Policy.MaxConsecutiveDays {
			issues = append(issues, PlanningIssue{ID: "consecutive-" + contract.ConsultantID, Severity: "hard", StaffID: contract.ConsultantID, Message: "Funcionário ultrapassa o limite de dias consecutivos."})
		}
	}

	for _, date := range dates {
		if !date.IsOpen {
			continue
		}
		dayShifts := engineDateShifts(shifts, date.ISODate)
		if len(dayShifts) == 0 {
			issues = append(issues, PlanningIssue{ID: "coverage-empty-" + date.ISODate, Severity: "warning", ISODate: date.ISODate, Message: "Dia aberto sem cobertura."})
			continue
		}
		earliest, latest := 24*60, 0
		for _, shift := range dayShifts {
			start, _ := clockMinutes(shift.StartsAt)
			end, _ := clockMinutes(shift.EndsAt)
			earliest = min(earliest, start)
			latest = max(latest, end)
		}
		open, _ := clockMinutes(date.OpensAt)
		close, _ := clockMinutes(date.ClosesAt)
		if earliest > open || latest < close {
			issues = append(issues, PlanningIssue{ID: "coverage-gap-" + date.ISODate, Severity: "warning", ISODate: date.ISODate, Message: "O dia não cobre todo o horário de funcionamento."})
		}
		coverage := context.Configuration.CoverageByLocationType[context.LocationType]
		if coverage.Enabled {
			openingCount, closingCount, peakCount := 0, 0, 0
			peakStart, peakStartOK := clockMinutes(coverage.PeakStartsAt)
			peakEnd, peakEndOK := clockMinutes(coverage.PeakEndsAt)
			for _, shift := range dayShifts {
				start, _ := clockMinutes(shift.StartsAt)
				end, _ := clockMinutes(shift.EndsAt)
				if start <= open {
					openingCount++
				}
				if end >= close {
					closingCount++
				}
				if peakStartOK && peakEndOK && start <= peakStart && end >= peakEnd {
					peakCount++
				}
			}
			if openingCount < coverage.OpeningMinimum {
				issues = append(issues, PlanningIssue{ID: "coverage-opening-" + date.ISODate, Severity: "warning", ISODate: date.ISODate, Message: fmt.Sprintf("Abertura com %d pessoa(s); mínimo configurado: %d.", openingCount, coverage.OpeningMinimum)})
			}
			if peakCount < coverage.PeakMinimum {
				issues = append(issues, PlanningIssue{ID: "coverage-peak-" + date.ISODate, Severity: "warning", ISODate: date.ISODate, Message: fmt.Sprintf("Pico com %d pessoa(s); mínimo configurado: %d.", peakCount, coverage.PeakMinimum)})
			}
			if closingCount < coverage.ClosingMinimum {
				issues = append(issues, PlanningIssue{ID: "coverage-closing-" + date.ISODate, Severity: "warning", ISODate: date.ISODate, Message: fmt.Sprintf("Fechamento com %d pessoa(s); mínimo configurado: %d.", closingCount, coverage.ClosingMinimum)})
			}
		}
	}

	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Severity == "hard" && issues[j].Severity != "hard" })
	return issues
}

func engineIssue(prefix, severity, message string, shift Shift) PlanningIssue {
	return PlanningIssue{ID: prefix + "-" + shift.StaffID + "-" + shift.ISODate, Severity: severity, Message: message, StaffID: shift.StaffID, ISODate: shift.ISODate}
}

func containsEngineWeekday(values []string, weekday string) bool {
	for _, value := range values {
		if value == weekday {
			return true
		}
	}
	return false
}

func engineStaffShifts(shifts []Shift, staffID string) []Shift {
	result := make([]Shift, 0)
	for _, shift := range shifts {
		if shift.StaffID == staffID {
			result = append(result, shift)
		}
	}
	return result
}

func engineDateShifts(shifts []Shift, isoDate string) []Shift {
	result := make([]Shift, 0)
	for _, shift := range shifts {
		if shift.ISODate == isoDate {
			result = append(result, shift)
		}
	}
	return result
}

func engineConsecutiveISODays(values []string) int {
	dates := make([]engineDate, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		if parsed, err := time.Parse("2006-01-02", value); err == nil {
			dates = append(dates, engineDate{Date: parsed})
		}
	}
	times := make([]time.Time, 0, len(dates))
	for _, date := range dates {
		times = append(times, date.Date)
	}
	return engineConsecutiveDays(times)
}
