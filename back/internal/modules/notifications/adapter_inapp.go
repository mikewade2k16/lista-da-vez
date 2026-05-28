package notifications

import "context"

type InAppAdapter struct {
	publisher RealtimePublisher
}

func NewInAppAdapter(publisher RealtimePublisher) *InAppAdapter {
	return &InAppAdapter{publisher: publisher}
}

func (adapter *InAppAdapter) Channel() string {
	return ChannelInApp
}

func (adapter *InAppAdapter) Send(ctx context.Context, notification Notification) error {
	if adapter.publisher == nil {
		return nil
	}

	adapter.publisher.PublishNotificationEvent(
		ctx,
		notification.UserID,
		EventNotificationCreated,
		notification.ID,
		map[string]any{"notification": notificationToPayload(notification)},
	)
	return nil
}

func notificationToPayload(notification Notification) map[string]any {
	payload := map[string]any{
		"id":           notification.ID,
		"sourceModule": notification.SourceModule,
		"sourceEvent":  notification.SourceEvent,
		"title":        notification.Title,
		"body":         notification.Body,
		"linkPath":     notification.LinkPath,
		"createdAt":    notification.CreatedAt,
	}
	if notification.ReadAt != nil {
		payload["readAt"] = *notification.ReadAt
	}
	if notification.ArchivedAt != nil {
		payload["archivedAt"] = *notification.ArchivedAt
	}
	if len(notification.Payload) > 0 {
		payload["payload"] = notification.Payload
	}
	return payload
}
