package tasks

import (
	"context"
	"time"
)

type Perspective string

const (
	PerspectiveAgency       Perspective = "agency"
	PerspectiveClientViewer Perspective = "client_viewer"
)

type AccessContext struct {
	UserID          string
	AccountID       string
	IsPlatformAdmin bool
	Perspective     Perspective
	Permissions     map[string]struct{}
}

func (access AccessContext) Has(permission string) bool {
	if access.IsPlatformAdmin {
		return true
	}
	if _, ok := access.Permissions[permission]; ok {
		return true
	}
	if permission == PermTasksView || permission == PermBoardsView {
		_, ok := access.Permissions[PermClientView]
		return ok
	}
	return false
}

type Board struct {
	ID              string    `json:"id"`
	AccountID       string    `json:"accountId,omitempty"`
	OrganizationID  *string   `json:"organizationId,omitempty"`
	Slug            string    `json:"slug"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Icon            string    `json:"icon"`
	Archived        bool      `json:"archived"`
	CreatedByUserID string    `json:"createdByUserId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Columns         []Column  `json:"columns,omitempty"`
	Fields          []Field   `json:"fields,omitempty"`
	Views           []View    `json:"views,omitempty"`
}

type Column struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"boardId"`
	Label     string    `json:"label"`
	Color     string    `json:"color"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
}

type Field struct {
	ID        string         `json:"id"`
	BoardID   string         `json:"boardId"`
	Key       string         `json:"key"`
	Label     string         `json:"label"`
	Type      string         `json:"type"`
	Required  bool           `json:"required"`
	Hidden    bool           `json:"hidden"`
	SortOrder int            `json:"sortOrder"`
	Config    map[string]any `json:"config"`
	Options   []FieldOption  `json:"options,omitempty"`
}

type FieldOption struct {
	ID        string `json:"id"`
	FieldID   string `json:"fieldId"`
	Value     string `json:"value"`
	Label     string `json:"label"`
	Color     string `json:"color"`
	SortOrder int    `json:"sortOrder"`
}

