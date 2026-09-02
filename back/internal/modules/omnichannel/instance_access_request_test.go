package omnichannel

import (
	"errors"
	"testing"
)

func TestNormalizeInstanceAccessRequestTranslatesLegacyUsersWithoutRemovingManager(t *testing.T) {
	managerID := "00000000-0000-0000-0000-000000000001"
	agentID := "00000000-0000-0000-0000-000000000002"
	userIDs := []string{agentID, agentID}
	responsible := managerID

	got, err := normalizeInstanceAccessRequest(InstanceAccessRequest{UserIDs: &userIDs}, storedInstanceAccess{
		AccessRevision:    7,
		AccessPolicy:      InstanceAccessPolicyRestricted,
		ResponsibleUserID: &responsible,
		Grants: []storedInstanceGrant{
			{UserID: managerID, AccessLevel: InstanceGrantManage, IsActive: true},
		},
	})
	if err != nil {
		t.Fatalf("normalize legacy request: %v", err)
	}
	if got.ExpectedRevision != 7 || got.AccessPolicy != InstanceAccessPolicyRestricted || got.ResponsibleUserID != managerID {
		t.Fatalf("unexpected legacy metadata: %+v", got)
	}
	levels := map[string]InstanceGrantLevel{}
	for _, grant := range got.Grants {
		levels[grant.UserID] = grant.AccessLevel
	}
	if levels[managerID] != InstanceGrantManage || levels[agentID] != InstanceGrantReply || len(levels) != 2 {
		t.Fatalf("unexpected translated grants: %#v", levels)
	}
}

func TestNormalizeInstanceAccessRequestRejectsMixedOrIncompleteContracts(t *testing.T) {
	revision := int64(1)
	policy := InstanceAccessPolicyRestricted
	grants := []InstanceGrantInput{{UserID: "00000000-0000-0000-0000-000000000001", AccessLevel: InstanceGrantManage}}
	legacy := []string{"00000000-0000-0000-0000-000000000002"}

	tests := []InstanceAccessRequest{
		{UserIDs: &legacy, AccessRevision: &revision},
		{AccessRevision: &revision, AccessPolicy: &policy},
		{AccessPolicy: &policy, Grants: &grants},
	}
	for index, input := range tests {
		if _, err := normalizeInstanceAccessRequest(input, storedInstanceAccess{}); !errors.Is(err, ErrInvalidBody) {
			t.Fatalf("case %d: expected invalid body, got %v", index, err)
		}
	}
}

func TestNormalizeInstanceAccessRequestPreservesExplicitEmptyGrantList(t *testing.T) {
	revision := int64(4)
	policy := InstanceAccessPolicyRestricted
	responsible := ""
	grants := []InstanceGrantInput{}

	got, err := normalizeInstanceAccessRequest(InstanceAccessRequest{
		AccessRevision: &revision, AccessPolicy: &policy,
		ResponsibleUserID: &responsible, Grants: &grants,
	}, storedInstanceAccess{})
	if err != nil {
		t.Fatalf("normalize explicit request: %v", err)
	}
	if got.ExpectedRevision != revision || got.AccessPolicy != policy || len(got.Grants) != 0 {
		t.Fatalf("unexpected explicit request: %+v", got)
	}
}
