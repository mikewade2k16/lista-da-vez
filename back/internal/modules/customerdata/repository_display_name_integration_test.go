package customerdata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

func TestRelationshipDisplayNameProviderCannotOverrideManualEdit(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL nao definido")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := database.ApplyMigrationsWithOptions(ctx, pool, database.MigrationOptions{
		SkipDataSeeds: true,
	}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "-")
	var accountID, actorID string
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		if accountID != "" {
			for _, query := range []string{
				`delete from customer_data.subject_source_links where account_id = $1::uuid`,
				`delete from customer_data.subject_identities where account_id = $1::uuid`,
				`delete from customer_data.outbox_events where account_id = $1::uuid`,
				`delete from customer_data.audit_events where account_id = $1::uuid`,
				`delete from customer_data.relationships where account_id = $1::uuid`,
				`delete from customer_data.subjects where account_id = $1::uuid`,
				`delete from customer_data.writer_states where account_id = $1::uuid`,
				`delete from customer_data.capability_states where account_id = $1::uuid`,
			} {
				_, _ = pool.Exec(cleanupCtx, query, accountID)
			}
		}
		if actorID != "" {
			_, _ = pool.Exec(cleanupCtx, `delete from core.users where id = $1::uuid`, actorID)
		}
		if accountID != "" {
			_, _ = pool.Exec(cleanupCtx, `delete from core.accounts where id = $1::uuid`, accountID)
		}
	})

	if err := pool.QueryRow(ctx, `
		insert into core.accounts (slug, name, is_active, is_agency)
		values ($1, 'Customer Data display-name integration', true, false)
		returning id::text
	`, "cd-display-name-"+suffix).Scan(&accountID); err != nil {
		t.Fatalf("create account fixture: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		insert into core.users (email, display_name, is_platform_admin, is_active)
		values ($1, 'Customer Data integration actor', true, true)
		returning id::text
	`, "cd-display-name-"+suffix+"@example.invalid").Scan(&actorID); err != nil {
		t.Fatalf("create actor fixture: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into customer_data.capability_states (
			account_id, client_account_id, capability_key, mode,
			revision, last_idempotency_key, updated_by_user_id
		) values
			($1::uuid, $1::uuid, 'core', 'on', 1, $2, $4::uuid),
			($1::uuid, $1::uuid, 'identity_resolution', 'on', 1, $3, $4::uuid)
	`, accountID, "display-name-core-"+suffix, "display-name-identity-"+suffix, actorID); err != nil {
		t.Fatalf("enable capabilities: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into customer_data.writer_states (
			account_id, client_account_id, entity_key, mode,
			source_checksum, target_checksum, approved_by_user_id,
			approved_at, revision, last_idempotency_key
		) values
			($1::uuid, $1::uuid, 'relationship', 'new', 'fixture', 'fixture',
			 $2::uuid, now(), 1, $3),
			($1::uuid, $1::uuid, 'identity', 'new', 'fixture', 'fixture',
			 $2::uuid, now(), 1, $4)
	`, accountID, actorID, "display-name-relationship-"+suffix, "display-name-identity-writer-"+suffix); err != nil {
		t.Fatalf("enable writers: %v", err)
	}

	repository := NewPostgresRepository(pool)
	service := NewService(
		repository,
		allowAllCustomerDataPermissions{},
		displayNameIntegrationProtector{},
	)
	source := SourceReference{
		SourceModule:     "omnichannel",
		SourceKey:        "evolution",
		SourceEntityType: "contact",
		SourceEntityID:   "provider-contact-" + suffix,
	}
	rawIdentity := "+5511999987654"
	resolve := func(requestID, displayName string, allowCreate bool) ResolveRelationshipResult {
		t.Helper()
		result, resolveErr := service.ResolveRelationship(ctx, ResolveRelationshipRequest{
			AccountID:       accountID,
			ClientAccountID: accountID,
			RequestID:       requestID,
			Source:          source,
			Identities: []IdentityInput{{
				Kind:               "whatsapp",
				Issuer:             "evolution:display-name-integration",
				Value:              rawIdentity,
				VerificationStatus: "verified",
				VerificationMethod: "provider_webhook",
			}},
			DisplayName: displayName,
			OccurredAt:  time.Now().UTC(),
			Purpose:     "customer_service",
			AllowCreate: allowCreate,
		})
		if resolveErr != nil {
			t.Fatalf("resolve relationship %q: %v", requestID, resolveErr)
		}
		return result
	}

	initialProviderName := "Nome inicial do WhatsApp"
	created := resolve("display-name-create-"+suffix, initialProviderName, true)
	if created.Status != "created" || created.RelationshipID == "" || created.SubjectID == "" {
		t.Fatalf("initial resolve = %#v", created)
	}
	assertRelationshipDisplayNameState(
		t, ctx, pool, accountID, created.RelationshipID,
		initialProviderName, "rule", 1,
	)

	refreshedProviderName := "Nome atualizado do WhatsApp"
	refreshed := resolve("display-name-refresh-"+suffix, refreshedProviderName, false)
	if refreshed.Status != "resolved" || !refreshed.Replayed ||
		refreshed.RelationshipID != created.RelationshipID {
		t.Fatalf("provider refresh resolve = %#v", refreshed)
	}
	assertRelationshipDisplayNameState(
		t, ctx, pool, accountID, created.RelationshipID,
		refreshedProviderName, "rule", 2,
	)

	manualName := "Nome confirmado manualmente"
	principal := auth.Principal{
		UserID: actorID, AccountID: accountID, Role: auth.RolePlatformAdmin,
	}
	manual, err := service.UpdateRelationship(ctx, principal, created.RelationshipID, RelationshipPatch{
		DisplayName:      &manualName,
		ExpectedRevision: 2,
	})
	if err != nil {
		t.Fatalf("manual relationship update: %v", err)
	}
	if manual.DisplayName != manualName || manual.ClassificationSource != "manual" ||
		manual.Revision != 3 {
		t.Fatalf("manual relationship = %#v", manual)
	}

	ignoredProviderName := "Nome posterior que deve ser ignorado"
	replayed := resolve("display-name-replay-"+suffix, ignoredProviderName, false)
	if replayed.Status != "resolved" || !replayed.Replayed ||
		replayed.RelationshipID != created.RelationshipID {
		t.Fatalf("provider replay after manual edit = %#v", replayed)
	}
	assertRelationshipDisplayNameState(
		t, ctx, pool, accountID, created.RelationshipID,
		manualName, "manual", 3,
	)

	rows, err := pool.Query(ctx, `
		select actor_type, actor_id, action, reason,
		       old_hash, new_hash, correlation_id, to_jsonb(a)::text
		from customer_data.audit_events a
		where account_id = $1::uuid
		  and client_account_id = $1::uuid
		  and relationship_id = $2::uuid
		  and entity_type = 'relationship'
		order by created_at, id
	`, accountID, created.RelationshipID)
	if err != nil {
		t.Fatalf("query relationship audit: %v", err)
	}
	defer rows.Close()

	type auditRow struct {
		actorType   string
		actorID     *string
		action      string
		reason      *string
		oldHash     *string
		newHash     *string
		correlation *string
		serialized  string
	}
	audits := make([]auditRow, 0, 2)
	for rows.Next() {
		var item auditRow
		if err := rows.Scan(
			&item.actorType,
			&item.actorID,
			&item.action,
			&item.reason,
			&item.oldHash,
			&item.newHash,
			&item.correlation,
			&item.serialized,
		); err != nil {
			t.Fatalf("scan relationship audit: %v", err)
		}
		audits = append(audits, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate relationship audit: %v", err)
	}
	if len(audits) != 2 {
		t.Fatalf("relationship audit count = %d, want 2: %#v", len(audits), audits)
	}

	providerAudit := audits[0]
	if providerAudit.actorType != "service" || providerAudit.actorID != nil ||
		providerAudit.action != "relationship.display_name.refreshed" ||
		stringPointerValue(providerAudit.reason) != "provider_supplied_name" {
		t.Fatalf("provider audit = %#v", providerAudit)
	}
	manualAudit := audits[1]
	if manualAudit.actorType != "user" ||
		stringPointerValue(manualAudit.actorID) != actorID ||
		manualAudit.action != "update" ||
		stringPointerValue(manualAudit.reason) != "manual_patch" {
		t.Fatalf("manual audit = %#v", manualAudit)
	}
	for _, audit := range audits {
		if audit.oldHash != nil || audit.newHash != nil || audit.correlation != nil {
			t.Fatalf("audit should contain metadata only: %#v", audit)
		}
		for _, forbidden := range []string{
			initialProviderName,
			refreshedProviderName,
			manualName,
			ignoredProviderName,
			rawIdentity,
		} {
			if strings.Contains(audit.serialized, forbidden) {
				t.Fatalf("audit leaked raw value %q: %s", forbidden, audit.serialized)
			}
		}
	}
}

type allowAllCustomerDataPermissions struct{}

func (allowAllCustomerDataPermissions) HasAccountPermission(
	context.Context,
	string,
	string,
	string,
) (bool, error) {
	return true, nil
}

type displayNameIntegrationProtector struct{}

func (displayNameIntegrationProtector) Protect(_ Scope, input IdentityInput) (ProtectedIdentity, error) {
	sum := sha256.Sum256([]byte(input.Kind + "\x00" + input.Issuer + "\x00" + input.Value))
	occurredAt := time.Now().UTC()
	if input.OccurredAt != nil {
		occurredAt = input.OccurredAt.UTC()
	}
	return ProtectedIdentity{
		Kind:               strings.ToLower(strings.TrimSpace(input.Kind)),
		Issuer:             strings.ToLower(strings.TrimSpace(input.Issuer)),
		Ciphertext:         hex.EncodeToString(sum[:]),
		Fingerprint:        hex.EncodeToString(sum[:]),
		KeyVersion:         "integration-v1",
		MaskedValue:        "***7654",
		VerificationStatus: input.VerificationStatus,
		VerificationMethod: input.VerificationMethod,
		SourceRefType:      input.SourceRefType,
		SourceRefID:        input.SourceRefID,
		Metadata:           json.RawMessage(`{}`),
		OccurredAt:         occurredAt,
		IdempotencyKey:     input.IdempotencyKey,
	}, nil
}

func (displayNameIntegrationProtector) ProtectContent(_ Scope, plaintext string) (string, string, error) {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:]), "integration-v1", nil
}

func (displayNameIntegrationProtector) RevealContent(_ Scope, ciphertext, _ string) (string, error) {
	return ciphertext, nil
}

func assertRelationshipDisplayNameState(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	accountID string,
	relationshipID string,
	wantName string,
	wantClassification string,
	wantRevision int64,
) {
	t.Helper()
	var displayName, classification string
	var revision int64
	if err := pool.QueryRow(ctx, `
		select display_name, classification_source, revision
		from customer_data.relationships
		where account_id = $1::uuid
		  and client_account_id = $1::uuid
		  and id = $2::uuid
	`, accountID, relationshipID).Scan(&displayName, &classification, &revision); err != nil {
		t.Fatalf("read relationship state: %v", err)
	}
	if displayName != wantName || classification != wantClassification || revision != wantRevision {
		t.Fatalf(
			"relationship state = name %q classification %q revision %d; want %q %q %d",
			displayName,
			classification,
			revision,
			wantName,
			wantClassification,
			wantRevision,
		)
	}
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
