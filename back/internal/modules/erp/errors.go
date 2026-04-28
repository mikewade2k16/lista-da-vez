package erp

import "errors"

var (
	ErrForbidden             = errors.New("erp: forbidden")
	ErrValidation            = errors.New("erp: validation")
	ErrTenantRequired        = errors.New("erp: tenant required")
	ErrStoreRequired         = errors.New("erp: store required")
	ErrStoreNotFound         = errors.New("erp: store not found")
	ErrManualSyncDisabled    = errors.New("erp: manual sync disabled")
	ErrSourceNotConfigured   = errors.New("erp: source not configured")
	ErrSourcePathOutsideRoot = errors.New("erp: source path outside root")
	ErrUnsupportedDataType   = errors.New("erp: unsupported data type")
)
