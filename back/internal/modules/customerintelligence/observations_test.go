package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

const (
	observationTestAccount      = "11111111-1111-4111-8111-111111111111"
	observationTestClient       = "22222222-2222-4222-8222-222222222222"
	observationTestRelationship = "33333333-3333-4333-8333-333333333333"
	observationTestSubject      = "44444444-4444-4444-8444-444444444444"
	observationTestID           = "55555555-5555-4555-8555-555555555555"
)

type observationFoundationFake struct {
	FoundationRepository
	records              []StoredObservation
	listCalls            int
	getCalls             int
	capabilityMode       string
	requestedPurposeKeys []string
	accessCalls          []observationAccessCall
	accessError          error
}

type observationAccessCall struct {
	scope       Scope
	actorUserID string
	record      StoredObservation
	reasonCode  string
	revealed    bool
	fieldCount  int
}

func (f *observationFoundationFake) GetCapability(
	context.Context,
	Scope,
	string,
	string,
) (Capability, error) {
	mode := f.capabilityMode
	if mode == "" {
		mode = "on"
	}
	return Capability{Mode: mode, Config: json.RawMessage(`{}`)}, nil
}

func (f *observationFoundationFake) ListRelationshipObservations(
	_ context.Context,
	scope Scope,
	relationshipID string,
	sourceKeys []string,
	purposeKeys []string,
	_ int,
) ([]StoredObservation, error) {
	f.listCalls++
	f.requestedPurposeKeys = append([]string(nil), purposeKeys...)
	if scope.AccountID != observationTestAccount ||
		scope.ClientAccountID != observationTestClient ||
		relationshipID != observationTestRelationship {
		return nil, ErrNotFound
	}
	allowed := make(map[string]bool, len(sourceKeys))
	for _, key := range sourceKeys {
		allowed[key] = true
	}
	allowedPurposes := make(map[string]bool, len(purposeKeys))
	for _, key := range purposeKeys {
		allowedPurposes[key] = true
	}
	items := make([]StoredObservation, 0, len(f.records))
	for _, item := range f.records {
		if (len(allowed) == 0 || allowed[item.SourceKey]) &&
			allowedPurposes[item.PurposeKey] &&
			item.Sensitivity != "restricted" {
			items = append(items, item)
		}
	}
	return items, nil
}

func (f *observationFoundationFake) GetObservation(
	_ context.Context,
	scope Scope,
	observationID string,
) (StoredObservation, error) {
	f.getCalls++
	if scope.AccountID != observationTestAccount ||
		scope.ClientAccountID != observationTestClient {
		return StoredObservation{}, ErrNotFound
	}
	for _, item := range f.records {
		if item.ID == observationID {
			return item, nil
		}
	}
	return StoredObservation{}, ErrNotFound
}

func (f *observationFoundationFake) RecordObservationAccess(
	_ context.Context,
	scope Scope,
	actorUserID string,
	record StoredObservation,
	reasonCode string,
	revealed bool,
	fieldCount int,
) error {
	f.accessCalls = append(f.accessCalls, observationAccessCall{
		scope:       scope,
		actorUserID: actorUserID,
		record:      record,
		reasonCode:  reasonCode,
		revealed:    revealed,
		fieldCount:  fieldCount,
	})
	return f.accessError
}

func newObservationService(
	t *testing.T,
	foundation *observationFoundationFake,
	relationshipAuthorizer RelationshipScopeAuthorizer,
) *Service {
	t.Helper()
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if relationshipAuthorizer == nil {
		relationshipAuthorizer = RelationshipScopeAuthorizerFunc(
			func(_ context.Context, accountID, clientAccountID, subjectID, relationshipID string) error {
				if accountID != observationTestAccount ||
					clientAccountID != observationTestClient ||
					relationshipID != observationTestRelationship {
					return ErrForbidden
				}
				if subjectID != "" && subjectID != observationTestSubject {
					return ErrForbidden
				}
				return nil
			},
		)
	}
	return NewServiceWithRepositories(
		foundation,
		nil,
		nil,
		box,
		nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(
			func(_ context.Context, accountID, clientAccountID string) error {
				if accountID != observationTestAccount ||
					clientAccountID != observationTestClient {
					return ErrForbidden
				}
				return nil
			},
		)),
		WithRelationshipScopeAuthorizer(relationshipAuthorizer),
	)
}

