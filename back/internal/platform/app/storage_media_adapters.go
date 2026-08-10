package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
	objectstorage "github.com/mikewade2k16/lista-da-vez/back/internal/modules/storage"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tasks"
)

type storageServiceProvider func() *objectstorage.Service

type hybridCalendarMediaStorage struct {
	service storageServiceProvider
	local   *calendar.DiskMediaStorage
}

func (adapter *hybridCalendarMediaStorage) SaveStream(ctx context.Context, accountID, actorID, idempotencyKey, fileName, contentType string, sizeBytes int64, content io.Reader, limits calendar.MediaLimits) (calendar.MediaItem, error) {
	service := adapter.resolve()
	useR2, err := r2UploadsEnabled(ctx, service)
	if err != nil {
		return calendar.MediaItem{}, calendar.ErrMediaUnavailable
	}
	if !useR2 || sizeBytes < objectstorage.MultipartThresholdBytes {
		bytes, readErr := io.ReadAll(io.LimitReader(content, sizeBytes+1))
		if readErr != nil || int64(len(bytes)) != sizeBytes {
			return calendar.MediaItem{}, calendar.ErrInvalidMedia
		}
		return adapter.Save(ctx, accountID, actorID, idempotencyKey, fileName, contentType, bytes, limits)
	}
	upload, err := service.StageMultipart(ctx, objectstorage.StagedUploadInput{MultipartUploadInput: objectstorage.MultipartUploadInput{AccountID: accountID, SourceModule: "calendar", IdempotencyKey: resolvedStorageIdempotencyKey(idempotencyKey), FileName: fileName, ContentType: contentType, SizeBytes: sizeBytes, CreatedBy: actorID}, Content: content})
	if err != nil {
		return calendar.MediaItem{}, mapCalendarStorageError(err)
	}
	object := upload.Object
	kind := "image"
	if strings.HasPrefix(object.ContentType, "video/") {
		kind = "video"
	}
	return calendar.MediaItem{ID: object.ID, URL: fmt.Sprintf("/uploads/calendar/%s/%s/%s", object.AccountID, object.ID, url.PathEscape(object.FileName)), Name: object.FileName, Type: kind, ContentType: object.ContentType, SizeBytes: int(object.SizeBytes)}, nil
}

func newHybridCalendarMediaStorage(provider storageServiceProvider, local *calendar.DiskMediaStorage) *hybridCalendarMediaStorage {
	return &hybridCalendarMediaStorage{service: provider, local: local}
}

func (adapter *hybridCalendarMediaStorage) Save(
	ctx context.Context,
	accountID, actorID, idempotencyKey, fileName, contentType string,
	content []byte,
	limits calendar.MediaLimits,
) (calendar.MediaItem, error) {
	service := adapter.resolve()
	useR2, err := r2UploadsEnabled(ctx, service)
	if err != nil {
		return calendar.MediaItem{}, calendar.ErrMediaUnavailable
	}
	if !useR2 {
		if adapter == nil || adapter.local == nil {
			return calendar.MediaItem{}, calendar.ErrMediaUnavailable
		}
		return adapter.local.Save(ctx, accountID, actorID, idempotencyKey, fileName, contentType, content, limits)
	}
	object, err := service.Upload(ctx, objectstorage.UploadInput{
		AccountID:      accountID,
		SourceModule:   "calendar",
		IdempotencyKey: resolvedStorageIdempotencyKey(idempotencyKey),
		FileName:       fileName,
		ContentType:    contentType,
		Content:        content,
		CreatedBy:      actorID,
	})
	if err != nil {
		return calendar.MediaItem{}, mapCalendarStorageError(err)
	}
	kind := "image"
	if strings.HasPrefix(object.ContentType, "video/") {
		kind = "video"
	}
	return calendar.MediaItem{
		ID:          object.ID,
		URL:         fmt.Sprintf("/uploads/calendar/%s/%s/%s", object.AccountID, object.ID, url.PathEscape(object.FileName)),
		Name:        object.FileName,
		Type:        kind,
		ContentType: object.ContentType,
		SizeBytes:   int(object.SizeBytes),
	}, nil
}

func (adapter *hybridCalendarMediaStorage) Stat(ctx context.Context, accountID, objectID string) (calendar.MediaContent, error) {
	service := adapter.resolve()
	if service == nil {
		return calendar.MediaContent{}, calendar.ErrMediaUnavailable
	}
	object, err := service.ObjectMetadata(ctx, accountID, objectID, "calendar")
	if err != nil {
		return calendar.MediaContent{}, mapCalendarStorageReadError(err)
	}
	return calendar.MediaContent{
		FileName: object.FileName, ContentType: object.ContentType,
		SizeBytes: object.SizeBytes, ContentLength: object.SizeBytes, ETag: object.ETag,
	}, nil
}

