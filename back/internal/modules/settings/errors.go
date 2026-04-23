package settings

import "errors"

var (
	ErrForbidden     = errors.New("settings: forbidden")
	ErrValidation    = errors.New("settings: validation failed")
	ErrStoreRequired = errors.New("settings: store required")
	ErrStoreNotFound = errors.New("settings: store not found")
)
