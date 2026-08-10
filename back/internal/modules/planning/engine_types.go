package planning

import "time"

type engineOperatingDay struct {
	Weekday  string `json:"weekday"`
	IsOpen   bool   `json:"isOpen"`
	OpensAt  string `json:"opensAt"`
	ClosesAt string `json:"closesAt"`
}

type engineShiftTemplate struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	StartsAt string `json:"startsAt"`
	EndsAt   string `json:"endsAt"`
}

type engineLaborPolicy struct {
	ID                 string  `json:"id"`
	MaxDailyHours      float64 `json:"maxDailyHours"`
	MaxConsecutiveDays int     `json:"maxConsecutiveDays"`
	MinDaysOff         int     `json:"minDaysOff"`
	BreakAfterHours    float64 `json:"breakAfterHours"`
	MinBreakMinutes    int     `json:"minBreakMinutes"`
}

type engineCoverageRule struct {
	Enabled        bool   `json:"enabled"`
	OpeningMinimum int    `json:"openingMinimum"`
	PeakMinimum    int    `json:"peakMinimum"`
	ClosingMinimum int    `json:"closingMinimum"`
	PeakStartsAt   string `json:"peakStartsAt"`
	PeakEndsAt     string `json:"peakEndsAt"`
}

type engineHoliday struct {
	ISODate  string `json:"isoDate"`
	Name     string `json:"name"`
	IsOpen   bool   `json:"isOpen"`
	OpensAt  string `json:"opensAt"`
	ClosesAt string `json:"closesAt"`
}

type engineStaffException struct {
	ID       string `json:"id"`
	StaffID  string `json:"staffId"`
	ISODate  string `json:"isoDate"`
	Kind     string `json:"kind"`
	AllDay   bool   `json:"allDay"`
	StartsAt string `json:"startsAt"`
	EndsAt   string `json:"endsAt"`
	Notes    string `json:"notes"`
}

type engineStaffRule struct {
	WorksSundays         *bool `json:"worksSundays"`
	AlternateSundays     bool  `json:"alternateSundays"`
	SundayRotationOffset int   `json:"sundayRotationOffset"`
	WorksHolidays        *bool `json:"worksHolidays"`
}

type engineContext struct {
	WeekStart     time.Time
	LocationType  string
	Configuration configurationDocument
	Policy        engineLaborPolicy
	Contracts     []StaffContract
}

type engineDate struct {
	Date      time.Time
	ISODate   string
	Weekday   string
	IsOpen    bool
	OpensAt   string
	ClosesAt  string
	IsHoliday bool
}
