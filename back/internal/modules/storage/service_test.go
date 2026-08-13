package storage

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	testAccountID = "11111111-1111-4111-8111-111111111111"
	testUserID    = "22222222-2222-4222-8222-222222222222"
)

type fakeRepository struct {
	state           ProviderState
	stateErr        error
	settings        Settings
	usage           Usage
	requestClasses  []string
	reserveErr      error
	reserveObject   Object
	reserveExisting bool
	initialized     bool
	touched         bool
	updatedBy       string
	failedObjectID  string
}

func (repository *fakeRepository) ProviderState(_ context.Context) (ProviderState, error) {
	return repository.state, repository.stateErr
}

func (repository *fakeRepository) InitializeProvider(_ context.Context, accountID, bucket string) (ProviderState, error) {
	repository.initialized = true
	repository.stateErr = nil
	repository.state = ProviderState{AccountID: accountID, Bucket: bucket, InitializedAt: time.Now()}
	return repository.state, nil
}

func (repository *fakeRepository) TouchProvider(_ context.Context) error {
	repository.touched = true
	return nil
}

func (repository *fakeRepository) Settings(_ context.Context) (Settings, error) {
	if repository.settings.StorageLimitBytes == 0 {
		return validSettings(), nil
	}
	return repository.settings, nil
}

func (repository *fakeRepository) UpdateSettings(
	_ context.Context,
	input UpdateSettingsInput,
	actorID string,
) (Settings, error) {
	repository.updatedBy = actorID
	repository.settings = Settings{
		UploadsEnabled:    input.UploadsEnabled,
		BillingCycleDay:   input.BillingCycleDay,
		StorageLimitBytes: input.StorageLimitBytes,
		ClassALimit:       input.ClassALimit,
		ClassBLimit:       input.ClassBLimit,
		MaxObjectBytes:    input.VideoMaxBytes,
		ImageMaxBytes:     input.ImageMaxBytes,
		VideoMaxBytes:     input.VideoMaxBytes,
		AudioMaxBytes:     input.AudioMaxBytes,
		DocumentMaxBytes:  input.DocumentMaxBytes,
		UpdatedBy:         actorID,
		UpdatedAt:         time.Now(),
	}
	return repository.settings, nil
}

func TestUploadStopsBeforeR2WhenOperationalToggleIsOff(t *testing.T) {
	settings := validSettings()
	settings.UploadsEnabled = false
	repository := &fakeRepository{
		state:    ProviderState{AccountID: validConfig().AccountID, Bucket: validConfig().Bucket},
		settings: settings,
	}
	client := &fakeClient{}
	service := newTestService(validConfig(), repository, client)

	_, err := service.Upload(context.Background(), validUpload())
	if !errors.Is(err, ErrUploadsDisabled) {
		t.Fatalf("expected ErrUploadsDisabled, got %v", err)
	}
	if client.putCalls != 0 {
		t.Fatalf("expected no R2 call while uploads are disabled, got %d", client.putCalls)
	}
}

func TestR2OperationClassMatchesCloudflareBillingClasses(t *testing.T) {
	for action, expected := range map[string]string{
		"PutObject": "A", "ListObjectsV2": "A", "GetObject": "B", "HeadObject": "B",
		"DeleteObject": "",
	} {
		if actual := r2OperationClass(action); actual != expected {
			t.Fatalf("action %s: expected %q, got %q", action, expected, actual)
		}
	}
}

func (repository *fakeRepository) ReserveRequest(_ context.Context, requestClass string, _ time.Time) error {
	repository.requestClasses = append(repository.requestClasses, requestClass)
	return nil
}

func (repository *fakeRepository) ReserveUpload(
	_ context.Context,
	object Object,
	_ time.Time,
) (Object, bool, error) {
	if repository.reserveErr != nil {
		return Object{}, false, repository.reserveErr
	}
	if repository.reserveObject.ID != "" {
		return repository.reserveObject, repository.reserveExisting, nil
	}
	return object, repository.reserveExisting, nil
}

func (repository *fakeRepository) MarkAvailable(_ context.Context, _ string, _ string, etag string) (Object, error) {
	object := repository.reserveObject
	object.ETag = etag
	object.Status = "available"
	return object, nil
}

func (repository *fakeRepository) MarkFailed(_ context.Context, _ string, objectID string) error {
	repository.failedObjectID = objectID
	return nil
}

func (repository *fakeRepository) Object(_ context.Context, _, _ string) (Object, error) {
	return repository.reserveObject, nil
}

func (repository *fakeRepository) Usage(_ context.Context, _ time.Time) (Usage, error) {
	return repository.usage, nil
}

type fakeClient struct {
	headCalls  int
	listCalls  int
	putCalls   int
	empty      bool
	putErr     error
	putContent []byte
}

