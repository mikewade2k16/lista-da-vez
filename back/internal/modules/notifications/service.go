package notifications

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository   Repository
	adapters     map[string]ChannelAdapter
	adapterOrder []string
	publisher    RealtimePublisher
}

func NewService(repository Repository, adapters ...ChannelAdapter) *Service {
	service := &Service{
		repository: repository,
		adapters:   make(map[string]ChannelAdapter),
	}
	for _, adapter := range adapters {
		if adapter == nil {
			continue
		}
		channel := strings.TrimSpace(adapter.Channel())
		if channel == "" {
			continue
		}
		service.adapters[channel] = adapter
		service.adapterOrder = append(service.adapterOrder, channel)
		if inAppAdapter, ok := adapter.(*InAppAdapter); ok && inAppAdapter.publisher != nil {
			service.publisher = inAppAdapter.publisher
		}
	}
	return service
}

func (service *Service) ResolveAccessContext(ctx context.Context, principal auth.Principal, accountID string) (AccessContext, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return AccessContext{}, ErrAccountRequired
	}

	exists, err := service.repository.AccountExists(ctx, accountID)
	if err != nil {
		return AccessContext{}, err
	}
	if !exists {
		return AccessContext{}, ErrAccountNotFound
	}

	isPlatformAdmin := principal.Role == auth.RolePlatformAdmin
	permissions := make(map[string]struct{})
	if !isPlatformAdmin {
		isMember, err := service.repository.IsAccountMember(ctx, accountID, principal.UserID)
		if err != nil {
			return AccessContext{}, err
		}
		if !isMember {
			return AccessContext{}, ErrAccountNotFound
		}

		permissionKeys, err := service.repository.ListPermissionsForUser(ctx, accountID, principal.UserID)
		if err != nil {
			return AccessContext{}, err
		}
		for _, key := range permissionKeys {
			key = strings.TrimSpace(key)
			if key != "" {
				permissions[key] = struct{}{}
			}
		}
	}

	return AccessContext{
		UserID:          strings.TrimSpace(principal.UserID),
		AccountID:       accountID,
		IsPlatformAdmin: isPlatformAdmin,
		Permissions:     permissions,
	}, nil
}

