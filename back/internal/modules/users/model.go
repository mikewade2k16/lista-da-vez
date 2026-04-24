package users

import (
	"context"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type User struct {
	ID                 string
	DisplayName        string
	Email              string
	EmployeeCode       string
	JobTitle           string
	Role               auth.Role
	TenantID           string
	StoreIDs           []string
	Active             bool
	HasPassword        bool
	MustChangePassword bool
	ManagedBy          string
	ManagedResourceID  string
	Invitation         InvitationSummary
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type InvitationSummary struct {
	Status    auth.InvitationStatus
	ExpiresAt *time.Time
}

type OnboardingView struct {
	Status              string     `json:"status"`
	HasPassword         bool       `json:"hasPassword"`
	MustChangePassword  bool       `json:"mustChangePassword"`
	InvitationExpiresAt *time.Time `json:"invitationExpiresAt,omitempty"`
}

type UserView struct {
	ID                string         `json:"id"`
	DisplayName       string         `json:"displayName"`
	Email             string         `json:"email"`
	EmployeeCode      string         `json:"employeeCode,omitempty"`
	JobTitle          string         `json:"jobTitle,omitempty"`
	Role              auth.Role      `json:"role"`
	TenantID          string         `json:"tenantId,omitempty"`
	StoreIDs          []string       `json:"storeIds,omitempty"`
	Active            bool           `json:"active"`
	ManagedBy         string         `json:"managedBy,omitempty"`
	ManagedResourceID string         `json:"managedResourceId,omitempty"`
	Onboarding        OnboardingView `json:"onboarding"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
}

type ListInput struct {
	TenantID string
	StoreID  string
	Role     auth.Role
	Active   *bool
}

type CreateInput struct {
	DisplayName  string
	Email        string
	EmployeeCode string
	Password     string
	Role         auth.Role
	TenantID     string
	StoreIDs     []string
	Active       *bool
}

type UpdateInput struct {
	ID           string
	DisplayName  *string
	Email        *string
	EmployeeCode *string
	Password     *string
	Role         *auth.Role
	TenantID     *string
	StoreIDs     *[]string
	Active       *bool
}

type CreateResult struct {
	User       UserView
	Invitation *auth.InvitationIssueResult
}

type InviteResult struct {
	User       UserView
	Invitation auth.InvitationIssueResult
}

type ResetPasswordResult struct {
	User              UserView `json:"user"`
	TemporaryPassword string   `json:"temporaryPassword"`
}

type StoreScope struct {
	ID       string
	TenantID string
	Active   bool
}

type Repository interface {
	ListAccessible(ctx context.Context, principal auth.Principal, input ListInput) ([]User, error)
	FindAccessibleByID(ctx context.Context, principal auth.Principal, userID string) (User, error)
	ResolveStoreScopes(ctx context.Context, storeIDs []string) ([]StoreScope, error)
	Create(ctx context.Context, user User, passwordHash *string) (User, error)
	Update(ctx context.Context, user User, passwordHash *string) (User, error)
}

func (user User) View() UserView {
	return UserView{
		ID:                user.ID,
		DisplayName:       user.DisplayName,
		Email:             user.Email,
		EmployeeCode:      user.EmployeeCode,
		JobTitle:          user.JobTitle,
		Role:              user.Role,
		TenantID:          user.TenantID,
		StoreIDs:          cloneStringSlice(user.StoreIDs),
		Active:            user.Active,
		ManagedBy:         user.ManagedBy,
		ManagedResourceID: user.ManagedResourceID,
		Onboarding: OnboardingView{
			Status:              user.OnboardingStatus(),
			HasPassword:         user.HasPassword,
			MustChangePassword:  user.MustChangePassword,
			InvitationExpiresAt: cloneTimePointer(user.Invitation.ExpiresAt),
		},
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func (user User) OnboardingStatus() string {
	if !user.Active {
		return "inactive"
	}

	if user.HasPassword {
		if user.MustChangePassword {
			return "password_change_required"
		}
		return "ready"
	}

	switch user.Invitation.Status {
	case auth.InvitationStatusPending:
		return "pending"
	case auth.InvitationStatusExpired:
		return "expired"
	case auth.InvitationStatusRevoked:
		return "needs_invite"
	default:
		return "needs_invite"
	}
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}
