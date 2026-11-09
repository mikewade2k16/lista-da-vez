package finance

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CT-15 pacote 4: explicit request details and recurring customers in the chat.
// Only values the user typed or saved before are used; nothing is inferred.

var (
	requestAmountRE   = regexp.MustCompile(`(?i)(?:r\$\s*|\bde\s+(?:r\$\s*)?|\bvalor\s+(?:de\s+)?(?:r\$\s*)?)(\d{1,3}(?:\.\d{3})+(?:,\d{1,2})?|\d+(?:,\d{1,2})?)`)
	requestCustomerRE = regexp.MustCompile(`(?i)\b(?:pro|pra|para o|para a|para)\s+(?:(?:o|a)\s+)?(?:cliente\s+)?(.+?)\s*(?:$|[,.;!?]|\s+(?:de|no valor|valor|por|referente|com)\b)`)
	requestThousands  = regexp.MustCompile(`^\d{1,3}(?:\.\d{3})+$`)
)

var requestCustomerStops = map[string]bool{"mim": true, "eu": true, "voce": true, "nos": true, "a gente": true, "gente": true, "hoje": true, "amanha": true, "ontem": true, "agora": true, "depois": true, "ele": true, "ela": true, "isso": true, "esse": true, "essa": true, "este": true, "esta": true, "mes": true, "semana": true, "mim mesmo": true, "revisar": true, "revisao": true}

// extractNoteRequest returns the customer name and amount explicitly written in
// a preparation request such as "nota de 500 pro João".
func extractNoteRequest(text string) (string, string) {
	customer, amount := "", ""
	if m := requestAmountRE.FindStringSubmatchIndex(text); m != nil {
		next := m[3]
		if next >= len(text) || !strings.ContainsRune("/-%", rune(text[next])) {
			amount = text[m[2]:m[3]]
			if requestThousands.MatchString(amount) {
				amount = strings.ReplaceAll(amount, ".", "")
			}
		}
	}
	if m := requestCustomerRE.FindStringSubmatch(text); m != nil {
		name := strings.TrimSpace(m[1])
		key := normalizedDocumentLabel(name)
		_, numeric := strconv.Atoi(strings.ReplaceAll(key, " ", ""))
		if !requestCustomerStops[key] && numeric != nil && len(name) <= 120 && !strings.HasPrefix(key, "nota") && !strings.HasPrefix(key, "emitir") {
			customer = name
		}
	}
	return customer, amount
}

// fillRequestDetails writes only fields that are still empty in the draft.
func (s *AccessConversationService) fillRequestDetails(ctx context.Context, account, user, id, original string) error {
	customer, amount := extractNoteRequest(original)
	if customer == "" && amount == "" {
		return nil
	}
	d, err := s.documents.Get(ctx, account, user, id)
	if err != nil {
		return err
	}
	lines := []string{}
	if customer != "" && d.Fields.CustomerName == "" {
		lines = append(lines, "Tomador: "+customer)
	}
	if amount != "" && d.Fields.Amount == nil {
		lines = append(lines, "Valor: "+amount)
	}
	if len(lines) == 0 {
		return nil
	}
	_, _, err = s.documents.UpdateFromConversation(ctx, account, user, id, strings.Join(lines, "\n"))
	return err
}

func documentTail(document string) string {
	if len(document) <= 4 {
		return document
	}
	return "final " + document[len(document)-4:]
}

func customerChoiceQuestion(matches []FiscalCustomer) string {
	out := "Encontrei mais de um cliente salvo com esse nome:"
	for i, c := range matches {
		out += fmt.Sprintf("\n%d) %s — CPF/CNPJ %s", i+1, c.Name, documentTail(c.Document))
	}
	return out + "\nQual deles? Responda o número ou informe o CPF/CNPJ. Se for um cliente novo, informe o CPF/CNPJ dele."
}

