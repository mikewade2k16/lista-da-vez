package storage

import "errors"

var (
	ErrDisabled             = errors.New("object storage is disabled")
	ErrUploadsDisabled      = errors.New("R2 uploads are disabled")
	ErrAnalyticsUnavailable = errors.New("cloudflare R2 analytics are unavailable")
	ErrInvalidConfig        = errors.New("invalid object storage configuration")
	ErrNotInitialized       = errors.New("object storage provider is not initialized")
	ErrProviderMismatch     = errors.New("configured provider differs from initialized provider")
	ErrBucketNotEmpty       = errors.New("R2 bucket must be empty on first initialization")
	ErrClassAQuotaExceeded  = errors.New("R2 Class A monthly quota exceeded")
	ErrClassBQuotaExceeded  = errors.New("R2 Class B monthly quota exceeded")
	ErrStorageQuotaExceeded = errors.New("R2 storage quota exceeded")
	ErrInvalidUpload        = errors.New("invalid object upload")
	ErrUnsupportedFileType  = errors.New("unsupported object file type")
	ErrFileTypeLimit        = errors.New("object exceeds its file type limit")
	ErrObjectNotFound       = errors.New("object not found")
	ErrInvalidRange         = errors.New("invalid object byte range")
	ErrInvalidSettings      = errors.New("invalid object storage settings")
)
