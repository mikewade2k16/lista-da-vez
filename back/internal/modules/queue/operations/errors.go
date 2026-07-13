package operations

import "errors"

var (
	ErrForbidden                                  = errors.New("operations: forbidden")
	ErrStoreRequired                              = errors.New("operations: store required")
	ErrStoreNotFound                              = errors.New("operations: store not found")
	ErrValidation                                 = errors.New("operations: validation error")
	ErrConsultantNotFound                         = errors.New("operations: consultant not found")
	ErrConsultantBusy                             = errors.New("operations: consultant busy")
	ErrConsultantNotAvailable                     = errors.New("operations: consultant not in service")
	ErrConcurrentServiceLimitReached              = errors.New("operations: concurrent service limit reached")
	ErrConcurrentServiceLimitPerConsultantReached = errors.New("operations: concurrent service limit per consultant reached")
	// ErrPendingNotFound: nao ha pendencia auto-encerrada (validation_status='pending')
	// para (store_id, service_id) ao validar/cancelar. Mapeado para 404.
	ErrPendingNotFound = errors.New("operations: pending validation not found")
)
