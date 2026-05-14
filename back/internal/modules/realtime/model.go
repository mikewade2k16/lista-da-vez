package realtime

import "time"

const (
	EventTypeConnected             = "realtime.connected"
	EventTypeOperationUpdated      = "operation.updated"
	EventTypeContextUpdated        = "context.updated"
	EventTypeTaskCreated           = "task.created"
	EventTypeTaskUpdated           = "task.updated"
	EventTypeTaskMoved             = "task.moved"
	EventTypeTaskDeleted           = "task.deleted"
	EventTypeTaskAssigned          = "task.assigned"
	EventTypeTaskCommentAdded      = "task.comment_added"
	EventTypeTaskRelationAdded     = "task.relation_added"
	EventTypeTaskRelationRemoved   = "task.relation_removed"
	EventTypeTaskShareAdded        = "task.share_added"
	EventTypeTaskShareRevoked      = "task.share_revoked"
	EventTypeTaskTimeStarted       = "task.time_started"
	EventTypeTaskTimePaused        = "task.time_paused"
	EventTypeTaskTimeResumed       = "task.time_resumed"
	EventTypeTaskTimeStopped       = "task.time_stopped"
	EventTypeBoardColumnAdded      = "board.column_added"
	EventTypeBoardColumnUpdated    = "board.column_updated"
	EventTypeBoardColumnDeleted    = "board.column_deleted"
	EventTypePresenceSnapshot      = "presence.snapshot"
	EventTypePresenceUserJoined    = "presence.user_joined"
	EventTypePresenceUserLeft      = "presence.user_left"
	EventTypePresenceFieldLocked   = "presence.field_locked"
	EventTypePresenceFieldUnlocked = "presence.field_unlocked"
	EventTypeNotificationCreated   = "notification.created"
	EventTypeNotificationRead      = "notification.read"
)

type Event struct {
	Type           string         `json:"type"`
	TenantID       string         `json:"tenantId,omitempty"`
	StoreID        string         `json:"storeId,omitempty"`
	AccountID      string         `json:"accountId,omitempty"`
	BoardID        string         `json:"boardId,omitempty"`
	TaskID         string         `json:"taskId,omitempty"`
	UserID         string         `json:"userId,omitempty"`
	DisplayName    string         `json:"displayName,omitempty"`
	AvatarPath     string         `json:"avatarPath,omitempty"`
	FieldKey       string         `json:"fieldKey,omitempty"`
	LockID         string         `json:"lockId,omitempty"`
	NotificationID string         `json:"notificationId,omitempty"`
	Action         string         `json:"action,omitempty"`
	Resource       string         `json:"resource,omitempty"`
	ResourceID     string         `json:"resourceId,omitempty"`
	PersonID       string         `json:"personId,omitempty"`
	Version        int            `json:"version,omitempty"`
	Participants   []PresenceUser `json:"participants,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	SavedAt        time.Time      `json:"savedAt,omitempty"`
}

type PresenceUser struct {
	UserID      string    `json:"userId"`
	DisplayName string    `json:"displayName,omitempty"`
	AvatarPath  string    `json:"avatarPath,omitempty"`
	FieldKey    string    `json:"fieldKey,omitempty"`
	LockID      string    `json:"lockId,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func operationTopic(storeID string) string {
	return "operations:" + storeID
}

func contextTopic(tenantID string) string {
	return "context:" + tenantID
}

func tasksAccountTopic(accountID string) string {
	return "tasks:account:" + accountID
}

func tasksBoardTopic(boardID string) string {
	return "tasks:board:" + boardID
}

func tasksTaskTopic(taskID string) string {
	return "tasks:task:" + taskID
}

func presenceBoardTopic(boardID string) string {
	return "presence:board:" + boardID
}

func presenceTaskTopic(taskID string) string {
	return "presence:task:" + taskID
}

func notificationsUserTopic(userID string) string {
	return "notifications:user:" + userID
}
