package app

import (
	"context"
	"errors"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/users"
)

type accessSubjectResolver struct {
	repository *users.PostgresRepository
}

func newAccessSubjectResolver(repository *users.PostgresRepository) *accessSubjectResolver {
	return &accessSubjectResolver{repository: repository}
}

func (resolver *accessSubjectResolver) FindAccessibleSubject(ctx context.Context, principal auth.Principal, userID string) (access.UserSubject, error) {
	user, err := resolver.repository.FindAccessibleByID(ctx, principal, userID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return access.UserSubject{}, access.ErrNotFound
		}

		return access.UserSubject{}, err
	}

	return access.UserSubject{
		UserID:    user.ID,
		Role:      user.Role,
		TenantID:  user.TenantID,
		StoreIDs:  append([]string{}, user.StoreIDs...),
		IsActive:  user.Active,
		ManagedBy: user.ManagedBy,
	}, nil
}
