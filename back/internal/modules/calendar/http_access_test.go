package calendar

import "testing"

func TestContainsCalendarPermissionMatchesExactKey(t *testing.T) {
	t.Parallel()

	permissions := []string{" calendar.view ", "calendar.manage.extra"}
	if !containsPermission(permissions, "calendar.view") {
		t.Fatal("expected exact trimmed permission to match")
	}
	if containsPermission(permissions, "calendar.manage") {
		t.Fatal("permission prefixes must not grant access")
	}
}
