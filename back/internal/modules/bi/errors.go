package bi

import "errors"

var (
	ErrConfiguration       = errors.New("bi: missing configuration")
	ErrValidation          = errors.New("bi: validation")
	ErrUnsupportedEndpoint = errors.New("bi: unsupported endpoint")
	ErrUpstream            = errors.New("bi: upstream request failed")
)
