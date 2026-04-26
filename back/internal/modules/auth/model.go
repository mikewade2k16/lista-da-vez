package auth

import (
	"context"
	"time"
)

const (
	DemoTenantID   = "tenant-demo"
	StoreRiomarID  = "loja-pj-riomar"
	StoreJardinsID = "loja-pj-jardins"
)

type User struct {
	ID                 string
	DisplayName        string
	Email              string
	PasswordHash       string
	MustChangePassword bool
	AvatarPath         string
	Role               Role
	TenantID           string
	StoreIDs           []string
	Active             bool
	CreatedAt          time.Time
}

type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusRevoked  InvitationStatus = "revoked"
	InvitationStatusExpired  InvitationStatus = "expired"
)

type UserView struct {
	ID                 string   `json:"id"`
	DisplayName        string   `json:"displayName"`
	Email              string   `json:"email"`
	AvatarPath         string   `json:"avatarPath,omitempty"`
	MustChangePassword bool     `json:"mustChangePassword"`
	Role               Role     `json:"role"`
	TenantID           string   `json:"tenantId,omitempty"`
	StoreIDs           []string `json:"storeIds,omitempty"`
	Active             bool     `json:"active"`
}

type Principal struct {
	UserID              string
	DisplayName         string
	Email               string
	Role                Role
	TenantID            string
	StoreIDs            []string
	Permissions         []string
	PermissionsResolved bool
	ExpiresAt           time.Time
}

type LoginInput struct {
	Email    string
	Password string
}

type SessionView struct {
	AccessToken string    `json:"accessToken"`
	TokenType   string    `json:"tokenType"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type LoginResult struct {
	User    UserView    `json:"user"`
	Session SessionView `json:"session"`
}

type Invitation struct {
	ID              string
	UserID          string
	Email           string
	InvitedByUserID string
	Status          InvitationStatus
	ExpiresAt       time.Time
	AcceptedAt      *time.Time
	RevokedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PasswordResetStatus string

const (
	PasswordResetStatusPending  PasswordResetStatus = "pending"
	PasswordResetStatusConsumed PasswordResetStatus = "consumed"
	PasswordResetStatusRevoked  PasswordResetStatus = "revoked"
)

type PasswordReset struct {
	ID         string
	UserID     string
	Email      string
	Status     PasswordResetStatus
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type InvitationView struct {
	ID          string           `json:"id"`
	Email       string           `json:"email"`
	DisplayName string           `json:"displayName"`
	Role        Role             `json:"role"`
	TenantID    string           `json:"tenantId,omitempty"`
	StoreIDs    []string         `json:"storeIds,omitempty"`
	Status      InvitationStatus `json:"status"`
	ExpiresAt   time.Time        `json:"expiresAt"`
}

type InvitationIssueResult struct {
	Invitation InvitationView `json:"invitation"`
	InviteURL  string         `json:"inviteUrl"`
}

type InvitationInspectResult struct {
	Invitation InvitationView `json:"invitation"`
}

type InvitationAcceptInput struct {
	Token    string
	Password string
}

type PasswordResetRequestInput struct {
	Email string
}

type PasswordResetConfirmInput struct {
	Email    string
	Code     string
	Password string
}

type InvitationIssueInput struct {
	User            User
	InvitedByUserID string
}

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
	UpdateProfile(ctx context.Context, userID string, displayName string, email string) (User, error)
	UpdatePassword(ctx context.Context, userID string, passwordHash string, mustChangePassword bool) (User, error)
	UpdateAvatarPath(ctx context.Context, userID string, avatarPath string) (User, error)
}

type InvitationRepository interface {
	ReplacePendingInvitation(ctx context.Context, user User, invitedByUserID string, tokenHash string, expiresAt time.Time) (Invitation, error)
	FindInvitationByTokenHash(ctx context.Context, tokenHash string) (Invitation, User, error)
	AcceptInvitation(ctx context.Context, invitationID string, userID string, passwordHash string, acceptedAt time.Time) (User, error)
}

type PasswordResetRepository interface {
	ReplacePendingPasswordReset(ctx context.Context, user User, codeHash string, expiresAt time.Time) (PasswordReset, error)
	FindPasswordResetByEmailAndCodeHash(ctx context.Context, email string, codeHash string) (PasswordReset, User, error)
	ConsumePasswordReset(ctx context.Context, passwordResetID string, userID string, passwordHash string, consumedAt time.Time) (User, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) error
}

type TokenManager interface {
	Issue(user User) (SessionView, error)
	Parse(token string) (Principal, error)
}

type PermissionResolver interface {
	ResolveUserPermissions(ctx context.Context, userID string, role Role) ([]string, error)
}

type PasswordResetDelivery interface {
	DeliverPasswordResetCode(ctx context.Context, user User, code string, expiresAt time.Time) error
}

func (user User) View() UserView {
	return UserView{
		ID:                 user.ID,
		DisplayName:        user.DisplayName,
		Email:              user.Email,
		AvatarPath:         user.AvatarPath,
		MustChangePassword: user.MustChangePassword,
		Role:               user.Role,
		TenantID:           user.TenantID,
		StoreIDs:           append([]string{}, user.StoreIDs...),
		Active:             user.Active,
	}
}

type UpdateProfileInput struct {
	DisplayName string
	Email       string
}

type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

type UpdateAvatarInput struct {
	FileName    string
	ContentType string
	Content     []byte
}

type AvatarStorage interface {
	Save(ctx context.Context, userID string, fileName string, contentType string, content []byte, previousPath string) (string, error)
}
