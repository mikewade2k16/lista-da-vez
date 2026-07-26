package customerintelligence

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProcessKeysAreClosedAndStable(t *testing.T) {
	t.Parallel()
	want := []string{
		"conversation.handoff_summary",
		"conversation.reply",
		"conversation.triage",
		"media.document_analysis",
		"media.image_analysis",
		"memory.extract",
		"portfolio.opportunity",
		"profile.summary",
		"quality.review",
		"recommendation.follow_up",
		"recommendation.important_dates",
		"recommendation.offer",
		"source.suggest",
	}
	if got := ProcessKeys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("process keys = %#v, want %#v", got, want)
	}
	if validProcessKey("conversation.anything") {
		t.Fatal("process key arbitraria foi aceita")
	}
}

func TestSourceConfigRejectsArbitraryConnectionSurface(t *testing.T) {
	t.Parallel()
	input := SourceConfigInput{
		ClientAccountID:  "11111111-1111-4111-8111-111111111111",
		SourceKey:        "erp",
		ConnectionKey:    "default",
		Status:           "enabled",
		Mode:             "scheduled",
		PurposeKey:       "customer_profile",
		FieldAllowlist:   []string{"order_date", "total_amount_cents"},
		FreshnessSeconds: 3600,
		Config:           json.RawMessage(`{"baseUrl":"http://postgres:5432","sql":"select * from users"}`),
	}
	if err := validateSourceConfig(input); err == nil {
		t.Fatal("config com URL/SQL arbitrarios foi aceita")
	}
	input.Config = json.RawMessage(`{"connectionId":"erp-main","entityTypes":["order"]}`)
	if err := validateSourceConfig(input); err != nil {
		t.Fatalf("config allowlisted rejeitada: %v", err)
	}
}

func TestSourceCatalogConfigKeysAreAllTyped(t *testing.T) {
	t.Parallel()
	for sourceKey, descriptor := range sourceCatalog {
		typed := make(map[string]bool, len(descriptor.ConfigSchema))
		for _, field := range descriptor.ConfigSchema {
			if typed[field.Key] {
				t.Fatalf("%s repete config key %q", sourceKey, field.Key)
			}
			typed[field.Key] = true
		}
		if len(typed) != len(descriptor.AllowedConfigKeys) {
			t.Fatalf(
				"%s publica %d config keys mas tipa %d",
				sourceKey,
				len(descriptor.AllowedConfigKeys),
				len(typed),
			)
		}
		for _, key := range descriptor.AllowedConfigKeys {
			if !typed[key] {
				t.Fatalf("%s publica config key sem tipo: %q", sourceKey, key)
			}
		}
	}
}

func TestSourceConfigValidatesEveryDescriptorFieldType(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		input  SourceConfigInput
		config string
	}{
		{
			name: "integer and boolean",
			input: validSourceConfigInput(
				"omnichannel",
				"event",
				"customer_service",
				[]string{"message_id"},
			),
			config: `{"lookbackDays":30,"includeMediaMetadata":false}`,
		},
		{
			name: "select",
			input: validSourceConfigInput(
				"manual.offline",
				"manual",
				"customer_profile",
				[]string{"note"},
			),
			config: `{"defaultSensitivity":"sensitive"}`,
		},
		{
			name: "safe key and string list",
			input: validSourceConfigInput(
				"erp",
				"scheduled",
				"marketing",
				[]string{"order_date"},
			),
			config: `{"connectionId":"erp.primary","entityTypes":["order","order_canceled"],"lookbackDays":365}`,
		},
		{
			name: "uuid",
			input: validSourceConfigInput(
				"site",
				"on_demand",
				"customer_profile",
				[]string{"page"},
			),
			config: `{"siteId":"22222222-2222-4222-8222-222222222222","entityTypes":["lead"]}`,
		},
		{
			name: "business context",
			input: validSourceConfigInput(
				"calendar.client_profile",
				"on_demand",
				"customer_service",
				[]string{"voice"},
			),
			config: `{"sections":["voice"],"maxBytes":4096}`,
		},
		{
			name: "aggregate without individual fields",
			input: validSourceConfigInput(
				"bi.perola",
				"on_demand",
				"portfolio_analysis",
				nil,
			),
			config: `{"datasetId":"inventory.daily","limit":100}`,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.input.Config = json.RawMessage(test.config)
			if err := validateSourceConfig(test.input); err != nil {
				t.Fatalf("config tipada rejeitada: %v", err)
			}
		})
	}
}

