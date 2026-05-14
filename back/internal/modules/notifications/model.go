package notifications

import "time"

const (
	ChannelInApp    = "in_app"
	ChannelEmail    = "email"
	ChannelWhatsApp = "whatsapp"
	ChannelPush     = "push"

	PermNotificationsRead              = "notifications.read"
	PermNotificationsPreferencesManage = "notifications.preferences.manage"

	EventNotificationCreated = "notification.created"
	EventNotificationRead    = "notification.read"
)

type AccessContext struct {
	UserID          string
	AccountID       string
	IsPlatformAdmin bool
	Permissions     map[string]struct{}
}

func (access AccessContext) Has(permission string) bool {
	if access.IsPlatformAdmin {
		return true
	}
	_, ok := access.Permissions[permission]
	return ok
}

type Notification struct {
	ID           string         `json:"id"`
	AccountID    string         `json:"accountId,omitempty"`
	UserID       string         `json:"userId,omitempty"`
	SourceModule string         `json:"sourceModule"`
	SourceEvent  string         `json:"sourceEvent"`
	Title        string         `json:"title"`
	Body         string         `json:"body"`
	LinkPath     string         `json:"linkPath"`
	Payload      map[string]any `json:"payload,omitempty"`
	ReadAt       *time.Time     `json:"readAt,omitempty"`
	ArchivedAt   *time.Time     `json:"archivedAt,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
}

type NotificationPreference struct {
	AccountID    string    `json:"accountId,omitempty"`
	UserID       string    `json:"userId,omitempty"`
	Channel      string    `json:"channel"`
	SourceModule string    `json:"sourceModule,omitempty"`
	SourceEvent  string    `json:"sourceEvent,omitempty"`
	Enabled      bool      `json:"enabled"`
	UpdatedAt    time.Time `json:"updatedAt,omitempty"`
}

type DeliveryLog struct {
	ID             int64     `json:"id"`
	NotificationID *string   `json:"notificationId,omitempty"`
	Channel        string    `json:"channel"`
	Status         string    `json:"status"`
	Error          string    `json:"error,omitempty"`
	AttemptedAt    time.Time `json:"attemptedAt"`
}

type Mute struct {
	AccountID    string    `json:"accountId,omitempty"`
	UserID       string    `json:"userId,omitempty"`
	ResourceType string    `json:"resourceType"`
	ResourceID   string    `json:"resourceId"`
	Until        time.Time `json:"until"`
	CreatedAt    time.Time `json:"createdAt"`
}

type CreateNotificationInput struct {
	AccountID    string
	UserID       string
	SourceModule string
	SourceEvent  string
	Title        string
	Body         string
	LinkPath     string
	Payload      map[string]any
}

type DispatchInput struct {
	AccountID    string
	UserIDs      []string
	SourceModule string
	SourceEvent  string
	Title        string
	Body         string
	LinkPath     string
	Payload      map[string]any
	ResourceType string
	ResourceID   string
}

type ListNotificationsInput struct {
	Cursor string
	Limit  int
}

type NotificationPage struct {
	Notifications []Notification `json:"notifications"`
	NextCursor    string         `json:"nextCursor,omitempty"`
}

type MuteInput struct {
	ResourceType    string `json:"resourceType"`
	ResourceID      string `json:"resourceId"`
	DurationMinutes int    `json:"durationMinutes"`
}

type listCursor struct {
	CreatedAt time.Time
	ID        string
}
