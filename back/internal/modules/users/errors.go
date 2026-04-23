package users

import "errors"

var (
	ErrForbidden         = errors.New("users: forbidden")
	ErrPasswordForbidden = errors.New("users: password management forbidden")
	ErrValidation        = errors.New("users: validation failed")
	ErrConflict          = errors.New("users: conflict")
	ErrNotFound          = errors.New("users: not found")
	ErrTenantRequired    = errors.New("users: tenant required")
	ErrStoreRequired     = errors.New("users: store required")
	ErrInvalidStoreScope = errors.New("users: invalid store scope")
	ErrSelfArchive       = errors.New("users: self archive forbidden")
	ErrInviteNotAllowed  = errors.New("users: invite not allowed")
	ErrConsultantManaged = errors.New("users: consultant managed by roster")
)
