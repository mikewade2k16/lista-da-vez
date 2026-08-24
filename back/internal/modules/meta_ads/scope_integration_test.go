package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAgencyTwoClientScopePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL nao definido; pulando integracao de escopo Meta")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	var schemaReady bool
	if err = pool.QueryRow(ctx, `select
		to_regclass('meta_ads.instagram_identity_client_mappings') is not null
		and exists (
			select 1 from information_schema.columns
			where table_schema = 'meta_ads' and table_name = 'ad_accounts'
			  and column_name = 'is_current'
		)`).Scan(&schemaReady); err != nil {
		t.Fatal(err)
	}
	if !schemaReady {
		t.Skip("banco de teste ainda nao recebeu as migrations Meta 0288/0291")
	}

	graph := newScopeGraphServer(t)
	defer graph.Close()
	store := NewStore(pool, "meta-scope-integration-test-key")
	service := NewService(store, NewMetaClient(graph.URL), nil)
	agencyID, clientAID, clientBID, outsiderID := insertScopeAccounts(t, ctx, pool)

	expiresAt := time.Now().UTC().Add(time.Hour)
	connection, err := store.SaveConnectionSnapshot(
		ctx, agencyID, "business-scope", "Meta scope test", "scope-token", &expiresAt,
		[]AdAccount{
			{MetaAdAccountID: "1001", Name: "Conta A", Currency: "BRL", Status: "active"},
			{MetaAdAccountID: "1002", Name: "Conta B", Currency: "BRL", Status: "active"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := store.ListAdAccounts(ctx, agencyID)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("ListAdAccounts(agency) = %#v, err %v", accounts, err)
	}
	byMetaID := map[string]AdAccount{}
	for _, account := range accounts {
		byMetaID[account.MetaAdAccountID] = account
	}
	accountA := byMetaID["1001"]
	accountB := byMetaID["1002"]
	if accountA.ID == "" || accountB.ID == "" {
		t.Fatalf("snapshot de ad accounts incompleto: %#v", byMetaID)
	}
	if _, err = service.SetAdAccountClient(ctx, agencyID, accountA.ID, clientAID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetAdAccountClient(ctx, agencyID, accountB.ID, clientBID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetAdAccountClient(ctx, agencyID, accountA.ID, outsiderID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("mapping cross-org error = %v, want pgx.ErrNoRows", err)
	}
	if _, err = service.SetAdAccountClient(ctx, clientAID, accountA.ID, clientAID); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("client remap error = %v, want ErrNotConnected", err)
	}

	if _, err = service.SetInstagramIdentityClient(ctx, agencyID, "2001", clientAID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetInstagramIdentityClient(ctx, agencyID, "2002", clientBID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetInstagramIdentityClient(ctx, agencyID, "2001", outsiderID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("instagram mapping cross-org error = %v, want pgx.ErrNoRows", err)
	}
	if _, err = service.SetInstagramIdentityClient(ctx, clientAID, "2001", clientAID); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("client instagram remap error = %v, want ErrNotConnected", err)
	}

	assertVisibleAdAccounts(t, ctx, service, agencyID, accountA.ID, accountB.ID)
	assertVisibleAdAccounts(t, ctx, service, clientAID, accountA.ID)
	assertVisibleAdAccounts(t, ctx, service, clientBID, accountB.ID)
	if _, err = service.ListAdAccounts(ctx, outsiderID); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("outside organization ListAdAccounts error = %v, want ErrNotConnected", err)
	}
	if _, err = service.ListCampaigns(ctx, clientAID, accountB.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("client A reading client B error = %v, want pgx.ErrNoRows", err)
	}

	assertAssistantScope(t, ctx, service, AssistantContextRequest{
		AccountID: agencyID, VisibleClientIDs: []string{clientAID, clientBID}, IsAgency: true,
	}, []string{accountA.ID, accountB.ID}, []string{"2001", "2002"})
	assertAssistantScope(t, ctx, service, AssistantContextRequest{
		AccountID: agencyID, ClientAccountID: clientAID,
		VisibleClientIDs: []string{clientAID, clientBID}, IsAgency: true,
	}, []string{accountA.ID}, []string{"2001"})
	assertAssistantScope(t, ctx, service, AssistantContextRequest{
		AccountID: clientAID, ClientAccountID: clientAID,
	}, []string{accountA.ID}, []string{"2001"})
	assertAssistantScope(t, ctx, service, AssistantContextRequest{
		AccountID: clientBID, ClientAccountID: clientBID,
	}, []string{accountB.ID}, []string{"2002"})

	if connection.AccountID != agencyID {
		t.Fatalf("connection owner = %q, want agency %q", connection.AccountID, agencyID)
	}
}

func insertScopeAccounts(
	t *testing.T, ctx context.Context, pool *pgxpool.Pool,
) (agencyID, clientAID, clientBID, outsiderID string) {
	t.Helper()
	suffix := time.Now().UTC().UnixNano()
	var organizationID, outsiderOrganizationID string
	if err := pool.QueryRow(ctx, `insert into core.organizations (slug, name, is_active)
		values ($1, $2, true) returning id::text`,
		fmt.Sprintf("meta-scope-org-%d", suffix), "Meta scope organization",
	).Scan(&organizationID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `insert into core.organizations (slug, name, is_active)
		values ($1, $2, true) returning id::text`,
		fmt.Sprintf("meta-scope-outsider-org-%d", suffix), "Meta scope outsider organization",
	).Scan(&outsiderOrganizationID); err != nil {
		t.Fatal(err)
	}
	insertAccount := func(slug, name, organization string, agency bool) string {
		var id string
		if err := pool.QueryRow(ctx, `insert into core.accounts
			(organization_id, slug, name, is_active, is_agency)
			values ($1::uuid, $2, $3, true, $4) returning id::text`,
			organization, slug, name, agency,
		).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	agencyID = insertAccount(fmt.Sprintf("meta-scope-agency-%d", suffix), "Meta scope agency", organizationID, true)
	clientAID = insertAccount(fmt.Sprintf("meta-scope-client-a-%d", suffix), "Meta scope client A", organizationID, false)
	clientBID = insertAccount(fmt.Sprintf("meta-scope-client-b-%d", suffix), "Meta scope client B", organizationID, false)
	outsiderID = insertAccount(fmt.Sprintf("meta-scope-outsider-%d", suffix), "Meta scope outsider", outsiderOrganizationID, false)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from core.accounts
			where organization_id in ($1::uuid, $2::uuid)`, organizationID, outsiderOrganizationID)
		_, _ = pool.Exec(cleanupCtx, `delete from core.organizations
			where id in ($1::uuid, $2::uuid)`, organizationID, outsiderOrganizationID)
	})
	return agencyID, clientAID, clientBID, outsiderID
}

func newScopeGraphServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer scope-token" {
			http.Error(w, `{"error":{"message":"unauthorized"}}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/me/accounts":
			writeScopeJSON(t, w, map[string]any{"data": []map[string]any{
				{"id": "3001", "name": "Page A", "instagram_business_account": map[string]string{"id": "2001", "username": "client_a"}},
				{"id": "3002", "name": "Page B", "instagram_business_account": map[string]string{"id": "2002", "username": "client_b"}},
			}})
		case "/2001/media":
			writeScopeJSON(t, w, map[string]any{"data": []map[string]string{{
				"id": "4001", "caption": "Post A", "media_type": "IMAGE",
				"media_url": "https://cdn.invalid/a.jpg", "permalink": "https://instagram.invalid/p/a",
				"timestamp": "2026-08-21T12:00:00+0000",
			}}})
		case "/2002/media":
			writeScopeJSON(t, w, map[string]any{"data": []map[string]string{{
				"id": "4002", "caption": "Post B", "media_type": "IMAGE",
				"media_url": "https://cdn.invalid/b.jpg", "permalink": "https://instagram.invalid/p/b",
				"timestamp": "2026-08-21T12:00:00+0000",
			}}})
		default:
			http.NotFound(w, r)
		}
	}))
}

func writeScopeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode graph fixture: %v", err)
	}
}

func assertVisibleAdAccounts(
	t *testing.T, ctx context.Context, service *Service, viewerID string, expectedIDs ...string,
) {
	t.Helper()
	rows, err := service.ListAdAccounts(ctx, viewerID)
	if err != nil {
		t.Fatal(err)
	}
	actual := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		actual[row.ID] = struct{}{}
	}
	if len(actual) != len(expectedIDs) {
		t.Fatalf("viewer %q ad accounts = %#v, want %v", viewerID, rows, expectedIDs)
	}
	for _, id := range expectedIDs {
		if _, ok := actual[id]; !ok {
			t.Fatalf("viewer %q cannot see expected ad account %q: %#v", viewerID, id, rows)
		}
	}
}

func assertAssistantScope(
	t *testing.T,
	ctx context.Context,
	service *Service,
	request AssistantContextRequest,
	expectedAdAccountIDs []string,
	expectedInstagramIDs []string,
) {
	t.Helper()
	result, err := service.AssistantContextForScope(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	actualAccounts := make(map[string]struct{}, len(result.AdAccounts))
	for _, account := range result.AdAccounts {
		actualAccounts[account.ID] = struct{}{}
	}
	actualInstagram := make(map[string]struct{}, len(result.Instagram.Accounts))
	for _, account := range result.Instagram.Accounts {
		actualInstagram[account.IGUserID] = struct{}{}
	}
	assertScopeIDs(t, "ad account", actualAccounts, expectedAdAccountIDs)
	assertScopeIDs(t, "instagram", actualInstagram, expectedInstagramIDs)
	if len(result.Instagram.Posts) != len(expectedInstagramIDs) {
		t.Fatalf("instagram posts = %#v, want %d scoped posts", result.Instagram.Posts, len(expectedInstagramIDs))
	}
}

func assertScopeIDs(t *testing.T, kind string, actual map[string]struct{}, expected []string) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("%s ids = %#v, want %v", kind, actual, expected)
	}
	for _, id := range expected {
		if _, ok := actual[id]; !ok {
			t.Fatalf("missing scoped %s %q in %#v", kind, id, actual)
		}
	}
}
