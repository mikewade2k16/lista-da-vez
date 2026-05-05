package alerts

import "errors"

var (
	ErrForbidden      = errors.New("alerts: forbidden")
	ErrNotFound       = errors.New("alerts: alert not found")
	ErrTenantRequired = errors.New("alerts: tenant required")
	ErrValidation     = errors.New("alerts: validation failed")
)