type fakeUsageClient struct{}

func (fakeUsageClient) Usage(_ context.Context, start, end time.Time) (CloudUsage, error) {
	return CloudUsage{Available: true, Configured: true, Source: "test", WindowStart: start, WindowEnd: end, FetchedAt: end}, nil
}

type sequenceUsageClient struct {
	calls  int
	errors []error
}

func (client *sequenceUsageClient) Usage(_ context.Context, start, end time.Time) (CloudUsage, error) {
	client.calls++
	if client.calls <= len(client.errors) && client.errors[client.calls-1] != nil {
		return CloudUsage{}, client.errors[client.calls-1]
	}
	return CloudUsage{Available: true, Configured: true, Source: "test", WindowStart: start, WindowEnd: end, FetchedAt: end}, nil
}

func newTestService(cfg Config, repository Repository, client ObjectClient) *Service {
	return NewService(cfg, repository, client, fakeUsageClient{})
}

func (client *fakeClient) HeadBucket(_ context.Context) error {
	client.headCalls++
	return nil
}

func (client *fakeClient) BucketIsEmpty(_ context.Context) (bool, error) {
	client.listCalls++
	return client.empty, nil
}

func (client *fakeClient) PutObject(_ context.Context, _, _ string, content []byte) (string, error) {
	client.putCalls++
	client.putContent = append([]byte(nil), content...)
	return "etag-1", client.putErr
}

func TestUploadPreservesOriginalBytesExactly(t *testing.T) {
	cfg := validConfig()
	repository := &fakeRepository{state: ProviderState{AccountID: cfg.AccountID, Bucket: cfg.Bucket}}
	client := &fakeClient{}
	service := newTestService(cfg, repository, client)
	upload := validUpload()
	upload.Content = []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0xff, 0x10, 0x0a, 0x1a, 0x0a}

	if _, err := service.Upload(context.Background(), upload); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if !reflect.DeepEqual(client.putContent, upload.Content) {
		t.Fatalf("provider received modified bytes: got %v want %v", client.putContent, upload.Content)
	}
}

func TestUploadRetriesTransientCloudUsageFailureOnce(t *testing.T) {
	cfg := validConfig()
	repository := &fakeRepository{state: ProviderState{AccountID: cfg.AccountID, Bucket: cfg.Bucket}}
	client := &fakeClient{}
	usageClient := &sequenceUsageClient{errors: []error{ErrAnalyticsUnavailable}}
	service := NewService(cfg, repository, client, usageClient)

	if _, err := service.Upload(context.Background(), validUpload()); err != nil {
		t.Fatalf("Upload returned error after transient metrics failure: %v", err)
	}
	if usageClient.calls != 2 {
		t.Fatalf("expected two analytics attempts, got %d", usageClient.calls)
	}
	if client.putCalls != 1 {
		t.Fatalf("expected upload after retry, got %d R2 calls", client.putCalls)
	}
}

func TestUploadFailsClosedAfterCloudUsageRetryIsExhausted(t *testing.T) {
	cfg := validConfig()
	repository := &fakeRepository{state: ProviderState{AccountID: cfg.AccountID, Bucket: cfg.Bucket}}
	client := &fakeClient{}
	usageClient := &sequenceUsageClient{errors: []error{ErrAnalyticsUnavailable, ErrAnalyticsUnavailable}}
	service := NewService(cfg, repository, client, usageClient)

	_, err := service.Upload(context.Background(), validUpload())
	if !errors.Is(err, ErrAnalyticsUnavailable) {
		t.Fatalf("expected ErrAnalyticsUnavailable, got %v", err)
	}
	if usageClient.calls != 2 {
		t.Fatalf("expected exactly two analytics attempts, got %d", usageClient.calls)
	}
	if client.putCalls != 0 {
		t.Fatalf("R2 upload must remain blocked without metrics, got %d calls", client.putCalls)
	}
}

func (client *fakeClient) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("content")), nil
}

func TestValidateSettingsRejectsLimitsAboveFreeTier(t *testing.T) {
	input := validSettingsInput()
	input.ClassALimit = OfficialFreeClassAOps + 1

	if err := ValidateSettings(input); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected ErrInvalidSettings, got %v", err)
	}
}

func TestUpdateSettingsPersistsAuthoritativeLimits(t *testing.T) {
	repository := &fakeRepository{}
	service := newTestService(validConfig(), repository, &fakeClient{})
	input := validSettingsInput()
	input.VideoMaxBytes = 100 * 1024 * 1024

	settings, err := service.UpdateSettings(context.Background(), input, testUserID)
	if err != nil {
		t.Fatalf("UpdateSettings returned error: %v", err)
	}
	if settings.VideoMaxBytes != input.VideoMaxBytes || repository.updatedBy != testUserID {
		t.Fatalf("settings were not persisted with actor: %+v actor=%s", settings, repository.updatedBy)
	}
}

