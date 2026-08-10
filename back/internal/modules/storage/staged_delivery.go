package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (service *Service) StageMultipart(ctx context.Context, input StagedUploadInput) (MultipartUpload, error) {
	if input.Content == nil || input.SizeBytes < MultipartThresholdBytes {
		return MultipartUpload{}, ErrInvalidUpload
	}
	root := strings.TrimSpace(service.config.UploadsDir)
	if root == "" {
		return MultipartUpload{}, ErrDisabled
	}
	dir := filepath.Join(root, "storage-staging")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return MultipartUpload{}, err
	}
	stagingID, err := randomObjectID()
	if err != nil {
		return MultipartUpload{}, err
	}
	path := filepath.Join(dir, stagingID+".original")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return MultipartUpload{}, err
	}
	written, copyErr := io.Copy(file, io.LimitReader(input.Content, input.SizeBytes+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written != input.SizeBytes {
		_ = os.Remove(path)
		return MultipartUpload{}, ErrInvalidUpload
	}
	probe, err := os.Open(path)
	if err != nil {
		_ = os.Remove(path)
		return MultipartUpload{}, err
	}
	head := make([]byte, minInt64(512, input.SizeBytes))
	_, err = io.ReadFull(probe, head)
	_ = probe.Close()
	if err != nil {
		_ = os.Remove(path)
		return MultipartUpload{}, ErrInvalidUpload
	}
	if _, err = validatedContentType(input.ContentType, head); err != nil {
		_ = os.Remove(path)
		return MultipartUpload{}, err
	}
	upload, _, err := service.reserveMultipart(ctx, input.MultipartUploadInput)
	if err != nil {
		_ = os.Remove(path)
		return MultipartUpload{}, err
	}
	if upload.Status == "completed" {
		_ = os.Remove(path)
		return upload, nil
	}
	repository, ok := service.repository.(MultipartRepository)
	if !ok {
		_ = os.Remove(path)
		return MultipartUpload{}, ErrDisabled
	}
	if _, existingErr := repository.MultipartDeliveryPath(ctx, upload.Object.AccountID, upload.Object.SourceModule, upload.Object.ID); existingErr == nil {
		_ = os.Remove(path)
		service.wakeDeliveryWorker()
		return upload, nil
	}
	err = repository.EnqueueMultipartDelivery(ctx, MultipartDelivery{UploadID: upload.ID, AccountID: upload.Object.AccountID, SourceModule: upload.Object.SourceModule, CreatedBy: upload.CreatedBy, StagingPath: path})
	if err != nil {
		_ = os.Remove(path)
		return MultipartUpload{}, err
	}
	service.wakeDeliveryWorker()
	return upload, nil
}

func (service *Service) startDeliveryWorker() {
	if !service.config.Enabled || strings.TrimSpace(service.config.UploadsDir) == "" {
		return
	}
	if _, ok := service.repository.(MultipartRepository); !ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.workerCancel = cancel
	service.workerWake = make(chan struct{}, 1)
	service.workerWG.Add(1)
	go func() {
		defer service.workerWG.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			service.processDeliveries(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-service.workerWake:
			}
		}
	}()
}

func (service *Service) wakeDeliveryWorker() {
	if service.workerWake == nil {
		return
	}
	select {
	case service.workerWake <- struct{}{}:
	default:
	}
}

func (service *Service) processDeliveries(ctx context.Context) {
	repository := service.repository.(MultipartRepository)
	for {
		if ctx.Err() != nil {
			return
		}
		delivery, err := repository.ClaimMultipartDelivery(ctx)
		if errors.Is(err, ErrObjectNotFound) {
			return
		}
		if err != nil {
			return
		}
		if err = service.deliverStaged(ctx, delivery); err != nil {
			delay := time.Duration(minInt(delivery.Attempts, 8)) * 15 * time.Second
			_ = repository.RetryMultipartDelivery(ctx, delivery.UploadID, safeDeliveryError(err), service.now().Add(delay))
			continue
		}
		_ = repository.CompleteMultipartDelivery(ctx, delivery.UploadID)
		_ = os.Remove(delivery.StagingPath)
	}
}

