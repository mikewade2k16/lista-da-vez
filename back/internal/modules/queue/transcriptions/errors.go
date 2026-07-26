package transcriptions

import "errors"

var (
	ErrForbidden             = errors.New("transcriptions: forbidden")
	ErrFeatureDisabled       = errors.New("transcriptions: experimental feature disabled")
	ErrNotFound              = errors.New("transcriptions: not found")
	ErrValidation            = errors.New("transcriptions: validation failed")
	ErrUnsupported           = errors.New("transcriptions: unsupported audio")
	ErrTooLarge              = errors.New("transcriptions: audio too large")
	ErrChunkConflict         = errors.New("transcriptions: chunk conflict")
	ErrNotReady              = errors.New("transcriptions: recording not ready")
	ErrCredentialUnavailable = errors.New("transcriptions: global ai credential unavailable")
)
