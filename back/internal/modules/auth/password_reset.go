package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const passwordResetCodeLength = 6

type PasswordResetService struct {
	users      UserRepository
	repository PasswordResetRepository
	password   PasswordHasher
	delivery   PasswordResetDelivery
	resetTTL   time.Duration
}

func NewPasswordResetService(users UserRepository, repository PasswordResetRepository, password PasswordHasher, delivery PasswordResetDelivery, resetTTL time.Duration) *PasswordResetService {
	return &PasswordResetService{
		users:      users,
		repository: repository,
		password:   password,
		delivery:   delivery,
		resetTTL:   resetTTL,
	}
}

func (service *PasswordResetService) Request(ctx context.Context, input PasswordResetRequestInput) error {
	email := normalizePasswordResetEmail(input.Email)
	if email == "" {
		return ErrInvalidCredentials
	}

	user, err := service.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return nil
		}

		return err
	}

	if !user.Active || strings.TrimSpace(user.PasswordHash) == "" {
		return nil
	}

	code, codeHash, err := generatePasswordResetCode(email)
	if err != nil {
		return err
	}

	expiresAt := time.Now().UTC().Add(service.normalizedTTL())
	if _, err := service.repository.ReplacePendingPasswordReset(ctx, user, codeHash, expiresAt); err != nil {
		return err
	}

	if service.delivery != nil {
		if err := service.delivery.DeliverPasswordResetCode(ctx, user, code, expiresAt); err != nil {
			return err
		}
	}

	return nil
}

func (service *PasswordResetService) Confirm(ctx context.Context, input PasswordResetConfirmInput) (UserView, error) {
	email := normalizePasswordResetEmail(input.Email)
	code := normalizePasswordResetCode(input.Code)
	password := strings.TrimSpace(input.Password)

	if email == "" || len(code) != passwordResetCodeLength || len(password) < 8 {
		return UserView{}, ErrInvalidCredentials
	}

	reset, user, err := service.repository.FindPasswordResetByEmailAndCodeHash(ctx, email, hashPasswordResetCode(email, code))
	if err != nil {
		return UserView{}, err
	}

	if err := validatePasswordReset(reset); err != nil {
		return UserView{}, err
	}

	if !user.Active {
		return UserView{}, ErrUserInactive
	}

	if strings.TrimSpace(user.PasswordHash) == "" {
		return UserView{}, ErrOnboardingRequired
	}

	passwordHash, err := service.password.Hash(password)
	if err != nil {
		return UserView{}, err
	}

	updated, err := service.repository.ConsumePasswordReset(ctx, reset.ID, user.ID, passwordHash, time.Now().UTC())
	if err != nil {
		return UserView{}, err
	}

	return updated.View(), nil
}

func (service *PasswordResetService) normalizedTTL() time.Duration {
	if service.resetTTL <= 0 {
		return 30 * time.Minute
	}

	return service.resetTTL
}

func validatePasswordReset(reset PasswordReset) error {
	switch reset.Status {
	case PasswordResetStatusConsumed:
		return ErrPasswordResetConsumed
	case PasswordResetStatusPending:
		if reset.ExpiresAt.Before(time.Now().UTC()) {
			return ErrPasswordResetExpired
		}

		return nil
	default:
		return ErrPasswordResetNotFound
	}
}

func generatePasswordResetCode(email string) (string, string, error) {
	max := big.NewInt(1000000)
	value, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", "", err
	}

	code := fmt.Sprintf("%06d", value.Int64())
	return code, hashPasswordResetCode(email, code), nil
}

func hashPasswordResetCode(email string, code string) string {
	sum := sha256.Sum256([]byte(normalizePasswordResetEmail(email) + ":" + normalizePasswordResetCode(code)))
	return hex.EncodeToString(sum[:])
}

func normalizePasswordResetEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizePasswordResetCode(value string) string {
	builder := strings.Builder{}
	builder.Grow(passwordResetCodeLength)

	for _, char := range value {
		if char < '0' || char > '9' {
			continue
		}

		builder.WriteRune(char)
		if builder.Len() == passwordResetCodeLength {
			break
		}
	}

	return builder.String()
}
