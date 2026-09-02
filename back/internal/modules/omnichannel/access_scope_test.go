package omnichannel

import "testing"

func TestResolveInstanceCapabilitiesMatrix(t *testing.T) {
	t.Parallel()

	all := instanceFeaturePermissions{View: true, Reply: true, Manage: true, ResetHistory: true}
	tests := []struct {
		name        string
		policy      InstanceAccessPolicy
		grant       InstanceGrantLevel
		permissions instanceFeaturePermissions
		want        InstanceCapabilities
	}{
		{
			name: "restricted without grant is closed", policy: InstanceAccessPolicyRestricted,
			permissions: all, want: InstanceCapabilities{},
		},
		{
			name: "view grant does not imply reply", policy: InstanceAccessPolicyRestricted,
			grant: InstanceGrantView, permissions: all,
			want: InstanceCapabilities{View: true},
		},
		{
			name: "reply grant includes view", policy: InstanceAccessPolicyRestricted,
			grant: InstanceGrantReply, permissions: all,
			want: InstanceCapabilities{View: true, Reply: true},
		},
		{
			name: "manage grant includes all data capabilities", policy: InstanceAccessPolicyRestricted,
			grant: InstanceGrantManage, permissions: all,
			want: InstanceCapabilities{View: true, Reply: true, Manage: true, ResetHistory: true},
		},
		{
			name: "shared implies view and reply but never manage", policy: InstanceAccessPolicyAccountShared,
			permissions: all,
			want:        InstanceCapabilities{View: true, Reply: true},
		},
		{
			name: "grant never replaces feature permission", policy: InstanceAccessPolicyRestricted,
			grant:       InstanceGrantManage,
			permissions: instanceFeaturePermissions{View: true, Manage: false, ResetHistory: true},
			want:        InstanceCapabilities{View: true},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, _ := resolveInstanceCapabilities(test.policy, test.grant, test.permissions)
			if got != test.want {
				t.Fatalf("capabilities=%+v, want %+v", got, test.want)
			}
		})
	}
}

func TestNormalizeInstanceGrantInputsRejectsConflictingDuplicates(t *testing.T) {
	t.Parallel()
	_, err := normalizeInstanceGrantInputs([]InstanceGrantInput{
		{UserID: "11111111-1111-4111-8111-111111111111", AccessLevel: InstanceGrantView},
		{UserID: "11111111-1111-4111-8111-111111111111", AccessLevel: InstanceGrantManage},
	})
	if err == nil {
		t.Fatal("conflicting duplicate grant was accepted")
	}
}

func TestConversationVisibilityUsesManageGrantWithoutLifecyclePermission(t *testing.T) {
	t.Parallel()
	scope := ConversationAccessScope{
		UserID: "55555555-5555-4555-8555-555555555555",
		Instances: map[string]InstanceAccessDecision{
			"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa": {
				InstanceID:   "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
				InstanceName: "main",
				GrantLevel:   InstanceGrantManage,
				IsActive:     true,
				Capabilities: InstanceCapabilities{View: true, Reply: true},
			},
		},
	}

	visibility := scope.conversationVisibility(InstanceGrantReply)
	if len(visibility.InstanceScopeKeys) != 1 || visibility.InstanceScopeKeys[0] != "main" {
		t.Fatalf("visible scope keys=%v", visibility.InstanceScopeKeys)
	}
	if len(visibility.ManageInstanceScopeKeys) != 1 || visibility.ManageInstanceScopeKeys[0] != "main" {
		t.Fatalf("manage scope keys=%v", visibility.ManageInstanceScopeKeys)
	}
}
