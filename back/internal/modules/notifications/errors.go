package notifications

import "errors"

var (
	ErrAccountRequired      = errors.New("notifications: account required")
	ErrAccountNotFound      = errors.New("notifications: account not found")
	ErrNotificationNotFound = errors.New("notifications: notification not found")
	ErrForbidden            = errors.New("notifications: forbidden")
	ErrInvalid              = errors.New("notifications: invalid")
	ErrMuted                = errors.New("notifications: muted")
	ErrNotConfigured        = errors.New("notifications: channel not configured")
)
