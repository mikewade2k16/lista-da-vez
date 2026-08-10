package goalperiod

import "time"

// Count returns the number of seven-day goal periods needed to cover a month.
func Count(month time.Time) int {
	lastDay := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if lastDay > 28 {
		return 5
	}
	return 4
}
