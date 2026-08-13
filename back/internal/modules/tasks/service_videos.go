package tasks

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"
)

func (service *Service) TaskVideoMaxBytes(ctx context.Context) int64 {
	return service.TaskMediaLimits(ctx).VideoMaxBytes
}

func (service *Service) TaskMediaLimits(ctx context.Context) TaskMediaLimits {
	fallback := TaskMediaLimits{ImageMaxBytes: maxTaskImageBytes, VideoMaxBytes: maxTaskVideoBytes}
	if storage, ok := service.videoStore.(TaskMediaLimitStorage); ok {
		limits, err := storage.MediaLimits(ctx)
		if err == nil && limits.ImageMaxBytes > 0 && limits.VideoMaxBytes > 0 {
			return limits
		}
	}
	storage, ok := service.videoStore.(TaskVideoLimitStorage)
	if !ok {
		return fallback
	}
	limit, err := storage.MaxVideoBytes(ctx)
	if err != nil || limit <= 0 {
		return fallback
	}
	fallback.VideoMaxBytes = limit
	return fallback
}

func (service *Service) UploadTaskVideoStream(ctx context.Context, access AccessContext, taskID, idempotencyKey, fileName, contentType, checklistItemID string, sizeBytes int64, content io.Reader) (TaskVideo, error) {
	if !access.Has(PermTasksEdit) {
		return TaskVideo{}, ErrForbidden
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || content == nil || sizeBytes <= 0 {
		return TaskVideo{}, ErrInvalidVideo
	}
	if _, err := service.repository.GetTask(ctx, access, taskID); err != nil {
		return TaskVideo{}, err
	}
	storage, ok := service.videoStore.(TaskVideoStreamStorage)
	if !ok {
		return TaskVideo{}, ErrVideoUnavailable
	}
	stored, err := storage.SaveStream(ctx, access.AccountID, access.UserID, taskID, idempotencyKey, fileName, contentType, sizeBytes, content)
	if err != nil {
		service.logVideoUploadFailure(ctx, access, taskID, err)
		return TaskVideo{}, err
	}
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = stored.ID
	}
	return TaskVideo{ID: stored.ID, Name: name, URL: stored.Path, Size: stored.SizeBytes, ContentType: stored.ContentType, ChecklistItemID: strings.TrimSpace(checklistItemID), UploadedAt: time.Now().UTC()}, nil
}

func (service *Service) logVideoUploadFailure(ctx context.Context, access AccessContext, taskID string, err error) {
	if service.logger == nil || (!errors.Is(err, ErrVideoMetricsUnavailable) &&
		!errors.Is(err, ErrVideoQuotaExceeded) && !errors.Is(err, ErrVideoUnavailable)) {
		return
	}
	kind := "storage_unavailable"
	switch {
	case errors.Is(err, ErrVideoMetricsUnavailable):
		kind = "metrics_unavailable"
	case errors.Is(err, ErrVideoQuotaExceeded):
		kind = "quota_exceeded"
	}
	service.logger.LogAttrs(ctx, slog.LevelWarn, "tasks.video_upload_failed",
		slog.String("account_id", access.AccountID),
		slog.String("user_id", access.UserID),
		slog.String("task_id", strings.TrimSpace(taskID)),
		slog.String("reason", kind),
		slog.String("error", err.Error()),
	)
}

func (service *Service) StatTaskVideo(ctx context.Context, accountID, objectID string) (TaskVideoContent, error) {
	storage, ok := service.videoStore.(TaskVideoContentStorage)
	if !ok {
		return TaskVideoContent{}, ErrVideoUnavailable
	}
	return storage.Stat(ctx, strings.TrimSpace(accountID), strings.TrimSpace(objectID))
}

func (service *Service) OpenTaskVideo(ctx context.Context, accountID, objectID, byteRange string) (TaskVideoContent, error) {
	storage, ok := service.videoStore.(TaskVideoContentStorage)
	if !ok {
		return TaskVideoContent{}, ErrVideoUnavailable
	}
	return storage.Open(ctx, strings.TrimSpace(accountID), strings.TrimSpace(objectID), strings.TrimSpace(byteRange))
}
