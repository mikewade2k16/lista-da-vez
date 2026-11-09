package finance

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// CT-15 pacote 6: "lançar comprovante" in the chat proposes an entry and only
// writes after "sim". The proposal pins document and sheet versions (CAS) in the
// server-side action; scope and kind come from the user's words, default pessoal/saída.

var (
	receiptIncomeRE   = regexp.MustCompile(`\b(entrada|recebimento|recebi|receita)\b`)
	receiptExpenseRE  = regexp.MustCompile(`\b(saida|despesa|gasto|pagamento)\b`)
	receiptBusinessRE = regexp.MustCompile(`\b(empresa|empresarial|trabalho|negocio|cnpj)\b`)
	receiptPersonalRE = regexp.MustCompile(`\b(pessoal|particular)\b`)
	receiptYesRE      = regexp.MustCompile(`^(sim|s|confirmo|confirmar|confirma|pode|pode lancar|ok|isso|isso mesmo|lanca|lancar)$`)
)

type receiptProposal struct {
	DocumentID, DocumentVersion, SheetID, SheetVersion, LineID, Kind, Scope string
}

func (p receiptProposal) action() string {
	return "receipt:" + strings.Join([]string{p.DocumentID, p.DocumentVersion, p.SheetID, p.SheetVersion, p.LineID, p.Kind, p.Scope}, "|")
}

func parseReceiptProposal(action string) (receiptProposal, bool) {
	raw, ok := strings.CutPrefix(action, "receipt:")
	parts := strings.Split(raw, "|")
	if !ok || len(parts) != 7 {
		return receiptProposal{}, false
	}
	return receiptProposal{parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], parts[6]}, true
}

// receiptChoices reads kind/scope only from what the user wrote now or confirmed before.
func receiptChoices(key string, previous receiptProposal) (string, string) {
	kind, scope := previous.Kind, previous.Scope
	if kind == "" {
		kind = "saida"
	}
	if scope == "" {
		scope = "pessoal"
	}
	switch {
	case receiptIncomeRE.MatchString(key):
		kind = "entrada"
	case receiptExpenseRE.MatchString(key):
		kind = "saida"
	}
	switch {
	case receiptBusinessRE.MatchString(key):
		scope = "empresarial"
	case receiptPersonalRE.MatchString(key):
		scope = "pessoal"
	}
	return kind, scope
}

func (s *AccessConversationService) proposeReceipt(ctx context.Context, account, user, draftID, text string, in AccessConversationInput, previous receiptProposal) (string, string, error) {
	if draftID == "" || s.documents == nil {
		return "Anexe o comprovante primeiro; depois peça “lançar comprovante”.", "", nil
	}
	d, err := s.documents.Get(ctx, account, user, draftID)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(d.ExtractedText) == "" {
		return "O rascunho atual é uma nota preparada no chat, não um comprovante. Anexe o comprovante e peça “lançar comprovante”.", "", nil
	}
	if link, e := s.documents.GetReceiptLink(ctx, account, user, d.ID); e == nil {
		return "Este comprovante já está lançado (" + link.Kind + " de R$ " + conversationMoney(link.Amount) + "). Não vou lançar de novo.", "", nil
	} else if !errors.Is(e, ErrFiscalNotFound) {
		return "", "", e
	}
	amount, err := receiptAmount(d)
	if err != nil {
		return "Antes de lançar, confirme o valor do comprovante (por exemplo “valor: 32,50”).", "", nil
	}
	if s.orchestrator == nil || s.orchestrator.plus == nil || s.orchestrator.plus.ports.Authorize == nil {
		return "A consulta de planilhas não está disponível neste ambiente.", "", nil
	}
	if err = s.orchestrator.plus.ports.Authorize(ctx, PlusScope{AccountID: account, ActorID: user, RequestID: in.RequestID}, "finance.sheets.view"); err != nil {
		return "", "", err
	}
	period := in.Month
	if len(d.Fields.PaymentDate) >= 7 {
		period = d.Fields.PaymentDate[:7]
	}
	choice, err := s.orchestrator.resolveChatSheet(ctx, account, in.FinanceSheetID, in.ChatSheetID, period)
	if err != nil {
		return "", "", err
	}
	if choice.Reply != "" {
		return choice.Reply + "\nDepois de escolher a planilha, peça “lançar comprovante” de novo.", choice.Action, nil
	}
	sheet, err := s.orchestrator.plus.assistant.finance.GetSheet(ctx, account, choice.SheetID)
	if err != nil {
		return "", "", err
	}
	kind, scope := receiptChoices(conversationKey(text), previous)
	p := receiptProposal{DocumentID: d.ID, DocumentVersion: d.UpdatedAt, SheetID: sheet.ID, SheetVersion: sheet.UpdatedAt, LineID: "new", Kind: kind, Scope: scope}
	lines := sheet.Saidas
	if kind == "entrada" {
		lines = sheet.Entradas
	}
	matches := []Line{}
	for _, l := range lines {
		if fmt.Sprintf("%.2f", l.Amount) == amount {
			matches = append(matches, l)
		}
	}
	target := "como novo lançamento “" + receiptDescription(d, "") + "”"
	if len(matches) == 1 {
		p.LineID = matches[0].ID
		target = "vinculando ao lançamento existente “" + assistantText(matches[0].Description, 80) + "”"
	}
	kindLabel := map[string]string{"saida": "saída", "entrada": "entrada"}[kind]
	when := ""
	if d.Fields.PaymentDate != "" {
		if t, e := time.Parse("2006-01-02", d.Fields.PaymentDate); e == nil {
			when = ", efetivado em " + t.Format("02/01/2006")
		}
	}
	reply := fmt.Sprintf("Vou lançar R$ %s como %s %s na planilha %s%s, %s. Confirma? Responda “sim”. Também pode dizer “empresarial”, “pessoal”, “entrada”, “saída” ou “cancelar”. Nenhuma nota fiscal é emitida a partir do comprovante.",
		conversationMoney(amount), kindLabel, scope, chatSheetLabel(SheetListItem{Title: sheet.Title, Period: sheet.Period, ClientName: sheet.ClientName}), when, target)
	return reply, p.action(), nil
}

