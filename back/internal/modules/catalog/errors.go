package catalog

import "errors"

var (
	ErrTenantRequired           = errors.New("catalog: tenant_id is required")
	ErrStoreRequired            = errors.New("catalog: store_id is required")
	ErrSearchTermTooShort       = errors.New("catalog: search term must have at least 3 characters")
	ErrUnsupportedProductSource = errors.New("catalog: unsupported product source")
)
