package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// CT-15 pacote 5: the chat chooses its own sheet/period inside the chat's account.
// Priority: period written in the message, sheet already chosen in this chat,
// sheet sent by the page (same account only), then a question with options.
// Nothing here changes the page selection or reads another account.

const chatSheetOptions = 6

type chatSheetChoice struct {
	SheetID string
	Reply   string
	Action  string
}

func chatSheetLabel(s SheetListItem) string {
	label := s.Title
	if label == "" {
		label = "Planilha"
	}
	if s.ClientName != "" {
		label += " — " + s.ClientName
	}
	return label + " (" + s.Period + ")"
}

func chatSheetQuestion(intro string, items []SheetListItem) string {
	out := intro
	for i, s := range items {
		out += fmt.Sprintf("\n%d) %s", i+1, chatSheetLabel(s))
	}
	return out
}

func (o *ConversationOrchestrator) chatSheets(ctx context.Context, account, period string) ([]SheetListItem, error) {
	items, _, err := o.plus.assistant.finance.ListSheets(ctx, account, ListFilter{Period: period, Limit: chatSheetOptions})
	return items, err
}

// sheetInAccount returns the sheet period when the id belongs to the account.
func (o *ConversationOrchestrator) sheetInAccount(ctx context.Context, account, id string) (string, bool, error) {
	if !isUUID(id) {
		return "", false, nil
	}
	sheet, err := o.plus.assistant.finance.GetSheet(ctx, account, id)
	if errors.Is(err, ErrSheetNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return sheet.Period, true, nil
}

func (o *ConversationOrchestrator) resolveChatSheet(ctx context.Context, account, pageSheet, chatSheet, period string) (chatSheetChoice, error) {
	for _, candidate := range []string{chatSheet, pageSheet} {
		sheetPeriod, ok, err := o.sheetInAccount(ctx, account, candidate)
		if err != nil {
			return chatSheetChoice{}, err
		}
		if ok && (period == "" || period == sheetPeriod) {
			return chatSheetChoice{SheetID: candidate}, nil
		}
	}
	items, err := o.chatSheets(ctx, account, period)
	if err != nil {
		return chatSheetChoice{}, err
	}
	switch {
	case len(items) == 1:
		return chatSheetChoice{SheetID: items[0].ID}, nil
	case len(items) == 0 && period != "":
		return chatSheetChoice{Reply: "Não encontrei planilha de " + period + " nesta conta. Confira o mês ou crie a planilha no Finance."}, nil
	case len(items) == 0:
		return chatSheetChoice{Reply: "Ainda não há planilhas nesta conta."}, nil
	case period != "":
		return chatSheetChoice{Reply: chatSheetQuestion("Encontrei mais de uma planilha de "+period+":", items) + "\nQual delas? Responda o número.", Action: "sheetpick:" + period}, nil
	}
	return chatSheetChoice{Reply: chatSheetQuestion("Qual planilha você quer consultar? As mais recentes desta conta:", items) + "\nResponda o número ou peça, por exemplo, “resumo de " + items[0].Period + "”.", Action: "sheetpick:"}, nil
}

// pickChatSheet resolves a numbered answer to the previous sheet question.
func (o *ConversationOrchestrator) pickChatSheet(ctx context.Context, account, previousAction, text string) (string, bool, error) {
	period, ok := strings.CutPrefix(previousAction, "sheetpick:")
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if !ok || err != nil || n < 1 {
		return "", false, nil
	}
	items, err := o.chatSheets(ctx, account, period)
	if err != nil || n > len(items) {
		return "", false, err
	}
	return items[n-1].ID, true, nil
}

func (o *ConversationOrchestrator) readChatSheet(ctx context.Context, sc PlusScope, sheetID string, input plusSheetInput) (string, error) {
	input.SheetID = sheetID
	raw, err := o.plus.readSheet(ctx, sc, plusJSON(map[string]any{"sheetId": input.SheetID, "period": "", "page": 1, "limit": 20, "kind": input.Kind, "effectiveOnly": input.EffectiveOnly}))
	if err != nil {
		return "", err
	}
	return conversationSheetReply(raw), nil
}

func decodeSheetInput(raw json.RawMessage) plusSheetInput {
	var in plusSheetInput
	_ = json.Unmarshal(raw, &in)
	return in
}