func (adapter *hybridCalendarMediaStorage) Limits(ctx context.Context) (calendar.MediaLimits, error) {
	service := adapter.resolve()
	if service == nil {
		return calendar.MediaLimits{ImageMaxBytes: 1 << 62, VideoMaxBytes: 1 << 62}, nil
	}
	enabled, err := service.UploadsEnabled(ctx)
	if err != nil {
		return calendar.MediaLimits{}, calendar.ErrMediaUnavailable
	}
	if !enabled {
		return calendar.MediaLimits{ImageMaxBytes: 1 << 62, VideoMaxBytes: 1 << 62}, nil
	}
	settings, err := service.Settings(ctx)
	if err != nil {
		return calendar.MediaLimits{}, calendar.ErrMediaUnavailable
	}
	return calendar.MediaLimits{ImageMaxBytes: settings.ImageMaxBytes, VideoMaxBytes: settings.VideoMaxBytes, R2UploadsEnabled: true, MultipartThresholdBytes: objectstorage.MultipartThresholdBytes}, nil
}

func (adapter *hybridCalendarMediaStorage) Open(ctx context.Context, accountID, objectID, byteRange string) (calendar.MediaContent, error) {
	service := adapter.resolve()
	if service == nil {
		return calendar.MediaContent{}, calendar.ErrMediaUnavailable
	}
	object, content, err := service.DownloadForSource(ctx, accountID, objectID, "calendar", byteRange)
	if err != nil {
		return calendar.MediaContent{}, mapCalendarStorageReadError(err)
	}
	return calendar.MediaContent{
		FileName: object.FileName, ContentType: object.ContentType, SizeBytes: object.SizeBytes,
		Body: content.Body, ContentLength: content.ContentLength, ContentRange: content.ContentRange,
		ETag: content.ETag,
	}, nil
}

func (adapter *hybridCalendarMediaStorage) resolve() *objectstorage.Service {
	if adapter == nil || adapter.service == nil {
		return nil
	}
	return adapter.service()
}

type hybridTaskVideoStorage struct {
	service storageServiceProvider
	local   *tasks.DiskVideoStorage
}

func newHybridTaskVideoStorage(provider storageServiceProvider, local *tasks.DiskVideoStorage) *hybridTaskVideoStorage {
	return &hybridTaskVideoStorage{service: provider, local: local}
}

func (adapter *hybridTaskVideoStorage) Save(
	ctx context.Context,
	accountID, actorID, taskID, idempotencyKey, fileName, contentType string,
	content []byte,
) (*tasks.StoredTaskVideo, error) {
	service := adapter.resolve()
	useR2, err := r2UploadsEnabled(ctx, service)
	if err != nil {
		return nil, tasks.ErrVideoUnavailable
	}
	if !useR2 {
		if adapter == nil || adapter.local == nil {
			return nil, tasks.ErrVideoUnavailable
		}
		return adapter.local.Save(ctx, accountID, actorID, taskID, idempotencyKey, fileName, contentType, content)
	}
	object, err := service.Upload(ctx, objectstorage.UploadInput{
		AccountID: accountID, SourceModule: "tasks",
		IdempotencyKey: resolvedStorageIdempotencyKey(idempotencyKey),
		FileName:       fileName, ContentType: contentType, Content: content, CreatedBy: actorID,
	})
	if err != nil {
		return nil, mapTaskStorageError(err)
	}
	return &tasks.StoredTaskVideo{
		ID:          object.ID,
		Path:        fmt.Sprintf("/uploads/tasks/%s/%s/%s", object.AccountID, object.ID, url.PathEscape(object.FileName)),
		ContentType: object.ContentType,
		SizeBytes:   int(object.SizeBytes),
	}, nil
}

func (adapter *hybridTaskVideoStorage) SaveStream(ctx context.Context, accountID, actorID, taskID, idempotencyKey, fileName, contentType string, sizeBytes int64, content io.Reader) (*tasks.StoredTaskVideo, error) {
	service := adapter.resolve()
	useR2, err := r2UploadsEnabled(ctx, service)
	if err != nil {
		return nil, tasks.ErrVideoUnavailable
	}
	if !useR2 || sizeBytes < objectstorage.MultipartThresholdBytes {
		bytes, readErr := io.ReadAll(io.LimitReader(content, sizeBytes+1))
		if readErr != nil || int64(len(bytes)) != sizeBytes {
			return nil, tasks.ErrInvalidVideo
		}
		return adapter.Save(ctx, accountID, actorID, taskID, idempotencyKey, fileName, contentType, bytes)
	}
	upload, err := service.StageMultipart(ctx, objectstorage.StagedUploadInput{MultipartUploadInput: objectstorage.MultipartUploadInput{AccountID: accountID, SourceModule: "tasks", IdempotencyKey: resolvedStorageIdempotencyKey(idempotencyKey), FileName: fileName, ContentType: contentType, SizeBytes: sizeBytes, CreatedBy: actorID}, Content: content})
	if err != nil {
		return nil, mapTaskStorageError(err)
	}
	object := upload.Object
	return &tasks.StoredTaskVideo{ID: object.ID, Path: fmt.Sprintf("/uploads/tasks/%s/%s/%s", object.AccountID, object.ID, url.PathEscape(object.FileName)), ContentType: object.ContentType, SizeBytes: int(object.SizeBytes)}, nil
}

