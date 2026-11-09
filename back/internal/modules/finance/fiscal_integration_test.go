package finance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

func TestFiscalProfileIsolationEncryptionAndRevision(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `create table if not exists core.users(id uuid primary key default gen_random_uuid())`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../platform/database/migrations/0304_finance_user_fiscal_profile.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = pool.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	account := financeTestAccount(t, pool)
	otherAccount := financeTestAccount(t, pool)
	var user, otherUser string
	if err = pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&otherUser); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, e := pool.Exec(ctx, `delete from core.users where id=any($1::uuid[])`, []string{user, otherUser}); e != nil {
			t.Error(e)
		}
	})
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	store := NewPostgresFiscalStore(pool)
	svc := NewFiscalService(store, box)
	view, err := svc.GetProfile(ctx, account, user)
	if err != nil || view.UpdatedAt != "" || len(view.MissingFields) == 0 {
		t.Fatalf("empty: %v %+v", err, view)
	}
	in := FiscalProfileInput{Profile: FiscalProfile{LegalName: "QA Fiscal", Document: "12.345.678/0001-95", IssuerType: "mei", PortalURL: "https://www.nfse.gov.br/EmissorNacional", Municipality: "Maceió", State: "al"}}
	saved, err := svc.SaveProfile(ctx, account, user, in)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Profile.Document != "12345678000195" || saved.Profile.State != "AL" || saved.UpdatedAt == "" {
		t.Fatal("normalization/revision missing")
	}
	if _, err = svc.SaveProfile(ctx, account, user, in); !errors.Is(err, ErrFiscalConflict) {
		t.Fatalf("stale update: %v", err)
	}
	for _, scope := range [][2]string{{account, otherUser}, {otherAccount, user}} {
		hidden, e := svc.GetProfile(ctx, scope[0], scope[1])
		if e != nil || hidden.UpdatedAt != "" || hidden.CredentialsSet {
			t.Fatalf("scope leak: %v", e)
		}
	}
	credential := FiscalCredentialInput{Username: "qa-login", Password: "qa-fiscal-secret-only", ExpectedUpdatedAt: saved.UpdatedAt}
	saved, err = svc.SaveCredentials(ctx, account, user, credential)
	if err != nil || !saved.CredentialsSet {
		t.Fatalf("credential: %v", err)
	}
	var ciphertext string
	if err = pool.QueryRow(ctx, `select ciphertext from finance.user_fiscal_credentials where account_id=$1::uuid and user_id=$2::uuid`, account, user).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ciphertext, credential.Password) || strings.Contains(ciphertext, credential.Username) || !strings.HasPrefix(ciphertext, "v1:") {
		t.Fatal("credential not encrypted")
	}
	raw, e := box.Decrypt(ciphertext)
	if e != nil || !strings.Contains(raw, credential.Password) {
		t.Fatal("ciphertext cannot be recovered by vault")
	}
	public, e := json.Marshal(saved)
	if e != nil || strings.Contains(string(public), credential.Password) || strings.Contains(string(public), "ciphertext") || strings.Contains(string(public), credential.Username) {
		t.Fatal("credential exposed in read model")
	}
	in.ExpectedUpdatedAt = saved.UpdatedAt
	in.Profile.LegalName = "QA Changed"
	saved, err = svc.SaveProfile(ctx, account, user, in)
	if err != nil || !saved.CredentialsSet {
		t.Fatal("ordinary profile save removed credential")
	}
	in.ExpectedUpdatedAt = saved.UpdatedAt
	in.Profile.PortalURL = "https://portal.example.com/emissor"
	saved, err = svc.SaveProfile(ctx, account, user, in)
	if err != nil || saved.CredentialsSet {
		t.Fatal("portal change must invalidate old credential")
	}
	credential.ExpectedUpdatedAt = saved.UpdatedAt
	saved, err = svc.SaveCredentials(ctx, account, user, credential)
	if err != nil {
		t.Fatal(err)
	}
	saved, err = svc.DeleteCredentials(ctx, account, user, saved.UpdatedAt)
	if err != nil || saved.CredentialsSet {
		t.Fatal("credential removal")
	}

	template := FiscalTemplateInput{Name: "Serviço QA", Fields: FiscalTemplateFields{ServiceDescription: "Descrição QA"}, IsDefault: true}
	first, err := svc.SaveTemplate(ctx, account, user, "", template)
	if err != nil {
		t.Fatal(err)
	}
	template.Name = "Segundo QA"
	second, err := svc.SaveTemplate(ctx, account, user, "", template)
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListTemplates(ctx, account, user)
	if err != nil || len(list) != 2 || !list[0].IsDefault || list[0].ID != second.ID || list[1].IsDefault {
		t.Fatal("default template not unique")
	}
	if list[0].Fields.ISSRate != nil || list[0].Fields.ISSWithheld != nil {
		t.Fatal("unspecified tax was defaulted")
	}
	for _, scope := range [][2]string{{account, otherUser}, {otherAccount, user}} {
		list, e := svc.ListTemplates(ctx, scope[0], scope[1])
		if e != nil || len(list) != 0 {
			t.Fatal("template list leaked")
		}
		if _, e = svc.SaveTemplate(ctx, scope[0], scope[1], second.ID, template); !errors.Is(e, ErrFiscalNotFound) {
			t.Fatal("foreign template editable")
		}
		if e = svc.DeleteTemplate(ctx, scope[0], scope[1], second.ID, second.UpdatedAt); !errors.Is(e, ErrFiscalNotFound) {
			t.Fatal("foreign template removable")
		}
	}
	template.ExpectedUpdatedAt = first.UpdatedAt
	if _, err = svc.SaveTemplate(ctx, account, user, first.ID, template); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("default replacement must invalidate stale template")
	}
	template.ExpectedUpdatedAt = second.UpdatedAt
	updated, err := svc.SaveTemplate(ctx, account, user, second.ID, template)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.DeleteTemplate(ctx, account, user, second.ID, second.UpdatedAt); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("stale delete allowed")
	}
	if err = svc.DeleteTemplate(ctx, account, user, second.ID, updated.UpdatedAt); err != nil {
		t.Fatal(err)
	}
}

func TestGovBRProfileRejectsStoredPassword(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `create table if not exists core.users(id uuid primary key default gen_random_uuid())`); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../platform/database/migrations/0304_finance_user_fiscal_profile.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	account := financeTestAccount(t, pool)
	var user string
	if err = pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `delete from core.users where id=$1::uuid`, user) })
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewFiscalService(NewPostgresFiscalStore(pool), box)
	view, err := svc.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: FiscalProfile{PortalURL: "https://www.nfse.gov.br/EmissorNacional/Login?ReturnUrl=%2femissornacional", PortalLoginMethod: "govbr"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(view.Profile.PortalURL, "?") {
		t.Fatal("ephemeral portal redirect persisted")
	}
	_, err = svc.SaveCredentials(ctx, account, user, FiscalCredentialInput{Username: "test", Password: "not-a-real-password", ExpectedUpdatedAt: view.UpdatedAt})
	if !errors.Is(err, ErrFiscalInvalid) {
		t.Fatal("gov.br password accepted")
	}
}
