package customerintelligence

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

func TestCustomerIntelligenceRepositoryLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL nao definido")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.ApplyMigrationsWithOptions(ctx, pool, database.MigrationOptions{
		SkipDataSeeds: true,
	}); err != nil {
		t.Fatal(err)
	}

	var accountID, clientAccountID, actorID string
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	err = pool.QueryRow(ctx, `
		insert into core.accounts (slug, name)
		values ($1, 'CI integration owner')
		returning id`,
		"ci-owner-"+suffix,
	).Scan(&accountID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `delete from core.accounts where id = $1`, accountID)
	}()
	err = pool.QueryRow(ctx, `
		insert into core.accounts (slug, name)
		values ($1, 'CI integration client')
		returning id`,
		"ci-client-"+suffix,
	).Scan(&clientAccountID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `delete from core.accounts where id = $1`, clientAccountID)
	}()
	err = pool.QueryRow(ctx, `
		insert into core.users (email, display_name)
		values ($1, 'CI integration actor')
		returning id`,
		"ci-"+suffix+"@example.invalid",
	).Scan(&actorID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `delete from core.users where id = $1`, actorID)
	}()

	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	repository := NewPostgresRepository(pool)
	service := NewService(
		repository, box, llmFake{},
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
		WithRelationshipScopeAuthorizer(RelationshipScopeAuthorizerFunc(allowEveryRelationship)),
	)
	scope := Scope{AccountID: accountID, ClientAccountID: clientAccountID}

	profileCapability, err := service.SetCapability(ctx, accountID, actorID, CapabilityInput{
		ClientAccountID: clientAccountID, Key: CapabilityProfile,
		Mode: "on", Config: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if profileCapability.Revision != 1 {
		t.Fatalf("profile revision = %d", profileCapability.Revision)
	}
	profileCapability, err = service.SetCapability(ctx, accountID, actorID, CapabilityInput{
		ClientAccountID: clientAccountID, Key: CapabilityProfile,
		Mode: "on", Config: json.RawMessage(`{}`),
		ExpectedRevision: profileCapability.Revision,
	})
	if err != nil || profileCapability.Revision != 2 {
		t.Fatalf("capability CAS: item=%#v err=%v", profileCapability, err)
	}
	retentionDraft, err := service.CreateRetentionPolicyDraft(
		ctx,
		accountID,
		actorID,
		defaultRetentionPolicyKey,
		RetentionPolicyDraftInput{
			SnapshotTTLSeconds: defaultSnapshotTTLSeconds,
			OnExpiry:           retentionActionTombstone,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PublishRetentionPolicy(
		ctx,
		accountID,
		actorID,
		retentionDraft.ID,
		PublishRetentionPolicyInput{
			ExpectedRevision:  retentionDraft.Revision,
			ReasonCode:        "legal_review_approved",
			ApprovalReference: "CI-INTEGRATION-RETENTION",
		},
	); err != nil {
		t.Fatal(err)
	}

	var subjectID, relationshipID string
	if err := pool.QueryRow(ctx, `
		insert into customer_data.subjects (
		    account_id, subject_type, idempotency_key, created_by_user_id
		)
		values ($1, 'person', $2, $3)
		returning id`,
		accountID,
		"ci-subject-"+suffix,
		actorID,
	).Scan(&subjectID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		insert into customer_data.relationships (
		    account_id, client_account_id, subject_id, display_name,
		    created_by_user_id, updated_by_user_id
		)
		values ($1, $2, $3, 'CI integration person', $4, $4)
		returning id`,
		accountID,
		clientAccountID,
		subjectID,
		actorID,
	).Scan(&relationshipID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(
			context.Background(),
			`delete from customer_data.relationships
			  where account_id = $1 and id = $2`,
			accountID,
			relationshipID,
		)
		_, _ = pool.Exec(
			context.Background(),
			`delete from customer_data.subjects
			  where account_id = $1 and id = $2`,
			accountID,
			subjectID,
		)
	}()
	source, err := service.ConfigureSource(ctx, accountID, actorID, SourceConfigInput{
		ClientAccountID: clientAccountID, SourceKey: "erp", ConnectionKey: "primary",
		Status: "enabled", Mode: "scheduled", PurposeKey: "customer_profile",
		FieldAllowlist: []string{"total_amount_cents"}, FreshnessSeconds: 3600,
		Config: json.RawMessage(`{"connectionId":"erp-primary","entityTypes":["order"]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceRun, _, err := repository.CreateSourceRun(ctx, SourceSyncRequest{
		AccountID: accountID, ClientAccountID: clientAccountID,
		SourceConfigID: source.ID, IdempotencyKey: "integration-source-run-1",
		Trigger: "manual", RelationshipID: relationshipID,
	})
	if err != nil {
		t.Fatal(err)
	}
	repeatedSourceRun, created, err := repository.CreateSourceRun(ctx, SourceSyncRequest{
		AccountID: accountID, ClientAccountID: clientAccountID,
		SourceConfigID: source.ID, IdempotencyKey: "integration-source-run-1",
		Trigger: "manual", RelationshipID: relationshipID,
	})
	if err != nil || created || repeatedSourceRun.ID != sourceRun.ID {
		t.Fatalf(
			"source run replay: first=%s repeated=%s created=%v err=%v",
			sourceRun.ID,
			repeatedSourceRun.ID,
			created,
			err,
		)
	}
	secondClientSource, err := service.ConfigureSource(ctx, accountID, actorID, SourceConfigInput{
		ClientAccountID: accountID, SourceKey: "erp", ConnectionKey: "secondary",
		Status: "enabled", Mode: "scheduled", PurposeKey: "customer_profile",
		FieldAllowlist: []string{"total_amount_cents"}, FreshnessSeconds: 3600,
		Config: json.RawMessage(`{"connectionId":"erp-secondary","entityTypes":["order"]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	secondClientRun, created, err := repository.CreateSourceRun(ctx, SourceSyncRequest{
		AccountID: accountID, ClientAccountID: accountID,
		SourceConfigID: secondClientSource.ID, IdempotencyKey: "integration-source-run-1",
		Trigger: "manual",
	})
	if err != nil || !created || secondClientRun.ID == sourceRun.ID {
		t.Fatalf(
			"same key in second client was not isolated: first=%s second=%s created=%v err=%v",
			sourceRun.ID,
			secondClientRun.ID,
			created,
			err,
		)
	}
	firstCiphertext, err := box.Encrypt(`{"total_amount_cents":300}`)
	if err != nil {
		t.Fatal(err)
	}
	secondCiphertext, err := box.Encrypt(`{"total_amount_cents":300}`)
	if err != nil {
		t.Fatal(err)
	}
	correctObservationOccurredAt := time.Now().UTC().Add(-time.Hour)
	observation := Observation{
		EntityType: "order", EntityID: "erp-order-1", Version: "1",
		SubjectID: subjectID, RelationshipID: relationshipID,
		Snapshot: json.RawMessage(`{"total_amount_cents":300}`), SnapshotCiphertext: firstCiphertext,
		Sensitivity: "personal", PurposeKey: "customer_profile",
		OccurredAt: &correctObservationOccurredAt,
	}
	accepted, err := repository.InsertObservations(ctx, sourceRun, []Observation{observation})
	if err != nil || accepted != 1 {
		t.Fatalf("first observation accepted=%d err=%v", accepted, err)
	}
	var classification string
	if err := pool.QueryRow(ctx, `
		select classification
		from intelligence.source_observations
		where account_id = $1::uuid
		  and source_config_id = $2::uuid
		  and source_entity_type = $3
		  and source_entity_id = $4`,
		accountID,
		source.ID,
		observation.EntityType,
		observation.EntityID,
	).Scan(&classification); err != nil {
		t.Fatal(err)
	}
	if classification != ObservationClassificationRelationship {
		t.Fatalf("classification = %q", classification)
	}
	observation.SnapshotCiphertext = secondCiphertext
	accepted, err = repository.InsertObservations(ctx, sourceRun, []Observation{observation})
	if err != nil || accepted != 0 {
		t.Fatalf("encrypted observation dedupe accepted=%d err=%v", accepted, err)
	}
	wrongPurposeObservedAt := time.Now().UTC()
	wrongPurposeObservation := observation
	wrongPurposeObservation.EntityID = "erp-order-wrong-purpose"
	wrongPurposeObservation.PurposeKey = "marketing"
	wrongPurposeObservation.OccurredAt = &wrongPurposeObservedAt
	accepted, err = repository.InsertObservations(
		ctx,
		sourceRun,
		[]Observation{wrongPurposeObservation},
	)
	if err != nil || accepted != 1 {
		t.Fatalf("wrong-purpose fixture accepted=%d err=%v", accepted, err)
	}
	profileObservations, err := repository.ListRelationshipObservations(
		ctx,
		scope,
		relationshipID,
		nil,
		[]string{"customer_profile", "customer_relationship"},
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(profileObservations) != 1 ||
		profileObservations[0].PurposeKey != "customer_profile" ||
		profileObservations[0].SourceEntityID != observation.EntityID {
		t.Fatalf(
			"purpose deveria ser aplicado antes do LIMIT: %#v",
			profileObservations,
		)
	}

	fact, err := service.CreateManualFact(ctx, accountID, actorID, ManualFactInput{
		ClientAccountID: clientAccountID, SubjectID: subjectID,
		RelationshipID: relationshipID, FactKey: "profile.preferred_name",
		Value: json.RawMessage(`"Ana"`), ValueType: "string",
		Sensitivity: "personal", EvidenceNote: "confirmado em atendimento",
		IdempotencyKey: "manual-fact-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := service.CreateManualFact(ctx, accountID, actorID, ManualFactInput{
		ClientAccountID: clientAccountID, SubjectID: subjectID,
		RelationshipID: relationshipID, FactKey: "profile.preferred_name",
		Value: json.RawMessage(`"Ana"`), ValueType: "string",
		Sensitivity: "personal", EvidenceNote: "confirmado em atendimento",
		IdempotencyKey: "manual-fact-1",
	})
	if err != nil || repeated.ID != fact.ID {
		t.Fatalf("manual fact idempotency: first=%s repeated=%s err=%v", fact.ID, repeated.ID, err)
	}
	envelope, err := service.BuildContext(ctx, ContextRequest{
		AccountID: accountID, ClientAccountID: clientAccountID,
		SubjectID: subjectID, RelationshipID: relationshipID,
		ProcessKeys: []string{"conversation.triage"}, Purpose: "customer_service",
	})
	if err != nil || len(envelope.Facts) != 1 || envelope.SnapshotID == "" {
		t.Fatalf("context envelope=%#v err=%v", envelope, err)
	}

	model, err := service.ConfigureModel(ctx, accountID, actorID, AIModel{
		Provider: "openai", Model: "integration-model", Status: "enabled",
		Config: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := service.SetCredential(ctx, accountID, actorID, CredentialInput{
		Provider: "openai", Label: "integration", APIKey: "integration-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !credential.Status.Set || credential.Status.Last4 != "cret" {
		t.Fatalf("credential status = %#v", credential)
	}
	agent, err := service.CreateAgent(
		ctx, accountID, actorID, clientAccountID,
		"conversation-agent", "Conversation Agent",
	)
	if err != nil {
		t.Fatal(err)
	}
	agentVersion, err := service.CreateAgentVersion(
		ctx, accountID, actorID, agent.ID, AIAgentVersionInput{
			ModelID: model.ID, CredentialID: credential.ID, Temperature: 0.2,
			MaxOutputTokens: 1200, TimeoutMS: 60000, Config: json.RawMessage(`{}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	agentVersion, err = service.PublishAgentVersion(ctx, accountID, actorID, agentVersion.ID)
	if err != nil || agentVersion.Status != "published" {
		t.Fatalf("publish agent=%#v err=%v", agentVersion, err)
	}

	for _, processKey := range []string{"conversation.triage", "conversation.reply"} {
		prompt, createErr := service.CreatePromptDraft(ctx, accountID, actorID, PromptDraftInput{
			ClientAccountID: clientAccountID, ProcessKey: processKey,
			Layer:   "process_prompt",
			Content: "Use somente {{context}} e {{input}} para " + processKey + ".",
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		prompt, validation, validateErr := service.ValidatePromptVersion(
			ctx, accountID, actorID, prompt.ID,
		)
		if validateErr != nil || !validation.Valid || prompt.Status != "validated" {
			t.Fatalf("validate prompt=%#v validation=%#v err=%v", prompt, validation, validateErr)
		}
		evaluation, testErr := service.TestPromptVersion(
			ctx, accountID, actorID, prompt.ID, json.RawMessage(`{}`),
		)
		if testErr != nil || evaluation.Status != "passed" {
			t.Fatalf("evaluation=%#v err=%v", evaluation, testErr)
		}
		if _, publishErr := service.PublishPromptVersion(
			ctx, accountID, actorID, prompt.ID, PublishPromptInput{
				AgentVersionID: agentVersion.ID,
				SourcePolicy:   json.RawMessage(`[]`), ToolPolicy: json.RawMessage(`[]`),
				KnowledgePolicy: json.RawMessage(`[]`), RuntimePolicy: json.RawMessage(`{}`),
			},
		); publishErr != nil {
			t.Fatal(publishErr)
		}
	}
	if _, err := service.SetCapability(ctx, accountID, actorID, CapabilityInput{
		ClientAccountID: clientAccountID, Key: CapabilityRuntime,
		Mode: "on", Config: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ResolveExecutionPlan(
		ctx, scope, "conversation.triage",
	); err != nil {
		t.Fatalf("resolve execution plan: %v", err)
	}
	interaction := InteractionRequest{
		SchemaVersion: "interaction.request.v1", RequestID: "integration-request-1",
		InteractionID: "integration-interaction-1", AccountID: accountID,
		ClientAccountID: clientAccountID, SubjectID: subjectID,
		RelationshipID: relationshipID,
		ConversationID: "55555555-5555-4555-8555-555555555555",
		PipelineKey:    "conversation.respond", AIGeneration: 1,
		Message: json.RawMessage(`{"type":"text","text":"Quero agendar"}`),
		Purpose: "customer_service", Locale: "pt-BR", Channel: "whatsapp",
	}
	probe := interaction
	probe.RequestID = "integration-probe-1"
	if _, err := service.executeProcess(
		ctx, probe, envelope,
		"12121212-1212-4212-8212-121212121212",
		"active",
		"conversation.triage",
	); err != nil {
		t.Fatalf("execute process: %v", err)
	}
	decision, err := service.ExecuteInteraction(ctx, interaction)
	if err != nil || decision.Outcome != OutcomeReply || decision.ReplyDraft == nil {
		t.Fatalf("decision=%#v err=%v", decision, err)
	}
	var triageRun ProcessRunRef
	for _, run := range decision.ProcessRuns {
		if run.ProcessKey == "conversation.triage" {
			triageRun = run
			break
		}
	}
	if triageRun.RunID == "" {
		t.Fatalf("triage run ausente: %#v", decision.ProcessRuns)
	}
	if len(envelope.Facts) == 0 || len(envelope.Facts[0].Evidence) == 0 {
		t.Fatalf("contexto sem referencias para persistencia headless: %#v", envelope)
	}
	fixtures := validNonConversationalProcessOutputs()
	headlessExecutions := make([]HeadlessPersistedExecution, 0, 5)
	for _, processKey := range defaultRelationshipRefreshProcesses {
		raw := fixtures[processKey]
		raw = json.RawMessage(strings.ReplaceAll(
			string(raw),
			`"sourceKey":"omnichannel"`,
			`"sourceKey":"`+envelope.Facts[0].Evidence[0].SourceKey+`"`,
		))
		raw = json.RawMessage(strings.ReplaceAll(
			string(raw),
			processObservationOne,
			envelope.Facts[0].Evidence[0].ObservationID,
		))
		raw = json.RawMessage(strings.ReplaceAll(
			string(raw),
			processFactOne,
			envelope.Facts[0].ID,
		))
		raw = json.RawMessage(strings.ReplaceAll(
			string(raw),
			`"factKey":"preferred_name"`,
			`"factKey":"`+envelope.Facts[0].Key+`"`,
		))
		ciphertext, encryptErr := box.Encrypt(string(raw))
		if encryptErr != nil {
			t.Fatal(encryptErr)
		}
		var runID, promptBindingID string
		if err := pool.QueryRow(ctx, `
			insert into intelligence.runtime_runs (
			    request_id, interaction_id, account_id, client_account_id,
			    subject_id, relationship_id, process_definition_id,
			    process_config_version_id, process_key, prompt_binding_id,
			    agent_version_id, model_id, context_snapshot_id,
			    output_schema_version, status, input_fingerprint,
			    output_ciphertext, output_hash, started_at, completed_at
			)
			select
			    $3, '', source.account_id, source.client_account_id,
			    $4, $5, definition.id, definition.active_config_version_id,
			    $6, source.prompt_binding_id, source.agent_version_id,
			    source.model_id, $7, config.schema_version, 'succeeded',
			    $8, $9, $10, now(), now()
			  from intelligence.runtime_runs source
			  join intelligence.process_definitions definition
			    on definition.process_key = $6
			  join intelligence.process_config_versions config
			    on config.id = definition.active_config_version_id
			 where source.account_id = $1 and source.id = $2
			returning id, prompt_binding_id`,
			accountID,
			triageRun.RunID,
			"integration-headless-"+strings.ReplaceAll(processKey, ".", "-"),
			subjectID,
			relationshipID,
			processKey,
			envelope.SnapshotID,
			hashBytes([]byte(processKey+":"+envelope.SnapshotID)),
			ciphertext,
			hashBytes(raw),
		).Scan(&runID, &promptBindingID); err != nil {
			t.Fatal(err)
		}
		headlessExecutions = append(headlessExecutions, HeadlessPersistedExecution{
			RunRef: ProcessRunRef{
				ProcessKey:        processKey,
				RunID:             runID,
				Status:            "succeeded",
				ExecutionMode:     "active",
				PromptBindingID:   promptBindingID,
				ContextSnapshotID: envelope.SnapshotID,
			},
			Output:           raw,
			OutputCiphertext: ciphertext,
			OutputHash:       hashBytes(raw),
		})
	}
	persistInput := HeadlessRefreshPersistence{
		Scope:          scope,
		SubjectID:      subjectID,
		RelationshipID: relationshipID,
		ContextID:      envelope.SnapshotID,
		Context:        envelope,
		AsOf:           time.Now().UTC(),
		Executions:     headlessExecutions,
	}
	persisted, err := repository.PersistHeadlessRefresh(ctx, persistInput)
	if err != nil || persisted != 5 {
		t.Fatalf("persist headless rows=%d err=%v", persisted, err)
	}
	persisted, err = repository.PersistHeadlessRefresh(ctx, persistInput)
	if err != nil || persisted != 0 {
		t.Fatalf("replay headless rows=%d err=%v", persisted, err)
	}
	profileView, err := service.Profile(ctx, scope, relationshipID)
	if err != nil || profileView.Summary == nil {
		t.Fatalf("profile headless=%#v err=%v", profileView, err)
	}
	recommendations, err := service.Recommendations(ctx, scope, relationshipID, 20)
	if err != nil || len(recommendations) != 3 {
		t.Fatalf("recommendations headless=%#v err=%v", recommendations, err)
	}
	suggestions, err := service.SourceSuggestions(ctx, scope, relationshipID, 20)
	if err != nil || len(suggestions) != 1 || suggestions[0].Rationale == "" {
		t.Fatalf("source suggestions headless=%#v err=%v", suggestions, err)
	}
	var observationID string
	if err := pool.QueryRow(ctx, `
		select id
		from intelligence.source_observations
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and relationship_id = $3::uuid
		order by created_at
		limit 1`,
		accountID, clientAccountID, relationshipID,
	).Scan(&observationID); err != nil {
		t.Fatal(err)
	}
	claimOutput := []byte(`{
		"extractedClaims":[{
			"factKey":"profile.preferred_name",
			"valueType":"string",
			"value":"Bia",
			"confidence":0.88,
			"evidenceObservationIds":["` + observationID + `"],
			"validFrom":null,
			"validUntil":null
		}]
	}`)
	claimOutputCiphertext, err := box.Encrypt(string(claimOutput))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		update intelligence.runtime_runs
		set output_ciphertext = $3, output_hash = $4
		where account_id = $1::uuid and id = $2::uuid`,
		accountID, triageRun.RunID, claimOutputCiphertext, hashBytes(claimOutput),
	); err != nil {
		t.Fatal(err)
	}
	outcome := AcceptedOutcome{
		AccountID: accountID, ClientAccountID: clientAccountID,
		EventID:       "aaaaaaaa-1111-4111-8111-111111111111",
		InteractionID: interaction.InteractionID, DecisionID: decision.DecisionID,
		SubjectID: subjectID, RelationshipID: relationshipID,
		ConversationID: interaction.ConversationID, OutcomeType: "reply",
		Accepted: true, ActorType: "system", ProcessRuns: decision.ProcessRuns,
		Claims: []AcceptedClaimRef{{
			Ordinal: 0, FactKey: "profile.preferred_name", ValueType: "string",
			Confidence: 0.88, EvidenceObservationIDs: []string{observationID},
			ProcessKey: triageRun.ProcessKey, RuntimeRunID: triageRun.RunID,
			PromptBindingID:     triageRun.PromptBindingID,
			OutputSchemaVersion: triageRun.OutputSchemaVersion,
		}},
		Payload: json.RawMessage(`{"reasonCode":"omnichannel_effect_accepted"}`),
	}
	created, err = service.RecordOutcome(ctx, outcome)
	if err != nil || !created {
		t.Fatalf("accepted outcome com claim: created=%v err=%v", created, err)
	}
	created, err = service.RecordOutcome(ctx, outcome)
	if err != nil || created {
		t.Fatalf("replay do accepted outcome: created=%v err=%v", created, err)
	}
	candidates, err := service.CandidateClaims(ctx, scope, relationshipID, "candidate", 20)
	if err != nil || len(candidates) != 1 ||
		string(candidates[0].Value) != `"Bia"` ||
		len(candidates[0].Evidence) != 1 {
		t.Fatalf("claims candidatas=%#v err=%v", candidates, err)
	}
	reviewed, err := service.ReviewCandidateClaim(
		ctx, scope, actorID, candidates[0].ID,
		ClaimReviewInput{
			Status: "accepted", ReasonCode: "reviewed_by_operator",
			ExpectedRevision: candidates[0].Revision,
		},
	)
	if err != nil || reviewed.Status != "accepted" ||
		reviewed.VerificationState != "unverified" {
		t.Fatalf("reviewed claim=%#v err=%v", reviewed, err)
	}
	factsAfterReview, err := service.Facts(ctx, scope, relationshipID, 20)
	if err != nil || len(factsAfterReview) != 1 ||
		factsAfterReview[0].ID != fact.ID ||
		string(factsAfterReview[0].Value) != `"Ana"` {
		t.Fatalf("claim aceita materializou/sobrescreveu fact: facts=%#v err=%v", factsAfterReview, err)
	}

	secondClientOutcome := outcome
	secondClientOutcome.ClientAccountID = accountID
	secondClientOutcome.SubjectID = ""
	secondClientOutcome.RelationshipID = ""
	secondClientOutcome.ConversationID = ""
	secondClientOutcome.ProcessRuns = nil
	secondClientOutcome.Claims = nil
	secondClientOutcome.Payload = json.RawMessage(`{"reasonCode":"client_scoped_replay"}`)
	created, err = service.RecordOutcome(ctx, secondClientOutcome)
	if err != nil || !created {
		t.Fatalf("same event em outro client: created=%v err=%v", created, err)
	}
	audit, err := service.AuditEvents(ctx, scope, 200)
	if err != nil || len(audit) == 0 {
		t.Fatalf("audit count=%d err=%v", len(audit), err)
	}
}
