package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	accountIDPattern         = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	cloudflareAccountPattern = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
	bucketPattern            = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,61}[a-z0-9])$`)
	sourceModulePattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
	byteRangePattern         = regexp.MustCompile(`^bytes=\d*-\d*$`)
)

type Service struct {
	config           Config
	repository       Repository
	client           ObjectClient
	usageClient      UsageClient
	now              func() time.Time
	usageMu          sync.Mutex
	cachedCloudUsage CloudUsage
	workerCancel     context.CancelFunc
	workerWG         sync.WaitGroup
	workerWake       chan struct{}
}

func NewService(cfg Config, repository Repository, client ObjectClient, usageClients ...UsageClient) *Service {
	service := &Service{
		config:     cfg,
		repository: repository,
		client:     client,
		now:        time.Now,
	}
	if len(usageClients) > 0 {
		service.usageClient = usageClients[0]
	}
	service.startDeliveryWorker()
	return service
}

func (service *Service) Close() {
	if service.workerCancel != nil {
		service.workerCancel()
		service.workerWG.Wait()
	}
}

func ValidateConfig(cfg Config) error {
	if !cfg.Enabled {
		return nil
	}
	if !cloudflareAccountPattern.MatchString(strings.TrimSpace(cfg.AccountID)) ||
		!bucketPattern.MatchString(strings.TrimSpace(cfg.Bucket)) ||
		strings.TrimSpace(cfg.AccessKeyID) == "" ||
		strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return fmt.Errorf("%w: R2 account, bucket and credentials are required", ErrInvalidConfig)
	}
	if cfg.RequestTimeout <= 0 {
		return fmt.Errorf("%w: request timeout must be positive", ErrInvalidConfig)
	}
	if cfg.UploadTimeout <= 0 {
		return fmt.Errorf("%w: upload timeout must be positive", ErrInvalidConfig)
	}
	return nil
}

func (service *Service) Status(ctx context.Context) (Status, error) {
	settings, err := service.repository.Settings(ctx)
	if err != nil {
		return Status{}, err
	}
	usage, err := service.repository.Usage(ctx, billingMonth(service.now()))
	if err != nil {
		return Status{}, err
	}
	status := Status{
		Enabled:  service.config.Enabled,
		Provider: providerName,
		Settings: settings,
		Usage:    usage,
	}
	// O painel administrativo e uma barreira financeira: cada leitura consulta
	// o provider, em vez de exibir um snapshot potencialmente vencido do processo.
	status.CloudUsage = service.readCloudUsage(ctx, true, settings.BillingCycleDay)
	if !service.config.Enabled {
		return status, nil
	}
	status.Bucket = service.config.Bucket
	state, err := service.repository.ProviderState(ctx)
	switch {
	case err == nil:
		if err := service.ensureMatchingProvider(state); err != nil {
			return Status{}, err
		}
		status.Initialized = true
	case errors.Is(err, ErrNotInitialized):
		status.Initialized = false
	default:
		return Status{}, err
	}
	return status, nil
}

func (service *Service) Settings(ctx context.Context) (Settings, error) {
	return service.repository.Settings(ctx)
}

// UploadsEnabled resolve o seletor operacional persistido sem afetar a leitura
// de objetos R2 ja existentes. Credenciais ausentes sempre forcam o modo local.
func (service *Service) UploadsEnabled(ctx context.Context) (bool, error) {
	settings, err := service.repository.Settings(ctx)
	if err != nil {
		return false, err
	}
	return service.config.Enabled && settings.UploadsEnabled, nil
}

func (service *Service) UpdateSettings(
	ctx context.Context,
	input UpdateSettingsInput,
	actorID string,
) (Settings, error) {
	if !accountIDPattern.MatchString(strings.TrimSpace(actorID)) {
		return Settings{}, ErrInvalidSettings
	}
	if err := ValidateSettings(input); err != nil {
		return Settings{}, err
	}
	if input.UploadsEnabled && strings.TrimSpace(service.config.AnalyticsToken) == "" {
		return Settings{}, ErrAnalyticsUnavailable
	}
	return service.repository.UpdateSettings(ctx, input, strings.TrimSpace(actorID))
}

func ValidateSettings(input UpdateSettingsInput) error {
	fileLimits := []int64{
		input.ImageMaxBytes,
		input.VideoMaxBytes,
		input.AudioMaxBytes,
		input.DocumentMaxBytes,
	}
	if input.StorageLimitBytes <= 0 || input.StorageLimitBytes > OfficialFreeStorageBytes ||
		input.ClassALimit <= 0 || input.ClassALimit > OfficialFreeClassAOps ||
		input.ClassBLimit <= 0 || input.ClassBLimit > OfficialFreeClassBOps ||
		input.BillingCycleDay < 1 || input.BillingCycleDay > 28 {
		return ErrInvalidSettings
	}
	for _, limit := range fileLimits {
		if limit <= 0 || limit > MaxManagedObjectBytes || limit > input.StorageLimitBytes {
			return ErrInvalidSettings
		}
	}
	return nil
}

func (service *Service) CheckConnection(ctx context.Context) (Status, error) {
	if !service.config.Enabled || service.client == nil {
		return Status{}, ErrDisabled
	}
	month := billingMonth(service.now())
	if err := service.repository.ReserveRequest(ctx, "B", month); err != nil {
		return Status{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, service.config.RequestTimeout)
	defer cancel()
	if err := service.client.HeadBucket(requestCtx); err != nil {
		return Status{}, fmt.Errorf("R2 HeadBucket: %w", err)
	}

	state, err := service.repository.ProviderState(ctx)
	if errors.Is(err, ErrNotInitialized) {
		if err := service.initializeProvider(ctx); err != nil {
			return Status{}, err
		}
	} else if err != nil {
		return Status{}, err
	} else if err := service.ensureMatchingProvider(state); err != nil {
		return Status{}, err
	}
	if err := service.repository.TouchProvider(ctx); err != nil {
		return Status{}, err
	}
	if err := service.reconcilePendingUploads(ctx); err != nil {
		return Status{}, err
	}
	return service.Status(ctx)
}

func (service *Service) reconcilePendingUploads(ctx context.Context) error {
	repository, repositoryOK := service.repository.(PendingObjectRepository)
	client, clientOK := service.client.(ObjectProbeClient)
	if !repositoryOK || !clientOK {
		return nil
	}
	objects, err := repository.PendingObjects(ctx, 100)
	if err != nil {
		return err
	}
	for _, object := range objects {
		if err := service.repository.ReserveRequest(ctx, "B", billingMonth(service.now())); err != nil {
			return err
		}
		requestCtx, cancel := context.WithTimeout(ctx, service.config.RequestTimeout)
		etag, exists, probeErr := client.HeadObject(requestCtx, object.ObjectKey)
		cancel()
		if probeErr != nil {
			return fmt.Errorf("R2 HeadObject reconciliation: %w", probeErr)
		}
		if exists {
			if _, err := service.repository.MarkAvailable(ctx, object.AccountID, object.ID, etag); err != nil {
				return err
			}
			continue
		}
		// R2 oferece consistencia forte, mas ainda preservamos uma janela maior
		// que o timeout total do PutObject antes de concluir que o envio falhou.
		// Assim um request ainda em curso nunca tem sua reserva liberada cedo.
		if service.now().Sub(object.CreatedAt) >= service.config.UploadTimeout {
			if err := service.repository.MarkFailed(ctx, object.AccountID, object.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (service *Service) Upload(ctx context.Context, input UploadInput) (Object, error) {
	if !service.config.Enabled || service.client == nil {
		return Object{}, ErrDisabled
	}
	state, err := service.repository.ProviderState(ctx)
	if err != nil {
		return Object{}, err
	}
	if err := service.ensureMatchingProvider(state); err != nil {
		return Object{}, err
	}
	settings, err := service.repository.Settings(ctx)
	if err != nil {
		return Object{}, err
	}
	if !settings.UploadsEnabled {
		return Object{}, ErrUploadsDisabled
	}

	object, err := service.prepareObject(input, settings)
	if err != nil {
		return Object{}, err
	}
	cloudUsage := service.readCloudUsage(ctx, true, settings.BillingCycleDay)
	if !cloudUsage.Available {
		return Object{}, ErrAnalyticsUnavailable
	}
	localUsage, err := service.repository.Usage(ctx, billingMonth(service.now()))
	if err != nil {
		return Object{}, err
	}
	if cloudUsage.StoredBytes+cloudUsage.MetadataBytes+localUsage.PendingBytes+object.SizeBytes > settings.StorageLimitBytes {
		return Object{}, ErrStorageQuotaExceeded
	}
	if cloudUsage.ClassARequests+1 > settings.ClassALimit {
		return Object{}, ErrClassAQuotaExceeded
	}
	reserved, existing, err := service.repository.ReserveUpload(
		ctx,
		object,
		billingMonth(service.now()),
	)
	if err != nil || existing {
		return reserved, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, service.config.UploadTimeout)
	defer cancel()
	etag, err := service.client.PutObject(requestCtx, reserved.ObjectKey, reserved.ContentType, input.Content)
	if err != nil {
		return Object{}, fmt.Errorf("R2 PutObject outcome is ambiguous; quota remains reserved: %w", err)
	}

	available, err := service.repository.MarkAvailable(ctx, reserved.AccountID, reserved.ID, etag)
	if err != nil {
		return Object{}, fmt.Errorf("R2 object uploaded but metadata remains pending: %w", err)
	}
	return available, nil
}

func (service *Service) readCloudUsage(ctx context.Context, force bool, cycleDay int) CloudUsage {
	configured := service.usageClient != nil && strings.TrimSpace(service.config.AnalyticsToken) != ""
	if !configured {
		return CloudUsage{Configured: false, Source: "cloudflare_account", Error: "analytics_token_missing"}
	}
	service.usageMu.Lock()
	defer service.usageMu.Unlock()
	end := service.now().UTC()
	start := billingCycleStart(end, cycleDay)
	if !force && service.cachedCloudUsage.Available && service.cachedCloudUsage.WindowStart.Equal(start) &&
		service.now().Sub(service.cachedCloudUsage.FetchedAt) < 2*time.Minute {
		return service.cachedCloudUsage
	}
	usage, err := service.usageClient.Usage(ctx, start, end)
	if err != nil {
		return CloudUsage{Configured: true, Source: "cloudflare_account", WindowStart: start, WindowEnd: end, Error: "cloudflare_metrics_unavailable"}
	}
	service.cachedCloudUsage = usage
	return usage
}

func (service *Service) Download(ctx context.Context, accountID, objectID string) (Object, io.ReadCloser, error) {
	object, content, err := service.DownloadForSource(ctx, accountID, objectID, "", "")
	if err != nil {
		return Object{}, nil, err
	}
	return object, content.Body, nil
}

// ObjectMetadata resolve metadados tenant-scoped sem chamar o provider nem
// consumir operacao B. E usado por HEAD e pela validacao da origem do objeto.
func (service *Service) ObjectMetadata(ctx context.Context, accountID, objectID, sourceModule string) (Object, error) {
	object, err := service.repository.Object(ctx, strings.TrimSpace(accountID), strings.TrimSpace(objectID))
	if err != nil {
		return Object{}, err
	}
	if source := strings.TrimSpace(sourceModule); source != "" && object.SourceModule != source {
		return Object{}, ErrObjectNotFound
	}
	return object, nil
}

// DownloadForSource le exatamente os bytes gravados, opcionalmente com Range.
// A origem e validada antes de reservar a operacao B para impedir que um modulo
// use a URL de outro como proxy de consumo.
func (service *Service) DownloadForSource(
	ctx context.Context,
	accountID, objectID, sourceModule, byteRange string,
) (Object, ObjectContent, error) {
	if !service.config.Enabled || service.client == nil {
		return Object{}, ObjectContent{}, ErrDisabled
	}
	state, err := service.repository.ProviderState(ctx)
	if err != nil {
		return Object{}, ObjectContent{}, err
	}
	if err := service.ensureMatchingProvider(state); err != nil {
		return Object{}, ObjectContent{}, err
	}
	object, err := service.ObjectMetadata(ctx, accountID, objectID, sourceModule)
	if err != nil {
		return Object{}, ObjectContent{}, err
	}
	if object.Status == "pending" {
		return service.downloadStaged(ctx, object, byteRange)
	}
	byteRange = strings.TrimSpace(byteRange)
	if byteRange != "" && (!byteRangePattern.MatchString(byteRange) || byteRange == "bytes=-") {
		return Object{}, ObjectContent{}, ErrInvalidRange
	}
	if err := service.repository.ReserveRequest(ctx, "B", billingMonth(service.now())); err != nil {
		return Object{}, ObjectContent{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, service.config.RequestTimeout)
	if byteRange != "" {
		rangeClient, ok := service.client.(RangeObjectClient)
		if !ok {
			cancel()
			return Object{}, ObjectContent{}, ErrInvalidRange
		}
		content, err := rangeClient.GetObjectRange(requestCtx, object.ObjectKey, byteRange)
		if err != nil {
			cancel()
			return Object{}, ObjectContent{}, fmt.Errorf("R2 GetObject range: %w", err)
		}
		content.Body = &cancelReadCloser{ReadCloser: content.Body, cancel: cancel}
		if content.ETag == "" {
			content.ETag = object.ETag
		}
		return object, content, nil
	}
	body, err := service.client.GetObject(requestCtx, object.ObjectKey)
	if err != nil {
		cancel()
		return Object{}, ObjectContent{}, fmt.Errorf("R2 GetObject: %w", err)
	}
	return object, ObjectContent{
		Body: &cancelReadCloser{ReadCloser: body, cancel: cancel}, ContentLength: object.SizeBytes,
		ETag: object.ETag,
	}, nil
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (reader *cancelReadCloser) Close() error {
	err := reader.ReadCloser.Close()
	reader.cancel()
	return err
}

func (service *Service) initializeProvider(ctx context.Context) error {
	month := billingMonth(service.now())
	if err := service.repository.ReserveRequest(ctx, "A", month); err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, service.config.RequestTimeout)
	defer cancel()
	empty, err := service.client.BucketIsEmpty(requestCtx)
	if err != nil {
		return fmt.Errorf("R2 ListObjectsV2: %w", err)
	}
	if !empty && !service.config.AllowNonEmptyBucketInitialization {
		return ErrBucketNotEmpty
	}
	_, err = service.repository.InitializeProvider(ctx, service.config.AccountID, service.config.Bucket)
	return err
}

func (service *Service) prepareObject(input UploadInput, settings Settings) (Object, error) {
	accountID := strings.TrimSpace(input.AccountID)
	createdBy := strings.TrimSpace(input.CreatedBy)
	sourceModule := strings.TrimSpace(input.SourceModule)
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	fileName := sanitizeFileName(input.FileName)
	contentType, err := validatedContentType(input.ContentType, input.Content)
	if err != nil ||
		!accountIDPattern.MatchString(accountID) ||
		!accountIDPattern.MatchString(createdBy) ||
		!sourceModulePattern.MatchString(sourceModule) ||
		idempotencyKey == "" || len(idempotencyKey) > 160 ||
		fileName == "" || len(input.Content) == 0 {
		return Object{}, ErrInvalidUpload
	}
	limit, err := fileTypeLimit(contentType, settings)
	if err != nil {
		return Object{}, err
	}
	if int64(len(input.Content)) > limit {
		return Object{}, ErrFileTypeLimit
	}

	id, err := randomObjectID()
	if err != nil {
		return Object{}, err
	}
	now := service.now().UTC()
	return Object{
		ID:             id,
		AccountID:      accountID,
		SourceModule:   sourceModule,
		IdempotencyKey: idempotencyKey,
		ObjectKey: fmt.Sprintf(
			"accounts/%s/%s/%04d/%s--%s",
			accountID,
			sourceModule,
			now.Year(),
			id,
			fileName,
		),
		FileName:    fileName,
		ContentType: strings.ToLower(contentType),
		SizeBytes:   int64(len(input.Content)),
		Status:      "pending",
		CreatedBy:   createdBy,
		CreatedAt:   now,
	}, nil
}

func validatedContentType(declared string, content []byte) (string, error) {
	parsed, _, err := mime.ParseMediaType(strings.TrimSpace(declared))
	if err != nil {
		return "", ErrUnsupportedFileType
	}
	parsed = strings.ToLower(parsed)
	category, ok := supportedContentTypes[parsed]
	if !ok {
		return "", ErrUnsupportedFileType
	}
	detected, _, detectErr := mime.ParseMediaType(http.DetectContentType(content))
	if detectErr != nil {
		return "", ErrUnsupportedFileType
	}
	detectedCategory, detectedSupported := supportedContentTypes[strings.ToLower(detected)]
	if detectedSupported && detectedCategory != category {
		return "", ErrUnsupportedFileType
	}
	if !detectedSupported && detected != "application/octet-stream" &&
		(category != "document" || detected != "application/zip") {
		return "", ErrUnsupportedFileType
	}
	return parsed, nil
}

func fileTypeLimit(contentType string, settings Settings) (int64, error) {
	switch supportedContentTypes[contentType] {
	case "image":
		return settings.ImageMaxBytes, nil
	case "video":
		return settings.VideoMaxBytes, nil
	case "audio":
		return settings.AudioMaxBytes, nil
	case "document":
		return settings.DocumentMaxBytes, nil
	default:
		return 0, ErrUnsupportedFileType
	}
}

var supportedContentTypes = map[string]string{
	"image/jpeg": "image", "image/png": "image", "image/gif": "image",
	"image/webp": "image", "image/avif": "image",
	"video/mp4": "video", "video/webm": "video", "video/quicktime": "video",
	"video/ogg": "video", "video/x-msvideo": "video", "video/x-m4v": "video",
	"audio/mpeg": "audio", "audio/mp4": "audio", "audio/wav": "audio",
	"audio/x-wav": "audio", "audio/ogg": "audio", "audio/webm": "audio",
	"application/pdf": "document", "text/plain": "document", "text/csv": "document",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "document",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       "document",
}

func (service *Service) ensureMatchingProvider(state ProviderState) error {
	if state.AccountID != strings.TrimSpace(service.config.AccountID) ||
		state.Bucket != strings.TrimSpace(service.config.Bucket) {
		return ErrProviderMismatch
	}
	return nil
}

func billingMonth(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func billingCycleStart(now time.Time, cycleDay int) time.Time {
	utc := now.UTC()
	if cycleDay < 1 || cycleDay > 28 {
		cycleDay = DefaultBillingCycleDay
	}
	start := time.Date(utc.Year(), utc.Month(), cycleDay, 0, 0, 0, 0, time.UTC)
	if utc.Before(start) {
		start = start.AddDate(0, -1, 0)
	}
	return start
}

func sanitizeFileName(value string) string {
	base := filepath.Base(strings.TrimSpace(value))
	base = strings.NewReplacer("/", "-", "\\", "-", "..", "-", ":", "-").Replace(base)
	base = strings.Trim(base, " .-")
	if len(base) > 120 {
		base = base[:120]
	}
	return base
}

func randomObjectID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
