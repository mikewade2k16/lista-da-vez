package finance

import "strings"

type documentPartyLabel struct{ label, target string }

var documentPartyLabels = []documentPartyLabel{
	{"dados do titular do cartao", "payer"}, {"dados do titular", "payer"}, {"dados do pagador", "payer"}, {"dados do remetente", "payer"},
	{"dados do estabelecimento", "payee"}, {"dados do recebedor", "payee"}, {"dados do destinatario", "payee"},
	{"nome do pagador", "payer"}, {"nome do recebedor", "payee"}, {"nome do beneficiario", "payee"},
	{"estabelecimento", "payee"}, {"beneficiario", "payee"}, {"destinatario", "payee"}, {"favorecido", "payee"}, {"recebedor", "payee"}, {"destino", "payee"},
	{"remetente", "payer"}, {"pagador", "payer"}, {"titular", "payer"}, {"origem", "payer"}, {"ongem", "payer"},
}
var documentPartyStops = []string{"cpf", "cnpj", "agencia", "conta", "numero do cartao", "cartao", "nsu", "id da transacao", "instituicao", "banco", "meio de pagamento", "codigo de autorizacao", "n.° de operacao", "n.º de operacao", "dados da transacao", "data e hora", "valor", "tipo de transacao", "tipo de transferencia", "nu pagamentos"}

func normalizedDocumentLabel(line string) string {
	return strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c").Replace(strings.ToLower(strings.TrimSpace(line)))
}
func documentLabelValue(line, label string) (string, bool) {
	normalized := normalizedDocumentLabel(line)
	if normalized == label {
		return "", true
	}
	if !strings.HasPrefix(normalized, label) {
		return "", false
	}
	rest := normalized[len(label):]
	if rest == "" || !strings.ContainsRune(" :\t/", rune(rest[0])) {
		return "", false
	}
	value := string([]rune(strings.TrimSpace(line))[len([]rune(label)):])
	return strings.TrimSpace(strings.TrimLeft(value, ": \t")), true
}
func parseDocumentParties(text string, out *DocumentFields) {
	section := ""
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		stopped := false
		for _, label := range documentPartyStops {
			if _, ok := documentLabelValue(line, label); ok {
				section = ""
				stopped = true
				break
			}
		}
		if stopped {
			continue
		}
		matched := false
		for _, label := range documentPartyLabels {
			if value, ok := documentLabelValue(line, label.label); ok {
				section = label.target
				appendPartyName(out, section, value)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if value, ok := documentLabelValue(line, "nome"); ok {
			if section != "" {
				appendPartyName(out, section, value)
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "mp*") && out.PayeeName == "" {
			out.PayeeName = line
			continue
		}
		if section != "" {
			appendPartyName(out, section, line)
		}
	}
}
func appendPartyName(out *DocumentFields, section, line string) {
	if len(line) > 300 {
		return
	}
	target := &out.PayeeName
	if section == "payer" {
		target = &out.PayerName
	}
	if len(*target)+len(line) > 600 {
		return
	}
	*target = strings.TrimSpace(*target + " " + line)
}

func recoverDocumentParties(fields DocumentFields, text string) (DocumentFields, bool) {
	recovered := parseDocumentText(text)
	changed := false
	if strings.TrimSpace(fields.PayerName) == "" && recovered.PayerName != "" {
		fields.PayerName = recovered.PayerName
		changed = true
	}
	if strings.TrimSpace(fields.PayeeName) == "" && recovered.PayeeName != "" {
		fields.PayeeName = recovered.PayeeName
		changed = true
	}
	for _, pair := range []struct {
		target *string
		value  string
	}{
		{&fields.PayerDocument, recovered.PayerDocument}, {&fields.PayeeDocument, recovered.PayeeDocument},
	} {
		if strings.TrimSpace(*pair.target) == "" && pair.value != "" {
			*pair.target = pair.value
			changed = true
		}
	}
	return fields, changed
}
