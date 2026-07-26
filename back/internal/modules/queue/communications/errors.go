package communications

import "errors"

var (
	ErrForbidden  = errors.New("communications: forbidden")
	ErrNotFound   = errors.New("communications: not found")
	ErrValidation = errors.New("communications: validation failed")
)
