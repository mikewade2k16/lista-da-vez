package stores

import "errors"

var (
	ErrForbidden       = errors.New("stores: forbidden")
	ErrStoreNotFound   = errors.New("stores: store not found")
	ErrValidation      = errors.New("stores: validation failed")
	ErrStoreConflict   = errors.New("stores: store conflict")
	ErrTenantRequired  = errors.New("stores: tenant required")
	ErrTenantForbidden = errors.New("stores: tenant forbidden")
	ErrDeleteBlocked   = errors.New("stores: delete blocked")
)

type DeleteBlockedError struct {
	StoreID      string
	Dependencies []DeleteDependency
}

func (err *DeleteBlockedError) Error() string {
	return ErrDeleteBlocked.Error()
}

func (err *DeleteBlockedError) Unwrap() error {
	return ErrDeleteBlocked
}
