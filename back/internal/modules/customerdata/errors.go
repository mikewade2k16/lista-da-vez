package customerdata

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound                      = errors.New("customer data: resource not found")
	ErrForbidden                     = errors.New("customer data: forbidden")
	ErrValidation                    = errors.New("customer data: invalid input")
	ErrConflict                      = errors.New("customer data: conflict")
	ErrCapabilityDisabled            = errors.New("customer data: capability disabled")
	ErrWriterInactive                = errors.New("customer data: writer is not authoritative")
	ErrIdentityProtectionUnavailable = errors.New("customer data: identity protection unavailable")
)

// ValidationError mantém reason codes sanitizados sem repetir o valor recebido.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ErrValidation.Error()
	}
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", ErrValidation, e.Reason)
	}
	return fmt.Sprintf("%s: %s (%s)", ErrValidation, e.Field, e.Reason)
}

func (e *ValidationError) Unwrap() error { return ErrValidation }

func invalid(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}