func (adapter *hybridTaskVideoStorage) Stat(ctx context.Context, accountID, objectID string) (tasks.TaskVideoContent, error) {
	service := adapter.resolve()
	if service == nil {
		return tasks.TaskVideoContent{}, tasks.ErrVideoUnavailable
	}
	object, err := service.ObjectMetadata(ctx, accountID, objectID, "tasks")
	if err != nil {
		return tasks.TaskVideoContent{}, mapTaskStorageReadError(err)
	}
	return tasks.TaskVideoContent{
		FileName: object.FileName, ContentType: object.ContentType,
		SizeBytes: object.SizeBytes, ContentLength: object.SizeBytes, ETag: object.ETag,
	}, nil
}

func (adapter *hybridTaskVideoStorage) MaxVideoBytes(ctx context.Context) (int64, error) {
	service := adapter.resolve()
	if service == nil {
		return 100 * 1024 * 1024, nil
	}
	enabled, err := service.UploadsEnabled(ctx)
	if err != nil {
		return 0, tasks.ErrVideoUnavailable
	}
	if !enabled {
		return 100 * 1024 * 1024, nil
	}
	settings, err := service.Settings(ctx)
	if err != nil {
		return 0, tasks.ErrVideoUnavailable
	}
	return settings.VideoMaxBytes, nil
}
func (adapter *hybridTaskVideoStorage) Open(ctx context.Context, accountID, objectID, byteRange string) (tasks.TaskVideoContent, error) {
	service := adapter.resolve()
	if service == nil {
		return tasks.TaskVideoContent{}, tasks.ErrVideoUnavailable
	}
	object, content, err := service.DownloadForSource(ctx, accountID, objectID, "tasks", byteRange)
	if err != nil {
		return tasks.TaskVideoContent{}, mapTaskStorageReadError(err)
	}
	return tasks.TaskVideoContent{
		FileName: object.FileName, ContentType: object.ContentType, SizeBytes: object.SizeBytes,
		Body: content.Body, ContentLength: content.ContentLength, ContentRange: content.ContentRange,
		ETag: content.ETag,
	}, nil
}

func (adapter *hybridTaskVideoStorage) resolve() *objectstorage.Service {
	if adapter == nil || adapter.service == nil {
		return nil
	}
	return adapter.service()
}

func r2UploadsEnabled(ctx context.Context, service *objectstorage.Service) (bool, error) {
	if service == nil {
		return false, nil
	}
	return service.UploadsEnabled(ctx)
}

func resolvedStorageIdempotencyKey(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("upload-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func mapCalendarStorageError(err error) error {
	switch {
	case errors.Is(err, objectstorage.ErrFileTypeLimit):
		return calendar.ErrMediaTooLarge
	case errors.Is(err, objectstorage.ErrInvalidUpload), errors.Is(err, objectstorage.ErrUnsupportedFileType):
		return calendar.ErrInvalidMedia
	default:
		return fmt.Errorf("%w: %v", calendar.ErrMediaUnavailable, err)
	}
}

func mapCalendarStorageReadError(err error) error {
	switch {
	case errors.Is(err, objectstorage.ErrObjectNotFound):
		return calendar.ErrNotFound
	case errors.Is(err, objectstorage.ErrInvalidRange):
		return calendar.ErrInvalidMediaRange
	default:
		return fmt.Errorf("%w: %v", calendar.ErrMediaUnavailable, err)
	}
}

func mapTaskStorageError(err error) error {
	switch {
	case errors.Is(err, objectstorage.ErrFileTypeLimit):
		return tasks.ErrVideoTooLarge
	case errors.Is(err, objectstorage.ErrInvalidUpload), errors.Is(err, objectstorage.ErrUnsupportedFileType):
		return tasks.ErrInvalidVideo
	default:
		return fmt.Errorf("%w: %v", tasks.ErrVideoUnavailable, err)
	}
}

func mapTaskStorageReadError(err error) error {
	switch {
	case errors.Is(err, objectstorage.ErrObjectNotFound):
		return tasks.ErrTaskNotFound
	case errors.Is(err, objectstorage.ErrInvalidRange):
		return tasks.ErrInvalidVideoRange
	default:
		return fmt.Errorf("%w: %v", tasks.ErrVideoUnavailable, err)
	}
}
