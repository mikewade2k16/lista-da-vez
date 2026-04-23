package auth

import "errors"

var (
	ErrInvalidCredentials    = errors.New("auth: invalid credentials")
	ErrUnauthorized          = errors.New("auth: unauthorized")
	ErrForbidden             = errors.New("auth: forbidden")
	ErrUserInactive          = errors.New("auth: user inactive")
	ErrOnboardingRequired    = errors.New("auth: onboarding required")
	ErrInvalidRoleScope      = errors.New("auth: invalid role scope")
	ErrConflict              = errors.New("auth: conflict")
	ErrInvalidAvatar         = errors.New("auth: invalid avatar")
	ErrInvitationNotFound    = errors.New("auth: invitation not found")
	ErrInvitationExpired     = errors.New("auth: invitation expired")
	ErrInvitationAccepted    = errors.New("auth: invitation accepted")
	ErrInvitationRevoked     = errors.New("auth: invitation revoked")
	ErrPasswordResetNotFound = errors.New("auth: password reset not found")
	ErrPasswordResetExpired  = errors.New("auth: password reset expired")
	ErrPasswordResetConsumed = errors.New("auth: password reset consumed")
)
