package storage

import (
	"context"
	"fmt"
	"math"
	"strings"
)

func (service *Service) BeginMultipart(ctx context.Context, input MultipartUploadInput) (MultipartUpload, error) {
	reserved, existing, err := service.reserveMultipart(ctx, input)
	if err != nil || existing {
		return reserved, err
	}
	return service.activateMultipartProvider(ctx, reserved)
}

func (service *Service) reserveMultipart(ctx context.Context, input MultipartUploadInput) (MultipartUpload, bool, error) {
	repository, repoOK := service.repository.(MultipartRepository)
	_, clientOK := service.client.(MultipartObjectClient)
	if !service.config.Enabled || !repoOK || !clientOK {
		return MultipartUpload{}, false, ErrDisabled
	}
	state, err := service.repository.ProviderState(ctx)
	if err != nil {
		return MultipartUpload{}, false, err
	}
	if err = service.ensureMatchingProvider(state); err != nil {
		return MultipartUpload{}, false, err
	}
	settings, err := service.repository.Settings(ctx)
	if err != nil {
		return MultipartUpload{}, false, err
	}
	if !settings.UploadsEnabled {
		return MultipartUpload{}, false, ErrUploadsDisabled
	}
	object, err := service.prepareMultipartObject(input, settings)
	if err != nil {
		return MultipartUpload{}, false, err
	}
	partCount := int(math.Ceil(float64(object.SizeBytes) / float64(MultipartPartSizeBytes)))
	planned := int64(partCount + 2)
	cloud := service.readCloudUsageForUpload(ctx, settings.BillingCycleDay)
	if !cloud.Available {
		return MultipartUpload{}, false, ErrAnalyticsUnavailable
	}
	local, err := service.repository.Usage(ctx, billingMonth(service.now()))
	if err != nil {
		return MultipartUpload{}, false, err
	}
	if cloud.StoredBytes+cloud.MetadataBytes+local.UploadedBytes+local.PendingBytes+object.SizeBytes > settings.StorageLimitBytes {
		return MultipartUpload{}, false, ErrStorageQuotaExceeded
	}
	if cloud.ClassARequests+local.ClassARequests+planned > settings.ClassALimit {
		return MultipartUpload{}, false, ErrClassAQuotaExceeded
	}
	sessionID, err := randomObjectID()
	if err != nil {
		return MultipartUpload{}, false, err
	}
	upload := MultipartUpload{ID: sessionID, Object: object, PartSizeBytes: MultipartPartSizeBytes, PartCount: partCount, Status: "creating", CreatedBy: object.CreatedBy}
	reserved, existing, err := repository.ReserveMultipart(ctx, upload, billingMonth(service.now()), planned)
	return reserved, existing, err
}

