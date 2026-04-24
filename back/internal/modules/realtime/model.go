package realtime

import "time"

const (
	EventTypeConnected        = "realtime.connected"
	EventTypeOperationUpdated = "operation.updated"
	EventTypeContextUpdated   = "context.updated"
)

type Event struct {
	Type       string    `json:"type"`
	TenantID   string    `json:"tenantId,omitempty"`
	StoreID    string    `json:"storeId,omitempty"`
	Action     string    `json:"action,omitempty"`
	Resource   string    `json:"resource,omitempty"`
	ResourceID string    `json:"resourceId,omitempty"`
	PersonID   string    `json:"personId,omitempty"`
	SavedAt    time.Time `json:"savedAt,omitempty"`
}

func operationTopic(storeID string) string {
	return "operations:" + storeID
}

func contextTopic(tenantID string) string {
	return "context:" + tenantID
}