func TestCheckConnectionInitializesOnlyEmptyBucket(t *testing.T) {
	repository := &fakeRepository{stateErr: ErrNotInitialized}
	client := &fakeClient{empty: true}
	service := newTestService(validConfig(), repository, client)

	status, err := service.CheckConnection(context.Background())
	if err != nil {
		t.Fatalf("CheckConnection returned error: %v", err)
	}
	if !status.Initialized || !repository.initialized || !repository.touched {
		t.Fatalf("expected initialized and touched provider, got %+v", status)
	}
	if client.headCalls != 1 || client.listCalls != 1 {
		t.Fatalf("expected one HeadBucket and one ListObjects, got head=%d list=%d", client.headCalls, client.listCalls)
	}
	if !reflect.DeepEqual(repository.requestClasses, []string{"B", "A"}) {
		t.Fatalf("expected B then A reservations, got %v", repository.requestClasses)
	}
}

func TestCheckConnectionRejectsNonEmptyBucket(t *testing.T) {
	repository := &fakeRepository{stateErr: ErrNotInitialized}
	client := &fakeClient{empty: false}
	service := newTestService(validConfig(), repository, client)

	_, err := service.CheckConnection(context.Background())
	if !errors.Is(err, ErrBucketNotEmpty) {
		t.Fatalf("expected ErrBucketNotEmpty, got %v", err)
	}
	if repository.initialized {
		t.Fatal("provider must not initialize a non-empty bucket")
	}
}

func TestCheckConnectionAdoptsNonEmptyBucketOnlyWhenExplicitlyAllowed(t *testing.T) {
	repository := &fakeRepository{stateErr: ErrNotInitialized}
	client := &fakeClient{empty: false}
	cfg := validConfig()
	cfg.AllowNonEmptyBucketInitialization = true
	service := newTestService(cfg, repository, client)

	status, err := service.CheckConnection(context.Background())
	if err != nil {
		t.Fatalf("CheckConnection returned error: %v", err)
	}
	if !status.Initialized || !repository.initialized || !repository.touched {
		t.Fatalf("expected existing bucket to be adopted, got %+v", status)
	}
	if client.headCalls != 1 || client.listCalls != 1 {
		t.Fatalf("expected one HeadBucket and one ListObjects, got head=%d list=%d", client.headCalls, client.listCalls)
	}
}

func TestUploadStopsBeforeR2WhenQuotaIsExhausted(t *testing.T) {
	cfg := validConfig()
	repository := &fakeRepository{
		state:      ProviderState{AccountID: cfg.AccountID, Bucket: cfg.Bucket},
		reserveErr: ErrStorageQuotaExceeded,
	}
	client := &fakeClient{empty: true}
	service := newTestService(cfg, repository, client)

	_, err := service.Upload(context.Background(), validUpload())
	if !errors.Is(err, ErrStorageQuotaExceeded) {
		t.Fatalf("expected ErrStorageQuotaExceeded, got %v", err)
	}
	if client.putCalls != 0 {
		t.Fatalf("R2 must not be called after quota rejection, got %d calls", client.putCalls)
	}
}

func TestUploadProtectsBytesConfirmedLocallyBeforeCloudflareReflectsThem(t *testing.T) {
	cfg := validConfig()
	settings := validSettings()
	settings.StorageLimitBytes = 95
	repository := &fakeRepository{
		state:    ProviderState{AccountID: cfg.AccountID, Bucket: cfg.Bucket},
		settings: settings,
		usage:    Usage{UploadedBytes: 90},
	}
	client := &fakeClient{}
	service := newTestService(cfg, repository, client)

	_, err := service.Upload(context.Background(), validUpload())
	if !errors.Is(err, ErrStorageQuotaExceeded) {
		t.Fatalf("expected local confirmed bytes to protect quota, got %v", err)
	}
	if client.putCalls != 0 {
		t.Fatalf("R2 must not be called after conservative quota rejection, got %d calls", client.putCalls)
	}
}

func TestUploadProtectsClassAReservedLocallyBeforeCloudflareReflectsIt(t *testing.T) {
	cfg := validConfig()
	settings := validSettings()
	settings.ClassALimit = 2
	repository := &fakeRepository{
		state:    ProviderState{AccountID: cfg.AccountID, Bucket: cfg.Bucket},
		settings: settings,
		usage:    Usage{ClassARequests: 2},
	}
	client := &fakeClient{}
	service := newTestService(cfg, repository, client)

	_, err := service.Upload(context.Background(), validUpload())
	if !errors.Is(err, ErrClassAQuotaExceeded) {
		t.Fatalf("expected local Class A reservations to protect quota, got %v", err)
	}
	if client.putCalls != 0 {
		t.Fatalf("R2 must not be called after conservative Class A rejection, got %d calls", client.putCalls)
	}
}

