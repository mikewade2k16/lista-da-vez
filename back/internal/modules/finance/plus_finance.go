package finance

import (
	"context"
	"encoding/json"
	"strconv"
)

type plusSheetInput struct {
	Kind          string `json:"kind,omitempty"`
	EffectiveOnly bool   `json:"effectiveOnly,omitempty"`
	SheetID       string `json:"sheetId"`
	Period        string `json:"period"`
	Page          int    `json:"page"`
	Limit         int    `json:"limit"`
}
type plusLine struct {
	ID               string  `json:"id"`
	Kind             string  `json:"kind"`
	Description      string  `json:"description"`
	Amount           string  `json:"amount"`
	AdjustmentAmount string  `json:"adjustmentAmount"`
	Effective        bool    `json:"effective"`
	EffectiveDate    string  `json:"effectiveDate"`
	DueDate          *string `json:"dueDate"`
}

func (s *PlusService) readSheet(ctx context.Context, sc PlusScope, raw json.RawMessage) (json.RawMessage, error) {
	var in plusSheetInput
	if e := strictPlus(raw, &in); e != nil {
		return nil, e
	}
	if in.Kind != "" && in.Kind != "entrada" && in.Kind != "saida" {
		return nil, plusError("invalid_input", 400)
	}
	if !isUUID(in.SheetID) {
		return nil, plusError("sheet_required", 422)
	}
	if in.Page == 0 {
		in.Page = 1
	}
	if in.Limit == 0 {
		in.Limit = 20
	}
	if in.Page < 1 || in.Page > 100000 || in.Limit < 1 || in.Limit > 100 {
		return nil, plusError("invalid_input", 400)
	}
	sheet, e := s.assistant.finance.GetSheet(ctx, sc.AccountID, in.SheetID)
	if e != nil {
		return nil, e
	}
	if in.Period != "" && in.Period != sheet.Period {
		return nil, plusError("sheet_period_mismatch", 422)
	}
	lines := make([]plusLine, 0, len(sheet.Entradas)+len(sheet.Saidas))
	for _, group := range []struct {
		kind  string
		lines []Line
	}{{"entrada", sheet.Entradas}, {"saida", sheet.Saidas}} {
		if in.Kind != "" && in.Kind != group.kind {
			continue
		}
		for _, line := range group.lines {
			if in.EffectiveOnly && !line.Effective {
				continue
			}
			lines = append(lines, plusLine{ID: line.ID, Kind: group.kind, Description: assistantText(line.Description, 300), Amount: strconv.FormatFloat(line.Amount, 'f', 2, 64), AdjustmentAmount: strconv.FormatFloat(line.AdjustmentAmount, 'f', 2, 64), Effective: line.Effective, EffectiveDate: line.EffectiveDate})
		}
	}
	total := len(lines)
	start := (in.Page - 1) * in.Limit
	if start > total {
		start = total
	}
	end := start + in.Limit
	if end > total {
		end = total
	}
	totals := map[string]string{"expectedIn": strconv.FormatFloat(sheet.Summary.ExpectedIn, 'f', 2, 64), "effectiveIn": strconv.FormatFloat(sheet.Summary.EffectiveIn, 'f', 2, 64), "expectedOut": strconv.FormatFloat(sheet.Summary.ExpectedOut, 'f', 2, 64), "effectiveOut": strconv.FormatFloat(sheet.Summary.EffectiveOut, 'f', 2, 64), "expectedBalance": strconv.FormatFloat(sheet.Summary.ExpectedBalance, 'f', 2, 64), "effectiveBalance": strconv.FormatFloat(sheet.Summary.EffectiveBalance, 'f', 2, 64)}
	return plusJSON(map[string]any{"sheetId": sheet.ID, "period": sheet.Period, "sourceVersion": sheet.UpdatedAt, "totals": totals, "lines": lines[start:end], "page": in.Page, "limit": in.Limit, "total": total, "truncated": start > 0 || end < total, "dueDatesAvailable": false, "precisionSource": "finance_legacy"}), nil
}
