package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/bi"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/crm/erp"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerdata"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerintelligence"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/site"
)

const (
	sourceTestAccountID      = "11111111-1111-4111-8111-111111111111"
	sourceTestClientID       = "22222222-2222-4222-8222-222222222222"
	sourceTestSubjectID      = "33333333-3333-4333-8333-333333333333"
	sourceTestRelationshipID = "44444444-4444-4444-8444-444444444444"
	sourceTestEntityID       = "55555555-5555-4555-8555-555555555555"
)

type calendarBusinessContextStub struct {
	request calendar.CustomerIntelligenceBusinessContextRequest
	result  calendar.CustomerIntelligenceBusinessContext
	err     error
}

func (stub *calendarBusinessContextStub) ReadCustomerIntelligenceBusinessContext(
	_ context.Context,
	request calendar.CustomerIntelligenceBusinessContextRequest,
) (calendar.CustomerIntelligenceBusinessContext, error) {
	stub.request = request
	return stub.result, stub.err
}

type customerDataEvidenceStub struct {
	request customerdata.SourceEvidenceRequest
	bundle  customerdata.SourceEvidenceBundle
	err     error
}

func (stub *customerDataEvidenceStub) GetSourceEvidence(
	_ context.Context,
	request customerdata.SourceEvidenceRequest,
) (customerdata.SourceEvidenceBundle, error) {
	stub.request = request
	return stub.bundle, stub.err
}

type erpEvidenceStub struct {
	requests []erp.CustomerIntelligenceEvidenceRequest
	result   erp.CustomerIntelligenceEvidence
	err      error
}

func (stub *erpEvidenceStub) ReadCustomerIntelligenceEvidence(
	_ context.Context,
	request erp.CustomerIntelligenceEvidenceRequest,
) (erp.CustomerIntelligenceEvidence, error) {
	stub.requests = append(stub.requests, request)
	return stub.result, stub.err
}

type siteEvidenceStub struct {
	requests []site.CustomerIntelligenceEvidenceRequest
	result   site.CustomerIntelligenceEvidence
	err      error
}

func (stub *siteEvidenceStub) ReadCustomerIntelligenceEvidence(
	_ context.Context,
	request site.CustomerIntelligenceEvidenceRequest,
) (site.CustomerIntelligenceEvidence, error) {
	stub.requests = append(stub.requests, request)
	return stub.result, stub.err
}

type biHealthStub struct {
	request bi.CustomerIntelligenceQueryRequest
	result  bi.CustomerIntelligenceQueryAvailability
	err     error
}

func (stub *biHealthStub) CustomerIntelligenceQueryHealth(
	_ context.Context,
	request bi.CustomerIntelligenceQueryRequest,
) (bi.CustomerIntelligenceQueryAvailability, error) {
	stub.request = request
	return stub.result, stub.err
}

func TestCalendarSourceProducesBusinessContextWithoutSubject(t *testing.T) {
	updatedAt := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	reader := &calendarBusinessContextStub{result: calendar.CustomerIntelligenceBusinessContext{
		SchemaVersion:   "calendar.client_business_context.v1",
		ClientAccountID: sourceTestClientID,
		Sections: map[string]json.RawMessage{
			"voice": json.RawMessage(`{"brand_voice":"consultivo"}`),
		},
		UpdatedAt: &updatedAt,
		Found:     true,
	}}
	adapter := calendarClientProfileSourceAdapter{
		calendar: func() calendarBusinessContextReader { return reader },
	}
	observations, err := adapter.Fetch(
		context.Background(),
		customerintelligence.SourceConfig{
			AccountID:       sourceTestAccountID,
			ClientAccountID: sourceTestClientID,
			Mode:            "on_demand",
			PurposeKey:      "customer_profile",
			FieldAllowlist:  []string{"voice"},
			Config:          json.RawMessage(`{"sections":["voice"],"maxBytes":4096}`),
		},
		sourceTestRelationshipID,
	)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected one observation, got %d", len(observations))
	}
	observation := observations[0]
	if observation.ScopeType != customerintelligence.ObservationScopeBusiness ||
		observation.SubjectID != "" ||
		observation.RelationshipID != "" {
		t.Fatalf("business context leaked into subject scope: %+v", observation)
	}
	if reader.request.AccountID != sourceTestAccountID ||
		reader.request.ClientAccountID != sourceTestClientID {
		t.Fatalf("unexpected owner scope: %+v", reader.request)
	}
}

