package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// The existing personal transcript is authoritative for local and model replies.
// Corporate tools keep their issuer-scoped contract and never inherit a personal scope.
type ConversationOrchestrator struct{ plus *PlusService }

func (o *ConversationOrchestrator) Reply(ctx context.Context, account, user string, in AccessConversationInput) (string, error) {
	text, _, err := o.ReplyWithAction(ctx, account, user, in)
	return text, err
}

// ReplyWithAction also returns the chat state to persist on the assistant message
// ("sheet:<id>" for the sheet chosen in this chat, "sheetpick:<period>" for a choice).
func (o *ConversationOrchestrator) ReplyWithAction(ctx context.Context, account, user string, in AccessConversationInput) (string, string, error) {
	fallback := "Posso consultar a planilha selecionada, preparar uma nota com uma pergunta por vez, abrir seu cadastro ou ler um comprovante. O que você gostaria de fazer?"
	if o == nil || o.plus == nil {
		return fallback, "", nil
	}
	sc := PlusScope{AccountID: account, ActorID: user, RequestID: in.RequestID}
	if o.plus.ports.Authorize == nil {
		return fallback, "", nil
	}
	if err := o.plus.ports.Authorize(ctx, sc, "finance.sheets.view"); err != nil {
		return "", "", err
	}
	cfg, err := o.plus.assistant.Config(ctx, account)
	if err != nil {
		return "", "", err
	}
	if picked, ok, e := o.pickChatSheet(ctx, account, in.ChatAction, in.Text); e != nil || ok {
		if e != nil {
			return "", "", e
		}
		return o.chatSheetReply(ctx, sc, picked, plusSheetInput{})
	}
	month := in.Month
	if in.ChatSheetID != "" {
		// The sheet chosen in this chat wins over the page month for follow-ups.
		month = ""
	}
	invocation := PlusInvocation{Question: in.Text, FinanceSheetID: in.FinanceSheetID, Month: month}
	action, reason := routePlus(invocation)
	switch {
	case action == nil && reason == "explicit_period_required":
		return "Qual mês você quer consultar? Pode responder, por exemplo, “saldo de novembro de 2026” ou “resumo 2026-11”. Um período por vez.", "", nil
	case action == nil && reason == "multiple_intents":
		return multipleRequestsReply(multipleRequests(in.Text, in.Month)), "", nil
	}
	if action != nil {
		if action.ToolID == "finance.sheet.read.v1" {
			input := decodeSheetInput(action.Input)
			choice, e := o.resolveChatSheet(ctx, account, in.FinanceSheetID, in.ChatSheetID, input.Period)
			if e != nil {
				return "", "", e
			}
			if choice.Reply != "" {
				return choice.Reply, choice.Action, nil
			}
			return o.chatSheetReply(ctx, sc, choice.SheetID, input)
		}
		if action.ToolID == "finance.due_dates.read.v1" {
			return "As linhas da planilha não têm vencimento individual cadastrado. Data de efetivação não é vencimento.", "", nil
		}
		if strings.HasPrefix(action.ToolID, "accounting.") || action.ToolID == "knowledge.search.v1" || action.ToolID == "plus.explanation.v1" {
			return "Essa consulta precisa de um emitente e de fontes autorizadas. Use o contexto Empresarial para selecionar a empresa; não vou presumir uma regra fiscal.", "", nil
		}
	}
	// Negation, injection and ambiguous periods do not fall through to paid interpretation.
	if plusUnsafe.MatchString(normalizedDocumentLabel(in.Text)) {
		return "Pode reformular com um único pedido? Não executo instruções de documentos, links ou comandos arbitrários.", "", nil
	}
	if !cfg.Enabled {
		return fallback, "", nil
	}
	text, err := o.modelReply(ctx, sc, in, cfg, fallback)
	return text, "", err
}

// chatSheetReply reads with authorization before and after, as the page would.
func (o *ConversationOrchestrator) chatSheetReply(ctx context.Context, sc PlusScope, sheetID string, input plusSheetInput) (string, string, error) {
	if err := o.plus.ports.Authorize(ctx, sc, "finance.sheets.view"); err != nil {
		return "", "", err
	}
	text, err := o.readChatSheet(ctx, sc, sheetID, input)
	if err != nil {
		var domain *PlusError
		if errors.As(err, &domain) && domain.Status == 422 {
			return "Não consegui ler essa planilha. Escolha outra ou informe o mês, por exemplo “resumo 2026-11”.", "", nil
		}
		return "", "", err
	}
	if err = o.plus.ports.Authorize(ctx, sc, "finance.sheets.view"); err != nil {
		return "", "", err
	}
	return text, "sheet:" + sheetID, nil
}

func conversationSheetReply(raw json.RawMessage) string {
	var value struct {
		Period string            `json:"period"`
		Totals map[string]string `json:"totals"`
		Lines  []struct {
			Description, Amount, Kind string
			Effective                 bool
		} `json:"lines"`
		Truncated bool `json:"truncated"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return "Não foi possível interpretar a consulta financeira."
	}
	out := fmt.Sprintf("Planilha %s: entradas previstas R$ %s; saídas previstas R$ %s; saldo previsto R$ %s; saldo efetivado R$ %s.", value.Period, conversationMoney(value.Totals["expectedIn"]), conversationMoney(value.Totals["expectedOut"]), conversationMoney(value.Totals["expectedBalance"]), conversationMoney(value.Totals["effectiveBalance"]))
	for _, line := range value.Lines {
		out += fmt.Sprintf("\n%s: %s — R$ %s", line.Kind, assistantText(line.Description, 100), conversationMoney(line.Amount))
	}
	if value.Truncated {
		out += "\nExibindo parte das linhas; consulte a planilha para ver todas. Os totais são do servidor."
	}
	if len(out) > 3500 {
		out = assistantText(out, 3400) + "\nResultado parcial; abra a planilha para consultar todas as linhas."
	}
	return out
}
func conversationMoney(value string) string { return strings.ReplaceAll(value, ".", ",") }
