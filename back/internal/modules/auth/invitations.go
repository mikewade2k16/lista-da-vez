package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"
)

type InvitationService struct {
	repository InvitationRepository
	password   PasswordHasher
	tokens     TokenManager
	webAppURL  string
	inviteTTL  time.Duration
}

func NewInvitationService(repository InvitationRepository, password PasswordHasher, tokens TokenManager, webAppURL string, inviteTTL time.Duration) *InvitationService {
	return &InvitationService{
		repository: repository,
		password:   password,
		tokens:     tokens,
		webAppURL:  strings.TrimRight(strings.TrimSpace(webAppURL), "/"),
		inviteTTL:  inviteTTL,
	}
}

func (service *InvitationService) Issue(ctx context.Context, input InvitationIssueInput) (InvitationIssueResult, error) {
	user := input.User
	if strings.TrimSpace(user.ID) == "" || strings.TrimSpace(user.Email) == "" || !user.Active {
		return InvitationIssueResult{}, ErrUserInactive
	}

	if strings.TrimSpace(user.PasswordHash) != "" {
		return InvitationIssueResult{}, ErrInvitationAccepted
	}

	token, tokenHash, err := generateInvitationToken()
	if err != nil {
		return InvitationIssueResult{}, err
	}

	expiresAt := time.Now().UTC().Add(service.normalizedTTL())
	invitation, err := service.repository.ReplacePendingInvitation(ctx, user, strings.TrimSpace(input.InvitedByUserID), tokenHash, expiresAt)
	if err != nil {
		return InvitationIssueResult{}, err
	}

	return InvitationIssueResult{
		Invitation: buildInvitationView(invitation, user),
		InviteURL:  service.buildInviteURL(token),
	}, nil
}

func (service *InvitationService) Inspect(ctx context.Context, token string) (InvitationInspectResult, error) {
	invitation, user, err := service.repository.FindInvitationByTokenHash(ctx, hashInvitationToken(token))
	if err != nil {
		return InvitationInspectResult{}, err
	}

	if err := validateInvitation(invitation); err != nil {
		return InvitationInspectResult{}, err
	}

	return InvitationInspectResult{
		Invitation: buildInvitationView(invitation, user),
	}, nil
}

func (service *InvitationService) Accept(ctx context.Context, input InvitationAcceptInput) (LoginResult, error) {
	token := strings.TrimSpace(input.Token)
	password := strings.TrimSpace(input.Password)

	if token == "" || password == "" {
		return LoginResult{}, ErrInvalidCredentials
	}

	invitation, user, err := service.repository.FindInvitationByTokenHash(ctx, hashInvitationToken(token))
	if err != nil {
		return LoginResult{}, err
	}

	if err := validateInvitation(invitation); err != nil {
		return LoginResult{}, err
	}

	passwordHash, err := service.password.Hash(password)
	if err != nil {
		return LoginResult{}, err
	}

	acceptedAt := time.Now().UTC()
	updatedUser, err := service.repository.AcceptInvitation(ctx, invitation.ID, user.ID, passwordHash, acceptedAt)
	if err != nil {
		return LoginResult{}, err
	}

	session, err := service.tokens.Issue(updatedUser)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		User:    updatedUser.View(),
		Session: session,
	}, nil
}

func (service *InvitationService) buildInviteURL(token string) string {
	base := service.webAppURL
	if base == "" {
		base = "http://localhost:3003"
	}

	return base + "/auth/convite/" + token
}

func (service *InvitationService) normalizedTTL() time.Duration {
	if service.inviteTTL <= 0 {
		return 7 * 24 * time.Hour
	}

	return service.inviteTTL
}

func buildInvitationView(invitation Invitation, user User) InvitationView {
	status := invitation.Status
	if status == InvitationStatusPending && invitation.ExpiresAt.Before(time.Now().UTC()) {
		status = InvitationStatusExpired
	}

	return InvitationView{
		ID:          invitation.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		TenantID:    user.TenantID,
		StoreIDs:    append([]string{}, user.StoreIDs...),
		Status:      status,
		ExpiresAt:   invitation.ExpiresAt,
	}
}

func validateInvitation(invitation Invitation) error {
	switch invitation.Status {
	case InvitationStatusAccepted:
		return ErrInvitationAccepted
	case InvitationStatusRevoked:
		return ErrInvitationRevoked
	case InvitationStatusPending:
		if invitation.ExpiresAt.Before(time.Now().UTC()) {
			return ErrInvitationExpired
		}

		return nil
	default:
		return ErrInvitationNotFound
	}
}

func generateInvitationToken() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}

	token := base64.RawURLEncoding.EncodeToString(buffer)
	return token, hashInvitationToken(token), nil
}

func hashInvitationToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
