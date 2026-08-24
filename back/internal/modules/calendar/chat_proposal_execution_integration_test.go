package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/automation"
)

func TestChatProposalExecutionPostgresIdempotencyAndSurfaceRevocation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL nao definido; pulando integracao da confirmacao Calendar")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var schemaReady bool
	if err := pool.QueryRow(ctx, `select to_regclass('calendar.chat_proposal_executions') is not null
		and exists (select 1 from information_schema.columns
			where table_schema='calendar' and table_name='chat_conversations' and column_name='entry_surface')
		and exists (select 1 from information_schema.columns
			where table_schema='automation' and table_name='omni_chat_configs' and column_name='surface_modules')`).
		Scan(&schemaReady); err != nil {
		t.Fatal(err)
	}
	if !schemaReady {
		t.Skip("banco de teste ainda nao recebeu as migrations 0282/0287")
	}

	suffix := time.Now().UTC().UnixNano()
	var accountID, userID string
	if err := pool.QueryRow(ctx, `insert into core.accounts (slug, name, is_active, is_agency)
		values ($1, $2, true, true) returning id::text`,
		fmt.Sprintf("calendar-confirm-%d", suffix), "Calendar confirm integration",
	).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `insert into core.users
		(email, display_name, is_platform_admin, is_active)
		values ($1, $2, true, true) returning id::text`,
		fmt.Sprintf("calendar-confirm-%d@example.test", suffix), "Calendar confirm test",
	).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from core.accounts where id=$1::uuid`, accountID)
		_, _ = pool.Exec(cleanupCtx, `delete from core.users where id=$1::uuid`, userID)
	})
	if _, err := pool.Exec(ctx, `insert into core.modules
		(id, schema_name, label, description, is_core)
		values ('calendar', 'calendar', 'Calendar', '', false)
		on conflict (id) do nothing`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into core.account_modules (account_id, module_id, enabled)
		values ($1::uuid, 'calendar', true)
		on conflict (account_id, module_id) do update set enabled=true`, accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into core.modules
		(id, schema_name, label, description, is_core)
		values ('tasks', 'tasks', 'Tasks', '', false)
		on conflict (id) do nothing`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into core.account_modules (account_id, module_id, enabled)
		values ($1::uuid, 'tasks', true)
		on conflict (account_id, module_id) do update set enabled=true`, accountID); err != nil {
		t.Fatal(err)
	}

	automationStore := automation.NewStore(pool)
	if _, err := automationStore.SaveOmniChatConfig(ctx, automation.OmniChatConfig{
		AccountID: accountID, Enabled: true, Provider: "openai", Model: "gpt-4.1-mini",
		Temperature: 0.2, HistoryWindow: 5,
		SurfaceModules: map[string]map[string]string{
			AssistantSurfaceCalendar: {"calendar": "write", "tasks": "write"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	automationService := automation.NewService(automationStore, nil, nil, nil, "")
	capabilityMode := func(
		providerCtx context.Context,
		tx pgx.Tx,
		providerAccountID, surface, module string,
	) (string, error) {
		return automationService.OmniChatSurfaceModuleModeTx(
			providerCtx, tx, providerAccountID, surface, module,
		)
	}

	store := NewStore(pool)
	conversation, err := store.CreateConversation(ctx, accountID, userID, ChatConversationInput{
		Title: "Confirm integration", EntrySurface: AssistantSurfaceCalendar, ScopeMode: chatScopeAll,
	})
	if err != nil {
		t.Fatal(err)
	}
	firstTitle := fmt.Sprintf("Idempotent event %d", suffix)
	proposal := StoredProposal{
		ID: "event-create", Action: "create", Kind: "event", Status: "pending",
		Fields: ChatProposalFields{Date: "2026-08-18", Title: firstTitle, Type: "post"},
	}
	message, err := store.AppendMessage(ctx, accountID, conversation.ID, ChatMessageInput{
		Role: chatRoleAssistant, Content: "Confirme o evento", Proposals: []StoredProposal{proposal},
	})
	if err != nil {
		t.Fatal(err)
	}
	requestHash, err := hashJSON(map[string]string{"intent": "create", "title": firstTitle})
	if err != nil {
		t.Fatal(err)
	}
	command := chatProposalExecutionCommand{
		AccountID: accountID, StorageAccountID: accountID,
		ConversationID: conversation.ID, MessageID: message.ID, ProposalID: proposal.ID,
		ConfirmationKey: "calendar-confirm:idempotent", ActorUserID: userID,
		ActorLabel: "Calendar confirm test", ActorRole: auth.RolePlatformAdmin,
		RequestHash: requestHash, Proposal: proposal, CapabilityMode: capabilityMode,
	}
	first, err := store.ExecuteChatProposal(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := store.ExecuteChatProposal(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed || !jsonEqual(first.Resource, replayed.Resource) {
		t.Fatalf("replay divergente: first=%s replay=%s replayed=%v",
			first.Resource, replayed.Resource, replayed.Replayed)
	}
	assertCalendarEventCount(t, ctx, pool, accountID, firstTitle, 1)

	var boardID string
	if err := pool.QueryRow(ctx, `insert into tasks.boards
		(account_id, slug, name, created_by_user_id)
		values ($1::uuid, $2, 'Assistant tasks', $3::uuid) returning id::text`,
		accountID, fmt.Sprintf("assistant-%d", suffix), userID).Scan(&boardID); err != nil {
		t.Fatal(err)
	}
	taskTitle := fmt.Sprintf("Idempotent task %d", suffix)
	taskProposal := StoredProposal{ID: "task-create", Action: "create", Kind: "task", Status: "pending",
		Fields: ChatProposalFields{BoardID: boardID, Title: taskTitle, Priority: "alta", DueDate: "2026-08-21"}}
	taskMessage, err := store.AppendMessage(ctx, accountID, conversation.ID, ChatMessageInput{
		Role: chatRoleAssistant, Content: "Confirme a task", Proposals: []StoredProposal{taskProposal},
	})
	if err != nil {
		t.Fatal(err)
	}
	taskHash, _ := hashJSON(map[string]string{"intent": "task", "title": taskTitle})
	taskCommand := chatProposalExecutionCommand{
		AccountID: accountID, StorageAccountID: accountID, ConversationID: conversation.ID,
		MessageID: taskMessage.ID, ProposalID: taskProposal.ID, ConfirmationKey: "calendar-confirm:task",
		ActorUserID: userID, ActorLabel: "Calendar confirm test", ActorRole: auth.RolePlatformAdmin,
		RequestHash: taskHash, Proposal: taskProposal, CapabilityMode: capabilityMode,
	}
	createdTask, err := store.ExecuteChatProposal(ctx, taskCommand)
	if err != nil {
		t.Fatal(err)
	}
	if replay, replayErr := store.ExecuteChatProposal(ctx, taskCommand); replayErr != nil || !replay.Replayed {
		t.Fatalf("task replay=%v err=%v", replay.Replayed, replayErr)
	}
	var taskID string
	if err := pool.QueryRow(ctx, `select id::text from tasks.tasks where account_id=$1::uuid and title=$2`, accountID, taskTitle).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	var taskCount int
	if err := pool.QueryRow(ctx, `select count(*) from tasks.tasks where account_id=$1::uuid and title=$2`, accountID, taskTitle).Scan(&taskCount); err != nil || taskCount != 1 {
		t.Fatalf("task count=%d err=%v resource=%s", taskCount, err, createdTask.Resource)
	}
	itemProposal := StoredProposal{ID: "task-item-create", Action: "create", Kind: "taskItem", Status: "pending",
		Fields: ChatProposalFields{TargetID: taskID, TaskItem: &ChatProposalTaskItem{Title: "Revisar arte"}}}
	itemMessage, err := store.AppendMessage(ctx, accountID, conversation.ID, ChatMessageInput{
		Role: chatRoleAssistant, Content: "Confirme o checklist", Proposals: []StoredProposal{itemProposal},
	})
	if err != nil {
		t.Fatal(err)
	}
	itemHash, _ := hashJSON(map[string]string{"intent": "task-item", "task": taskID})
	itemCommand := chatProposalExecutionCommand{
		AccountID: accountID, StorageAccountID: accountID, ConversationID: conversation.ID,
		MessageID: itemMessage.ID, ProposalID: itemProposal.ID, ConfirmationKey: "calendar-confirm:task-item",
		ActorUserID: userID, ActorLabel: "Calendar confirm test", ActorRole: auth.RolePlatformAdmin,
		RequestHash: itemHash, Proposal: itemProposal, CapabilityMode: capabilityMode,
	}
	if _, err := store.ExecuteChatProposal(ctx, itemCommand); err != nil {
		t.Fatal(err)
	}
	if replay, replayErr := store.ExecuteChatProposal(ctx, itemCommand); replayErr != nil || !replay.Replayed {
		t.Fatalf("task item replay=%v err=%v", replay.Replayed, replayErr)
	}
	var checklistCount int
	if err := pool.QueryRow(ctx, `select jsonb_array_length(coalesce(ui_metadata->'checklist','[]'::jsonb))
		from tasks.tasks where account_id=$1::uuid and id=$2::uuid`, accountID, taskID).Scan(&checklistCount); err != nil || checklistCount != 1 {
		t.Fatalf("checklist count=%d err=%v", checklistCount, err)
	}

	mismatch := command
	mismatch.RequestHash, err = hashJSON(map[string]string{"intent": "different"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecuteChatProposal(ctx, mismatch); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("hash mismatch deveria falhar com ErrIdempotencyConflict, veio %v", err)
	}
	assertCalendarEventCount(t, ctx, pool, accountID, firstTitle, 1)

	revokedTitle := fmt.Sprintf("Revoked event %d", suffix)
	revokedProposal := StoredProposal{
		ID: "event-revoked", Action: "create", Kind: "event", Status: "pending",
		Fields: ChatProposalFields{Date: "2026-08-19", Title: revokedTitle, Type: "post"},
	}
	revokedMessage, err := store.AppendMessage(ctx, accountID, conversation.ID, ChatMessageInput{
		Role: chatRoleAssistant, Content: "Este card sera revogado", Proposals: []StoredProposal{revokedProposal},
	})
	if err != nil {
		t.Fatal(err)
	}
	preflightTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	mode, err := automationService.OmniChatSurfaceModuleModeTx(
		ctx, preflightTx, accountID, AssistantSurfaceCalendar, "calendar",
	)
	_ = preflightTx.Rollback(ctx)
	if err != nil || mode != "write" {
		t.Fatalf("preflight mode=%q err=%v", mode, err)
	}
	if _, err := automationStore.SaveOmniChatConfig(ctx, automation.OmniChatConfig{
		AccountID: accountID, Enabled: true, Provider: "openai", Model: "gpt-4.1-mini",
		Temperature: 0.2, HistoryWindow: 5,
		SurfaceModules: map[string]map[string]string{
			AssistantSurfaceCalendar: {"calendar": "off", "tasks": "write"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	revokedHash, err := hashJSON(map[string]string{"intent": "revoked", "title": revokedTitle})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecuteChatProposal(ctx, chatProposalExecutionCommand{
		AccountID: accountID, StorageAccountID: accountID,
		ConversationID: conversation.ID, MessageID: revokedMessage.ID, ProposalID: revokedProposal.ID,
		ConfirmationKey: "calendar-confirm:revoked", ActorUserID: userID,
		ActorLabel: "Calendar confirm test", ActorRole: auth.RolePlatformAdmin,
		RequestHash: revokedHash, Proposal: revokedProposal, CapabilityMode: capabilityMode,
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("surface revogada deveria fechar em ErrForbidden, veio %v", err)
	}
	assertCalendarEventCount(t, ctx, pool, accountID, revokedTitle, 0)
	var receiptStatus string
	if err := pool.QueryRow(ctx, `select status from calendar.chat_proposal_executions
		where account_id=$1::uuid and message_id=$2::uuid and proposal_id=$3`,
		accountID, revokedMessage.ID, revokedProposal.ID).Scan(&receiptStatus); err != nil {
		t.Fatal(err)
	}
	if receiptStatus != "pending" {
		t.Fatalf("receipt revogado deveria continuar pending, veio %q", receiptStatus)
	}
}

func jsonEqual(left, right []byte) bool {
	var leftValue any
	var rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func assertCalendarEventCount(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	accountID, title string,
	want int,
) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, `select count(*) from calendar.events
		where account_id=$1::uuid and title=$2`, accountID, title).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("event count para %q = %d, want %d", title, got, want)
	}
}