func TestUploadDoesNotRepeatExistingIdempotentObject(t *testing.T) {
	cfg := validConfig()
	existing := Object{ID: "existing", Status: "available", AccountID: testAccountID}
	repository := &fakeRepository{
		state:           ProviderState{AccountID: cfg.AccountID, Bucket: cfg.Bucket},
		reserveObject:   existing,
		reserveExisting: true,
	}
	client := &fakeClient{}
	service := newTestService(cfg, repository, client)

	object, err := service.Upload(context.Background(), validUpload())
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if object.ID != existing.ID {
		t.Fatalf("expected existing object, got %+v", object)
	}
	if client.putCalls != 0 {
		t.Fatalf("idempotent replay must not call R2, got %d calls", client.putCalls)
	}
}

func TestUploadFailureKeepsPendingReservation(t *testing.T) {
	cfg := validConfig()
	reserved := Object{ID: "pending", Status: "pending", AccountID: testAccountID}
	repository := &fakeRepository{
		state:         ProviderState{AccountID: cfg.AccountID, Bucket: cfg.Bucket},
		reserveObject: reserved,
	}
	client := &fakeClient{putErr: errors.New("timeout")}
	service := newTestService(cfg, repository, client)

	_, err := service.Upload(context.Background(), validUpload())
	if err == nil || !strings.Contains(err.Error(), "quota remains reserved") {
		t.Fatalf("expected ambiguous upload error with retained quota, got %v", err)
	}
	if client.putCalls != 1 {
		t.Fatalf("expected one R2 attempt, got %d", client.putCalls)
	}
}

func TestPrepareObjectUsesNavigableAnnualKey(t *testing.T) {
	service := newTestService(validConfig(), &fakeRepository{}, &fakeClient{})
	service.now = func() time.Time { return time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC) }

	object, err := service.prepareObject(validUpload(), validSettings())
	if err != nil {
		t.Fatalf("prepare object: %v", err)
	}
	prefix := "accounts/" + testAccountID + "/tests/2026/"
	if !strings.HasPrefix(object.ObjectKey, prefix) || !strings.HasSuffix(object.ObjectKey, "--photo.png") {
		t.Fatalf("unexpected object key %q", object.ObjectKey)
	}
	if strings.Contains(strings.TrimPrefix(object.ObjectKey, prefix), "/") {
		t.Fatalf("object ID must not create another directory: %q", object.ObjectKey)
	}
}

func TestBillingCycleStartUsesConfiguredDay(t *testing.T) {
	if got := billingCycleStart(time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC), 27); !got.Equal(time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected current cycle start: %s", got)
	}
	if got := billingCycleStart(time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC), 27); !got.Equal(time.Date(2026, time.June, 27, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected previous cycle start: %s", got)
	}
}

func validConfig() Config {
	return Config{
		Enabled:         true,
		AccountID:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Bucket:          "omni-private",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		RequestTimeout:  5 * time.Second,
		UploadTimeout:   time.Minute,
		AnalyticsToken:  "analytics-token",
	}
}

func validSettings() Settings {
	input := validSettingsInput()
	return Settings{
		UploadsEnabled:    true,
		BillingCycleDay:   input.BillingCycleDay,
		StorageLimitBytes: input.StorageLimitBytes,
		ClassALimit:       input.ClassALimit,
		ClassBLimit:       input.ClassBLimit,
		MaxObjectBytes:    input.VideoMaxBytes,
		ImageMaxBytes:     input.ImageMaxBytes,
		VideoMaxBytes:     input.VideoMaxBytes,
		AudioMaxBytes:     input.AudioMaxBytes,
		DocumentMaxBytes:  input.DocumentMaxBytes,
		UpdatedAt:         time.Now(),
	}
}

func validSettingsInput() UpdateSettingsInput {
	return UpdateSettingsInput{
		UploadsEnabled:    true,
		BillingCycleDay:   DefaultBillingCycleDay,
		StorageLimitBytes: DefaultStorageLimitBytes,
		ClassALimit:       DefaultClassALimit,
		ClassBLimit:       DefaultClassBLimit,
		ImageMaxBytes:     DefaultImageMaxBytes,
		VideoMaxBytes:     DefaultVideoMaxBytes,
		AudioMaxBytes:     DefaultAudioMaxBytes,
		DocumentMaxBytes:  DefaultDocumentMaxBytes,
	}
}

func validUpload() UploadInput {
	return UploadInput{
		AccountID:      testAccountID,
		SourceModule:   "tests",
		IdempotencyKey: "request-1",
		FileName:       "photo.png",
		ContentType:    "image/png",
		Content:        []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a},
		CreatedBy:      testUserID,
	}
}
