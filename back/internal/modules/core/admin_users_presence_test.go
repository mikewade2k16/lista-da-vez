package core

import (
	"context"
	"testing"
)

type presenceFilterRepo struct {
	AdminUserRepository
	gotFilter AdminUserListFilter
}

func (r *presenceFilterRepo) ListUsers(_ context.Context, filter AdminUserListFilter) ([]AdminUserListItem, int, error) {
	r.gotFilter = filter
	return []AdminUserListItem{}, 0, nil
}

func TestListUsersIncludesPresenceOnlyForPlatformAdmin(t *testing.T) {
	cases := []struct {
		name            string
		isPlatformAdmin bool
	}{
		{name: "platform admin", isPlatformAdmin: true},
		{name: "delegated admin", isPlatformAdmin: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &presenceFilterRepo{}
			scope := NewAdminScopeResolver(fakeScopeRepo{isPlatformAdmin: tc.isPlatformAdmin, isAdminOfAny: true})
			svc := NewAdminUserService(repo, nil, scope, nil)

			// IncludePresence vindo "de fora" nunca deve prevalecer sobre o ator.
			_, err := svc.ListUsers(context.Background(), AdminUserListFilter{ActorUserID: "actor-1", IncludePresence: true})
			if err != nil {
				t.Fatalf("ListUsers: %v", err)
			}
			if repo.gotFilter.IncludePresence != tc.isPlatformAdmin {
				t.Fatalf("IncludePresence = %v, want %v", repo.gotFilter.IncludePresence, tc.isPlatformAdmin)
			}
		})
	}
}