func TestSourceConfigRejectsValuesOutsideClosedDescriptor(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*SourceConfigInput)
	}{
		{
			name: "purpose outside source",
			mutate: func(input *SourceConfigInput) {
				input.PurposeKey = "portfolio_analysis"
			},
		},
		{
			name: "required config absent",
			mutate: func(input *SourceConfigInput) {
				input.Config = json.RawMessage(`{"entityTypes":["order"]}`)
			},
		},
		{
			name: "untyped config key",
			mutate: func(input *SourceConfigInput) {
				input.Config = json.RawMessage(
					`{"connectionId":"erp-main","filters":[{"field":"email"}]}`,
				)
			},
		},
		{
			name: "safe key is URL",
			mutate: func(input *SourceConfigInput) {
				input.Config = json.RawMessage(`{"connectionId":"https://erp.local"}`)
			},
		},
		{
			name: "integer is string",
			mutate: func(input *SourceConfigInput) {
				input.Config = json.RawMessage(
					`{"connectionId":"erp-main","lookbackDays":"30"}`,
				)
			},
		},
		{
			name: "integer outside bounds",
			mutate: func(input *SourceConfigInput) {
				input.Config = json.RawMessage(
					`{"connectionId":"erp-main","lookbackDays":3651}`,
				)
			},
		},
		{
			name: "list element outside registry",
			mutate: func(input *SourceConfigInput) {
				input.Config = json.RawMessage(
					`{"connectionId":"erp-main","entityTypes":["user"]}`,
				)
			},
		},
		{
			name: "duplicate list element",
			mutate: func(input *SourceConfigInput) {
				input.Config = json.RawMessage(
					`{"connectionId":"erp-main","entityTypes":["order","order"]}`,
				)
			},
		},
		{
			name: "field outside allowlist",
			mutate: func(input *SourceConfigInput) {
				input.FieldAllowlist = []string{"email"}
			},
		},
		{
			name: "duplicate field",
			mutate: func(input *SourceConfigInput) {
				input.FieldAllowlist = []string{"order_date", "order_date"}
			},
		},
		{
			name: "unsafe connection key",
			mutate: func(input *SourceConfigInput) {
				input.ConnectionKey = "postgres://database"
			},
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := validSourceConfigInput(
				"erp",
				"scheduled",
				"customer_profile",
				[]string{"order_date"},
			)
			input.Config = json.RawMessage(
				`{"connectionId":"erp-main","entityTypes":["order"],"lookbackDays":30}`,
			)
			test.mutate(&input)
			if err := validateSourceConfig(input); err == nil {
				t.Fatal("config fora do descriptor foi aceita")
			}
		})
	}
}

func TestSourceWithoutIndividualFieldCatalogRejectsArbitraryAllowlist(t *testing.T) {
	t.Parallel()
	input := validSourceConfigInput(
		"bi.perola",
		"on_demand",
		"customer_profile",
		[]string{"email"},
	)
	input.Config = json.RawMessage(`{"datasetId":"inventory"}`)
	if err := validateSourceConfig(input); err == nil {
		t.Fatal("fonte sem campos individuais aceitou allowlist arbitraria")
	}
}

func validSourceConfigInput(
	sourceKey string,
	mode string,
	purposeKey string,
	fields []string,
) SourceConfigInput {
	return SourceConfigInput{
		ClientAccountID:  "11111111-1111-4111-8111-111111111111",
		SourceKey:        sourceKey,
		ConnectionKey:    "default",
		Status:           "disabled",
		Mode:             mode,
		PurposeKey:       purposeKey,
		FieldAllowlist:   fields,
		FreshnessSeconds: 3600,
		Config:           json.RawMessage(`{}`),
	}
}

