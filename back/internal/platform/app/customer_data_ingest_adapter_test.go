package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerdata"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerintelligence"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
)

type customerDataSourceReaderStub struct {
	source omnichannelCustomerDataSource
	err    error
	calls  int
}

func (stub *customerDataSourceReaderStub) ReadInboundCustomerDataSource(
	_ context.Context,
	_ omnichannel.CustomerDataInboundEvent,
) (omnichannelCustomerDataSource, error) {
	stub.calls++
	return stub.source, stub.err
}

type customerDataRelationshipResolverStub struct {
	requests []customerdata.ResolveRelationshipRequest
	result   customerdata.ResolveRelationshipResult
	err      error
}

type customerIntelligenceSourceEventTriggerStub struct {
	requests []customerintelligence.SourceEventRequest
	err      error
}

func (stub *customerIntelligenceSourceEventTriggerStub) TriggerSourceEvent(
	_ context.Context,
	request customerintelligence.SourceEventRequest,
) (customerintelligence.SourceEventResult, error) {
	stub.requests = append(stub.requests, request)
	return customerintelligence.SourceEventResult{MatchedConfigs: 1, CreatedRuns: 1}, stub.err
}

func (stub *customerDataRelationshipResolverStub) ResolveRelationship(
	_ context.Context,
	request customerdata.ResolveRelationshipRequest,
) (customerdata.ResolveRelationshipResult, error) {
	stub.requests = append(stub.requests, request)
	return stub.result, stub.err
}

func customerDataAdapterEvent() omnichannel.CustomerDataInboundEvent {
	return omnichannel.CustomerDataInboundEvent{
		SchemaVersion:          "omnichannel.customer_data.inbound.v1",
		EventID:                "3f2a06a0-c43d-4666-b9d0-e47fcebd5a18",
		AccountID:              "a35c3e9b-745f-4e3b-8e75-da276586996e",
		ClientAccountID:        "7f31befd-8483-466c-a5fe-2aef3c6aa388",
		ContactID:              "840875b2-af4b-4c15-abf5-627ce9a7971d",
		ConversationID:         "302f5979-9cd5-4472-992c-0a74b30a5e4b",
		MessageID:              "54446149-a667-4177-b995-6fbe8de834dc",
		ChannelClientBindingID: "d7b0cd72-8012-48a7-bf21-746ba52fb9d9",
		Channel:                "WHATSAPP",
		Provider:               "evolution",
		OccurredAt:             time.Date(2026, 7, 23, 19, 0, 0, 0, time.UTC),
	}
}

func TestOmnichannelCustomerDataIngestAdapterIsIdempotentByClientAndContact(t *testing.T) {
	t.Parallel()
	source := &customerDataSourceReaderStub{source: omnichannelCustomerDataSource{
		DisplayName: "Maria",
		Phone:       "5511999999999",
		ExternalID:  "5511999999999@s.whatsapp.net",
	}}
	resolver := &customerDataRelationshipResolverStub{result: customerdata.ResolveRelationshipResult{
		Status: "created", SubjectID: "subject", RelationshipID: "relationship",
	}}
	trigger := &customerIntelligenceSourceEventTriggerStub{}
	adapter := omnichannelCustomerDataIngestAdapter{
		source: source,
		customerData: func() customerDataRelationshipResolver {
			return resolver
		},
		customerIntelligence: func() customerIntelligenceSourceEventTrigger {
			return trigger
		},
	}
	event := customerDataAdapterEvent()
	if err := adapter.ResolveInboundRelationship(context.Background(), event); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	resolver.result = customerdata.ResolveRelationshipResult{
		Status: "resolved", SubjectID: "subject", RelationshipID: "relationship", Replayed: true,
	}
	if err := adapter.ResolveInboundRelationship(context.Background(), event); err != nil {
		t.Fatalf("replay resolve: %v", err)
	}
	if len(resolver.requests) != 2 {
		t.Fatalf("resolve calls = %d", len(resolver.requests))
	}
	if len(trigger.requests) != 2 ||
		trigger.requests[0].EventID != event.EventID ||
		trigger.requests[0].RelationshipID != "relationship" ||
		trigger.requests[0].SourceKey != "omnichannel" {
		t.Fatalf("unexpected source events: %#v", trigger.requests)
	}
	first, replay := resolver.requests[0], resolver.requests[1]
	if first.RequestID != replay.RequestID ||
		first.RequestID != "omnichannel-contact:"+event.ClientAccountID+":"+event.ContactID {
		t.Fatalf("unstable request id: %q / %q", first.RequestID, replay.RequestID)
	}
	if first.AccountID != event.AccountID || first.ClientAccountID != event.ClientAccountID ||
		first.Source.SourceEntityID != event.ContactID ||
		first.Source.SourceModule != "omnichannel" ||
		first.Source.SourceEntityType != "contact" {
		t.Fatalf("unexpected scoped request: %#v", first)
	}
	if len(first.Identities) != 1 ||
		first.Identities[0].Kind != "whatsapp" ||
		first.Identities[0].Value != "5511999999999" ||
		first.Identities[0].VerificationStatus != "verified" {
		t.Fatalf("unexpected identities: %#v", first.Identities)
	}
}

