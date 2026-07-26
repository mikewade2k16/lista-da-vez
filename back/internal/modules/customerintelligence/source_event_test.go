package customerintelligence

import (
	"context"
	"encoding/json"
	"testing"
)

type sourceEventFoundationFake struct {
	FoundationRepository
	configs  []SourceConfig
	requests []SourceSyncRequest
}

func (f *sourceEventFoundationFake) GetCapability(
	_ context.Context,
	scope Scope,
	key, scopeKey string,
) (Capability, error) {
	return Capability{
		AccountID: scope.AccountID, ClientAccountID: scope.ClientAccountID,
		Key: key, ScopeKey: scopeKey, Mode: "on", Config: json.RawMessage(`{}`),
	}, nil
}

func (f *sourceEventFoundationFake) ListSourceConfigs(
	_ context.Context,
	_ Scope,
) ([]SourceConfig, error) {
	return append([]SourceConfig(nil), f.configs...), nil
}

func (f *sourceEventFoundationFake) CreateSourceRun(
	_ context.Context,
	request SourceSyncRequest,
) (SourceRun, bool, error) {
	f.requests = append(f.requests, request)
	return SourceRun{}, true, nil
}

func TestTriggerSourceEventSchedulesOnlyEnabledEventConfigs(t *testing.T) {
	foundation := &sourceEventFoundationFake{configs: []SourceConfig{
		{
			ID:        "66666666-6666-4666-8666-666666666666",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceKey: "omnichannel", Status: "enabled", Mode: "event",
		},
		{
			ID:        "77777777-7777-4777-8777-777777777777",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceKey: "omnichannel", Status: "enabled", Mode: "scheduled",
		},
		{
			ID:        "88888888-8888-4888-8888-888888888888",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceKey: "manual.offline", Status: "enabled", Mode: "event",
		},
	}}
	service := NewServiceWithRepositories(
		foundation,
		nil,
		nil,
		nil,
		nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(
			func(context.Context, string, string) error { return nil },
		)),
		WithRelationshipScopeAuthorizer(RelationshipScopeAuthorizerFunc(
			func(context.Context, string, string, string, string) error { return nil },
		)),
	)

	result, err := service.TriggerSourceEvent(context.Background(), SourceEventRequest{
		AccountID:       testAccount,
		ClientAccountID: testClient,
		SourceKey:       "omnichannel",
		RelationshipID:  testRelationship,
		EventID:         "message:99999999-9999-4999-8999-999999999999",
	})
	if err != nil {
		t.Fatalf("TriggerSourceEvent: %v", err)
	}
	if result.MatchedConfigs != 1 || result.CreatedRuns != 1 ||
		len(foundation.requests) != 1 {
		t.Fatalf("resultado inesperado: %#v / %#v", result, foundation.requests)
	}
	if foundation.requests[0].RelationshipID != testRelationship ||
		foundation.requests[0].Trigger != "event" {
		t.Fatalf("request inesperado: %#v", foundation.requests[0])
	}
}
