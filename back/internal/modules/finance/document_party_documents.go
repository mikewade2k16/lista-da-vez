package finance

import "strings"

// Keep bank identifiers separate from party identifiers; masked documents remain unknown.
func parseDocumentPartyDocuments(text string, out *DocumentFields) {
	section, pending := "", ""
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		normalized := normalizedDocumentLabel(line)
		if strings.HasPrefix(normalized, "nu pagamentos") || strings.HasPrefix(normalized, "informacoes adicionais") || strings.HasPrefix(normalized, "id da transacao") {
			section, pending = "", ""
			continue
		}
		for _, label := range documentPartyLabels {
			if _, ok := documentLabelValue(line, label.label); ok {
				section, pending = label.target, ""
				break
			}
		}
		if section == "" {
			continue
		}
		value := ""
		for _, label := range []string{"cpf/cnpj", "cnpj", "cpf"} {
			if rest, ok := documentLabelValue(line, label); ok {
				value, pending = rest, section
				break
			}
		}
		if pending == "" || value == "" && (normalized == "cpf" || normalized == "cnpj" || normalized == "cpf/cnpj") {
			continue
		}
		if value == "" {
			value = line
		}
		document := completeReceiptDocument(value)
		if pending == "payer" && out.PayerDocument == "" {
			out.PayerDocument = document
		}
		if pending == "payee" && out.PayeeDocument == "" {
			out.PayeeDocument = document
		}
		section, pending = "", ""
	}
}

func completeReceiptDocument(value string) string {
	var digits strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case strings.ContainsRune(".-/ \t", r):
		default:
			return ""
		}
	}
	if digits.Len() != 11 && digits.Len() != 14 {
		return ""
	}
	return digits.String()
}