func (service *Service) deliverStaged(ctx context.Context, delivery MultipartDelivery) error {
	upload, err := service.Multipart(ctx, delivery.AccountID, delivery.SourceModule, delivery.UploadID, delivery.CreatedBy)
	if err != nil {
		return err
	}
	if upload.Status == "completed" {
		return nil
	}
	if upload.Status == "creating" {
		upload, err = service.activateMultipartProvider(ctx, upload)
		if err != nil {
			return err
		}
	}
	file, err := os.Open(delivery.StagingPath)
	if err != nil {
		return err
	}
	defer file.Close()
	uploaded := make(map[int]bool, len(upload.UploadedParts))
	for _, part := range upload.UploadedParts {
		uploaded[part.PartNumber] = true
	}
	for number := 1; number <= upload.PartCount; number++ {
		if uploaded[number] {
			continue
		}
		start := int64(number-1) * upload.PartSizeBytes
		size := upload.PartSizeBytes
		if number == upload.PartCount {
			size = upload.Object.SizeBytes - start
		}
		if _, err = file.Seek(start, io.SeekStart); err != nil {
			return err
		}
		content := make([]byte, size)
		if _, err = io.ReadFull(file, content); err != nil {
			return err
		}
		if _, err = service.UploadMultipartPart(ctx, delivery.AccountID, delivery.SourceModule, delivery.UploadID, delivery.CreatedBy, number, content); err != nil {
			return err
		}
	}
	_, err = service.CompleteMultipart(ctx, delivery.AccountID, delivery.SourceModule, delivery.UploadID, delivery.CreatedBy)
	return err
}

func (service *Service) downloadStaged(ctx context.Context, object Object, byteRange string) (Object, ObjectContent, error) {
	repository, ok := service.repository.(MultipartRepository)
	if !ok {
		return Object{}, ObjectContent{}, ErrObjectNotFound
	}
	path, err := repository.MultipartDeliveryPath(ctx, object.AccountID, object.SourceModule, object.ID)
	if err != nil {
		return Object{}, ObjectContent{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return Object{}, ObjectContent{}, ErrObjectNotFound
	}
	if byteRange == "" {
		return object, ObjectContent{Body: file, ContentLength: object.SizeBytes}, nil
	}
	start, end, err := parseStagedRange(byteRange, object.SizeBytes)
	if err != nil {
		_ = file.Close()
		return Object{}, ObjectContent{}, ErrInvalidRange
	}
	if _, err = file.Seek(start, io.SeekStart); err != nil {
		_ = file.Close()
		return Object{}, ObjectContent{}, err
	}
	length := end - start + 1
	return object, ObjectContent{Body: &limitedFileReadCloser{Reader: io.LimitReader(file, length), file: file}, ContentLength: length, ContentRange: fmt.Sprintf("bytes %d-%d/%d", start, end, object.SizeBytes)}, nil
}

type limitedFileReadCloser struct {
	io.Reader
	file *os.File
}

func (reader *limitedFileReadCloser) Close() error { return reader.file.Close() }

func parseStagedRange(value string, size int64) (int64, int64, error) {
	parts := strings.Split(strings.TrimPrefix(value, "bytes="), "-")
	if len(parts) != 2 {
		return 0, 0, ErrInvalidRange
	}
	if parts[0] == "" {
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffix <= 0 {
			return 0, 0, ErrInvalidRange
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, nil
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, ErrInvalidRange
	}
	end := size - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return 0, 0, ErrInvalidRange
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, nil
}

func safeDeliveryError(err error) string {
	message := err.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func minInt64(a, b int64) int {
	if a < b {
		return int(a)
	}
	return int(b)
}
