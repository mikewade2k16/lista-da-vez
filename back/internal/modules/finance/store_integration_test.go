package finance

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func financeTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("FINANCE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("FINANCE_TEST_DATABASE_URL ausente: requer banco descartavel finance_test")
	}
	ctx := context.Background()
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	if !strings.HasPrefix(p.Config().ConnConfig.Database, "finance_test") {
		t.Fatal("testes exigem banco descartavel finance_test*")
	}
	_, err = p.Exec(ctx, `create schema if not exists core;create table if not exists core.accounts(id uuid primary key default gen_random_uuid(),name text not null default '')`)
	if err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile("../../platform/database/migrations/0187_finance_module.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, `create table if not exists core.users(id uuid primary key default gen_random_uuid())`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"0304_finance_user_fiscal_profile.sql", "0306_finance_document_drafts.sql", "0307_finance_document_reviews.sql", "0308_finance_access_conversation.sql", "0309_finance_portal_sessions.sql", "0317_public_knowledge_conversation.sql", "0319_finance_fiscal_customers.sql", "0320_finance_receipt_links.sql", "0322_finance_document_originals.sql", "0323_public_knowledge_curation.sql"} {
		migration, e := os.ReadFile("../../platform/database/migrations/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = p.Exec(ctx, string(migration)); e != nil {
			t.Fatal(e)
		}
	}
	return p
}
func financeTestAccount(t *testing.T, p *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := p.QueryRow(context.Background(), `insert into core.accounts(name) values ('Finance regression') returning id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := p.Exec(context.Background(), `delete from core.accounts where id=$1::uuid`, id); err != nil {
			t.Error(err)
		}
	})
	return id
}
func TestSheetsPersistenceDatesAndScope(t *testing.T) {
	p := financeTestPool(t)
	account := financeTestAccount(t, p)
	other := financeTestAccount(t, p)
	ctx := context.Background()
	svc := NewService(NewPostgresSheetStore(p), NewPostgresConfigStore(p))
	period := "2026-11"
	d, err := svc.CreateSheet(ctx, account, SheetInput{Period: &period, Entradas: []Line{{Amount: 100, Effective: true, EffectiveDate: "2026-11-01", Adjustments: []Adjustment{{Amount: 10, Date: "2026-11-01"}}}}, Saidas: []Line{{Amount: 30}}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Entradas[0].EffectiveDate != "2026-11-01" || d.Entradas[0].Adjustments[0].Date != "2026-11-01" || d.Summary.ExpectedBalance != 80 || d.Summary.EffectiveBalance != 110 {
		t.Fatalf("round-trip: %+v", d)
	}
	if _, err := svc.GetSheet(ctx, other, d.ID); !errors.Is(err, ErrSheetNotFound) {
		t.Fatalf("cross-account: %v", err)
	}
	f := false
	patch, err := svc.PatchLine(ctx, account, d.ID, d.Entradas[0].ID, LinePatchInput{Effective: &f})
	if err != nil {
		t.Fatal(err)
	}
	if patch.Line.EffectiveDate != "" || patch.Summary.EffectiveIn != 0 {
		t.Fatal("des-efetivar nao limpou data/resumo")
	}
	title := "Updated"
	d, err = svc.UpdateSheet(ctx, account, d.ID, SheetInput{Title: &title, Period: &period, Entradas: d.Entradas, Saidas: d.Saidas})
	if err != nil || d.Title != title {
		t.Fatal("update", err)
	}
	if err = svc.DeleteSheet(ctx, account, d.ID); err != nil {
		t.Fatal(err)
	}
}
func TestFixedAccountsKeepAllMembers(t *testing.T) {
	p := financeTestPool(t)
	account := financeTestAccount(t, p)
	svc := NewService(NewPostgresSheetStore(p), NewPostgresConfigStore(p))
	cfg, err := svc.SaveConfig(context.Background(), account, ConfigInput{FixedAccounts: []FixedAccount{{Name: "First", Members: []FixedAccountMember{{Name: "One", Amount: 100}}}, {Name: "Second", Members: []FixedAccountMember{{Name: "Two", Amount: 200}}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range cfg.FixedAccounts {
		if len(a.Members) != 1 {
			t.Errorf("%s perdeu composicao", a.Name)
		}
	}
}

func TestClosedSheetAndConcurrentUpdate(t *testing.T) {
	p := financeTestPool(t)
	account := financeTestAccount(t, p)
	ctx := context.Background()
	svc := NewService(NewPostgresSheetStore(p), NewPostgresConfigStore(p))
	period := "2026-11"
	d, err := svc.CreateSheet(ctx, account, SheetInput{Period: &period, Entradas: []Line{{Amount: 100}}})
	if err != nil {
		t.Fatal(err)
	}
	closed, err := svc.ChangeStatus(ctx, account, d.ID, "fechada", d.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateSheet(ctx, account, d.ID, SheetInput{Entradas: []Line{{Amount: 1}}}); !errors.Is(err, ErrSheetClosed) {
		t.Fatalf("closed update: %v", err)
	}
	effective := true
	if _, err = svc.PatchLine(ctx, account, d.ID, d.Entradas[0].ID, LinePatchInput{Effective: &effective}); !errors.Is(err, ErrSheetClosed) {
		t.Fatalf("closed patch: %v", err)
	}
	if err = svc.DeleteSheet(ctx, account, d.ID); !errors.Is(err, ErrSheetClosed) {
		t.Fatalf("closed delete: %v", err)
	}
	reopened, err := svc.ChangeStatus(ctx, account, d.ID, "aberta", closed.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	title := "First write"
	saved, err := svc.UpdateSheet(ctx, account, d.ID, SheetInput{ExpectedUpdatedAt: reopened.UpdatedAt, Title: &title, Period: &period, Entradas: d.Entradas})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateSheet(ctx, account, d.ID, SheetInput{ExpectedUpdatedAt: reopened.UpdatedAt}); !errors.Is(err, ErrSheetConflict) {
		t.Fatalf("stale update: %v", err)
	}
	reread, err := svc.GetSheet(ctx, account, d.ID)
	if err != nil || reread.Title != saved.Title || reread.Entradas[0].Amount != 100 {
		t.Fatal("snapshot overwritten", err)
	}
}
