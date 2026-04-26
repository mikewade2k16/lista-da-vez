package tenants

import "errors"

var (
	ErrForbidden      = errors.New("tenants: forbidden")
	ErrValidation     = errors.New("tenants: validation failed")
	ErrTenantNotFound = errors.New("tenants: tenant not found")
	ErrTenantConflict = errors.New("tenants: tenant conflict")
)