// applySavedCustomer reuses the saved customer when the draft identifies exactly
// one. It returns the updated draft, a note of what was reused, or a choice question.
func (s *AccessConversationService) applySavedCustomer(ctx context.Context, account, user string, d DocumentDraft) (DocumentDraft, string, string, error) {
	f := d.Fields
	var match FiscalCustomer
	switch {
	case f.CustomerDocument != "":
		found, err := s.documents.customers.FindByDocument(ctx, account, user, f.CustomerDocument)
		if errors.Is(err, ErrFiscalNotFound) {
			return d, "", "", nil
		}
		if err != nil {
			return d, "", "", err
		}
		match = found
	case f.CustomerName != "":
		matches, err := s.documents.customers.FindByName(ctx, account, user, f.CustomerName)
		if err != nil {
			return d, "", "", err
		}
		if len(matches) > 1 {
			return d, "", customerChoiceQuestion(matches), nil
		}
		if len(matches) == 0 {
			return d, "", "", nil
		}
		match = matches[0]
		f.CustomerName = match.Name
		f.CustomerDocument = match.Document
	default:
		return d, "", "", nil
	}
	reused := []string{}
	if d.Fields.CustomerDocument == "" {
		reused = append(reused, "CPF/CNPJ "+documentTail(match.Document))
	}
	if serviceEmpty(f.Service) && f.TemplateID == "" {
		if match.TemplateID != "" {
			templates, err := s.fiscal.ListTemplates(ctx, account, user)
			if err != nil {
				return d, "", "", err
			}
			for _, t := range templates {
				if t.ID == match.TemplateID {
					f.TemplateID, f.Service = t.ID, t.Fields
				}
			}
		}
		if match.ServiceDescription != "" {
			f.Service.ServiceDescription = match.ServiceDescription
			reused = append(reused, "serviço “"+assistantText(match.ServiceDescription, 80)+"”")
		}
	}
	if f.Amount == nil && match.Amount != nil {
		f.Amount = match.Amount
		reused = append(reused, "valor R$ "+conversationMoney(fmt.Sprintf("%.2f", *match.Amount)))
	}
	if len(reused) == 0 {
		return d, "", "", nil
	}
	saved, err := s.documents.Save(ctx, account, user, d.ID, DocumentDraftInput{Fields: f, ExpectedUpdatedAt: d.UpdatedAt})
	if err != nil {
		return d, "", "", err
	}
	note := "Usei o cadastro salvo de " + match.Name + ": " + strings.Join(reused, ", ") + ". Se algo mudou, me diga, por exemplo “valor: 600”.\n\n"
	return saved, note, "", nil
}

func serviceEmpty(f FiscalTemplateFields) bool {
	return f.ServiceDescription == "" && f.ServiceCode == "" && f.CNAE == "" && f.NBS == "" && f.ISSRate == nil && f.ISSWithheld == nil && f.Notes == ""
}

// pickCustomer resolves the numbered choice or an explicit CPF/CNPJ answer.
func (s *AccessConversationService) pickCustomer(ctx context.Context, account, user, id, text string) (bool, string, error) {
	d, err := s.documents.Get(ctx, account, user, id)
	if err != nil {
		return false, "", err
	}
	matches, err := s.documents.customers.FindByName(ctx, account, user, d.Fields.CustomerName)
	if err != nil {
		return false, "", err
	}
	f := d.Fields
	if n, e := strconv.Atoi(strings.TrimSpace(text)); e == nil && n >= 1 && n <= len(matches) {
		f.CustomerName, f.CustomerDocument = matches[n-1].Name, matches[n-1].Document
	} else if document := normalizeCustomerDocument(text); fiscalDocumentPattern.MatchString(document) {
		f.CustomerDocument = document
	} else {
		return false, customerChoiceQuestion(matches), nil
	}
	_, err = s.documents.Save(ctx, account, user, id, DocumentDraftInput{Fields: f, ExpectedUpdatedAt: d.UpdatedAt})
	return err == nil, "", err
}

// rememberCustomer saves the customer of a complete draft for later notes.
func (s *AccessConversationService) rememberCustomer(ctx context.Context, account, user string, review DocumentReview, draftID string) (string, error) {
	saved, state, err := s.documents.customers.Remember(ctx, account, user, review.Fields, draftID)
	switch {
	case err != nil:
		return "", err
	case state == "created":
		return " Salvei " + saved.Name + " para as próximas notas.", nil
	case state == "updated":
		return fmt.Sprintf(" Atualizei o cadastro de %s (versão %d).", saved.Name, saved.Version), nil
	}
	return "", nil
}
