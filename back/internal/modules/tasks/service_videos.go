package tasks

import (
	"context"
	"io"
	"strings"
	"time"
)

func (service *Service) TaskVideoMaxBytes(ctx context.Context) int64 {
	storage, ok := service.videoStore.(TaskVideoLimitStorage)
	if !ok {
		return maxTaskVideoBytes
	}
	limit, err := storage.MaxVideoBytes(ctx)
	if err != nil || limit <= 0 {
		return maxTaskVideoBytes
	}
	return limit
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
		return TaskVideo{}, err
	}
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = stored.ID
	}
	return TaskVideo{ID: stored.ID, Name: name, URL: stored.Path, Size: stored.SizeBytes, ContentType: stored.ContentType, ChecklistItemID: strings.TrimSpace(checklistItemID), UploadedAt: time.Now().UTC()}, nil
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
