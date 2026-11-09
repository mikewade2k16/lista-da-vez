package finance

import (
	"regexp"
	"strings"
)

// Natural-language normalization for the main conversation (CT-15 pacote 3).
// It only rewrites recognized commands into the canonical phrases already
// handled by accessReply/routePlus. It never infers fiscal values, client,
// regime, period or authorization; unknown text is returned unchanged.

type conversationIntent struct {
	Kind      string   // "", "command", "cancel", "resume", "multiple"
	Canonical string   // canonical command text when Kind is command/cancel
	Parts     []string // human labels of each detected request when Kind is multiple
}

var (
	intentPunctuation = regexp.MustCompile(`[?!.,;:"()]+`)
	intentSpaces      = regexp.MustCompile(`\s+`)
	intentSeparator   = regexp.MustCompile(`\s*(?:[,;]|\be depois\b|\be em seguida\b|\bem seguida\b|\bdepois\b|\be tambem\b|\btambem\b|\be alem disso\b|\balem disso\b|\be ainda\b|\be\b)\s*`)
	intentPrepare     = regexp.MustCompile(`^(emitir|emite|emita|fazer|faz|faca|criar|cria|crie|gerar|gera|gere|preparar|prepara|prepare|montar|monta|monte|tirar|tira|lancar)\s+(uma\s+|a\s+|minha\s+|outra\s+|nova\s+|mais uma\s+)?(nota|nf|nfse|nfs-e)\b`)
	intentNewNote     = regexp.MustCompile(`^(uma\s+)?((nova|outra)\s+(nota|nf|nfse|nfs-e)\b|(nota|nf|nfse|nfs-e)(\s+fiscal)?\s+(de\s+(r\s*\$\s*)?[0-9]+|para|pro|pra|no valor)\b)`)
	intentReadVerb    = regexp.MustCompile(`^(qual|quais|quanto|quanta|ver|veja|mostra|mostrar|mostre|me mostra|me mostre|me diz|me diga|consultar|consulta|como esta|como ta|listar|lista|liste|meu|minha|meus|minhas|o|a|os|as)\b`)
	intentSaldo       = regexp.MustCompile(`\bsaldo\b`)
	intentResumo      = regexp.MustCompile(`\b(resumo|balanco)\b`)
	intentPaid        = regexp.MustCompile(`^(quanto|o que|oque|onde)\s+(eu\s+)?(gastei|paguei|saiu)\b`)
	intentReceived    = regexp.MustCompile(`^(quanto|o que|oque)\s+(eu\s+)?(recebi|entrou|ganhei)\b`)
	intentOutList     = regexp.MustCompile(`\b(saidas|despesas|gastos)\b`)
	intentInList      = regexp.MustCompile(`\b(entradas|receitas|recebimentos)\b`)
	// A request may start with the list itself ("entradas e saídas de 2026-11").
	intentListStart  = regexp.MustCompile(`^(entradas|saidas|receitas|despesas|gastos|recebimentos)\b`)
	intentReceipt    = regexp.MustCompile(`^(lancar|lanca|lance|registrar|registra|registre|adicionar|adiciona|adicione|colocar|coloca|coloque|incluir|inclui|inclua|vincular|vincula|vincule)\s+(o\s+|a\s+|esse\s+|essa\s+|este\s+|esta\s+|meu\s+|minha\s+)?(comprovante|recibo|despesa|gasto|pagamento|corrida)\b`)
	intentSheet      = regexp.MustCompile(`^(ver|abrir|abre|mostrar|mostra|consultar|consulta)\s+(a\s+|minha\s+)?planilha\b`)
	intentNumMonth   = regexp.MustCompile(`\b(0?[1-9]|1[0-2])/(20[0-9]{2})\b`)
	intentNamedYear  = regexp.MustCompile(`\b(janeiro|fevereiro|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)(?:\s+de)?\s+(20[0-9]{2})\b`)
	intentDeclined   = regexp.MustCompile(`^nao\s+(quero|preciso|vou)\s+(mais|emitir|fazer|criar|gerar|preparar|a nota|nota|isso)\b|^nao precisa mais\b`)
	intentPay        = regexp.MustCompile(`^(pague|pagar|paga|quitar|quite|transferir|transfira)\b`)
	intentRegister   = regexp.MustCompile(`^(cadastre|cadastrar|cadastra)\b`)
	intentFindSource = regexp.MustCompile(`^(busca|buscar|busque|procura|procurar|procure|localiza|localizar|localize|qual)\b.*\b(fonte|fontes|regra|regras|lei)\b`)
)