func TestConversationAndOfflineSourceFieldsShareCanonicalSnakeCase(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		sourceKey string
		mode      string
		fields    []string
		snapshot  json.RawMessage
	}{
		{
			sourceKey: "omnichannel",
			mode:      "event",
			fields:    []string{"message_id", "content", "occurred_at"},
			snapshot:  json.RawMessage(`{"message_id":"m1","content":"oi","occurred_at":"2026-07-23T10:00:00Z","not_allowed":"x"}`),
		},
		{
			sourceKey: "manual.offline",
			mode:      "manual",
			fields:    []string{"interaction_type", "title", "occurred_at"},
			snapshot:  json.RawMessage(`{"interaction_type":"meeting","title":"Retorno","occurred_at":"2026-07-23T10:00:00Z","not_allowed":"x"}`),
		},
	} {
		input := SourceConfigInput{
			ClientAccountID:  "11111111-1111-4111-8111-111111111111",
			SourceKey:        test.sourceKey,
			ConnectionKey:    "default",
			Status:           "enabled",
			Mode:             test.mode,
			PurposeKey:       "customer_profile",
			FieldAllowlist:   test.fields,
			FreshnessSeconds: 3600,
			Config:           json.RawMessage(`{}`),
		}
		if err := validateSourceConfig(input); err != nil {
			t.Fatalf("%s config rejeitada: %v", test.sourceKey, err)
		}
		filtered, err := filterObservation(
			SourceConfig{
				PurposeKey:     "customer_profile",
				FieldAllowlist: test.fields,
			},
			Observation{
				EntityType:     "event",
				EntityID:       "source-1",
				ScopeType:      ObservationScopeSubject,
				SubjectID:      "22222222-2222-4222-8222-222222222222",
				RelationshipID: "33333333-3333-4333-8333-333333333333",
				Snapshot:       test.snapshot,
				Sensitivity:    "personal",
				PurposeKey:     "customer_profile",
			},
		)
		if err != nil {
			t.Fatalf("%s snapshot rejeitado: %v", test.sourceKey, err)
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(filtered.Snapshot, &values); err != nil {
			t.Fatal(err)
		}
		if len(values) != len(test.fields) || values["not_allowed"] != nil {
			t.Fatalf("%s allowlist nao aplicada: %s", test.sourceKey, filtered.Snapshot)
		}
	}
}

func TestPortfolioAggregateRejectsIndividualFields(t *testing.T) {
	t.Parallel()
	if !containsIndividualFields(json.RawMessage(`{"segment":"dentists","rows":[{"email":"x@example.com"}]}`)) {
		t.Fatal("linhas individuais nao foram bloqueadas")
	}
	if containsIndividualFields(json.RawMessage(`{"segment":"dentists","counts":{"highIntent":42},"conversionRate":0.18}`)) {
		t.Fatal("agregado anonimo foi bloqueado")
	}
}

func TestPromptValidationRejectsUnknownVariable(t *testing.T) {
	t.Parallel()
	schema := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	got := validatePrompt(
		"Use {{context}} e ignore {{database.password}}.",
		[]string{"context", "input"},
		schema,
	)
	if got.Valid {
		t.Fatal("variavel fora do contrato foi aceita")
	}
	if !reflect.DeepEqual(got.ReasonCodes, []string{"variable_not_allowed:database.password"}) {
		t.Fatalf("reason codes = %#v", got.ReasonCodes)
	}
}

func TestCredentialDTOIsWriteOnly(t *testing.T) {
	t.Parallel()
	dto := credentialDTO(credentialRecord{
		ID: "credential-id", Provider: "openai", Label: "primary",
		Ciphertext: "v1:SUPER_SECRET_CIPHERTEXT", Last4: "1234", Status: "active",
	})
	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" || json.Valid(raw) == false {
		t.Fatalf("JSON invalido: %s", raw)
	}
	if contains(string(raw), "SUPER_SECRET") || contains(string(raw), "Ciphertext") {
		t.Fatalf("segredo/ciphertext vazou no DTO: %s", raw)
	}
	if !contains(string(raw), `"last4":"1234"`) {
		t.Fatalf("status mascarado ausente: %s", raw)
	}
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
