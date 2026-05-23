package tasks

import (
	"context"
	"strings"
	"time"
)

func (service *Service) UploadTaskVideo(ctx context.Context, access AccessContext, taskID string, upload *TaskVideoUpload) (TaskVideo, error) {
	if !access.Has(PermTasksEdit) {
		return TaskVideo{}, ErrForbidden
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || upload == nil || len(upload.Content) == 0 {
		return TaskVideo{}, ErrInvalidVideo
	}
	if service.videoStore == nil {
		return TaskVideo{}, ErrInvalidVideo
	}
	if _, err := service.repository.GetTask(ctx, access, taskID); err != nil {
		return TaskVideo{}, err
	}

	stored, err := service.videoStore.Save(ctx, taskID, upload.FileName, upload.ContentType, upload.Content)
	if err != nil {
		return TaskVideo{}, err
	}

	name := strings.TrimSpace(upload.FileName)
	if name == "" {
		name = stored.ID
	}

	return TaskVideo{
		ID:          strings.TrimSpace(stored.ID),
		Name:        name,
		URL:         strings.TrimSpace(stored.Path),
		Size:        stored.SizeBytes,
		ContentType: strings.TrimSpace(stored.ContentType),
		UploadedAt:  time.Now().UTC(),
	}, nil
}