func TestObservationViewDecryptsAndReappliesCurrentAllowlist(t *testing.T) {
	t.Parallel()
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt(`{"total_amount_cents":300,"preferred_name":"Ana","removed_field":"never expose"}`)
	if err != nil {
		t.Fatal(err)
	}
	foundation := &observationFoundationFake{records: []StoredObservation{{
		ID:                 observationTestID,
		SubjectID:          observationTestSubject,
		RelationshipID:     observationTestRelationship,
		SourceKey:          "erp",
		SourceEntityType:   "customer",
		SourceEntityID:     "erp-42",
		SnapshotCiphertext: ciphertext,
		FieldAllowlist:     []string{"total_amount_cents", "preferred_name"},
		Sensitivity:        "internal",
		PurposeKey:         "customer_profile",
		ObservedAt:         time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC),
	}}}
	service := newObservationService(t, foundation, nil)

	item, err := service.Observation(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(item.ProvenanceRef, observationProvenancePrefix) ||
		strings.Contains(item.ProvenanceRef, "erp-42") ||
		item.RetentionState != "active" ||
		len(item.SnapshotFields) != 2 {
		t.Fatalf("projection inesperada: %#v", item)
	}
	for _, field := range item.SnapshotFields {
		if field.Label == "removed_field" || field.Masked {
			t.Fatalf("allowlist/masking incorretos: %#v", item.SnapshotFields)
		}
	}
	evidence, err := service.protectEvidenceProvenance(
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		[]EvidenceRef{{
			ObservationID: observationTestID,
			SourceKey:     "erp",
			Locator:       "customer:erp-42",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || evidence[0].Locator != item.ProvenanceRef {
		t.Fatalf("painel/contexto divergiram na proveniencia: view=%q evidence=%#v",
			item.ProvenanceRef, evidence)
	}
}

func TestProtectedObservationSensitivitiesNeverReturnDisplayValue(t *testing.T) {
	t.Parallel()
	for _, sensitivity := range []string{"personal", "sensitive", "restricted", "unknown"} {
		sensitivity := sensitivity
		t.Run(sensitivity, func(t *testing.T) {
			t.Parallel()
			foundation := &observationFoundationFake{records: []StoredObservation{{
				ID:                 observationTestID,
				SubjectID:          observationTestSubject,
				RelationshipID:     observationTestRelationship,
				SourceKey:          "erp",
				SourceEntityType:   "customer",
				SourceEntityID:     "erp-secret-42",
				SnapshotCiphertext: "ciphertext-nao-deve-ser-decifrado",
				FieldAllowlist:     []string{"preferred_name"},
				Sensitivity:        sensitivity,
				PurposeKey:         "customer_profile",
				ObservedAt:         time.Now().UTC(),
			}}}
			service := newObservationService(t, foundation, nil)

			item, err := service.Observation(
				context.Background(),
				Scope{
					AccountID:       observationTestAccount,
					ClientAccountID: observationTestClient,
				},
				observationTestID,
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(item.SnapshotFields) != 1 ||
				!item.SnapshotFields[0].Masked ||
				item.SnapshotFields[0].DisplayValue != "[conteudo protegido]" {
				t.Fatalf("sensibilidade %s nao foi mascarada: %#v", sensitivity, item)
			}
			if strings.Contains(item.ProvenanceRef, "erp-secret-42") {
				t.Fatalf("proveniencia vazou source_entity_id: %q", item.ProvenanceRef)
			}
			items, err := service.Observations(
				context.Background(),
				Scope{
					AccountID:       observationTestAccount,
					ClientAccountID: observationTestClient,
				},
				observationTestRelationship,
				nil,
				10,
			)
			if err != nil {
				t.Fatal(err)
			}
			if sensitivity == "restricted" {
				if len(items) != 0 {
					t.Fatalf("lista exibiu restricted: %#v", items)
				}
			} else if len(items) != 1 ||
				len(items[0].SnapshotFields) != 1 ||
				!items[0].SnapshotFields[0].Masked {
				t.Fatalf("lista nao mascarou %s: %#v", sensitivity, items)
			}
		})
	}
}

func TestRevealObservationReturnsOnlyAllowlistedFieldsAndAuditsAccess(t *testing.T) {
	t.Parallel()
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt(
		`{"preferred_name":"Ana","phone":"11999999999","raw_provider_payload":"never expose"}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	foundation := &observationFoundationFake{records: []StoredObservation{{
		ID:                 observationTestID,
		SubjectID:          observationTestSubject,
		RelationshipID:     observationTestRelationship,
		SourceKey:          "whatsapp",
		SourceEntityType:   "contact",
		SourceEntityID:     "provider-secret-id",
		SnapshotCiphertext: ciphertext,
		FieldAllowlist:     []string{"preferred_name", "phone"},
		Sensitivity:        "personal",
		PurposeKey:         "customer_profile",
		ObservedAt:         time.Now().UTC(),
	}}}
	service := newObservationService(t, foundation, nil)

	masked, err := service.Observation(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if masked.Revealed || len(masked.SnapshotFields) != 2 {
		t.Fatalf("detalhe mascarado inesperado: %#v", masked)
	}
	for _, field := range masked.SnapshotFields {
		if !field.Masked || strings.Contains(field.DisplayValue, "Ana") ||
			strings.Contains(field.DisplayValue, "11999999999") {
			t.Fatalf("detalhe comum vazou valor: %#v", masked.SnapshotFields)
		}
	}

	revealed, err := service.RevealObservation(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestSubject,
		observationTestID,
		ObservationRevealInput{ReasonCode: "customer_support_investigation"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !revealed.Revealed || len(revealed.SnapshotFields) != 2 {
		t.Fatalf("reveal inesperado: %#v", revealed)
	}
	values := make(map[string]string, len(revealed.SnapshotFields))
	for _, field := range revealed.SnapshotFields {
		if field.Masked {
			t.Fatalf("campo continuou mascarado apos reveal auditado: %#v", field)
		}
		values[field.Label] = field.DisplayValue
	}
	if values["preferred_name"] != "Ana" ||
		values["phone"] != "11999999999" ||
		values["raw_provider_payload"] != "" {
		t.Fatalf("allowlist do reveal foi violada: %#v", values)
	}
	if len(foundation.accessCalls) != 1 {
		t.Fatalf("acessos auditados = %d, esperado 1", len(foundation.accessCalls))
	}
	call := foundation.accessCalls[0]
	if call.scope.AccountID != observationTestAccount ||
		call.scope.ClientAccountID != observationTestClient ||
		call.actorUserID != observationTestSubject ||
		call.record.ID != observationTestID ||
		call.reasonCode != "customer_support_investigation" ||
		!call.revealed ||
		call.fieldCount != 2 {
		t.Fatalf("auditoria do reveal inesperada: %#v", call)
	}
}

func TestRevealObservationRejectsInvalidReasonBeforeReadOrAudit(t *testing.T) {
	t.Parallel()
	foundation := &observationFoundationFake{}
	service := newObservationService(t, foundation, nil)

	_, err := service.RevealObservation(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestSubject,
		observationTestID,
		ObservationRevealInput{ReasonCode: "texto livre com dados"},
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("erro = %v, esperado invalid input", err)
	}
	if foundation.getCalls != 0 || len(foundation.accessCalls) != 0 {
		t.Fatalf(
			"reason invalido chegou ao repository: get=%d audit=%d",
			foundation.getCalls,
			len(foundation.accessCalls),
		)
	}
}

func TestRevealObservationFailsClosedWhenAuditWriteFails(t *testing.T) {
	t.Parallel()
	foundation := &observationFoundationFake{
		records: []StoredObservation{{
			ID:               observationTestID,
			SubjectID:        observationTestSubject,
			RelationshipID:   observationTestRelationship,
			SourceKey:        "erp",
			SourceEntityType: "customer",
			SourceEntityID:   "erp-42",
			Snapshot:         json.RawMessage(`{"preferred_name":"Ana"}`),
			FieldAllowlist:   []string{"preferred_name"},
			Sensitivity:      "personal",
			PurposeKey:       "customer_profile",
			ObservedAt:       time.Now().UTC(),
		}},
		accessError: errors.New("audit unavailable"),
	}
	service := newObservationService(t, foundation, nil)

	item, err := service.RevealObservation(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestSubject,
		observationTestID,
		ObservationRevealInput{ReasonCode: "customer_support_investigation"},
	)
	if err == nil {
		t.Fatal("reveal deveria falhar quando a auditoria nao pode ser persistida")
	}
	if item.Revealed || len(item.SnapshotFields) != 0 {
		t.Fatalf("servico retornou dados apesar da falha de auditoria: %#v", item)
	}
	if len(foundation.accessCalls) != 1 {
		t.Fatalf("tentativas de auditoria = %d, esperado 1", len(foundation.accessCalls))
	}
}

func TestRevealObservationRejectsCrossClientBeforeRepositoryRead(t *testing.T) {
	t.Parallel()
	foundation := &observationFoundationFake{}
	service := newObservationService(t, foundation, nil)

	_, err := service.RevealObservation(
		context.Background(),
		Scope{
			AccountID:       observationTestAccount,
			ClientAccountID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		},
		observationTestSubject,
		observationTestID,
		ObservationRevealInput{ReasonCode: "customer_support_investigation"},
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("erro = %v, esperado forbidden", err)
	}
	if foundation.getCalls != 0 || len(foundation.accessCalls) != 0 {
		t.Fatalf(
			"cross-client chegou ao repository: get=%d audit=%d",
			foundation.getCalls,
			len(foundation.accessCalls),
		)
	}
}

func TestRevealObservationRequiresRelationshipAuthorization(t *testing.T) {
	t.Parallel()
	foundation := &observationFoundationFake{records: []StoredObservation{{
		ID:             observationTestID,
		SubjectID:      observationTestSubject,
		RelationshipID: observationTestRelationship,
	}}}
	service := newObservationService(
		t,
		foundation,
		RelationshipScopeAuthorizerFunc(
			func(context.Context, string, string, string, string) error {
				return ErrForbidden
			},
		),
	)

	_, err := service.RevealObservation(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestSubject,
		observationTestID,
		ObservationRevealInput{ReasonCode: "customer_support_investigation"},
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("erro = %v, esperado forbidden", err)
	}
	if foundation.getCalls != 1 || len(foundation.accessCalls) != 0 {
		t.Fatalf(
			"autorizacao de relacionamento inesperada: get=%d audit=%d",
			foundation.getCalls,
			len(foundation.accessCalls),
		)
	}
}

func TestRestrictedObservationNeverRevealsFieldValues(t *testing.T) {
	t.Parallel()
	foundation := &observationFoundationFake{records: []StoredObservation{{
		ID:               observationTestID,
		SubjectID:        observationTestSubject,
		RelationshipID:   observationTestRelationship,
		SourceKey:        "manual.offline",
		SourceEntityType: "note",
		SourceEntityID:   "note-1",
		Snapshot:         json.RawMessage(`{"note":"diagnostico privado"}`),
		FieldAllowlist:   []string{"note"},
		Sensitivity:      "restricted",
		PurposeKey:       "customer_profile",
		ObservedAt:       time.Now().UTC(),
	}}}
	service := newObservationService(t, foundation, nil)

	item, err := service.Observation(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.SnapshotFields) != 1 ||
		!item.SnapshotFields[0].Masked ||
		item.SnapshotFields[0].DisplayValue == "diagnostico privado" {
		t.Fatalf("observacao restrita vazou: %#v", item.SnapshotFields)
	}
	items, err := service.Observations(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestRelationship,
		nil,
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("profile.view recebeu metadado restricted: %#v", items)
	}
}

func TestObservationRejectsCrossClientBeforeRepositoryRead(t *testing.T) {
	t.Parallel()
	foundation := &observationFoundationFake{}
	service := newObservationService(t, foundation, nil)

	_, err := service.Observation(
		context.Background(),
		Scope{
			AccountID:       observationTestAccount,
			ClientAccountID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		},
		observationTestID,
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("erro = %v, esperado forbidden", err)
	}
	if foundation.getCalls != 0 {
		t.Fatalf("repository consultado antes do client scope: %d", foundation.getCalls)
	}
}

func TestObservationRequiresRelationshipAuthorizationAfterScopedRead(t *testing.T) {
	t.Parallel()
	foundation := &observationFoundationFake{records: []StoredObservation{{
		ID:             observationTestID,
		SubjectID:      observationTestSubject,
		RelationshipID: observationTestRelationship,
	}}}
	service := newObservationService(
		t,
		foundation,
		RelationshipScopeAuthorizerFunc(
			func(context.Context, string, string, string, string) error {
				return ErrForbidden
			},
		),
	)

	_, err := service.Observation(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestID,
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("erro = %v, esperado forbidden", err)
	}
	if foundation.getCalls != 1 {
		t.Fatalf("repository scoped read calls = %d", foundation.getCalls)
	}
}

func TestObservationsFiltersSourceKeys(t *testing.T) {
	t.Parallel()
	foundation := &observationFoundationFake{records: []StoredObservation{
		{
			ID:               observationTestID,
			RelationshipID:   observationTestRelationship,
			SourceKey:        "erp",
			SourceEntityType: "customer",
			SourceEntityID:   "1",
			Snapshot:         json.RawMessage(`{"total_amount_cents":200}`),
			FieldAllowlist:   []string{"total_amount_cents"},
			Sensitivity:      "internal",
			PurposeKey:       "customer_profile",
		},
		{
			ID:               "66666666-6666-4666-8666-666666666666",
			RelationshipID:   observationTestRelationship,
			SourceKey:        "calendar",
			SourceEntityType: "event",
			SourceEntityID:   "2",
			Snapshot:         json.RawMessage(`{"title":"Retorno"}`),
			FieldAllowlist:   []string{"title"},
			Sensitivity:      "internal",
			PurposeKey:       "customer_profile",
		},
	}}
	service := newObservationService(t, foundation, nil)

	items, err := service.Observations(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestRelationship,
		[]string{"erp"},
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SourceKey != "erp" {
		t.Fatalf("source filter inesperado: %#v", items)
	}
	if len(foundation.requestedPurposeKeys) != 2 ||
		foundation.requestedPurposeKeys[0] != "customer_profile" ||
		foundation.requestedPurposeKeys[1] != "customer_relationship" {
		t.Fatalf("finalidades profile_view nao chegaram ao repository: %#v",
			foundation.requestedPurposeKeys)
	}
}

func TestObservationsRequiresEnabledProfileCapabilityBeforeRepository(t *testing.T) {
	t.Parallel()
	foundation := &observationFoundationFake{capabilityMode: "off"}
	service := newObservationService(t, foundation, nil)

	items, err := service.Observations(
		context.Background(),
		Scope{AccountID: observationTestAccount, ClientAccountID: observationTestClient},
		observationTestRelationship,
		nil,
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("capability off retornou observacoes: %#v", items)
	}
	if foundation.listCalls != 0 {
		t.Fatalf("repository consultado com capability off: %d", foundation.listCalls)
	}
}
