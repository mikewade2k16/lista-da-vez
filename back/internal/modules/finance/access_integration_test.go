package finance

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
	"os"
	"strings"
	"testing"
)

func TestAccessConversationAndPortalCheckIsolation(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `create table if not exists core.users(id uuid primary key default gen_random_uuid())`); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"0304_finance_user_fiscal_profile.sql", "0308_finance_access_conversation.sql"} {
		raw, err := os.ReadFile("../../platform/database/migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err = pool.Exec(ctx, string(raw)); err != nil {
				t.Fatal(err)
			}
		}
	}
	account, other := financeTestAccount(t, pool), financeTestAccount(t, pool)
	var user, otherUser string
	for _, id := range []*string{&user, &otherUser} {
		if err := pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `delete from core.users where id=any($1::uuid[])`, []string{user, otherUser})
	})
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	fiscal := NewFiscalService(NewPostgresFiscalStore(pool), box)
	chat := &AccessConversationService{pool: pool, fiscal: fiscal}
	request := AccessConversationInput{RequestID: "00000000-0000-4000-8000-000000000001", Text: "CPF/CNPJ e senha do Emissor Nacional"}
	first, err := chat.Send(ctx, account, user, request)
	if err != nil || len(first.Messages) != 2 {
		t.Fatal("conversation did not persist", err)
	}
	repeated, err := chat.Send(ctx, account, user, request)
	if err != nil || len(repeated.Messages) != 2 {
		t.Fatal("retry duplicated conversation", err)
	}
	p, err := fiscal.GetProfile(ctx, account, user)
	if err != nil || p.Profile.PortalLoginMethod != "portal" || p.Profile.PortalURL != nationalPortalLogin {
		t.Fatal("chat did not save canonical method", err)
	}
	if _, err = fiscal.TestPortalAccess(ctx, account, user, p.UpdatedAt); !errors.Is(err, ErrFiscalInvalid) {
		t.Fatal("test without credentials succeeded")
	}
	request.RequestID = "00000000-0000-4000-8000-000000000002"
	request.Text = "minha senha é qa-unique-secret"
	messages, err := chat.Send(ctx, account, user, request)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(messages)
	if err != nil || strings.Contains(string(raw), "qa-unique-secret") {
		t.Fatal("password leaked to messages")
	}
	p, err = fiscal.SaveCredentials(ctx, account, user, FiscalCredentialInput{Username: "qa-user", Password: "qa-portal-secret", ExpectedUpdatedAt: p.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	fiscal.portalProbe = func(_ context.Context, c portalCredentials) PortalCheck {
		calls++
		if c.Username != "qa-user" || c.Password != "qa-portal-secret" {
			t.Error("vault not used")
		}
		return portalCheck("authenticated")
	}
	checked, err := fiscal.TestPortalAccess(ctx, account, user, p.UpdatedAt)
	if err != nil || checked.Status != "authenticated" || checked.CheckedAt == "" || calls != 1 {
		t.Fatal("portal check failed", err)
	}
	checked, err = fiscal.TestPortalAccess(ctx, account, user, p.UpdatedAt)
	if err != nil || checked.Status != "authenticated" || calls != 1 {
		t.Fatal("immediate test repeated credential submission")
	}
	for _, scope := range [][2]string{{other, user}, {account, otherUser}} {
		hidden, e := chat.List(ctx, scope[0], scope[1])
		if e != nil || len(hidden.Messages) != 0 {
			t.Fatal("conversation leaked")
		}
		status, e := fiscal.PortalStatus(ctx, scope[0], scope[1])
		if e != nil || status.Status != "not_tested" {
			t.Fatal("portal check leaked")
		}
	}
	p, err = fiscal.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: p.Profile, ExpectedUpdatedAt: p.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	status, err := fiscal.PortalStatus(ctx, account, user)
	if err != nil || status.Status != "not_tested" {
		t.Fatal("old test reported against new profile")
	}
	if _, err = fiscal.TestPortalAccess(ctx, account, user, checked.ProfileVersion); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("stale portal request accepted")
	}
}
