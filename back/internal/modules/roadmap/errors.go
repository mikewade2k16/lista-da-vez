package roadmap

import "errors"

var (
	ErrAccountRequired    = errors.New("roadmap: account required")
	ErrAccountNotFound    = errors.New("roadmap: account not found")
	ErrForbidden          = errors.New("roadmap: forbidden")
	ErrInvalid            = errors.New("roadmap: invalid")
	ErrNotFound           = errors.New("roadmap: not found")
	ErrCannotDeleteGlobal = errors.New("roadmap: cannot delete global record")
)
