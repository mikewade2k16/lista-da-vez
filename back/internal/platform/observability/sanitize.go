package observability

import (
	"regexp"
	"strings"
)

// Sanitização central: todo texto livre que vai para logs ou para o schema obs passa
// por aqui. Mascara credenciais e dados pessoais; nunca tenta "consertar" o texto.

const masked = "[redacted]"

var (
	bearerPattern   = regexp.MustCompile(`(?i)\b(bearer|basic|token)\s+[A-Za-z0-9._~+/=-]{8,}`)
	jwtPattern      = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\b`)
	emailPattern    = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	cpfPattern      = regexp.MustCompile(`\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`)
	cnpjPattern     = regexp.MustCompile(`\b\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2}\b`)
	cardPattern     = regexp.MustCompile(`\b(?:\d{4} \d{4} \d{4} \d{1,7}|\d{4}-\d{4}-\d{4}-\d{1,7}|\d{13,19})\b`)
	phonePattern    = regexp.MustCompile(`\+?\b55\s?\(?\d{2}\)?\s?9?\d{4}-?\d{4}\b`)
	secretKVPattern = regexp.MustCompile(`(?i)("?(?:password|passwd|senha|secret|token|api[_-]?key|access[_-]?key|authorization|cookie|cpf|cnpj)"?\s*[:=]\s*)("[^"]*"|[^\s,&}]+)`)
	queryPattern    = regexp.MustCompile(`\?[^\s"']*`)
)

var sensitiveKeys = []string{
	"password", "passwd", "senha", "secret", "token", "authorization", "cookie",
	"apikey", "api_key", "accesskey", "access_key", "cpf", "cnpj", "card", "cartao",
	"email", "phone", "telefone", "whatsapp", "document",
}

// SanitizeText mascara tokens, e-mails, CPF/CNPJ, cartões, telefones e pares
// chave=valor sensíveis, e limita o tamanho.
func SanitizeText(value string, limit int) string {
	if value == "" {
		return ""
	}
	out := bearerPattern.ReplaceAllString(value, "${1} "+masked)
	out = secretKVPattern.ReplaceAllString(out, "${1}"+masked)
	out = jwtPattern.ReplaceAllString(out, masked)
	out = emailPattern.ReplaceAllString(out, "[email]")
	out = cnpjPattern.ReplaceAllString(out, "[cnpj]")
	out = cpfPattern.ReplaceAllString(out, "[cpf]")
	out = phonePattern.ReplaceAllString(out, "[phone]")
	out = cardPattern.ReplaceAllStringFunc(out, maskCardNumber)
	return truncate(out, limit)
}

// SanitizeURLPath remove a query string (pode carregar token/e-mail) e mascara o resto.
func SanitizeURLPath(value string, limit int) string {
	return SanitizeText(queryPattern.ReplaceAllString(value, ""), limit)
}

// IsSensitiveKey informa se o nome de um campo/header carrega dado sensível.
func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(key))
	for _, candidate := range sensitiveKeys {
		if strings.Contains(normalized, strings.ReplaceAll(candidate, "_", "")) {
			return true
		}
	}
	return false
}

// SanitizeValue percorre mapas/listas decodificados de JSON mascarando chaves
// sensíveis e textos livres. Profundidade limitada para não custar caro.
func SanitizeValue(value any, depth int) any {
	if depth > 4 {
		return masked
	}
	switch typed := value.(type) {
	case string:
		return SanitizeText(typed, 500)
	case map[string]any:
		out := make(map[string]any, len(typed))
		count := 0
		for key, item := range typed {
			if count >= 30 {
				break
			}
			count++
			if IsSensitiveKey(key) {
				out[key] = masked
				continue
			}
			out[truncate(key, 60)] = SanitizeValue(item, depth+1)
		}
		return out
	case []any:
		limit := min(len(typed), 20)
		out := make([]any, 0, limit)
		for _, item := range typed[:limit] {
			out = append(out, SanitizeValue(item, depth+1))
		}
		return out
	default:
		return typed
	}
}

func truncate(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	cut := limit
	for cut > 0 && !isRuneStart(value[cut]) {
		cut--
	}
	return value[:cut] + "…"
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// maskCardNumber só mascara sequências que passam no Luhn: evita apagar ids,
// timestamps e correlation IDs numéricos que não são cartão.
func maskCardNumber(match string) string {
	digits := make([]int, 0, len(match))
	for _, r := range match {
		if r >= '0' && r <= '9' {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return match
	}
	sum := 0
	for index := len(digits) - 1; index >= 0; index-- {
		digit := digits[index]
		if (len(digits)-1-index)%2 == 1 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	if sum%10 != 0 {
		return match
	}
	return "[card]"
}
