package performancefeedback

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/goalperiod"
)

const monthLayout = "2006-01"

func resolvePeriod(rawMonth string, rawWeek int, now time.Time) (Period, error) {
	monthValue := strings.TrimSpace(rawMonth)
	if monthValue == "" {
		monthValue = now.Format(monthLayout)
	}

	month, err := time.Parse(monthLayout, monthValue)
	if err != nil || rawWeek < 0 || rawWeek > goalperiod.Count(month) {
		return Period{}, ErrValidation
	}

	location := now.Location()
	startDay := 1
	endDay := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, location).Day()
	label := month.Format("01/2006")
	if rawWeek > 0 {
		startDay = (rawWeek-1)*7 + 1
		if rawWeek < goalperiod.Count(month) {
			endDay = rawWeek * 7
		}
		label = fmt.Sprintf("Semana %d · %s", rawWeek, label)
	} else {
		label = "Mês · " + label
	}

	dateFrom := time.Date(month.Year(), month.Month(), startDay, 0, 0, 0, 0, location)
	dateTo := time.Date(month.Year(), month.Month(), endDay, 0, 0, 0, 0, location)
	return Period{
		Month:    month.Format(monthLayout),
		Week:     rawWeek,
		DateFrom: dateFrom.Format(time.DateOnly),
		DateTo:   dateTo.Format(time.DateOnly),
		Label:    label,
	}, nil
}

func periodFromStorage(month time.Time, week int) Period {
	period, err := resolvePeriod(month.Format(monthLayout), week, time.Now())
	if err != nil {
		return Period{Month: month.Format(monthLayout), Week: week, Label: strconv.Itoa(week)}
	}
	return period
}
