package performancefeedback

import "errors"

var (
	ErrForbidden          = errors.New("performance feedback forbidden")
	ErrNotFound           = errors.New("performance feedback not found")
	ErrConsultantNotFound = errors.New("performance feedback consultant not found")
	ErrStoreRequired      = errors.New("performance feedback store required")
	ErrValidation         = errors.New("performance feedback validation")
	ErrConflict           = errors.New("performance feedback conflict")
)