// answerReceiptProposal returns handled=false when the message is not about the proposal.
func (s *AccessConversationService) answerReceiptProposal(ctx context.Context, account, user, previousAction, text string, in AccessConversationInput) (string, string, bool, error) {
	p, ok := parseReceiptProposal(previousAction)
	if !ok {
		return "", "", false, nil
	}
	key := conversationKey(text)
	if !receiptYesRE.MatchString(key) {
		if receiptIncomeRE.MatchString(key) || receiptExpenseRE.MatchString(key) || receiptBusinessRE.MatchString(key) || receiptPersonalRE.MatchString(key) {
			answer, action, err := s.proposeReceipt(ctx, account, user, p.DocumentID, text, in, p)
			return answer, action, true, err
		}
		return "", "", false, nil
	}
	sc := PlusScope{AccountID: account, ActorID: user, RequestID: in.RequestID}
	if err := s.orchestrator.plus.ports.Authorize(ctx, sc, "finance.sheets.manage"); err != nil {
		return "Você não tem permissão para editar planilhas nesta conta. Nada foi lançado.", "", true, nil
	}
	line := p.LineID
	if line == "new" {
		line = ""
	}
	link, err := s.documents.LinkReceipt(ctx, account, user, p.DocumentID, ReceiptLinkInput{SheetID: p.SheetID, ExistingLineID: line, Kind: p.Kind, Scope: p.Scope, Confirmed: true, ExpectedDocumentUpdatedAt: p.DocumentVersion, ExpectedSheetUpdatedAt: p.SheetVersion})
	switch {
	case errors.Is(err, ErrSheetConflict), errors.Is(err, ErrFiscalConflict):
		return "A planilha ou o comprovante mudou desde a proposta. Nada foi lançado; peça “lançar comprovante” para revisar de novo.", "", true, nil
	case errors.Is(err, ErrSheetClosed):
		return "A planilha está fechada. Nada foi lançado; reabra a planilha ou escolha outra.", "", true, nil
	case errors.Is(err, ErrReceiptAlreadyLinked):
		return "Este comprovante já está vinculado a outro lançamento. Nada foi alterado.", "", true, nil
	case err != nil:
		return "", "", true, err
	}
	verb := "Vinculei o comprovante ao lançamento existente"
	if link.CreatedLine {
		verb = "Lancei o comprovante como novo lançamento"
	}
	return fmt.Sprintf("%s: %s %s de R$ %s. Confira na planilha.", verb, map[string]string{"saida": "saída", "entrada": "entrada"}[link.Kind], link.Scope, conversationMoney(link.Amount)), "sheet:" + link.SheetID, true, nil
}
