package notifications

import (
	"context"
	"time"
)

type Notifier interface {
	Dispatch(ctx context.Context, input DispatchInput) error
}

type ChannelAdapter interface {
	Channel() string
	Send(ctx context.Context, notification Notification) error
}

type RealtimePublisher interface {
	PublishNotificationEvent(ctx context.Context, userID string, eventType string, notificationID string, payload map[string]any)
}

type Repository interface {
	AccountExists(ctx context.Context, accountID string) (bool, error)
	IsAccountMember(ctx context.Context, accountID, userID string) (bool, error)
	ListPermissionsForUser(ctx context.Context, accountID, userID string) ([]string, error)
	InsertNotification(ctx context.Context, input CreateNotificationInput) (Notification, error)
	InsertDeliveryLog(ctx context.Context, entry DeliveryLog) error
	ListNotifications(ctx context.Context, accountID, userID string, cursor *listCursor, limit int) ([]Notification, error)
	MarkRead(ctx context.Context, accountID, userID, notificationID string) (Notification, error)
	MarkAllRead(ctx context.Context, accountID, userID string) (int64, error)
	ListPreferences(ctx context.Context, accountID, userID string) ([]NotificationPreference, error)
	SavePreferences(ctx context.Context, accountID, userID string, preferences []NotificationPreference) ([]NotificationPreference, error)
	IsChannelEnabled(ctx context.Context, accountID, userID, channel, sourceModule, sourceEvent string) (bool, error)
	FindActiveMute(ctx context.Context, accountID, userID, resourceType, resourceID string, now time.Time) (*Mute, error)
	UpsertMute(ctx context.Context, accountID, userID string, input MuteInput, until time.Time) (Mute, error)
}

type noopNotifier struct{}

func NewNoopNotifier() Notifier {
	return noopNotifier{}
}

func (noopNotifier) Dispatch(context.Context, DispatchInput) error {
	return nil
}
