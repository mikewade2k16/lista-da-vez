package operationgoals

import "errors"

var (
	ErrForbidden          = errors.New("operationgoals: forbidden")
	ErrValidation         = errors.New("operationgoals: validation failed")
	ErrStoreRequired      = errors.New("operationgoals: store required")
	ErrStoreNotFound      = errors.New("operationgoals: store not found")
	ErrConsultantNotFound = errors.New("operationgoals: consultant not found")
	ErrGoalNotFound       = errors.New("operationgoals: goal not found")
	ErrGoalConflict       = errors.New("operationgoals: goal conflict")
	ErrTenantRequired     = errors.New("operationgoals: tenant required")
)
