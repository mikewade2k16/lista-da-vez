package stores

import "errors"

var (
	ErrForbidden       = errors.New("stores: forbidden")
	ErrStoreNotFound   = errors.New("stores: store not found")
	ErrValidation      = errors.New("stores: validation failed")
	ErrStoreConflict   = errors.New("stores: store conflict")
	ErrTenantRequired  = errors.New("stores: tenant required")
	ErrTenantForbidden = errors.New("stores: tenant forbidden")
)
