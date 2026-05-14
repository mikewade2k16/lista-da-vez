package tasks

import (
	"context"
	"strings"
)

func (service *Service) ListActiveTimeEntries(ctx context.Context, access AccessContext) ([]TimeEntry, error) {
	if !access.Has(PermTrackingUse) && !access.Has(PermTrackingViewAll) {
		return nil, ErrForbidden
	}
	return service.repository.ListActiveTimeEntries(ctx, access)
}

func (service *Service) StartTracking(ctx context.Context, access AccessContext, taskID string) (TimeEntry, error) {
	if !access.Has(PermTrackingUse) {
		return TimeEntry{}, ErrForbidden
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return TimeEntry{}, ErrValidation
	}
	task, err := service.repository.GetTask(ctx, access, taskID)
	if err != nil {
		return TimeEntry{}, err
	}
	entry, err := service.repository.StartTracking(ctx, access.AccountID, taskID, access.UserID)
	if err != nil {
		return TimeEntry{}, err
	}
	service.audit(ctx, access, "task.time_started", "task", taskID, nil, entry)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.time_started", AccountID: access.AccountID, BoardID: task.BoardID, TaskID: task.ID, Version: task.Version})
	return entry, nil
}

func (service *Service) PauseTracking(ctx context.Context, access AccessContext, taskID string, expectedVersion *int) (TimeEntry, error) {
	if !access.Has(PermTrackingUse) {
		return TimeEntry{}, ErrForbidden
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return TimeEntry{}, ErrValidation
	}
	task, err := service.repository.GetTask(ctx, access, taskID)
	if err != nil {
		return TimeEntry{}, err
	}
	entry, err := service.repository.PauseTracking(ctx, access.AccountID, taskID, access.UserID, expectedVersion)
	if err != nil {
		return TimeEntry{}, err
	}
	service.audit(ctx, access, "task.time_paused", "task", taskID, nil, entry)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.time_paused", AccountID: access.AccountID, BoardID: task.BoardID, TaskID: task.ID, Version: task.Version})
	return entry, nil
}

func (service *Service) ResumeTracking(ctx context.Context, access AccessContext, taskID string, expectedVersion *int) (TimeEntry, error) {
	if !access.Has(PermTrackingUse) {
		return TimeEntry{}, ErrForbidden
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return TimeEntry{}, ErrValidation
	}
	task, err := service.repository.GetTask(ctx, access, taskID)
	if err != nil {
		return TimeEntry{}, err
	}
	entry, err := service.repository.ResumeTracking(ctx, access.AccountID, taskID, access.UserID, expectedVersion)
	if err != nil {
		return TimeEntry{}, err
	}
	service.audit(ctx, access, "task.time_resumed", "task", taskID, nil, entry)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.time_resumed", AccountID: access.AccountID, BoardID: task.BoardID, TaskID: task.ID, Version: task.Version})
	return entry, nil
}

func (service *Service) StopTracking(ctx context.Context, access AccessContext, taskID string, expectedVersion *int) (TimeEntry, error) {
	if !access.Has(PermTrackingUse) {
		return TimeEntry{}, ErrForbidden
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return TimeEntry{}, ErrValidation
	}
	task, err := service.repository.GetTask(ctx, access, taskID)
	if err != nil {
		return TimeEntry{}, err
	}
	entry, err := service.repository.StopTracking(ctx, access.AccountID, taskID, access.UserID, expectedVersion)
	if err != nil {
		return TimeEntry{}, err
	}
	service.audit(ctx, access, "task.time_stopped", "task", taskID, nil, entry)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.time_stopped", AccountID: access.AccountID, BoardID: task.BoardID, TaskID: task.ID, Version: task.Version})
	return entry, nil
}

func (service *Service) TrackingMetrics(ctx context.Context, access AccessContext, input TrackingMetricsInput) (TrackingMetrics, error) {
	if !access.Has(PermTrackingViewAll) {
		return TrackingMetrics{}, ErrForbidden
	}
	return service.repository.TrackingMetrics(ctx, access.AccountID, input)
}
