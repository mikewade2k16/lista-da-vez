package customerintelligence

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

func TestAuditEventPageRepositoryFiltersBeforeLimitAndPaginatesTuple(
	t *testing.T,
) {
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

	suffix := time.Now().UTC().Format("20060102150405.000000000")
	var accountID, otherAccountID string
	if err := pool.QueryRow(ctx, `
		insert into core.accounts (slug, name)
		values ($1, 'CI audit pagination owner')
		returning id`,
		"ci-audit-page-"+suffix,
	).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(
			context.Background(),
			`delete from core.accounts where id = $1::uuid`,
			accountID,
		)
	}()
	if err := pool.QueryRow(ctx, `
		insert into core.accounts (slug, name)
		values ($1, 'CI audit pagination other owner')
		returning id`,
		"ci-audit-page-other-"+suffix,
	).Scan(&otherAccountID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(
			context.Background(),
			`delete from core.accounts where id = $1::uuid`,
			otherAccountID,
		)
	}()

	var clientID, otherClientID string
	var firstTupleID, secondTupleID string
	var mismatchID, globalID, otherClientEventID, otherAccountEventID string
	if err := pool.QueryRow(ctx, `
		select gen_random_uuid()::text, gen_random_uuid()::text,
		       gen_random_uuid()::text, gen_random_uuid()::text,
		       gen_random_uuid()::text, gen_random_uuid()::text,
		       gen_random_uuid()::text, gen_random_uuid()::text`,
	).Scan(
		&clientID,
		&otherClientID,
		&firstTupleID,
		&secondTupleID,
		&mismatchID,
		&globalID,
		&otherClientEventID,
		&otherAccountEventID,
	); err != nil {
		t.Fatal(err)
	}
	highTupleID, lowTupleID := firstTupleID, secondTupleID
	if strings.Compare(highTupleID, lowTupleID) < 0 {
		highTupleID, lowTupleID = lowTupleID, highTupleID
	}

	occurredAt := time.Date(2026, 7, 23, 15, 0, 0, 0, time.UTC)
	insert := func(
		id, ownerID string,
		eventClientID any,
		action, entityType string,
		eventTime time.Time,
	) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			insert into intelligence.audit_events (
			    id, account_id, client_account_id, event_type,
			    aggregate_type, aggregate_id, metadata, occurred_at
			)
			values (
			    $1::uuid, $2::uuid, $3::uuid, $4, $5, $1, '{}'::jsonb, $6
			)`,
			id,
			ownerID,
			eventClientID,
			action,
			entityType,
			eventTime,
		); err != nil {
			t.Fatal(err)
		}
	}
	insert(
		firstTupleID, accountID, clientID,
		"fact.created", "fact", occurredAt,
	)
	insert(
		secondTupleID, accountID, clientID,
		"fact.created", "fact", occurredAt,
	)
	insert(
		mismatchID, accountID, clientID,
		"prompt.updated", "prompt", occurredAt.Add(time.Hour),
	)
	insert(
		globalID, accountID, nil,
		"global.audit", "account", occurredAt.Add(-time.Hour),
	)
	insert(
		otherClientEventID, accountID, otherClientID,
		"fact.created", "fact", occurredAt.Add(2*time.Hour),
	)
	insert(
		otherAccountEventID, otherAccountID, clientID,
		"fact.created", "fact", occurredAt.Add(3*time.Hour),
	)

	service := NewService(
		NewPostgresRepository(pool),
		nil,
		nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
	)
	scope := Scope{AccountID: accountID, ClientAccountID: clientID}
	filtered := AuditEventQuery{
		Action:       "fact.created",
		EntityType:   "fact",
		OccurredFrom: occurredAt.Format(time.RFC3339Nano),
		OccurredTo:   occurredAt.Format(time.RFC3339Nano),
		Limit:        1,
	}
	firstPage, err := service.AuditEventPage(ctx, scope, filtered)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Items) != 1 ||
		firstPage.Items[0].ID != highTupleID ||
		firstPage.NextCursor == "" {
		t.Fatalf("first page = %#v", firstPage)
	}

	filtered.Cursor = firstPage.NextCursor
	secondPage, err := service.AuditEventPage(ctx, scope, filtered)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage.Items) != 1 ||
		secondPage.Items[0].ID != lowTupleID ||
		secondPage.NextCursor != "" {
		t.Fatalf("second page = %#v", secondPage)
	}

	unfiltered, err := service.AuditEventPage(
		ctx,
		scope,
		AuditEventQuery{Limit: 20},
	)
	if err != nil {
		t.Fatal(err)
	}
	found := make(map[string]bool, len(unfiltered.Items))
	for _, item := range unfiltered.Items {
		found[item.ID] = true
	}
	for _, expected := range []string{
		firstTupleID,
		secondTupleID,
		mismatchID,
		globalID,
	} {
		if !found[expected] {
			t.Fatalf("evento esperado %s ausente em %#v", expected, unfiltered.Items)
		}
	}
	for _, excluded := range []string{otherClientEventID, otherAccountEventID} {
		if found[excluded] {
			t.Fatalf("evento fora do escopo %s retornado", excluded)
		}
	}
}
