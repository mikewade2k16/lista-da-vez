package analytics

import "errors"

var (
	ErrForbidden     = errors.New("analytics: forbidden")
	ErrStoreRequired = errors.New("analytics: store required")
	ErrScopeRequired = errors.New("analytics: scope required")
)
