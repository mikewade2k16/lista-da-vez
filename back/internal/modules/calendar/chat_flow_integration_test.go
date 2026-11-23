package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tenants"
)

func TestChatFlowPostgresFirstReplyAndResume(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var accountID, userID, eventID string
	suffix := time.Now().UnixNano()
	err = pool.QueryRow(ctx, `insert into core.accounts(slug,name,is_active,is_agency)
		values ($1,'Chat fixture',true,false) returning id::text`, fmt.Sprintf("chat-fixture-%d", suffix)).Scan(&accountID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `delete from core.accounts where id=$1::uuid`, accountID)
		if userID != "" {
			_, _ = pool.Exec(context.Background(), `delete from core.users where id=$1::uuid`, userID)
		}
	}()
	err = pool.QueryRow(ctx, `insert into core.users(email,display_name,is_platform_admin,is_active)
		values ($1,'Chat fixture',true,true) returning id::text`, fmt.Sprintf("chat-%d@example.test", suffix)).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `insert into calendar.events(account_id,client_id,event_date,event_time,type,title,status,priority,responsible_id)
		values ($1::uuid,$1::uuid,'2026-11-13','','post','Evento comprovado','planejado','media','') returning id::text`, accountID).Scan(&eventID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `insert into calendar.events(account_id,client_id,event_date,event_time,type,title,status,priority,responsible_id)
		select $1::uuid,$1::uuid,'2026-11-13'::date,'','post','Outro evento '||n,'planejado','media',''
		from generate_series(1,6) n`, accountID); err != nil {
		t.Fatal(err)
	}
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload chatWebhookPayload
		if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
			t.Error(decodeErr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		calls++
		if calls == 2 && len(payload.History) != 2 {
			t.Errorf("segunda mensagem sem memória: %d", len(payload.History))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"answer": "Hoje há 13 eventos em 22/10.", "eventIds": []string{eventID, "inventado"}})
	}))
	defer upstream.Close()
	store := NewStore(pool)
	svc := NewService(store, nil).WithChat(upstream.URL, "").
		WithClientScope(assistantScopeLister{clients: []tenants.TenantView{{ID: accountID, Name: "Chat fixture"}}}).
		WithAssistantRuntimeProvider(func(context.Context, string) (AssistantRuntime, error) {
			return AssistantRuntime{Enabled: true, APIKey: "synthetic-test-only", Provider: "openai", Model: "test",
				SurfaceModules: map[string]map[string]string{"calendar": {"calendar": "read"}}}, nil
		}).WithAssistantModuleAvailability(func(context.Context, string, string) (bool, error) { return true, nil })
	p := auth.Principal{UserID: userID, AccountID: accountID, Role: auth.RolePlatformAdmin}
	req := ChatAskRequest{Question: "O que temos em 13/11/2026?", Month: "2026-11", Surface: "calendar",
		ScopeMode: "client", ScopeClientID: accountID, IdempotencyKey: "chat-first-fixture"}
	first, err := svc.ChatAsk(ctx, accountID, p, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Message.CalendarItems) != 7 || !strings.Contains(first.Answer, "7 eventos") || !strings.Contains(first.Answer, "13/11/2026") {
		t.Fatalf("primeira resposta sem card autoritativo: %#v", first.Message.CalendarItems)
	}
	req.ConversationID = first.ConversationID
	req.Question = "13 eventos? liste eles aqui"
	req.IdempotencyKey = "chat-second-fixture"
	second, err := svc.ChatAsk(ctx, accountID, p, req)
	if err != nil || second.ConversationID != first.ConversationID || len(second.Message.CalendarItems) != 7 || !strings.Contains(second.Answer, "7 eventos") {
		t.Fatalf("continuação falhou: %v", err)
	}
	if _, err = svc.ChatAsk(ctx, accountID, p, req); err != nil || calls != 2 {
		t.Fatalf("replay executou novamente: %d chamadas, %v", calls, err)
	}
	messages, err := store.ListMessages(ctx, accountID, first.ConversationID)
	if err != nil || len(messages) != 4 || len(messages[3].CalendarItems) != 7 {
		t.Fatalf("histórico ou cards não persistiram: %d mensagens, %v", len(messages), err)
	}
	req.Question = "Quais eventos em 14/11/2026?"
	req.IdempotencyKey = "chat-empty-day-fixture"
	empty, err := svc.ChatAsk(ctx, accountID, p, req)
	if err != nil || len(empty.Message.CalendarItems) != 0 || !strings.Contains(empty.Answer, "Não há eventos") {
		t.Fatalf("dia vazio recebeu contagem inventada: %q, %v", empty.Answer, err)
	}
	if _, err = store.GetConversation(ctx, first.ConversationID, testForeign); err != pgx.ErrNoRows {
		t.Fatalf("conversa vazou para outra conta: %v", err)
	}
	if _, err = pool.Exec(ctx, `update calendar.chat_conversations set assistant_mode='plus' where id=$1::uuid`, first.ConversationID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.GetConversation(ctx, first.ConversationID, accountID); err != pgx.ErrNoRows {
		t.Fatalf("conversa Plus entrou no fluxo legado: %v", err)
	}
}
