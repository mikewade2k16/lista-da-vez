package feedback

import "errors"

var (
	ErrNotFound  = errors.New("feedback_not_found")
	ErrForbidden = errors.New("feedback_forbidden")
	ErrClosed    = errors.New("feedback_closed")
	ErrInvalid   = errors.New("feedback_invalid")
	ErrInvalidImage = errors.New("feedback_invalid_image")
)
