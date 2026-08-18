package contentoperations

import "time"

const (
	PermissionTasksView    = "tasks.tasks.view"
	PermissionCalendarView = "calendar.view"
)

type ScopeClient struct {
	ID   string
	Name string
}

type Scope struct {
	StorageAccountID string
	LockedClientID   string
	Clients          []ScopeClient
}

type Access struct {
	Allowed bool
}

type ChecklistItem struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Completed     bool   `json:"completed"`
	Status        string `json:"status,omitempty"`
	StatusDate    string `json:"statusDate,omitempty"`
	CompletedDate string `json:"completedDate,omitempty"`
}

type TaskSnapshot struct {
	ID       string
	ClientID string
	Title    string
	Updated  time.Time
	Items    []ChecklistItem
}

type EventSnapshot struct {
	ID       string
	ClientID string
	Date     time.Time
	Type     string
	Title    string
	Status   string
	TaskID   string
}

type Severity string

const (
	SeverityCritical  Severity = "critical"
	SeverityAttention Severity = "attention"
	SeverityInfo      Severity = "info"
)

type Alert struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Severity   Severity `json:"severity"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	ClientID   string   `json:"clientId"`
	ClientName string   `json:"clientName"`
	SourceKind string   `json:"sourceKind,omitempty"`
	SourceID   string   `json:"sourceId,omitempty"`
	OccurredOn string   `json:"occurredOn,omitempty"`
	LinkPath   string   `json:"linkPath"`
}

type ClientHealth struct {
	ClientID     string `json:"clientId"`
	ClientName   string `json:"clientName"`
	Critical     int    `json:"critical"`
	Attention    int    `json:"attention"`
	Info         int    `json:"info"`
	LastPostedOn string `json:"lastPostedOn,omitempty"`
}

type Counts struct {
	Critical  int `json:"critical"`
	Attention int `json:"attention"`
	Info      int `json:"info"`
	Total     int `json:"total"`
}

type Brief struct {
	GeneratedAt time.Time      `json:"generatedAt"`
	Today       string         `json:"today"`
	Mode        string         `json:"mode"`
	Headline    string         `json:"headline"`
	Summary     string         `json:"summary"`
	Counts      Counts         `json:"counts"`
	Clients     []ClientHealth `json:"clients"`
	Alerts      []Alert        `json:"alerts"`
}