var intentMonths = map[string]string{"janeiro": "01", "fevereiro": "02", "marco": "03", "abril": "04", "maio": "05", "junho": "06", "julho": "07", "agosto": "08", "setembro": "09", "outubro": "10", "novembro": "11", "dezembro": "12"}

var intentCancel = map[string]bool{"cancelar": true, "cancela": true, "cancele": true, "parar": true, "para": true, "pare": true, "esquece": true, "esqueca": true, "deixa pra la": true, "deixa para la": true, "deixa": true, "desistir": true, "desisto": true, "nao quero mais": true}

var intentResume = map[string]bool{"continuar": true, "continua": true, "continue": true, "retomar": true, "retoma": true, "seguir": true, "prosseguir": true, "onde paramos": true, "onde a gente parou": true, "onde eu parei": true, "voltar para a nota": true, "voltar pra nota": true, "voltar a nota": true, "continuar a nota": true, "continuar nota": true, "retomar a nota": true, "retomar nota": true}

// conversationKey lowercases, strips accents/punctuation and the polite or
// request wrappers that do not change the meaning ("por favor", "você pode").
// "quero saber"/"preciso saber" stay informational because "saber" remains.
func conversationKey(text string) string {
	n := normalizedDocumentLabel(text)
	n = intentSpaces.ReplaceAllString(intentPunctuation.ReplaceAllString(n, " "), " ")
	n = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(n), " por favor"))
	for changed := true; changed; {
		changed = false
		for _, prefix := range []string{"oi ", "ola ", "bom dia ", "boa tarde ", "boa noite ", "crow ", "por favor ", "por gentileza ", "entao ", "e ai ", "voce pode ", "vc pode ", "voce consegue ", "pode ", "consegue ", "poderia ", "eu gostaria de ", "gostaria de ", "eu queria ", "queria ", "eu quero ", "quero ", "eu preciso ", "preciso ", "me ajuda a ", "me ajude a ", "tem como ", "da pra ", "vamos "} {
			if strings.HasPrefix(n, prefix) && !strings.HasPrefix(n[len(prefix):], "saber") {
				n = strings.TrimSpace(n[len(prefix):])
				changed = true
			}
		}
	}
	return n
}

// periodTokens keeps every period hint of the original request so routePlus can
// still demand an explicit period; month/year forms become YYYY-MM.
func periodTokens(key, month string) []string {
	key = intentNamedYear.ReplaceAllStringFunc(key, func(m string) string {
		p := intentNamedYear.FindStringSubmatch(m)
		return p[2] + "-" + intentMonths[p[1]]
	})
	key = intentNumMonth.ReplaceAllStringFunc(key, func(m string) string {
		p := intentNumMonth.FindStringSubmatch(m)
		return p[2] + "-" + strings.Repeat("0", 2-len(p[1])) + p[1]
	})
	out := plusMonth.FindAllString(key, -1)
	out = append(out, plusOtherDate.FindAllString(key, -1)...)
	out = append(out, plusRelativeDate.FindAllString(key, -1)...)
	for _, name := range plusNamedMonth.FindAllString(key, -1) {
		// A bare month name only resolves against the chat's own selected year.
		if len(month) == 7 {
			out = append(out, month[:4]+"-"+intentMonths[name])
			continue
		}
		out = append(out, name)
	}
	return out
}

