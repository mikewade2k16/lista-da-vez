package reports

import (
	"testing"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestCanViewReportsAcceptsLegacyQueueReportsPermission(t *testing.T) {
	principal := auth.Principal{
		Role:                auth.RoleStoreTerminal,
		PermissionsResolved: true,
		Permissions:         []string{accesscontrol.PermissionQueueReportsRead},
	}

	if !canViewReports(principal) {
		t.Fatalf("expected legacy queue reports permission to allow reports")
	}
}
