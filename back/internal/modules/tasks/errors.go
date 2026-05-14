package tasks

import "errors"

var (
	ErrAccountRequired   = errors.New("tasks: account required")
	ErrAccountNotFound   = errors.New("tasks: account not found")
	ErrBoardNotFound     = errors.New("tasks: board not found")
	ErrColumnNotFound    = errors.New("tasks: column not found")
	ErrFieldNotFound     = errors.New("tasks: field not found")
	ErrTaskNotFound      = errors.New("tasks: task not found")
	ErrTimeEntryNotFound = errors.New("tasks: time entry not found")
	ErrVersionConflict   = errors.New("tasks: version conflict")
	ErrShareRequired     = errors.New("tasks: share required")
	ErrForbidden         = errors.New("tasks: forbidden")
	ErrValidation        = errors.New("tasks: validation error")
)
