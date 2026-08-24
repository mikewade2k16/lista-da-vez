package auth

import "testing"

func TestCanConfigureAssistant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		principal Principal
		want      bool
	}{
		{name: "platform admin bypass", principal: Principal{Role: RolePlatformAdmin}, want: true},
		{name: "owner bypass", principal: Principal{Role: RoleOwner}, want: true},
		{
			name: "calendar manager",
			principal: Principal{
				PermissionsResolved: true,
				Permissions:         []string{"calendar.manage"},
			},
			want: true,
		},
		{
			name: "meta ads manager",
			principal: Principal{
				PermissionsResolved: true,
				Permissions:         []string{"meta_ads.manage"},
			},
			want: true,
		},
		{
			name: "omnichannel manager",
			principal: Principal{
				PermissionsResolved: true,
				Permissions:         []string{"omnichannel.agents.manage"},
			},
			want: true,
		},
		{
			name: "automation manager with whitespace",
			principal: Principal{
				PermissionsResolved: true,
				Permissions:         []string{" automation.manage "},
			},
			want: true,
		},
		{
			name: "workspace manager",
			principal: Principal{
				PermissionsResolved: true,
				Permissions:         []string{"workspace.configuracoes.edit"},
			},
			want: true,
		},
		{
			name: "core account manager",
			principal: Principal{
				PermissionsResolved: true,
				Permissions:         []string{"core.account.manage"},
			},
			want: true,
		},
		{
			name: "read only denied",
			principal: Principal{
				PermissionsResolved: true,
				Permissions:         []string{"calendar.view", "meta_ads.view"},
			},
		},
		{
			name:      "unresolved permissions fail closed",
			principal: Principal{Permissions: []string{"calendar.manage"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CanConfigureAssistant(tt.principal); got != tt.want {
				t.Fatalf("CanConfigureAssistant() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestCanManageAssistantCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		permission string
		want       bool
	}{
		{name: "automation admin", permission: "automation.manage", want: true},
		{name: "core account admin", permission: "core.account.manage", want: true},
		{name: "workspace admin", permission: "workspace.configuracoes.edit", want: true},
		{name: "calendar manager denied", permission: "calendar.manage"},
		{name: "meta ads manager denied", permission: "meta_ads.manage"},
		{name: "omnichannel manager denied on neutral vault", permission: "omnichannel.agents.manage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			principal := Principal{
				PermissionsResolved: true,
				Permissions:         []string{tt.permission},
			}
			if got := CanManageAssistantCredentials(principal); got != tt.want {
				t.Fatalf("CanManageAssistantCredentials() = %t, want %t", got, tt.want)
			}
		})
	}

	if !CanManageAssistantCredentials(Principal{Role: RoleOwner}) {
		t.Fatal("owner must manage assistant credentials")
	}
	if !CanManageAssistantCredentials(Principal{Role: RolePlatformAdmin}) {
		t.Fatal("platform admin must manage assistant credentials")
	}
}

func TestCanManageAssistantConfiguration(t *testing.T) {
	t.Parallel()

	for _, permission := range []string{
		"automation.manage", "core.account.manage", "workspace.configuracoes.edit",
	} {
		principal := Principal{PermissionsResolved: true, Permissions: []string{permission}}
		if !CanManageAssistantConfiguration(principal) {
			t.Fatalf("permission %q should manage the shared assistant configuration", permission)
		}
	}
	for _, permission := range []string{
		"calendar.manage", "meta_ads.manage", "omnichannel.agents.manage",
	} {
		principal := Principal{PermissionsResolved: true, Permissions: []string{permission}}
		if CanManageAssistantConfiguration(principal) {
			t.Fatalf("module-only permission %q must not reconfigure other surfaces", permission)
		}
	}
	if CanManageAssistantConfiguration(Principal{Permissions: []string{"automation.manage"}}) {
		t.Fatal("unresolved permissions must fail closed")
	}
	if !CanManageAssistantConfiguration(Principal{Role: RoleOwner}) ||
		!CanManageAssistantConfiguration(Principal{Role: RolePlatformAdmin}) {
		t.Fatal("owner and platform admin must manage the shared assistant configuration")
	}
}