func TestERPSourceRequiresDeterministicCustomerDataLink(t *testing.T) {
	dataReader := &customerDataEvidenceStub{bundle: customerdata.SourceEvidenceBundle{
		SubjectID:      sourceTestSubjectID,
		RelationshipID: sourceTestRelationshipID,
		SourceLinks: []customerdata.SourceReference{
			{
				SourceModule:     "site",
				SourceKey:        "default",
				SourceEntityType: "lead",
				SourceEntityID:   sourceTestEntityID,
			},
			{
				SourceModule:     "erp",
				SourceKey:        "main",
				SourceEntityType: erp.CustomerIntelligenceEntityCustomer,
				SourceEntityID:   "erp-customer-42",
			},
		},
	}}
	occurredAt := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	erpReader := &erpEvidenceStub{result: erp.CustomerIntelligenceEvidence{
		EntityType: erp.CustomerIntelligenceEntityCustomer,
		EntityID:   "erp-customer-42",
		Version:    "file-1",
		OccurredAt: &occurredAt,
		Fields:     map[string]any{"preferred_name": "Cliente"},
	}}
	adapter := erpSourceAdapter{
		customerData: func() customerDataSourceEvidenceReader { return dataReader },
		erp:          func() erpCustomerEvidenceReader { return erpReader },
	}
	observations, err := adapter.Fetch(
		context.Background(),
		customerintelligence.SourceConfig{
			AccountID:       sourceTestAccountID,
			ClientAccountID: sourceTestClientID,
			PurposeKey:      "customer_profile",
			FieldAllowlist:  []string{"preferred_name"},
			Config: json.RawMessage(
				`{"connectionId":"main","entityTypes":["customer"]}`,
			),
		},
		sourceTestRelationshipID,
	)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(erpReader.requests) != 1 || len(observations) != 1 {
		t.Fatalf("expected one exact ERP read, requests=%d observations=%d",
			len(erpReader.requests), len(observations))
	}
	if erpReader.requests[0].EntityID != "erp-customer-42" ||
		erpReader.requests[0].ClientAccountID != sourceTestClientID {
		t.Fatalf("adapter did not preserve exact owner scope: %+v", erpReader.requests[0])
	}
	if observations[0].SubjectID != sourceTestSubjectID ||
		observations[0].RelationshipID != sourceTestRelationshipID {
		t.Fatalf("unexpected subject scope: %+v", observations[0])
	}
}

func TestERPSourceDoesNotReadWithoutERPSourceLink(t *testing.T) {
	dataReader := &customerDataEvidenceStub{bundle: customerdata.SourceEvidenceBundle{
		SubjectID:      sourceTestSubjectID,
		RelationshipID: sourceTestRelationshipID,
		SourceLinks: []customerdata.SourceReference{{
			SourceModule:     "site",
			SourceEntityType: "lead",
			SourceEntityID:   sourceTestEntityID,
		}},
	}}
	erpReader := &erpEvidenceStub{}
	adapter := erpSourceAdapter{
		customerData: func() customerDataSourceEvidenceReader { return dataReader },
		erp:          func() erpCustomerEvidenceReader { return erpReader },
	}
	observations, err := adapter.Fetch(
		context.Background(),
		customerintelligence.SourceConfig{
			AccountID:       sourceTestAccountID,
			ClientAccountID: sourceTestClientID,
			PurposeKey:      "customer_profile",
			FieldAllowlist:  []string{"preferred_name"},
			Config:          json.RawMessage(`{"entityTypes":["customer"]}`),
		},
		sourceTestRelationshipID,
	)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(observations) != 0 || len(erpReader.requests) != 0 {
		t.Fatal("ERP must not be queried without a deterministic ERP source link")
	}
}

