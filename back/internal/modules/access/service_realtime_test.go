package access

import (
	"context"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type accessRealtimeTestRepository struct {
	activeTenantIDs       []string
	replacedRole          auth.Role
	replacedRoleKeys      []string
	replacedUserID        string
	replacedUserOverrides []UserOverride
}

func (repository *accessRealtimeTestRepository) ListRolePermissions(context.Context, auth.Role) ([]string, error) {
	return nil, nil
}

func (repository *accessRealtimeTestRepository) ListAllRolePermissions(context.Context) ([]RoleGrant, error) {
	return nil, nil
}

func (repository *accessRealtimeTestRepository) ListActiveTenantIDs(context.Context) ([]string, error) {
	return append([]string{}, repository.activeTenantIDs...), nil
}

func (repository *accessRealtimeTestRepository) ReplaceRolePermissions(_ context.Context, role auth.Role, permissionKeys []string) error {
	repository.replacedRole = role
	repository.replacedRoleKeys = append([]string{}, permissionKeys...)
	return nil
}

func (repository *accessRealtimeTestRepository) ListUserOverrides(context.Context, string) ([]UserOverride, error) {
	return nil, nil
}

func (repository *accessRealtimeTestRepository) ReplaceUserOverrides(_ context.Context, userID string, overrides []UserOverride, _ string) ([]UserOverride, error) {
	repository.replacedUserID = userID
	repository.replacedUserOverrides = append([]UserOverride{}, overrides...)
	return append([]UserOverride{}, overrides...), nil
}

type accessRealtimeTestSubjectResolver struct {
	subject UserSubject
}

func (resolver accessRealtimeTestSubjectResolver) FindAccessibleSubject(context.Context, auth.Principal, string) (UserSubject, error) {
	return resolver.subject, nil
}

type accessRealtimePublishedEvent struct {
	tenantID   string
	resource   string
	action     string
	resourceID string
	savedAt    time.Time
}

type accessRealtimeTestPublisher struct {
	events []accessRealtimePublishedEvent
}

func (publisher *accessRealtimeTestPublisher) PublishContextEvent(_ context.Context, tenantID string, resource string, action string, resourceID string, savedAt time.Time) {
	publisher.events = append(publisher.events, accessRealtimePublishedEvent{
		tenantID:   tenantID,
		resource:   resource,
		action:     action,
		resourceID: resourceID,
		savedAt:    savedAt,
	})
}

func TestUpdateRolePermissionsPublishesContextUpdatesForActiveTenants(t *testing.T) {
	repository := &accessRealtimeTestRepository{
		activeTenantIDs: []string{"tenant-a", "tenant-b", "tenant-a"},
	}
	publisher := &accessRealtimeTestPublisher{}
	service := NewService(repository, nil)
	service.SetContextPublisher(publisher)

	_, err := service.UpdateRolePermissions(context.Background(), auth.Principal{Role: auth.RolePlatformAdmin}, auth.RoleManager, []string{PermissionUsersView})
	if err != nil {
		t.Fatalf("UpdateRolePermissions returned error: %v", err)
	}

	if len(publisher.events) != 2 {
		t.Fatalf("expected 2 context events, got %d", len(publisher.events))
	}

	for _, event := range publisher.events {
		if event.resource != "access" {
			t.Fatalf("expected access resource, got %q", event.resource)
		}
		if event.action != "role-defaults-updated" {
			t.Fatalf("expected role-defaults-updated action, got %q", event.action)
		}
		if event.resourceID != string(auth.RoleManager) {
			t.Fatalf("expected resource id %q, got %q", auth.RoleManager, event.resourceID)
		}
		if event.savedAt.IsZero() {
			t.Fatal("expected savedAt to be set")
		}
	}
}

func TestUpdateUserOverridesPublishesContextUpdateForSubjectTenant(t *testing.T) {
	repository := &accessRealtimeTestRepository{}
	publisher := &accessRealtimeTestPublisher{}
	service := NewService(repository, accessRealtimeTestSubjectResolver{subject: UserSubject{
		UserID:   "user-1",
		Role:     auth.RoleManager,
		TenantID: "tenant-a",
		StoreIDs: []string{"store-1"},
	}})
	service.SetContextPublisher(publisher)

	_, err := service.UpdateUserOverrides(context.Background(), auth.Principal{
		UserID: "admin-1",
		Role:   auth.RoleOwner,
		PermissionsResolved: true,
		Permissions: []string{PermissionUsersEdit},
	}, "user-1", []UserOverride{{
		PermissionKey: PermissionUsersView,
		Effect:        EffectAllow,
	}})
	if err != nil {
		t.Fatalf("UpdateUserOverrides returned error: %v", err)
	}

	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 context event, got %d", len(publisher.events))
	}

	event := publisher.events[0]
	if event.tenantID != "tenant-a" {
		t.Fatalf("expected tenant-a, got %q", event.tenantID)
	}
	if event.resource != "access" {
		t.Fatalf("expected access resource, got %q", event.resource)
	}
	if event.action != "user-overrides-updated" {
		t.Fatalf("expected user-overrides-updated action, got %q", event.action)
	}
	if event.resourceID != "user-1" {
		t.Fatalf("expected resource id user-1, got %q", event.resourceID)
	}
}