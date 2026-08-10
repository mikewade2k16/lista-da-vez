package calendar

import (
	"context"
	"io"
	"strings"
)

// GetCalendarScope devolve somente o contrato publico do recorte. A conta de
// armazenamento fica em campo json:"-" e e usada apenas pelos metodos scoped.
func (s *Service) GetCalendarScope(ctx context.Context, activeAccountID string) (CalendarScope, error) {
	scope, err := s.store.ResolveCalendarScope(ctx, strings.TrimSpace(activeAccountID))
	if err != nil {
		return CalendarScope{}, mapNotFound(err)
	}
	return scope, nil
}

func (s *Service) ListScopedEvents(ctx context.Context, activeAccountID string, filter EventFilter) ([]EventView, error) {
	scope, err := s.GetCalendarScope(ctx, activeAccountID)
	if err != nil {
		return nil, err
	}
	filter = scopeEventFilter(scope, filter)
	return s.ListEvents(ctx, scope.StorageAccountID, filter)
}

func (s *Service) GetScopedEvent(ctx context.Context, activeAccountID, eventID string) (EventView, error) {
	scope, err := s.GetCalendarScope(ctx, activeAccountID)
	if err != nil {
		return EventView{}, err
	}
	event, err := s.store.GetEvent(ctx, strings.TrimSpace(eventID), scope.StorageAccountID, scope.LockedClientID)
	if err != nil {
		return EventView{}, mapNotFound(err)
	}
	return event.view(), nil
}

func (s *Service) CreateScopedEvent(ctx context.Context, activeAccountID string, input EventInput) (EventView, error) {
	scope, err := s.GetCalendarScope(ctx, activeAccountID)
	if err != nil {
		return EventView{}, err
	}
	input = scopeEventInput(scope, input)
	return s.CreateEvent(ctx, scope.StorageAccountID, input)
}

func (s *Service) UpdateScopedEvent(ctx context.Context, activeAccountID, eventID string, input EventInput, expectedVersion *int) (EventView, error) {
	scope, err := s.GetCalendarScope(ctx, activeAccountID)
	if err != nil {
		return EventView{}, err
	}
	input = scopeEventInput(scope, input)
	return s.updateEvent(ctx, scope.StorageAccountID, eventID, scope.LockedClientID, input, expectedVersion)
}

func (s *Service) DeleteScopedEvent(ctx context.Context, activeAccountID, eventID string, archiveTask bool) error {
	scope, err := s.GetCalendarScope(ctx, activeAccountID)
	if err != nil {
		return err
	}
	return s.deleteEvent(ctx, scope.StorageAccountID, eventID, scope.LockedClientID, archiveTask)
}

func (s *Service) CreateTaskForScopedEvent(ctx context.Context, activeAccountID, eventID string) (string, error) {
	scope, err := s.GetCalendarScope(ctx, activeAccountID)
	if err != nil {
		return "", err
	}
	return s.createTaskForEvent(ctx, scope.StorageAccountID, eventID, scope.LockedClientID)
}

func (s *Service) SaveScopedMedia(ctx context.Context, activeAccountID, actorID, idempotencyKey, filename, contentType string, content []byte) (MediaItem, error) {
	scope, err := s.GetCalendarScope(ctx, activeAccountID)
	if err != nil {
		return MediaItem{}, err
	}
	return s.SaveMedia(ctx, scope.StorageAccountID, actorID, idempotencyKey, filename, contentType, content)
}

func (s *Service) SaveScopedMediaStream(ctx context.Context, activeAccountID, actorID, idempotencyKey, filename, contentType string, sizeBytes int64, content io.Reader) (MediaItem, error) {
	scope, err := s.GetCalendarScope(ctx, activeAccountID)
	if err != nil {
		return MediaItem{}, err
	}
	return s.SaveMediaStream(ctx, scope.StorageAccountID, actorID, idempotencyKey, filename, contentType, sizeBytes, content)
}

func scopeAllowsClient(scope CalendarScope, clientID string) bool {
	locked := strings.TrimSpace(scope.LockedClientID)
	return locked == "" || strings.TrimSpace(clientID) == locked
}

func scopeEventFilter(scope CalendarScope, filter EventFilter) EventFilter {
	if locked := strings.TrimSpace(scope.LockedClientID); locked != "" {
		filter.ClientID = locked
	}
	return filter
}

func scopeEventInput(scope CalendarScope, input EventInput) EventInput {
	if locked := strings.TrimSpace(scope.LockedClientID); locked != "" {
		input.ClientID = locked
	}
	return input
}