func TestOmnichannelCustomerDataIngestAdapterRetriesSourceTriggerFailure(t *testing.T) {
	t.Parallel()
	source := &customerDataSourceReaderStub{source: omnichannelCustomerDataSource{
		DisplayName: "Maria", Phone: "5511999999999",
	}}
	resolver := &customerDataRelationshipResolverStub{result: customerdata.ResolveRelationshipResult{
		Status: "resolved", SubjectID: "subject", RelationshipID: "relationship",
	}}
	triggerErr := errors.New("source trigger unavailable")
	trigger := &customerIntelligenceSourceEventTriggerStub{err: triggerErr}
	adapter := omnichannelCustomerDataIngestAdapter{
		source: source,
		customerData: func() customerDataRelationshipResolver {
			return resolver
		},
		customerIntelligence: func() customerIntelligenceSourceEventTrigger {
			return trigger
		},
	}
	err := adapter.ResolveInboundRelationship(context.Background(), customerDataAdapterEvent())
	if !errors.Is(err, triggerErr) {
		t.Fatalf("expected trigger failure for worker retry, got %v", err)
	}
}

func TestOmnichannelCustomerDataIngestAdapterFailsClosedBeforeResolver(t *testing.T) {
	t.Parallel()
	source := &customerDataSourceReaderStub{err: customerdata.ErrNotFound}
	resolver := &customerDataRelationshipResolverStub{}
	adapter := omnichannelCustomerDataIngestAdapter{
		source: source,
		customerData: func() customerDataRelationshipResolver {
			return resolver
		},
	}
	err := adapter.ResolveInboundRelationship(context.Background(), customerDataAdapterEvent())
	if !errors.Is(err, customerdata.ErrNotFound) {
		t.Fatalf("expected concealed not found, got %v", err)
	}
	if len(resolver.requests) != 0 {
		t.Fatal("out-of-scope source reached Customer Data")
	}
}

func TestCustomerDataIdentityInputsUseProviderVerifiedIdentity(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 23, 19, 30, 0, 0, time.UTC)
	whatsapp := customerDataIdentityInputs(
		"WHATSAPP", "contact", "",
		"5511988887777@s.whatsapp.net", at,
	)
	if len(whatsapp) != 1 || whatsapp[0].Kind != "whatsapp" ||
		whatsapp[0].Value != "5511988887777" ||
		whatsapp[0].OccurredAt == nil || !whatsapp[0].OccurredAt.Equal(at) {
		t.Fatalf("unexpected WhatsApp identity: %#v", whatsapp)
	}
	instagram := customerDataIdentityInputs(
		"INSTAGRAM", "contact", "", "17841400000000000", at,
	)
	if len(instagram) != 1 || instagram[0].Kind != "instagram" ||
		instagram[0].Value != "17841400000000000" {
		t.Fatalf("unexpected Instagram identity: %#v", instagram)
	}
}

var (
	_ omnichannelCustomerDataSourceReader    = (*customerDataSourceReaderStub)(nil)
	_ customerDataRelationshipResolver       = (*customerDataRelationshipResolverStub)(nil)
	_ customerIntelligenceSourceEventTrigger = (*customerIntelligenceSourceEventTriggerStub)(nil)
)