type View struct {
	ID          string         `json:"id"`
	BoardID     string         `json:"boardId"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Scope       string         `json:"scope"`
	OwnerUserID *string        `json:"ownerUserId,omitempty"`
	Config      map[string]any `json:"config"`
	SortOrder   int            `json:"sortOrder"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type Task struct {
	ID                string     `json:"id"`
	AccountID         string     `json:"accountId,omitempty"`
	BoardID           string     `json:"boardId"`
	ColumnID          *string    `json:"columnId,omitempty"`
	Title             string     `json:"title"`
	ContentHTML       string     `json:"contentHtml"`
	Status            *string    `json:"status,omitempty"`
	Priority          string     `json:"priority"`
	DueDate           *time.Time `json:"dueDate,omitempty"`
	StartDate         *time.Time `json:"startDate,omitempty"`
	Archived          bool       `json:"archived"`
	SortOrder         float64    `json:"sortOrder"`
	CreatedByUserID   string     `json:"createdByUserId,omitempty"`
	ResponsibleUserID *string    `json:"responsibleUserId,omitempty"`
	ClientAccountID   *string    `json:"clientAccountId,omitempty"`
	Version           int        `json:"version"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type Comment struct {
	ID           string     `json:"id"`
	TaskID       string     `json:"taskId"`
	AuthorUserID string     `json:"authorUserId"`
	BodyHTML     string     `json:"bodyHtml"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	DeletedAt    *time.Time `json:"deletedAt,omitempty"`
}

type Relation struct {
	ID            string         `json:"id"`
	TaskID        string         `json:"taskId"`
	Module        string         `json:"module"`
	ResourceType  string         `json:"resourceType"`
	ResourceID    string         `json:"resourceId"`
	LabelCache    string         `json:"labelCache"`
	MetadataCache map[string]any `json:"metadataCache"`
	RefreshedAt   time.Time      `json:"refreshedAt"`
}

type Share struct {
	ID              string     `json:"id"`
	TaskID          string     `json:"taskId"`
	ClientAccountID string     `json:"clientAccountId"`
	Permission      string     `json:"permission"`
	SharedByUserID  string     `json:"sharedByUserId"`
	CreatedAt       time.Time  `json:"createdAt"`
	RevokedAt       *time.Time `json:"revokedAt,omitempty"`
}

type TimeEntry struct {
	ID         string     `json:"id"`
	TaskID     string     `json:"taskId"`
	UserID     string     `json:"userId"`
	AccountID  string     `json:"accountId"`
	StartedAt  time.Time  `json:"startedAt"`
	PausedAt   *time.Time `json:"pausedAt,omitempty"`
	ResumedAt  *time.Time `json:"resumedAt,omitempty"`
	StoppedAt  *time.Time `json:"stoppedAt,omitempty"`
	DurationMs int64      `json:"durationMs"`
	Notes      string     `json:"notes"`
	Version    int        `json:"version"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type AuditEntry struct {
	ID           int64          `json:"id"`
	AccountID    string         `json:"accountId"`
	UserID       *string        `json:"userId,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resourceType"`
	ResourceID   string         `json:"resourceId"`
	Before       map[string]any `json:"before,omitempty"`
	After        map[string]any `json:"after,omitempty"`
	At           time.Time      `json:"at"`
}

type CreateBoardInput struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type UpdateBoardInput struct {
	ID          string
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Archived    *bool   `json:"archived"`
}

type CreateColumnInput struct {
	BoardID   string
	Label     string `json:"label"`
	Color     string `json:"color"`
	SortOrder int    `json:"sortOrder"`
}

type UpdateColumnInput struct {
	ID        string
	Label     *string `json:"label"`
	Color     *string `json:"color"`
	SortOrder *int    `json:"sortOrder"`
}

type DeleteColumnInput struct {
	ID              string
	RemapToColumnID string `json:"remapToColumnId"`
}

type CreateFieldInput struct {
	BoardID   string
	Key       string         `json:"key"`
	Label     string         `json:"label"`
	Type      string         `json:"type"`
	Required  bool           `json:"required"`
	Hidden    bool           `json:"hidden"`
	SortOrder int            `json:"sortOrder"`
	Config    map[string]any `json:"config"`
	Options   []FieldOption  `json:"options"`
}

type ListTasksInput struct {
	BoardID         string
	Limit           int
	Cursor          string
	IncludeArchived bool
}

type CreateTaskInput struct {
	BoardID           string
	ColumnID          *string    `json:"columnId"`
	Title             string     `json:"title"`
	ContentHTML       string     `json:"contentHtml"`
	Status            *string    `json:"status"`
	Priority          string     `json:"priority"`
	DueDate           *time.Time `json:"dueDate"`
	StartDate         *time.Time `json:"startDate"`
	SortOrder         float64    `json:"sortOrder"`
	ResponsibleUserID *string    `json:"responsibleUserId"`
	ClientAccountID   *string    `json:"clientAccountId"`
}

type UpdateTaskInput struct {
	ID                string
	ExpectedVersion   *int
	ColumnID          **string    `json:"columnId"`
	Title             *string     `json:"title"`
	ContentHTML       *string     `json:"contentHtml"`
	Status            **string    `json:"status"`
	Priority          *string     `json:"priority"`
	DueDate           **time.Time `json:"dueDate"`
	StartDate         **time.Time `json:"startDate"`
	Archived          *bool       `json:"archived"`
	SortOrder         *float64    `json:"sortOrder"`
	ResponsibleUserID **string    `json:"responsibleUserId"`
	ClientAccountID   **string    `json:"clientAccountId"`
}

type MoveTaskInput struct {
	ID              string
	ExpectedVersion *int
	ColumnID        *string  `json:"columnId"`
	SortOrder       *float64 `json:"sortOrder"`
}

type AddCommentInput struct {
	TaskID           string
	BodyHTML         string   `json:"bodyHtml"`
	MentionedUserIDs []string `json:"mentionedUserIds"`
}

type AddShareInput struct {
	TaskID          string
	ClientAccountID string `json:"clientAccountId"`
	Permission      string `json:"permission"`
}

type AddRelationInput struct {
	TaskID        string
	Module        string         `json:"module"`
	ResourceType  string         `json:"resourceType"`
	ResourceID    string         `json:"resourceId"`
	LabelCache    string         `json:"labelCache"`
	MetadataCache map[string]any `json:"metadataCache"`
}

type TrackingMetricsInput struct {
	UserID          string
	ClientAccountID string
	From            *time.Time
	To              *time.Time
}

type Repository interface {
	AccountExists(ctx context.Context, accountID string) (bool, error)
	IsAccountMember(ctx context.Context, accountID, userID string) (bool, error)
	ListPermissionsForUser(ctx context.Context, accountID, userID string) ([]string, error)
	FindOrganizationIDForAccount(ctx context.Context, accountID string) (*string, error)

	ListBoards(ctx context.Context, access AccessContext) ([]Board, error)
	GetBoard(ctx context.Context, access AccessContext, boardID string) (Board, error)
	CreateBoard(ctx context.Context, accountID string, input CreateBoardInput, createdByUserID string, organizationID *string) (Board, error)
	UpdateBoard(ctx context.Context, accountID string, input UpdateBoardInput) (Board, error)

	CreateColumn(ctx context.Context, accountID string, input CreateColumnInput) (Column, error)
	UpdateColumn(ctx context.Context, accountID string, input UpdateColumnInput) (Column, error)
	DeleteColumn(ctx context.Context, accountID string, input DeleteColumnInput) (string, error)
	CreateField(ctx context.Context, accountID string, input CreateFieldInput) (Field, error)

	ListTasks(ctx context.Context, access AccessContext, input ListTasksInput) ([]Task, error)
	GetTask(ctx context.Context, access AccessContext, taskID string) (Task, error)
	CreateTask(ctx context.Context, accountID string, input CreateTaskInput, createdByUserID string) (Task, error)
	UpdateTask(ctx context.Context, accountID string, input UpdateTaskInput) (Task, error)
	MoveTask(ctx context.Context, accountID string, input MoveTaskInput) (Task, error)
	ArchiveTask(ctx context.Context, accountID, taskID string) (Task, error)

	AddComment(ctx context.Context, accountID string, input AddCommentInput, authorUserID string) (Comment, error)
	AddCommentMentions(ctx context.Context, accountID, taskID, commentID string, mentionedUserIDs []string) ([]string, error)
	ListComments(ctx context.Context, access AccessContext, taskID string) ([]Comment, error)
	UpsertSubscribers(ctx context.Context, accountID, taskID string, userIDs []string) error
	ListSubscriberUserIDs(ctx context.Context, accountID, taskID string) ([]string, error)
	AddShare(ctx context.Context, accountID string, input AddShareInput, sharedByUserID string) (Share, error)
	ListRelations(ctx context.Context, access AccessContext, taskID string) ([]Relation, error)
	AddRelation(ctx context.Context, accountID string, input AddRelationInput) (Relation, error)
	ListAudit(ctx context.Context, accountID, taskID string) ([]AuditEntry, error)
	InsertAuditEntry(ctx context.Context, entry AuditEntry) error

	ListActiveTimeEntries(ctx context.Context, access AccessContext) ([]TimeEntry, error)
	StartTracking(ctx context.Context, accountID, taskID, userID string) (TimeEntry, error)
	PauseTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error)
	ResumeTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error)
	StopTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error)
	TrackingMetrics(ctx context.Context, accountID string, input TrackingMetricsInput) (TrackingMetrics, error)
}

type TrackingMetrics struct {
	TotalDurationMs int64 `json:"totalDurationMs"`
	EntryCount      int64 `json:"entryCount"`
}
