package finance

import (
	"encoding/json"
	"regexp"
	"strings"
)

var plusUnsafe = regexp.MustCompile(`\b(nao|talvez|hipoteticamente|suponha|ignore|ignorar|execute sql|shell|curl|anexo|anexado|documento diz|instrucao)\b|\bse eu\b|https?://`)
var plusMonth = regexp.MustCompile(`\b20[0-9]{2}-(0[1-9]|1[0-2])\b`)
var plusRelativeDate = regexp.MustCompile(`\b(hoje|ontem|amanha|semana)\b`)
var plusNamedMonth = regexp.MustCompile(`\b(janeiro|fevereiro|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)\b`)
var plusOtherDate = regexp.MustCompile(`\b[0-9]{1,2}[/.-][0-9]{1,2}[/.-][0-9]{2,4}\b`)

func routePlus(in PlusInvocation) (*PlusAction, string) {
	text := strings.TrimSpace(normalizedDocumentLabel(in.Question))
	if plusUnsafe.MatchString(text) {
		return nil, "clarification_required"
	}
	if in.Action != nil {
		return in.Action, "structured_action"
	}
	if multipleRequests(in.Question, in.Month) != nil {
		return nil, "multiple_intents"
	}
	// Natural read variations reuse the canonical phrases; preparation keeps its exact
	// phrase here because corporate/Plus preparation stays conservative.
	if intent := interpretConversation(in.Question, in.Month); intent.Kind == "command" && intent.Canonical != "preparar nota" {
		text = intent.Canonical
	}
	switch {
	case strings.HasPrefix(text, "obrigacoes") || strings.HasPrefix(text, "memoria de calculo"):
		return &PlusAction{ToolID: "accounting.obligations.read.v1", Input: plusJSON(map[string]string{"period": ""})}, "obligations_requested"
	case strings.Contains(text, "venc"):
		return &PlusAction{ToolID: "finance.due_dates.read.v1", Input: json.RawMessage(`{}`)}, "due_dates_unavailable"
	case strings.HasPrefix(text, "resuma fonte") || strings.HasPrefix(text, "explique") || strings.HasPrefix(text, "interprete") || strings.HasPrefix(text, "o que significa"):
		return &PlusAction{ToolID: "plus.explanation.v1", Input: json.RawMessage(`{}`)}, "interpretation_requested"
	case strings.HasPrefix(text, "buscar fonte") || strings.HasPrefix(text, "localizar regra") || strings.HasPrefix(text, "qual regra"):
		query := strings.TrimSpace(in.Question)
		words := strings.Fields(query)
		if len(words) <= 2 {
			return nil, "knowledge_query_required"
		}
		query = strings.Join(words[2:], " ")
		return &PlusAction{ToolID: "knowledge.search.v1", Input: plusJSON(map[string]any{"query": query, "competenceDate": in.CompetenceDate, "topK": 5})}, "knowledge_requested"
	case strings.HasPrefix(text, "revisar dados") || strings.HasPrefix(text, "ver revisao"):
		if in.DocumentID == "" {
			return nil, "document_required"
		}
		return &PlusAction{ToolID: "accounting.document.review.v1", Input: plusJSON(map[string]string{"documentId": in.DocumentID})}, "review_requested"
	case text == "preparar nota" || text == "prepare nota" || text == "preparar documento":
		if in.FinanceDraftID != "" {
			return &PlusAction{ToolID: "finance.personal.prepare.v1", Input: plusJSON(map[string]string{"draftId": in.FinanceDraftID})}, "personal_preparation_requested"
		}
		return nil, "preparation_data_required"
	case text == "abrir perfil" || strings.HasPrefix(text, "abrir perfil ") || text == "cadastro fiscal" || text == "cadastrar acesso" || text == "anexar comprovante":
		return &PlusAction{ToolID: "finance.personal.guide.v1", Input: plusJSON(map[string]string{"text": in.Question})}, "personal_flow_requested"
	case strings.HasPrefix(text, "pendencias") || strings.HasPrefix(text, "quais dados faltam") || strings.HasPrefix(text, "diagnostico") || strings.HasPrefix(text, "documentos pendentes"):
		return &PlusAction{ToolID: "accounting.diagnosis.read.v1", Input: plusJSON(map[string]string{"diagnosisId": in.DiagnosisID})}, "diagnosis_requested"
	case strings.HasPrefix(text, "resumo") || strings.HasPrefix(text, "saldo") || strings.HasPrefix(text, "consultar planilha") || strings.HasPrefix(text, "o que paguei") || strings.HasPrefix(text, "listar saidas") || strings.HasPrefix(text, "listar entradas"):
		if plusOtherDate.MatchString(text) || plusRelativeDate.MatchString(text) || (plusNamedMonth.MatchString(text) && !plusMonth.MatchString(text)) {
			return nil, "explicit_period_required"
		}
		period := in.Month
		matches := plusMonth.FindAllString(text, -1)
		if len(matches) > 1 {
			return nil, "explicit_period_required"
		}
		if len(matches) == 1 {
			period = matches[0]
		}
		kind := ""
		effectiveOnly := strings.HasPrefix(text, "o que paguei")
		if strings.Contains(text, "saidas") || effectiveOnly {
			kind = "saida"
		}
		if strings.Contains(text, "entradas") {
			kind = "entrada"
		}
		return &PlusAction{ToolID: "finance.sheet.read.v1", Input: plusJSON(map[string]any{"sheetId": in.FinanceSheetID, "period": period, "page": 1, "limit": 20, "kind": kind, "effectiveOnly": effectiveOnly})}, "financial_read_requested"
	default:
		return nil, "clarification_required"
	}
}
func plusTools() []PlusTool {
	definitions := []struct{ id, mode, permission, properties string }{
		{"finance.sheet.read.v1", "read", "finance.sheets.view", `"kind":{"type":"string","enum":["","entrada","saida"]},"effectiveOnly":{"type":"boolean"},"sheetId":{"type":"string"},"period":{"type":"string"},"page":{"type":"integer"},"limit":{"type":"integer"}`},
		{"accounting.diagnosis.read.v1", "read", "accounting.issuers.view", `"diagnosisId":{"type":"string"}`},
		{"accounting.diagnosis.save.v1", "prepare", "accounting.issuers.manage", `"diagnosisId":{"type":["string","null"]},"expectedVersion":{"type":"integer"},"expectedProfileVersion":{"type":"integer"},"intent":{"type":"string"},"competenceDate":{"type":["string","null"]},"answers":{"type":"object"}`},
		{"accounting.document.prepare.v1", "prepare", "accounting.documents.manage", `"expectedVersion":{"type":"integer"},"diagnosisId":{"type":"string"},"expectedDiagnosisVersion":{"type":"integer"},"expectedProfileVersion":{"type":"integer"},"kind":{"type":"string"},"fields":{"type":"object"},"source":{"type":["object","null"]}`},
		{"accounting.document.review.v1", "read", "accounting.documents.view", `"documentId":{"type":"string"}`},
		{"accounting.obligations.read.v1", "read", "accounting.obligations.view", `"period":{"type":"string"},"obligationId":{"type":"string"}`},
		{"accounting.obligation.finance.prepare.v1", "prepare", "accounting.obligations.manage", `"obligationId":{"type":"string"},"expectedVersion":{"type":"integer"},"confirmed":{"type":"boolean"},"sheetId":{"type":"string"},"expectedSheetUpdatedAt":{"type":"string"},"existingLineId":{"type":"string"}`},
		{"knowledge.search.v1", "read", "accounting.knowledge.view", `"query":{"type":"string"},"competenceDate":{"type":"string"},"topK":{"type":"integer"}`},
		{"finance.personal.prepare.v1", "prepare", "finance.sheets.view", `"draftId":{"type":"string"}`},
		{"finance.personal.guide.v1", "prepare", "finance.sheets.view", `"text":{"type":"string"}`},
	}
	out := make([]PlusTool, 0, len(definitions))
	for _, d := range definitions {
		confirmation := "none"
		if d.mode == "prepare" {
			confirmation = "explicit_intent"
		}
		out = append(out, PlusTool{ID: d.id, Version: 1, Mode: d.mode, Permission: d.permission, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{` + d.properties + `}}`), OutputSchema: json.RawMessage(`{"type":"object"}`), TimeoutMS: 10000, MaxOutputBytes: 32768, Confirmation: confirmation})
	}
	return out
}