func TestSiteSourceReadsOnlyExactLeadLink(t *testing.T) {
	dataReader := &customerDataEvidenceStub{bundle: customerdata.SourceEvidenceBundle{
		SubjectID:      sourceTestSubjectID,
		RelationshipID: sourceTestRelationshipID,
		SourceLinks: []customerdata.SourceReference{{
			SourceModule:     "site",
			SourceKey:        "landing-main",
			SourceEntityType: site.CustomerIntelligenceEntityLead,
			SourceEntityID:   sourceTestEntityID,
		}},
	}}
	occurredAt := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	siteReader := &siteEvidenceStub{result: site.CustomerIntelligenceEvidence{
		EntityType: site.CustomerIntelligenceEntityLead,
		EntityID:   sourceTestEntityID,
		Version:    "v1",
		OccurredAt: occurredAt,
		Fields:     map[string]any{"page": "/campanha"},
	}}
	adapter := siteSourceAdapter{
		customerData: func() customerDataSourceEvidenceReader { return dataReader },
		site:         func() siteCustomerEvidenceReader { return siteReader },
	}
	observations, err := adapter.Fetch(
		context.Background(),
		customerintelligence.SourceConfig{
			AccountID:       sourceTestAccountID,
			ClientAccountID: sourceTestClientID,
			PurposeKey:      "customer_profile",
			FieldAllowlist:  []string{"page"},
			Config: json.RawMessage(
				`{"siteId":"landing-main","entityTypes":["lead"]}`,
			),
		},
		sourceTestRelationshipID,
	)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(siteReader.requests) != 1 || len(observations) != 1 {
		t.Fatalf("expected one exact Site read, requests=%d observations=%d",
			len(siteReader.requests), len(observations))
	}
	if siteReader.requests[0].AccountID != sourceTestClientID ||
		siteReader.requests[0].EntityID != sourceTestEntityID {
		t.Fatalf("unexpected Site owner scope: %+v", siteReader.requests[0])
	}
}

func TestBIPerolaSourceIsPermanentUnavailableAfterClosedValidation(t *testing.T) {
	reader := &biHealthStub{result: bi.CustomerIntelligenceQueryAvailability{
		Status:     bi.CustomerIntelligenceAvailabilityUnavailable,
		ReasonCode: bi.CustomerIntelligenceUnavailableReason,
		DatasetID:  "inventario",
	}}
	adapter := biPerolaSourceAdapter{
		bi: func() biCustomerIntelligenceHealthReader { return reader },
	}
	_, err := adapter.Fetch(
		context.Background(),
		customerintelligence.SourceConfig{
			Mode:       "on_demand",
			PurposeKey: "portfolio_analysis",
			Config: json.RawMessage(
				`{"datasetId":"inventario","limit":1,"filters":[{"field":"itemSaldoId","operator":"eq","value":123}]}`,
			),
		},
		"",
	)
	if err == nil {
		t.Fatal("BI must remain explicitly unavailable until deterministic linking exists")
	}
	var classified interface {
		SourceFailureCode() string
		SourceRetryable() bool
	}
	if !errors.As(err, &classified) ||
		classified.SourceFailureCode() != bi.CustomerIntelligenceUnavailableReason ||
		classified.SourceRetryable() {
		t.Fatalf("unexpected BI failure classification: %v", err)
	}
	if reader.request.DatasetID != "inventario" ||
		len(reader.request.Query.Filters) != 1 ||
		reader.request.Query.PageNumber != 1 {
		t.Fatalf("closed BI query was not validated: %+v", reader.request)
	}
}

func TestSourceOptionsRejectUnknownConnectionSurface(t *testing.T) {
	t.Parallel()
	var options struct {
		DatasetID string `json:"datasetId"`
	}
	err := decodeSourceOptions(
		json.RawMessage(`{"datasetId":"inventario","sql":"select * from users"}`),
		&options,
	)
	if err == nil {
		t.Fatal("unknown connection surface was accepted")
	}
	var classified interface {
		SourceFailureCode() string
		SourceRetryable() bool
	}
	if !errors.As(err, &classified) ||
		classified.SourceFailureCode() != "source_config_invalid" ||
		classified.SourceRetryable() {
		t.Fatalf("unexpected failure classification: %v", err)
	}
}