func (service *Service) Dispatch(ctx context.Context, input DispatchInput) error {
	input.AccountID = strings.TrimSpace(input.AccountID)
	input.SourceModule = strings.TrimSpace(input.SourceModule)
	input.SourceEvent = strings.TrimSpace(input.SourceEvent)
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	input.LinkPath = strings.TrimSpace(input.LinkPath)
	input.ResourceType = strings.TrimSpace(input.ResourceType)
	input.ResourceID = strings.TrimSpace(input.ResourceID)

	if input.AccountID == "" || input.SourceModule == "" || input.SourceEvent == "" || input.Title == "" {
		return ErrInvalid
	}

	userIDs := uniqueStrings(input.UserIDs)
	if len(userIDs) == 0 {
		return ErrInvalid
	}

	for _, userID := range userIDs {
		if err := service.dispatchToUser(ctx, userID, input); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) dispatchToUser(ctx context.Context, userID string, input DispatchInput) error {
	if input.ResourceType != "" && input.ResourceID != "" {
		mute, err := service.repository.FindActiveMute(ctx, input.AccountID, userID, input.ResourceType, input.ResourceID, time.Now().UTC())
		if err != nil {
			return err
		}
		if mute != nil {
			return nil
		}
	}

	enabledAdapters := make([]ChannelAdapter, 0, len(service.adapterOrder))
	for _, channel := range service.adapterOrder {
		adapter := service.adapters[channel]
		enabled, err := service.repository.IsChannelEnabled(ctx, input.AccountID, userID, channel, input.SourceModule, input.SourceEvent)
		if err != nil {
			return err
		}
		if enabled {
			enabledAdapters = append(enabledAdapters, adapter)
		}
	}
	if len(enabledAdapters) == 0 {
		return nil
	}

	notification, err := service.repository.InsertNotification(ctx, CreateNotificationInput{
		AccountID:    input.AccountID,
		UserID:       userID,
		SourceModule: input.SourceModule,
		SourceEvent:  input.SourceEvent,
		Title:        input.Title,
		Body:         input.Body,
		LinkPath:     input.LinkPath,
		Payload:      input.Payload,
	})
	if err != nil {
		return err
	}

	for _, adapter := range enabledAdapters {
		status := "sent"
		errorText := ""
		err := adapter.Send(ctx, notification)
		if errors.Is(err, ErrNotConfigured) {
			status = "not_configured"
			errorText = err.Error()
			err = nil
		} else if err != nil {
			status = "failed"
			errorText = err.Error()
		}

		notificationID := notification.ID
		if logErr := service.repository.InsertDeliveryLog(ctx, DeliveryLog{
			NotificationID: &notificationID,
			Channel:        adapter.Channel(),
			Status:         status,
			Error:          errorText,
		}); logErr != nil {
			return logErr
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (service *Service) ListNotifications(ctx context.Context, access AccessContext, input ListNotificationsInput) (NotificationPage, error) {
	if !access.Has(PermNotificationsRead) {
		return NotificationPage{}, ErrForbidden
	}
	if input.Limit <= 0 || input.Limit > 100 {
		input.Limit = 20
	}

	cursor, err := parseCursor(input.Cursor)
	if err != nil {
		return NotificationPage{}, ErrInvalid
	}

	items, err := service.repository.ListNotifications(ctx, access.AccountID, access.UserID, cursor, input.Limit+1)
	if err != nil {
		return NotificationPage{}, err
	}

	page := NotificationPage{Notifications: items}
	if len(items) > input.Limit {
		page.Notifications = items[:input.Limit]
		page.NextCursor = encodeCursor(items[input.Limit-1])
	}
	return page, nil
}

func (service *Service) MarkRead(ctx context.Context, access AccessContext, notificationID string) (Notification, error) {
	if !access.Has(PermNotificationsRead) {
		return Notification{}, ErrForbidden
	}
	notificationID = strings.TrimSpace(notificationID)
	if notificationID == "" {
		return Notification{}, ErrInvalid
	}

	notification, err := service.repository.MarkRead(ctx, access.AccountID, access.UserID, notificationID)
	if err != nil {
		return Notification{}, err
	}
	if service.publisher != nil {
		service.publisher.PublishNotificationEvent(ctx, access.UserID, EventNotificationRead, notification.ID, map[string]any{
			"notificationId": notification.ID,
		})
	}
	return notification, nil
}

func (service *Service) MarkAllRead(ctx context.Context, access AccessContext) (int64, error) {
	if !access.Has(PermNotificationsRead) {
		return 0, ErrForbidden
	}
	return service.repository.MarkAllRead(ctx, access.AccountID, access.UserID)
}

func (service *Service) ListPreferences(ctx context.Context, access AccessContext) ([]NotificationPreference, error) {
	if !access.Has(PermNotificationsPreferencesManage) {
		return nil, ErrForbidden
	}
	return service.repository.ListPreferences(ctx, access.AccountID, access.UserID)
}

func (service *Service) SavePreferences(ctx context.Context, access AccessContext, preferences []NotificationPreference) ([]NotificationPreference, error) {
	if !access.Has(PermNotificationsPreferencesManage) {
		return nil, ErrForbidden
	}
	normalized := make([]NotificationPreference, 0, len(preferences))
	for _, preference := range preferences {
		preference.Channel = strings.TrimSpace(preference.Channel)
		preference.SourceModule = strings.TrimSpace(preference.SourceModule)
		preference.SourceEvent = strings.TrimSpace(preference.SourceEvent)
		if !isValidChannel(preference.Channel) {
			return nil, ErrInvalid
		}
		normalized = append(normalized, NotificationPreference{
			Channel:      preference.Channel,
			SourceModule: preference.SourceModule,
			SourceEvent:  preference.SourceEvent,
			Enabled:      preference.Enabled,
		})
	}
	return service.repository.SavePreferences(ctx, access.AccountID, access.UserID, normalized)
}

func (service *Service) Mute(ctx context.Context, access AccessContext, input MuteInput) (Mute, error) {
	if !access.Has(PermNotificationsPreferencesManage) {
		return Mute{}, ErrForbidden
	}
	input.ResourceType = strings.TrimSpace(input.ResourceType)
	input.ResourceID = strings.TrimSpace(input.ResourceID)
	if input.ResourceType == "" || input.ResourceID == "" || input.DurationMinutes <= 0 {
		return Mute{}, ErrInvalid
	}
	until := time.Now().UTC().Add(time.Duration(input.DurationMinutes) * time.Minute)
	return service.repository.UpsertMute(ctx, access.AccountID, access.UserID, input, until)
}

func parseCursor(raw string) (*listCursor, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parts := strings.SplitN(value, "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("invalid cursor")
	}
	return &listCursor{CreatedAt: createdAt.UTC(), ID: strings.TrimSpace(parts[1])}, nil
}

func encodeCursor(notification Notification) string {
	return notification.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + notification.ID
}

func isValidChannel(channel string) bool {
	switch channel {
	case ChannelInApp, ChannelEmail, ChannelWhatsApp, ChannelPush:
		return true
	default:
		return false
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
