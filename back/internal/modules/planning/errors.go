package planning

import "errors"

var (
	ErrForbidden            = errors.New("planning forbidden")
	ErrValidation           = errors.New("planning validation")
	ErrStoreNotFound        = errors.New("planning store not found")
	ErrVersionConflict      = errors.New("planning version conflict")
	ErrPublished            = errors.New("planning schedule published")
	ErrScheduleRestrictions = errors.New("planning schedule has hard restrictions")
	ErrScheduleNotFound     = errors.New("planning schedule not found")
)
