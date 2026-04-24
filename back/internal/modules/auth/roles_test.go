package auth

import "testing"

func TestValidateUserScopeAllowsDirectorTenantScope(t *testing.T) {
	err := ValidateUserScope(User{
		ID:       "usr-director",
		Role:     RoleDirector,
		TenantID: "tenant-1",
	})
	if err != nil {
		t.Fatalf("expected director tenant scope to be valid, got %v", err)
	}
}

func TestValidateUserScopeRejectsDirectorWithoutTenant(t *testing.T) {
	err := ValidateUserScope(User{
		ID:   "usr-director",
		Role: RoleDirector,
	})
	if err == nil {
		t.Fatal("expected director without tenant to be rejected")
	}
}