func canonicalCommand(key, month string) (string, string) {
	withPeriod := func(base string) string {
		return strings.TrimSpace(base + " " + strings.Join(periodTokens(key, month), " "))
	}
	readVerb := intentReadVerb.MatchString(key) || intentListStart.MatchString(key)
	switch {
	case intentReceipt.MatchString(key):
		return "lancar comprovante", "lançar um comprovante"
	case intentPrepare.MatchString(key) || intentNewNote.MatchString(key):
		return "preparar nota", "preparar uma nota"
	case intentSheet.MatchString(key):
		return withPeriod("consultar planilha"), "consultar a planilha"
	case intentPaid.MatchString(key):
		return withPeriod("o que paguei"), "consultar a planilha"
	case intentReceived.MatchString(key):
		return withPeriod("listar entradas"), "consultar a planilha"
	case intentSaldo.MatchString(key) && (readVerb || strings.HasPrefix(key, "saldo")):
		return withPeriod("saldo"), "consultar a planilha"
	case intentResumo.MatchString(key) && (readVerb || strings.HasPrefix(key, "resumo") || strings.HasPrefix(key, "balanco")):
		return withPeriod("resumo"), "consultar a planilha"
	case readVerb && intentOutList.MatchString(key) && intentInList.MatchString(key):
		return withPeriod("resumo"), "consultar a planilha"
	case readVerb && intentOutList.MatchString(key):
		return withPeriod("listar saidas"), "consultar a planilha"
	case readVerb && intentInList.MatchString(key):
		return withPeriod("listar entradas"), "consultar a planilha"
	}
	return "", ""
}

// requestLabel names one segment's request, used only to detect and describe
// composed requests; it never executes anything.
func requestLabel(segment, month string) string {
	if _, label := canonicalCommand(segment, month); label != "" {
		return label
	}
	switch {
	case intentPay.MatchString(segment):
		return "fazer um pagamento"
	case intentRegister.MatchString(segment):
		return "fazer um cadastro"
	case intentFindSource.MatchString(segment):
		return "buscar fontes"
	case strings.HasPrefix(segment, "explique") || strings.HasPrefix(segment, "resuma fonte") || strings.HasPrefix(segment, "interprete") || strings.HasPrefix(segment, "o que significa"):
		return "explicar uma fonte"
	case strings.HasPrefix(segment, "revis") || strings.HasPrefix(segment, "ver revisao"):
		return "revisar dados"
	case strings.HasPrefix(segment, "abrir perfil") || strings.HasPrefix(segment, "anexar"):
		return "abrir um formulário"
	case publicKnowledgeQuestion(segment):
		return "buscar fontes"
	}
	return ""
}

// text keeps punctuation so commas and semicolons still separate requests.
func multipleRequests(text, month string) []string {
	labels := []string{}
	seen := map[string]bool{}
	for _, segment := range intentSeparator.Split(normalizedDocumentLabel(text), -1) {
		label := requestLabel(conversationKey(segment), month)
		if label != "" && !seen[label] {
			seen[label] = true
			labels = append(labels, label)
		}
	}
	if len(labels) < 2 {
		return nil
	}
	return labels
}

func interpretConversation(text, month string) conversationIntent {
	key := conversationKey(text)
	switch {
	case key == "":
		return conversationIntent{}
	case intentCancel[key] || intentDeclined.MatchString(key):
		return conversationIntent{Kind: "cancel", Canonical: "cancelar"}
	case intentResume[key]:
		return conversationIntent{Kind: "resume"}
	}
	if parts := multipleRequests(text, month); parts != nil {
		return conversationIntent{Kind: "multiple", Parts: parts}
	}
	if canonical, _ := canonicalCommand(key, month); canonical != "" {
		return conversationIntent{Kind: "command", Canonical: canonical}
	}
	return conversationIntent{}
}

func multipleRequestsReply(parts []string) string {
	out := "Entendi mais de um pedido: " + strings.Join(parts[:len(parts)-1], ", ") + " e " + parts[len(parts)-1] + ". Qual você quer que eu faça primeiro?"
	for _, p := range parts {
		if p == "fazer um pagamento" {
			return out + " Pagamentos e emissões não são executados pela conversa."
		}
	}
	return out
}
