package storage

import (
	"io"
	"time"
)

const (
	OfficialFreeStorageBytes int64 = 10_000_000_000
	OfficialFreeClassAOps    int64 = 1_000_000
	OfficialFreeClassBOps    int64 = 10_000_000
	MaxManagedObjectBytes    int64 = 512 * 1024 * 1024

	DefaultStorageLimitBytes int64 = 9_000_000_000
	DefaultClassALimit       int64 = 900_000
	DefaultClassBLimit       int64 = 9_000_000
	DefaultMaxObjectBytes    int64 = 25 * 1024 * 1024
	DefaultImageMaxBytes     int64 = 25 * 1024 * 1024
	DefaultVideoMaxBytes     int64 = 25 * 1024 * 1024
	DefaultAudioMaxBytes     int64 = 25 * 1024 * 1024
	DefaultDocumentMaxBytes  int64 = 25 * 1024 * 1024
	DefaultBillingCycleDay         = 27
)

type Config struct {
	Enabled                           bool
	AccountID                         string
	Bucket                            string
	AccessKeyID                       string
	SecretAccessKey                   string
	RequestTimeout                    time.Duration
	UploadTimeout                     time.Duration
	AnalyticsToken                    string
	AllowNonEmptyBucketInitialization bool
	UploadsDir                        string
}

type ProviderState struct {
	AccountID     string
	Bucket        string
	InitializedAt time.Time
	CheckedAt     time.Time
}

type Usage struct {
	BillingMonth     string `json:"billingMonth"`
	StoredBytes      int64  `json:"storedBytes"`
	PendingBytes     int64  `json:"pendingBytes"`
	AvailableObjects int64  `json:"availableObjects"`
	PendingObjects   int64  `json:"pendingObjects"`
	ClassARequests   int64  `json:"classARequests"`
	ClassBRequests   int64  `json:"classBRequests"`
	UploadedBytes    int64  `json:"uploadedBytes"`
}

type Status struct {
	Enabled     bool       `json:"enabled"`
	Initialized bool       `json:"initialized"`
	Provider    string     `json:"provider"`
	Bucket      string     `json:"bucket,omitempty"`
	Usage       Usage      `json:"usage"`
	CloudUsage  CloudUsage `json:"cloudUsage"`
	Settings    Settings   `json:"settings"`
}

type CloudUsage struct {
	Available      bool      `json:"available"`
	Configured     bool      `json:"configured"`
	Source         string    `json:"source"`
	WindowStart    time.Time `json:"windowStart,omitempty"`
	WindowEnd      time.Time `json:"windowEnd,omitempty"`
	FetchedAt      time.Time `json:"fetchedAt,omitempty"`
	StoredBytes    int64     `json:"storedBytes"`
	MetadataBytes  int64     `json:"metadataBytes"`
	ObjectCount    int64     `json:"objectCount"`
	ClassARequests int64     `json:"classARequests"`
	ClassBRequests int64     `json:"classBRequests"`
	Error          string    `json:"error,omitempty"`
}

type Settings struct {
	UploadsEnabled    bool      `json:"uploadsEnabled"`
	BillingCycleDay   int       `json:"billingCycleDay"`
	StorageLimitBytes int64     `json:"storageLimitBytes"`
	ClassALimit       int64     `json:"classALimit"`
	ClassBLimit       int64     `json:"classBLimit"`
	MaxObjectBytes    int64     `json:"maxObjectBytes"`
	ImageMaxBytes     int64     `json:"imageMaxBytes"`
	VideoMaxBytes     int64     `json:"videoMaxBytes"`
	AudioMaxBytes     int64     `json:"audioMaxBytes"`
	DocumentMaxBytes  int64     `json:"documentMaxBytes"`
	UpdatedBy         string    `json:"updatedBy,omitempty"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type UpdateSettingsInput struct {
	UploadsEnabled    bool  `json:"uploadsEnabled"`
	BillingCycleDay   int   `json:"billingCycleDay"`
	StorageLimitBytes int64 `json:"storageLimitBytes"`
	ClassALimit       int64 `json:"classALimit"`
	ClassBLimit       int64 `json:"classBLimit"`
	ImageMaxBytes     int64 `json:"imageMaxBytes"`
	VideoMaxBytes     int64 `json:"videoMaxBytes"`
	AudioMaxBytes     int64 `json:"audioMaxBytes"`
	DocumentMaxBytes  int64 `json:"documentMaxBytes"`
}

type Object struct {
	ID             string     `json:"id"`
	AccountID      string     `json:"accountId"`
	SourceModule   string     `json:"sourceModule"`
	IdempotencyKey string     `json:"idempotencyKey"`
	ObjectKey      string     `json:"objectKey"`
	FileName       string     `json:"fileName"`
	ContentType    string     `json:"contentType"`
	SizeBytes      int64      `json:"sizeBytes"`
	ETag           string     `json:"etag,omitempty"`
	Status         string     `json:"status"`
	CreatedBy      string     `json:"createdBy"`
	CreatedAt      time.Time  `json:"createdAt"`
	AvailableAt    *time.Time `json:"availableAt,omitempty"`
}

type UploadInput struct {
	AccountID      string
	SourceModule   string
	IdempotencyKey string
	FileName       string
	ContentType    string
	Content        []byte
	CreatedBy      string
}

const (
	MultipartThresholdBytes int64 = 100 * 1024 * 1024
	MultipartPartSizeBytes  int64 = 16 * 1024 * 1024
)

type MultipartUploadInput struct {
	AccountID, SourceModule, IdempotencyKey, FileName, ContentType, CreatedBy string
	SizeBytes                                                                 int64
}

type MultipartPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"-"`
	SizeBytes  int64  `json:"sizeBytes"`
}

type MultipartUpload struct {
	ID               string          `json:"sessionId"`
	Object           Object          `json:"object"`
	ProviderUploadID string          `json:"-"`
	PartSizeBytes    int64           `json:"partSizeBytes"`
	PartCount        int             `json:"partCount"`
	UploadedParts    []MultipartPart `json:"uploadedParts"`
	Status           string          `json:"status"`
	CreatedBy        string          `json:"-"`
	CreatedAt        time.Time       `json:"-"`
}

type StagedUploadInput struct {
	MultipartUploadInput
	Content io.Reader
}

type MultipartDelivery struct {
	UploadID, AccountID, SourceModule, CreatedBy, StagingPath string
	Attempts                                                  int
}

// ObjectContent descreve uma leitura do binario original. ContentRange fica
// preenchido somente quando o provider atendeu um Range HTTP.
type ObjectContent struct {
	Body          io.ReadCloser
	ContentLength int64
	ContentRange  string
	ETag          string
}
