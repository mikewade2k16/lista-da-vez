package planning

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

type StoreConfiguration struct {
	ID            string          `json:"id"`
	StoreID       string          `json:"storeId"`
	Configuration json.RawMessage `json:"configuration"`
	Version       int64           `json:"version"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

type Shift struct {
	StaffID      string `json:"staffId"`
	ISODate      string `json:"isoDate"`
	TemplateID   string `json:"templateId"`
	StartsAt     string `json:"startsAt"`
	EndsAt       string `json:"endsAt"`
	BreakMinutes int    `json:"breakMinutes"`
}

type StaffContract struct {
	ConsultantID      string   `json:"consultantId"`
	WeeklyHours       float64  `json:"weeklyHours"`
	MaxDailyHours     float64  `json:"maxDailyHours"`
	TargetWeight      float64  `json:"targetWeight"`
	AvailableWeekdays []string `json:"availableWeekdays"`
	Version           int64    `json:"version"`
}

type GoalAllocation struct {
	ConsultantID   string  `json:"staffId"`
	ScheduledHours float64 `json:"scheduledHours"`
	WeightedHours  float64 `json:"weightedHours"`
	Share          float64 `json:"share"`
	Target         float64 `json:"target"`
}

type PlanningIssue struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	StaffID  string `json:"staffId,omitempty"`
	ISODate  string `json:"isoDate,omitempty"`
}

type Schedule struct {
	ID              string           `json:"id"`
	StoreID         string           `json:"storeId"`
	WeekStart       string           `json:"weekStart"`
	TargetMonth     string           `json:"targetMonth"`
	GoalWeek        int              `json:"goalWeek"`
	Status          string           `json:"status"`
	Shifts          []Shift          `json:"shifts"`
	GoalAllocations []GoalAllocation `json:"goalAllocations"`
	Version         int64            `json:"version"`
	PublishedAt     *time.Time       `json:"publishedAt,omitempty"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	Issues          []PlanningIssue  `json:"issues"`
}

type ScheduleRevision struct {
	Version       int64     `json:"version"`
	Status        string    `json:"status"`
	ChangedByName string    `json:"changedByName"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Snapshot struct {
	Configuration *StoreConfiguration `json:"configuration"`
	Schedule      *Schedule           `json:"schedule"`
	Contracts     []StaffContract     `json:"contracts"`
	History       []ScheduleRevision  `json:"history"`
}

type GetInput struct {
	StoreID   string
	WeekStart string
}

type SaveConfigurationInput struct {
	StoreID       string
	Configuration json.RawMessage
}

type SaveScheduleInput struct {
	StoreID         string
	WeekStart       string
	TargetMonth     string
	GoalWeek        int
	Status          string
	Shifts          []Shift
	ExpectedVersion *int64
}

type GenerateScheduleInput struct {
	StoreID         string
	WeekStart       string
	TargetMonth     string
	GoalWeek        int
	ExpectedVersion *int64
}

type RepositoryScheduleInput struct {
	TenantID        string
	StoreID         string
	UserID          string
	WeekStart       time.Time
	TargetMonth     time.Time
	GoalWeek        int
	Status          string
	Shifts          []Shift
	Allocations     []GoalAllocation
	ExpectedVersion *int64
}

type Repository interface {
	FindConfiguration(context.Context, string, string) (*StoreConfiguration, error)
	FindSchedule(context.Context, string, string, time.Time) (*Schedule, error)
	ListScheduleRevisions(context.Context, string, string, string) ([]ScheduleRevision, error)
	ListContracts(context.Context, string, string) ([]StaffContract, error)
	UpsertConfiguration(context.Context, string, string, string, json.RawMessage, []StaffContract) (StoreConfiguration, error)
	SaveSchedule(context.Context, RepositoryScheduleInput) (Schedule, error)
	ReopenSchedule(context.Context, string, string, string, time.Time, int64) (Schedule, error)
	FindStoreWeeklyGoal(context.Context, string, string, time.Time, int) (float64, error)
	CountStoreConsultants(context.Context, string, string, []string) (int, error)
}

type StoreFinder interface {
	FindAccessible(context.Context, auth.Principal, string) (stores.StoreView, error)
}
