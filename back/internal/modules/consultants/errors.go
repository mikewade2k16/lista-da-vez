package consultants

import "errors"

var (
	ErrForbidden          = errors.New("consultants: forbidden")
	ErrValidation         = errors.New("consultants: validation failed")
	ErrStoreRequired      = errors.New("consultants: store required")
	ErrStoreNotFound      = errors.New("consultants: store not found")
	ErrConsultantNotFound = errors.New("consultants: consultant not found")
	ErrConsultantConflict = errors.New("consultants: consultant conflict")
	ErrAccessConflict     = errors.New("consultants: access conflict")
	ErrAccessProvisioning = errors.New("consultants: access provisioning failed")
)
