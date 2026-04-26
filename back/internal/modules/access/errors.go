package access

import "errors"

var (
	ErrForbidden  = errors.New("access: forbidden")
	ErrValidation = errors.New("access: validation")
	ErrNotFound   = errors.New("access: not found")
)
