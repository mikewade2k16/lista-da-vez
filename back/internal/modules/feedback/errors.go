package feedback

import "errors"

var (
	ErrNotFound   = errors.New("feedback_not_found")
	ErrForbidden  = errors.New("feedback_forbidden")
)
