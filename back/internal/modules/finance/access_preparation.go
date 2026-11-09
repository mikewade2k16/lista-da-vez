package finance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func wantsDocumentPreparation(n string) bool {
	for _, prefix := range []string{"preparar", "prepare", "criar nota", "quero criar nota", "gerar nota", "quero gerar", "quero emitir", "revisar nota", "revisar dados", "emitir nota"} {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}

func conversationCommand(text string) bool {
	n := normalizedDocumentLabel(text)
	if n == "cancelar" || n == "parar" || wantsDocumentPreparation(n) || conversationRead(text) || interpretConversation(text, "").Kind != "" {
		return true
	}
	for _, prefix := range []string{"abrir ", "quero ", "testar ", "anexar ", "cadastrar acesso", "usar gov", "usar cpf"} {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}

// Every step is derived from authoritative saved values, never receipt-role inference.
func nextPreparationQuestion(review DocumentReview) (string, string) {
	f := review.Fields
	for _, step := range []struct{ key, value, question string }{
		{"Tomador", f.CustomerName, "Para qual cliente vamos preparar a nota?"},
		{"CPF/CNPJ do tomador", f.CustomerDocument, "Qual é o CPF ou CNPJ desse cliente?"},
		{"Valor", amountPresent(f.Amount), "Qual é o valor do serviço?"},
		{"Competência", f.CompetenceDate, "Qual é a data de competência do serviço? Pode responder dd/mm/aaaa."},
		{"Serviço", f.Service.ServiceDescription, "Qual é a descrição do serviço prestado?"},
	} {
		if step.value == "" {
			return "ask:" + step.key, step.question
		}
	}
	if len(review.MissingFields) > 0 {
		field := strings.TrimPrefix(review.MissingFields[0], "Emitente: ")
		if field != review.MissingFields[0] {
			return "issuer:" + field, "Para completar seu cadastro de emitente, qual é o campo “" + field + "”?"
		}
		return "ask:CPF/CNPJ do tomador", "Precisamos conferir um dado: " + review.MissingFields[0] + ". Qual é o valor correto?"
	}
	return "", fmt.Sprintf("Dados preparados: cliente %s; valor R$ %.2f; competência %s; serviço %s. Nenhuma nota foi emitida. Você pode pedir abrir formulário da nota para conferir a revisão.", f.CustomerName, *f.Amount, f.CompetenceDate, f.Service.ServiceDescription)
}
func amountPresent(amount *float64) string {
	if amount == nil || *amount <= 0 {
		return ""
	}
	return "set"
}

func conversationAsking(previousAction string) bool {
	return strings.HasPrefix(previousAction, "ask:") || strings.HasPrefix(previousAction, "issuer:") || strings.HasPrefix(previousAction, "pick:")
}

// original is the user's text before canonical rewriting; explicitDraft means the
// client selected the draft (e.g. a receipt) instead of inheriting the chat's one.
func (s *AccessConversationService) prepareConversation(ctx context.Context, account, user, id, text, original, answer, action, previousAction, requestID string, explicitDraft bool) (string, string, string, error) {
	n := normalizedDocumentLabel(text)
	if publicKnowledgeQuestion(text) {
		return answer, "", id, nil
	}
	if n == "cancelar" || n == "parar" {
		return answer, "", id, nil
	}
	asking := conversationAsking(previousAction)
	if action == "prepare" && id != "" {
		// A receipt entered as an expense is never a note source, even when selected.
		// A finished inherited draft stays as it is; a new request starts another note.
		reset := s.linkedAsExpense(ctx, account, user, id)
		if !reset && !explicitDraft {
			next, _, err := s.pendingQuestion(ctx, account, user, id)
			if err != nil {
				return "", "", id, err
			}
			reset = next == ""
		}
		if reset {
			id = ""
		}
	}
	if id == "" && action == "prepare" {
		hash := sha256.Sum256([]byte("finance-chat-draft:" + requestID))
		d, err := s.documents.store.Create(ctx, account, user, hex.EncodeToString(hash[:]), "Nota preparada no chat", "", DocumentFields{DocumentType: "documento"})
		if err != nil {
			return "", "", "", err
		}
		id = d.ID
	}
	if id == "" {
		return answer, action, id, nil
	}
	if action == "prepare" {
		if err := s.fillRequestDetails(ctx, account, user, id, original); err != nil {
			return "", "", id, err
		}
	}
	if asking && action == "" && strings.HasPrefix(previousAction, "pick:") {
		picked, question, err := s.pickCustomer(ctx, account, user, id, text)
		if err != nil || !picked {
			return question, previousAction, id, err
		}
	} else if asking && action == "" && !hasExplicitPreparationField(text) {
		if n == "nao sei" || n == "desconhecido" {
			return "Tudo bem, esse dado continua pendente. Você pode informar depois ou pedir cancelar.", previousAction, id, nil
		}
		if strings.HasPrefix(previousAction, "issuer:") {
			if err := s.saveConversationIssuerField(ctx, account, user, strings.TrimPrefix(previousAction, "issuer:"), text); err != nil {
				return "", "", id, err
			}
		} else {
			text = strings.TrimPrefix(previousAction, "ask:") + ": " + text
		}
	}
	continuing := asking && action == ""
	if strings.Contains(text, ":") && action == "" {
		_, changed, err := s.documents.UpdateFromConversation(ctx, account, user, id, text)
		if err != nil {
			return "", "", id, err
		}
		continuing = continuing || changed
	}
	if action != "prepare" && !continuing {
		return answer, action, id, nil
	}
	d, err := s.documents.Get(ctx, account, user, id)
	if err != nil {
		return "", "", id, err
	}
	d, note, choice, err := s.applySavedCustomer(ctx, account, user, d)
	if err != nil {
		return "", "", id, err
	}
	if choice != "" {
		return choice, "pick:Tomador", id, nil
	}
	// Applying an existing default template is local; preparing the conversation never logs into a portal.
	if _, err = s.documents.applyPreparationTemplate(ctx, account, user, d); err != nil {
		return "", "", id, err
	}
	review, err := s.documents.Review(ctx, account, user, id)
	if err != nil {
		return "", "", id, err
	}
	next, question := nextPreparationQuestion(review)
	if next == "" && review.Ready {
		saved, e := s.rememberCustomer(ctx, account, user, review, id)
		if e != nil {
			return "", "", id, e
		}
		question += saved
	}
	return note + question, next, id, nil
}

func hasExplicitPreparationField(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		key, _, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch normalizedDocumentLabel(key) {
		case "tomador", "nome do tomador", "cpf/cnpj do tomador", "documento do tomador", "competencia", "competencia do servico", "servico", "descricao do servico", "valor", "valor do servico", "tomador pagador", "tomador recebedor", "template":
			return true
		}
	}
	return false
}

func conversationRead(text string) bool {
	if publicKnowledgeQuestion(text) {
		return true
	}
	action, reason := routePlus(PlusInvocation{Question: text})
	if reason == "explicit_period_required" {
		return true
	}
	return action != nil && (action.ToolID == "finance.sheet.read.v1" || action.ToolID == "finance.due_dates.read.v1" || action.ToolID == "knowledge.search.v1" || action.ToolID == "plus.explanation.v1")
}

// linkedAsExpense: a receipt already entered as an expense is not a service note source.
func (s *AccessConversationService) linkedAsExpense(ctx context.Context, account, user, id string) bool {
	link, err := s.documents.GetReceiptLink(ctx, account, user, id)
	return err == nil && link.Kind == "saida"
}

// pendingQuestion recomputes the next missing field from the saved draft only.
func (s *AccessConversationService) pendingQuestion(ctx context.Context, account, user, draftID string) (string, string, error) {
	review, err := s.documents.Review(ctx, account, user, draftID)
	if err != nil {
		return "", "", err
	}
	next, question := nextPreparationQuestion(review)
	return next, question, nil
}

func (s *AccessConversationService) resumeConversation(ctx context.Context, account, user, draftID string) (string, string, error) {
	if s.documents == nil || draftID == "" {
		return "Não encontrei uma nota em andamento. Para começar, diga “criar nota”.", "", nil
	}
	next, question, err := s.pendingQuestion(ctx, account, user, draftID)
	return question, next, err
}
