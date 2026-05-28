package settings

import "errors"

var (
	ErrForbidden      = errors.New("settings: forbidden")
	ErrValidation     = errors.New("settings: validation failed")
	ErrTenantRequired = errors.New("settings: tenant required")
	ErrTenantNotFound = errors.New("settings: tenant not found")
)
