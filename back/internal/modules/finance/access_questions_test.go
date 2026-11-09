package finance

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestConversationQuestionsUseSavedValues(t *testing.T) {
	amount := 85.5
	review := DocumentReview{Fields: DocumentFields{Amount: &amount, Service: FiscalTemplateFields{ServiceDescription: "Serviço salvo"}}}
	action, question := nextPreparationQuestion(review)
	if action != "ask:Tomador" || strings.Contains(question, "valor") {
		t.Fatal(action, question)
	}
	review.Fields.CustomerName = "Cliente Nota QA"
	action, _ = nextPreparationQuestion(review)
	if action != "ask:CPF/CNPJ do tomador" {
		t.Fatal(action)
	}
	if !wantsDocumentPreparation("criar nota") || conversationCommand("Cliente Nota QA") {
		t.Fatal("intent consumed an ordinary answer")
	}
}

func TestConversationQuestionPersistenceAndOrdinaryMessages(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	account := financeTestAccount(t, pool)
	var user string
	if err := pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `delete from core.users where id=$1::uuid`, user) })
	fiscal := NewFiscalService(NewPostgresFiscalStore(pool), nil)
	documents := newDocumentService(&DocumentStore{pool: pool}, fiscal)
	chat := &AccessConversationService{pool: pool, fiscal: fiscal, documents: documents}
	send := func(id, text string) AccessMessage {
		t.Helper()
		out, err := chat.Send(ctx, account, user, AccessConversationInput{RequestID: id, Text: text})
		if err != nil {
			t.Fatal(err)
		}
		if out.Messages[len(out.Messages)-2].Text != text {
			t.Fatal("ordinary user message hidden")
		}
		return out.Messages[len(out.Messages)-1]
	}
	send("00000000-0000-4000-8000-000000000091", "abrir acesso")
	send("00000000-0000-4000-8000-000000000092", "olá")
	first := send("00000000-0000-4000-8000-000000000093", "criar nota")
	if first.Action != "ask:Tomador" || first.DraftID == "" {
		t.Fatal("form opened instead of question", first)
	}
	// Rebuild service and retry the original request: no second draft or turn.
	chat = &AccessConversationService{pool: pool, fiscal: fiscal, documents: documents}
	replay := send("00000000-0000-4000-8000-000000000093", "criar nota")
	if replay.ID != first.ID {
		t.Fatal("duplicate turn")
	}
	second := send("00000000-0000-4000-8000-000000000094", "Cliente Nota QA")
	if second.Action != "ask:CPF/CNPJ do tomador" {
		t.Fatal(second)
	}
	third := send("00000000-0000-4000-8000-000000000095", "98765432000198")
	if third.Action != "ask:Valor" {
		t.Fatal(third)
	}
	fourth := send("00000000-0000-4000-8000-000000000096", "R$ 85,50")
	if fourth.Action != "ask:Competência" {
		t.Fatal(fourth)
	}
	saved, err := documents.Get(ctx, account, user, first.DraftID)
	if err != nil || saved.Fields.CustomerName != "Cliente Nota QA" || saved.Fields.Amount == nil || *saved.Fields.Amount != 85.5 {
		t.Fatal("answers not saved", err)
	}
	fifth := send("00000000-0000-4000-8000-000000000097", "06/11/2026")
	if fifth.Action != "ask:Serviço" {
		t.Fatal(fifth)
	}
	sixth := send("00000000-0000-4000-8000-000000000098", "Edição de vídeo")
	if sixth.Action != "issuer:Nome/razão social" {
		t.Fatal(sixth)
	}
	seventh := send("00000000-0000-4000-8000-000000000099", "Emitente QA")
	if seventh.Action != "issuer:CPF/CNPJ" {
		t.Fatal(seventh)
	}
	p, err := fiscal.GetProfile(ctx, account, user)
	if err != nil || p.Profile.LegalName != "Emitente QA" {
		t.Fatal("profile answer not persisted", err)
	}
	send("00000000-0000-4000-8000-000000000100", "cancelar")
	list, err := documents.List(ctx, account, user)
	if err != nil || len(list) != 1 {
		t.Fatal("retry created another draft", err)
	}
}

func TestConversationDoesNotOpenFormsForInformationalQuestions(t *testing.T) {
	for _, text := range []string{"o que é uma nota?", "como funciona o comprovante?", "onde fica meu perfil?", "qual a diferença do gov.br?"} {
		_, action, method := accessReply(text, FiscalProfileView{})
		if action != "" || method != "" {
			t.Fatalf("unsolicited editor for %q: %s", text, action)
		}
	}
	_, action, _ := accessReply("abrir perfil", FiscalProfileView{})
	if action != "profile" {
		t.Fatal("explicit editor unavailable")
	}
}

// CT-15 pacote 3: side requests, cancel, resume and composed requests keep the pending field intact.
func TestConversationContinuityAcrossSideRequests(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	account := financeTestAccount(t, pool)
	var user string
	if err := pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `delete from core.users where id=$1::uuid`, user) })
	fiscal := NewFiscalService(NewPostgresFiscalStore(pool), nil)
	documents := newDocumentService(&DocumentStore{pool: pool}, fiscal)
	chat := &AccessConversationService{pool: pool, fiscal: fiscal, documents: documents}
	n := 200
	send := func(text string) AccessMessage {
		t.Helper()
		n++
		out, err := chat.Send(ctx, account, user, AccessConversationInput{RequestID: fmt.Sprintf("00000000-0000-4000-8000-%012d", n), Text: text})
		if err != nil {
			t.Fatal(text, err)
		}
		return out.Messages[len(out.Messages)-1]
	}
	first := send("faz uma nota pra mim")
	if first.Action != "ask:Tomador" || first.DraftID == "" {
		t.Fatal("natural preparation not recognized", first)
	}
	side := send("quanto eu gastei esse mês?")
	if side.Action != "ask:Tomador" || !strings.Contains(side.Text, "seguimos com a nota") {
		t.Fatal("side request dropped the pending field", side)
	}
	multi := send("mostra meu saldo e depois cadastra outra empresa")
	if multi.Action != "ask:Tomador" || !strings.Contains(multi.Text, "mais de um pedido") {
		t.Fatal("composed request not clarified", multi)
	}
	saved, err := documents.Get(ctx, account, user, first.DraftID)
	if err != nil || saved.Fields.CustomerName != "" {
		t.Fatal("command saved as customer name", saved.Fields.CustomerName, err)
	}
	answer := send("Cliente Continuidade QA")
	if answer.Action != "ask:CPF/CNPJ do tomador" {
		t.Fatal(answer)
	}
	cancel := send("deixa pra lá")
	if cancel.Action != "" || cancel.DraftID != first.DraftID {
		t.Fatal("cancel lost the saved draft", cancel)
	}
	resumed := send("vamos continuar")
	if resumed.Action != "ask:CPF/CNPJ do tomador" || resumed.DraftID != first.DraftID {
		t.Fatal("resume did not re-ask the pending field", resumed)
	}
	saved, err = documents.Get(ctx, account, user, first.DraftID)
	if err != nil || saved.Fields.CustomerName != "Cliente Continuidade QA" {
		t.Fatal("answer not kept after cancel/resume", err)
	}
	list, err := documents.List(ctx, account, user)
	if err != nil || len(list) != 1 {
		t.Fatal("side requests created another draft", err)
	}
}
