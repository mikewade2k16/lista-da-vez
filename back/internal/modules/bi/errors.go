package bi

import "errors"

var (
	ErrConfiguration       = errors.New("bi: missing configuration")
	ErrValidation          = errors.New("bi: validation")
	ErrUnsupportedEndpoint = errors.New("bi: unsupported endpoint")
	ErrUnsupportedDataset  = errors.New("bi: unsupported dataset")
	ErrFilterRequired      = errors.New("bi: required filter missing")
	ErrSalesUnauthorized   = errors.New("bi: sales endpoint unauthorized")
	ErrUpstream            = errors.New("bi: upstream request failed")
)
