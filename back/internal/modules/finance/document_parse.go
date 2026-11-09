package finance

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DocumentFields struct {
	DocumentType     string               `json:"documentType"`
	Amount           *float64             `json:"amount"`
	PaymentDate      string               `json:"paymentDate"`
	PayerDocument    string               `json:"payerDocument"`
	PayeeDocument    string               `json:"payeeDocument"`
	PayerName        string               `json:"payerName"`
	PayeeName        string               `json:"payeeName"`
	CustomerName     string               `json:"customerName"`
	CustomerDocument string               `json:"customerDocument"`
	CompetenceDate   string               `json:"competenceDate"`
	Service          FiscalTemplateFields `json:"service"`
	TemplateID       string               `json:"templateId"`
}
type DocumentDraft struct {
	ID            string         `json:"id"`
	SourceName    string         `json:"sourceName"`
	ExtractedText string         `json:"extractedText"`
	Fields        DocumentFields `json:"fields"`
	MissingFields []string       `json:"missingFields"`
	Warnings      []string       `json:"warnings"`
	UpdatedAt     string         `json:"updatedAt"`
}
type DocumentDraftInput struct {
	Fields            DocumentFields `json:"fields"`
	ExpectedUpdatedAt string         `json:"expectedUpdatedAt"`
}

var documentMoneyRE = regexp.MustCompile(`(?i)R\s*\$\s*([0-9.]+,[0-9]{2})`)
var documentLabeledMoneyRE = regexp.MustCompile(`(?im)^valor(?: original| final)?[ \t]*(?:[RA][S$]|R\s*\$)?\s*([0-9.]+,[0-9]{2})[ \t]*$`)
var documentDateRE = regexp.MustCompile(`(?i)\b(\d{2})[ /]+(JAN(?:EIRO)?|FEV(?:EREIRO)?|MAR(?:ÇO|CO)?|ABR(?:IL)?|MAI(?:O)?|JUN(?:HO)?|JUL(?:HO)?|AGO(?:STO)?|SET(?:EMBRO)?|OUT(?:UBRO)?|NOV(?:EMBRO)?|DEZ(?:EMBRO)?)[ /]+(\d{4})\b`)
var documentNumericDateRE = regexp.MustCompile(`\b(\d{2})/(\d{2})/(\d{4})\b`)

func parseDocumentText(text string) DocumentFields {
	out := DocumentFields{DocumentType: "documento"}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "transferência") || strings.Contains(lower, "transferencia") || strings.Contains(lower, "pix") || strings.Contains(lower, "comprovante") {
		out.DocumentType = "comprovante"
	}
	values := documentMoneyRE.FindAllStringSubmatch(text, -1)
	values = append(values, documentLabeledMoneyRE.FindAllStringSubmatch(text, -1)...)
	// Multiple differing values need manual selection instead of silently picking a total.
	var amount string
	for _, match := range values {
		if amount != "" && amount != match[1] {
			amount = ""
			break
		}
		amount = match[1]
	}
	if amount != "" {
		n, e := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(amount, ".", ""), ",", "."), 64)
		if e == nil && n >= 0 {
			out.Amount = &n
		}
	}
	if m := documentDateRE.FindStringSubmatch(text); len(m) > 0 {
		months := map[string]string{"JAN": "01", "FEV": "02", "MAR": "03", "ABR": "04", "MAI": "05", "JUN": "06", "JUL": "07", "AGO": "08", "SET": "09", "OUT": "10", "NOV": "11", "DEZ": "12"}
		out.PaymentDate = m[3] + "-" + months[strings.ToUpper(m[2])[:3]] + "-" + m[1]
	} else if m := documentNumericDateRE.FindStringSubmatch(text); len(m) > 0 {
		out.PaymentDate = m[3] + "-" + m[2] + "-" + m[1]
	}
	if _, err := time.Parse("2006-01-02", out.PaymentDate); err != nil {
		out.PaymentDate = ""
	}
	if out.DocumentType == "comprovante" {
		parseDocumentParties(text, &out)
		parseDocumentPartyDocuments(text, &out)
	}
	return out
}
func documentView(out DocumentDraft) DocumentDraft {
	out.MissingFields = []string{}
	if out.Fields.Amount == nil {
		out.MissingFields = append(out.MissingFields, "Valor do serviço")
	}
	if out.Fields.CustomerName == "" {
		out.MissingFields = append(out.MissingFields, "Tomador do serviço")
	}
	if out.Fields.CustomerDocument == "" {
		out.MissingFields = append(out.MissingFields, "CPF/CNPJ do tomador")
	}
	if out.Fields.CompetenceDate == "" {
		out.MissingFields = append(out.MissingFields, "Competência do serviço")
	}
	if out.Fields.Service.ServiceDescription == "" {
		out.MissingFields = append(out.MissingFields, "Descrição do serviço")
	}
	out.Warnings = []string{"A leitura pode conter erros: confira os campos com o documento original.", "Rascunho para revisão; não emite nota e não confirma tributação. O emitente vem do seu perfil fiscal."}
	if out.Fields.DocumentType == "comprovante" {
		out.Warnings = append(out.Warnings, "Comprovante não é nota fiscal. Pagador/recebedor não determinam emitente/tomador; data do pagamento não determina competência.")
	}
	return out
}
