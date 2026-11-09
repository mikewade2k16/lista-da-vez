package finance

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestExtractNoteRequest(t *testing.T) {
	for _, c := range []struct{ text, customer, amount string }{
		{"faz uma nota de 500 pro João", "João", "500"},
		{"nota de R$ 1.500,50 para a Padaria Sol", "Padaria Sol", "1.500,50"},
		{"preparar nota para o cliente Acme Ltda no valor de 1.200", "Acme Ltda", "1200"},
		{"faz uma nota pra mim", "", ""},
		{"criar nota para hoje", "", ""},
		{"criar nota", "", ""},
		{"nota de 15/11 pro Pedro", "Pedro", ""},
		{"emitir nota para Silva e Filhos Ltda", "Silva e Filhos Ltda", ""},
	} {
		customer, amount := extractNoteRequest(c.text)
		if customer != c.customer || amount != c.amount {
			t.Errorf("%q: got %q/%q want %q/%q", c.text, customer, amount, c.customer, c.amount)
		}
	}
}

func completeTestProfile(t *testing.T, ctx context.Context, fiscal *FiscalService, account, user string) {
	t.Helper()
	view, err := fiscal.GetProfile(ctx, account, user)
	if err != nil {
		t.Fatal(err)
	}
	p := FiscalProfile{LegalName: "Emitente Sintético QA", Document: "12345678000195", IssuerType: "mei", TaxRegime: "MEI sintético", Municipality: "Aracaju", State: "SE", Street: "Rua QA", Number: "1", District: "Centro", PostalCode: "49000000", PortalURL: nationalPortalLogin}
	if _, err = fiscal.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: p, ExpectedUpdatedAt: view.UpdatedAt}); err != nil {
		t.Fatal(err)
	}
}

// CT-15 pacote 4: details in the request, new customer saved, recurring reuse,
// versioned update, homonym choice, new draft after completion and isolation.
func TestConversationRecurringCustomers(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	account := financeTestAccount(t, pool)
	newUser := func() string {
		var id string
		if err := pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `delete from core.users where id=$1::uuid`, id) })
		return id
	}
	user, other := newUser(), newUser()
	fiscal := NewFiscalService(NewPostgresFiscalStore(pool), nil)
	documents := newDocumentService(&DocumentStore{pool: pool}, fiscal)
	chat := &AccessConversationService{pool: pool, fiscal: fiscal, documents: documents}
	completeTestProfile(t, ctx, fiscal, account, user)
	completeTestProfile(t, ctx, fiscal, account, other)
	n := 400
	send := func(who, text string) AccessMessage {
		t.Helper()
		n++
		out, err := chat.Send(ctx, account, who, AccessConversationInput{RequestID: fmt.Sprintf("00000000-0000-4000-8000-%012d", n), Text: text})
		if err != nil {
			t.Fatal(text, err)
		}
		return out.Messages[len(out.Messages)-1]
	}
	first := send(user, "faz uma nota de 500 pro João Recorrente")
	if first.Action != "ask:CPF/CNPJ do tomador" {
		t.Fatal("request details not used", first)
	}
	send(user, "123.456.789-09")
	send(user, "06/11/2026")
	done := send(user, "Edição de vídeo")
	if done.Action != "" || !strings.Contains(done.Text, "Salvei João Recorrente") {
		t.Fatal("new customer not saved", done)
	}
	second := send(user, "faz outra nota pro João Recorrente")
	if second.DraftID == first.DraftID {
		t.Fatal("finished draft reused")
	}
	if second.Action != "ask:Competência" || !strings.Contains(second.Text, "Usei o cadastro salvo de João Recorrente") || !strings.Contains(second.Text, "500,00") {
		t.Fatal("recurring customer not reused", second)
	}
	send(user, "07/11/2026")
	d, err := documents.Get(ctx, account, user, second.DraftID)
	if err != nil || d.Fields.CustomerDocument != "12345678909" || d.Fields.Service.ServiceDescription != "Edição de vídeo" {
		t.Fatal("saved customer fields not applied", d.Fields, err)
	}
	third := send(user, "nota de 650 para João Recorrente")
	if third.Action != "ask:Competência" {
		t.Fatal("third note", third)
	}
	third = send(user, "09/11/2026")
	if !strings.Contains(third.Text, "Atualizei o cadastro de João Recorrente (versão 2)") {
		t.Fatal("changed amount not versioned", third)
	}
	customers, err := documents.customers.List(ctx, account, user)
	if err != nil || len(customers) != 1 || *customers[0].Amount != 650 {
		t.Fatal("customer list", customers, err)
	}
	versions, err := documents.customers.Versions(ctx, account, user, customers[0].ID)
	if err != nil || len(versions) != 2 {
		t.Fatal("history", versions, err)
	}
	if _, err = pool.Exec(ctx, `update finance.fiscal_customer_versions set data='{}' where customer_id=$1::uuid`, customers[0].ID); err == nil {
		t.Fatal("customer history is mutable")
	}
	// Homonym: a second João requires an explicit choice.
	send(user, "criar nota de 100 pro João Segundo")
	send(user, "98765432000198")
	send(user, "08/11/2026")
	if saved := send(user, "Consultoria"); !strings.Contains(saved.Text, "Salvei João Segundo") {
		t.Fatal("second customer not saved", saved)
	}
	choice := send(user, "criar nota pro João")
	if choice.Action != "pick:Tomador" || !strings.Contains(choice.Text, "1)") || !strings.Contains(choice.Text, "2)") {
		t.Fatal("ambiguous customer not asked", choice)
	}
	picked := send(user, "2")
	d, err = documents.Get(ctx, account, user, choice.DraftID)
	if err != nil || picked.Action != "ask:Competência" || d.Fields.CustomerDocument == "" {
		t.Fatal("choice not applied", picked, d.Fields, err)
	}
	// Another user in the same account never sees these customers.
	foreign := send(other, "faz uma nota pro João Recorrente")
	if strings.Contains(foreign.Text, "cadastro salvo") || foreign.Action != "ask:CPF/CNPJ do tomador" {
		t.Fatal("customer leaked across users", foreign)
	}
	if list, _ := documents.customers.List(ctx, account, other); len(list) != 0 {
		t.Fatal("foreign list not empty")
	}
	if _, err = documents.customers.Versions(ctx, account, other, customers[0].ID); err != ErrFiscalNotFound {
		t.Fatal("foreign history visible", err)
	}
}
