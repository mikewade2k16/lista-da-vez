package customerintelligence

import (
	"testing"
)

func TestModuleCatalogMatchesGovernedSurface(t *testing.T) {
	t.Parallel()
	module := New()
	if module.ID() != ModuleID || module.Metadata().SchemaName != "intelligence" {
		t.Fatalf("metadata inesperada: %#v", module.Metadata())
	}
	permissions := module.Permissions()
	if len(permissions) != 14 {
		t.Fatalf("permissions = %d, want 14", len(permissions))
	}
	platform := 0
	for _, permission := range permissions {
		if permission.Scope == "platform" {
			platform++
		}
	}
	if platform != 2 {
		t.Fatalf("permissoes platform = %d, want 2", platform)
	}
	if module.Service() != nil || module.Runtime() != nil {
		t.Fatal("facades devem ser nil antes de Build")
	}
}