func (service *Service) activateMultipartProvider(ctx context.Context, reserved MultipartUpload) (MultipartUpload, error) {
	client := service.client.(MultipartObjectClient)
	repository := service.repository.(MultipartRepository)
	if err := repository.BeginMultipartProviderAttempt(ctx, reserved.ID, billingMonth(service.now())); err != nil {
		return MultipartUpload{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, service.config.RequestTimeout)
	defer cancel()
	providerID, err := client.CreateMultipartUpload(requestCtx, reserved.Object.ObjectKey, reserved.Object.ContentType)
	if err != nil {
		return MultipartUpload{}, fmt.Errorf("R2 CreateMultipartUpload: %w", err)
	}
	if err = repository.ActivateMultipart(ctx, reserved.ID, providerID); err != nil {
		abortCtx, abortCancel := context.WithTimeout(context.Background(), service.config.RequestTimeout)
		defer abortCancel()
		_ = client.AbortMultipartUpload(abortCtx, reserved.Object.ObjectKey, providerID)
		return MultipartUpload{}, err
	}
	reserved.ProviderUploadID = providerID
	reserved.Status = "uploading"
	return reserved, nil
}

func (service *Service) Multipart(ctx context.Context, accountID, source, uploadID, actorID string) (MultipartUpload, error) {
	repository, ok := service.repository.(MultipartRepository)
	if !ok {
		return MultipartUpload{}, ErrDisabled
	}
	upload, err := repository.Multipart(ctx, strings.TrimSpace(accountID), strings.TrimSpace(source), strings.TrimSpace(uploadID))
	if err != nil {
		return MultipartUpload{}, err
	}
	if upload.CreatedBy != strings.TrimSpace(actorID) {
		return MultipartUpload{}, ErrObjectNotFound
	}
	return upload, nil
}

func (service *Service) UploadMultipartPart(ctx context.Context, accountID, source, uploadID, actorID string, number int, content []byte) (MultipartPart, error) {
	repository, repoOK := service.repository.(MultipartRepository)
	client, clientOK := service.client.(MultipartObjectClient)
	if !repoOK || !clientOK {
		return MultipartPart{}, ErrDisabled
	}
	upload, err := service.Multipart(ctx, accountID, source, uploadID, actorID)
	if err != nil {
		return MultipartPart{}, err
	}
	if upload.Status != "uploading" || number < 1 || number > upload.PartCount || len(content) == 0 {
		return MultipartPart{}, ErrInvalidUpload
	}
	expected := upload.PartSizeBytes
	if number == upload.PartCount {
		expected = upload.Object.SizeBytes - int64(number-1)*upload.PartSizeBytes
	}
	if int64(len(content)) != expected {
		return MultipartPart{}, ErrInvalidUpload
	}
	if number == 1 {
		if _, err = validatedContentType(upload.Object.ContentType, content); err != nil {
			return MultipartPart{}, err
		}
	}
	existing, err := repository.BeginPartAttempt(ctx, upload.ID, number, billingMonth(service.now()))
	if err != nil {
		return MultipartPart{}, err
	}
	if existing != nil {
		return *existing, nil
	}
	requestCtx, cancel := context.WithTimeout(ctx, service.config.UploadTimeout)
	defer cancel()
	etag, err := client.UploadPart(requestCtx, upload.Object.ObjectKey, upload.ProviderUploadID, number, content)
	if err != nil {
		return MultipartPart{}, fmt.Errorf("R2 UploadPart outcome is ambiguous; durable delivery will retry: %w", err)
	}
	part := MultipartPart{PartNumber: number, ETag: etag, SizeBytes: int64(len(content))}
	if err = repository.SaveMultipartPart(ctx, upload.ID, part); err != nil {
		return MultipartPart{}, err
	}
	return part, nil
}

func (service *Service) CompleteMultipart(ctx context.Context, accountID, source, uploadID, actorID string) (Object, error) {
	repository, repoOK := service.repository.(MultipartRepository)
	client, clientOK := service.client.(MultipartObjectClient)
	if !repoOK || !clientOK {
		return Object{}, ErrDisabled
	}
	upload, err := service.Multipart(ctx, accountID, source, uploadID, actorID)
	if err != nil {
		return Object{}, err
	}
	if upload.Status == "completed" {
		return upload.Object, nil
	}
	parts, err := repository.BeginMultipartCompletion(ctx, upload.ID, billingMonth(service.now()))
	if err != nil {
		return Object{}, err
	}
	if len(parts) != upload.PartCount {
		return Object{}, ErrInvalidUpload
	}
	var total int64
	for i, p := range parts {
		if p.PartNumber != i+1 {
			return Object{}, ErrInvalidUpload
		}
		total += p.SizeBytes
	}
	if total != upload.Object.SizeBytes {
		return Object{}, ErrInvalidUpload
	}
	requestCtx, cancel := context.WithTimeout(ctx, service.config.UploadTimeout)
	defer cancel()
	etag, err := client.CompleteMultipartUpload(requestCtx, upload.Object.ObjectKey, upload.ProviderUploadID, parts)
	if err != nil {
		return Object{}, fmt.Errorf("R2 CompleteMultipartUpload outcome is ambiguous: %w", err)
	}
	return repository.CompleteMultipart(ctx, upload.ID, etag)
}

func (service *Service) prepareMultipartObject(input MultipartUploadInput, settings Settings) (Object, error) {
	accountID, createdBy, source, key := strings.TrimSpace(input.AccountID), strings.TrimSpace(input.CreatedBy), strings.TrimSpace(input.SourceModule), strings.TrimSpace(input.IdempotencyKey)
	fileName := sanitizeFileName(input.FileName)
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(input.ContentType, ";")[0]))
	if !accountIDPattern.MatchString(accountID) || !accountIDPattern.MatchString(createdBy) || !sourceModulePattern.MatchString(source) || key == "" || len(key) > 160 || fileName == "" || input.SizeBytes < MultipartThresholdBytes {
		return Object{}, ErrInvalidUpload
	}
	limit, err := fileTypeLimit(contentType, settings)
	if err != nil {
		return Object{}, err
	}
	if input.SizeBytes > limit {
		return Object{}, ErrFileTypeLimit
	}
	id, err := randomObjectID()
	if err != nil {
		return Object{}, err
	}
	now := service.now().UTC()
	return Object{ID: id, AccountID: accountID, SourceModule: source, IdempotencyKey: key, ObjectKey: fmt.Sprintf("accounts/%s/%s/%04d/%s--%s", accountID, source, now.Year(), id, fileName), FileName: fileName, ContentType: contentType, SizeBytes: input.SizeBytes, Status: "pending", CreatedBy: createdBy, CreatedAt: now}, nil
}
