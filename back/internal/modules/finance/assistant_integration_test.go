package finance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestFinanceAssistantSettingsAndContext(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `create table if not exists core.users(id uuid primary key default gen_random_uuid()); create schema if not exists calendar; create table if not exists calendar.chat_conversations(entry_surface text); create table if not exists calendar.chat_ask_requests(entry_surface text)`); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"0304_finance_user_fiscal_profile.sql", "0305_finance_assistant.sql"} {
		raw, err := os.ReadFile("../../platform/database/migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	account := financeTestAccount(t, pool)
	other := financeTestAccount(t, pool)
	finance := NewService(NewPostgresSheetStore(pool), NewPostgresConfigStore(pool))
	service := &AssistantService{pool: pool, finance: finance, fiscal: NewFiscalService(NewPostgresFiscalStore(pool), nil), resolver: func(context.Context, string, AssistantConfig) (AssistantRuntime, error) {
		return AssistantRuntime{Provider: "openai", Model: "test", APIKey: "private-ai-key-only-test"}, nil
	}}
	initial, err := service.Config(ctx, account)
	if err != nil || initial.Enabled || initial.UpdatedAt != "" {
		t.Fatal("unexpected inherited config")
	}
	cfg := AssistantConfig{Enabled: true, CredentialID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Model: "test", HistoryWindow: 5, Instructions: "Instrução específica"}
	saved, err := service.SaveConfig(ctx, account, AssistantConfigInput{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SaveConfig(ctx, account, AssistantConfigInput{Config: cfg}); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("stale config allowed")
	}
	foreign, err := service.Config(ctx, other)
	if err != nil || foreign.Enabled {
		t.Fatal("config leaked across accounts")
	}
	raw, _ := json.Marshal(saved)
	if strings.Contains(string(raw), "private-ai-key") {
		t.Fatal("AI key exposed")
	}
	runtime, err := service.Runtime(ctx, account)
	if err != nil || !strings.Contains(runtime.Instructions, "não afirme") {
		t.Fatal("missing emission guard")
	}
	period := "2026-11"
	sheet, err := finance.CreateSheet(ctx, account, SheetInput{Period: &period, Entradas: []Line{{Description: "Entrada QA", Amount: 100}}})
	if err != nil {
		t.Fatal(err)
	}
	p := auth.Principal{AccountID: account, UserID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", PermissionsResolved: true, Permissions: []string{"finance.sheets.view"}}
	data, err := service.Context(ctx, account, p, sheet.ID, period)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(data)
	if !strings.Contains(string(raw), "Entrada QA") || strings.Contains(string(raw), "ciphertext") || strings.Contains(string(raw), "credentialsSet") {
		t.Fatal("unsafe context")
	}
	p.AccountID = other
	if _, err = service.Context(ctx, account, p, sheet.ID, period); !errors.Is(err, ErrFiscalNotFound) {
		t.Fatal("scope forgery")
	}
	p.AccountID = account
	if _, err = service.Context(ctx, other, p, sheet.ID, period); !errors.Is(err, ErrFiscalNotFound) {
		t.Fatal("foreign ledger read")
	}
}
