package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreActionProposalCapturesAndRevalidatesAtomicGuard(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL nao definido; pulando integracao do action guard")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var guardsAvailable bool
	if err := pool.QueryRow(ctx, `select exists (
		select 1 from information_schema.columns
		where table_schema = 'meta_ads' and table_name = 'action_proposals'
		  and column_name = 'guard_snapshot_hash'
	)`).Scan(&guardsAvailable); err != nil {
		t.Fatal(err)
	}
	if !guardsAvailable {
		t.Skip("banco de teste ainda nao recebeu a migration Meta 0290")
	}

	suffix := time.Now().UTC().UnixNano()
	var accountID string
	if err := pool.QueryRow(ctx, `insert into core.accounts (slug, name, is_active)
		values ($1, $2, true) returning id::text`,
		fmt.Sprintf("meta-action-guard-%d", suffix), "Meta action guard test",
	).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from core.accounts where id = $1::uuid`, accountID)
	})

	store := NewStore(pool, "meta-action-guard-test-key")
	expiresAt := time.Now().UTC().Add(time.Hour)
	connection, err := store.SaveConnectionSnapshot(
		ctx, accountID, "business-guard", "Connection", "token", &expiresAt,
		[]AdAccount{{
			MetaAdAccountID: "act_guard", Name: "Conta guard", Currency: "BRL", Status: "ACTIVE",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	adAccounts, err := store.ListAdAccounts(ctx, accountID)
	if err != nil || len(adAccounts) != 1 {
		t.Fatalf("ListAdAccounts() = %#v, err %v", adAccounts, err)
	}
	adAccount := adAccounts[0]
	dailyBudget := 10.0
	if err := store.ReplaceReportingSnapshotAtRevision(
		ctx, accountID, connection.ID, adAccount.ID, connection.Revision,
		[]Campaign{{
			MetaCampaignID: "987654321", Name: "Campanha guard",
			Objective: "OUTCOME_TRAFFIC", Status: "ACTIVE", DailyBudget: &dailyBudget,
		}}, nil, time.Now().UTC().AddDate(0, 0, -1), time.Now().UTC(),
	); err != nil {
		t.Fatal(err)
	}
	campaigns, err := store.ListCampaigns(ctx, accountID, adAccount.ID)
	if err != nil || len(campaigns) != 1 {
		t.Fatalf("ListCampaigns() = %#v, err %v", campaigns, err)
	}
	campaign := campaigns[0]
	maxDaily := 50.0
	if _, err := store.UpsertActionPolicy(ctx, accountID, adAccount, "", ActionPolicyInput{
		MaxDailyBudget: &maxDaily,
	}); err != nil {
		t.Fatal(err)
	}

	proposal := createStoreActionProposalForTest(t, ctx, store, accountID, adAccount, campaign, "guard:create:0001")
	if proposal.GuardSnapshotVersion != actionGuardSnapshotVersion ||
		proposal.GuardSnapshotHash == "" || proposal.ConnectionIDSnapshot == nil ||
		proposal.ConnectionRevisionSnapshot == nil || *proposal.ConnectionIDSnapshot != connection.ID ||
		*proposal.ConnectionRevisionSnapshot != connection.Revision ||
		proposal.CampaignSyncedAtSnapshot == nil || !proposal.PolicyConfiguredSnapshot {
		t.Fatalf("proposal guard incompleto: %#v", proposal)
	}
	claimed, execute, err := store.BeginActionExecution(
		ctx, accountID, proposal.ID, "", "guard:confirm:0001", false,
	)
	if err != nil || !execute || claimed.Status != ActionExecuting ||
		claimed.ClaimedConnectionID == nil || claimed.ClaimedConnectionRevision == nil ||
		*claimed.ClaimedConnectionID != connection.ID || *claimed.ClaimedConnectionRevision != connection.Revision {
		t.Fatalf("BeginActionExecution() = %#v, execute %v, err %v", claimed, execute, err)
	}

	stale := createStoreActionProposalForTest(t, ctx, store, accountID, adAccount, campaign, "guard:create:0002")
	if _, err := pool.Exec(ctx, `update meta_ads.campaigns
		set status = 'PAUSED', synced_at = now(), is_current = true
		where account_id = $1::uuid and id = $2::uuid`, accountID, campaign.ID); err != nil {
		t.Fatal(err)
	}
	staleResult, execute, err := store.BeginActionExecution(
		ctx, accountID, stale.ID, "", "guard:confirm:0002", false,
	)
	if !errors.Is(err, ErrActionProposalStale) || execute ||
		staleResult.Status != ActionFailed || staleResult.AttemptCount != 0 ||
		staleResult.ClaimedConnectionRevision != nil {
		t.Fatalf("stale claim = %#v, execute %v, err %v", staleResult, execute, err)
	}
}

func createStoreActionProposalForTest(
	t *testing.T,
	ctx context.Context,
	store *Store,
	accountID string,
	adAccount AdAccount,
	campaign Campaign,
	idempotencyKey string,
) ActionProposal {
	t.Helper()
	payload := json.RawMessage(`{"campaignId":"` + campaign.ID + `"}`)
	proposal, created, err := store.CreateActionProposal(ctx, actionProposalInsert{
		AccountID: accountID, ResourceAccountID: accountID,
		AdAccount: adAccount, Action: ActionPauseCampaign, Source: ActionSourceManual,
		TargetCampaign: &campaign, Payload: payload, Summary: "Pausar campanha guard",
		RequestHash:    actionRequestHash(ActionPauseCampaign, adAccount.ID, payload),
		IdempotencyKey: idempotencyKey,
	})
	if err != nil || !created {
		t.Fatalf("CreateActionProposal() = %#v, created %v, err %v", proposal, created, err)
	}
	return proposal
}
