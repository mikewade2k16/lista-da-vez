package omnichannel

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// safePreferredPersonalName returns a conservative name for customer-facing
// personalization. Channel display names remain available to operators, but
// phrases, businesses, handles and phone numbers are never presented to the
// model as a person's name.
func safePreferredPersonalName(candidates ...string) (string, string) {
	for index, candidate := range candidates {
		if name, ok := likelyPersonalName(candidate); ok {
			source := "crm"
			if index > 0 {
				source = "channel"
			}
			return name, source
		}
	}
	return "", "unknown"
}

func likelyPersonalName(raw string) (string, bool) {
	name := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 80 {
		return "", false
	}
	parts := strings.Fields(name)
	if len(parts) == 0 || len(parts) > 5 {
		return "", false
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsSpace(r) || r == '\'' || r == '’' || r == '-' {
			continue
		}
		return "", false
	}
	rejected := map[string]struct{}{
		"deus": {}, "fiel": {}, "gratidão": {}, "gratidao": {}, "abençoado": {}, "abencoado": {},
		"é": {}, "eh": {}, "sou": {}, "somos": {}, "estou": {}, "estamos": {}, "amo": {},
		"sempre": {}, "só": {}, "so": {}, "loja": {}, "empresa": {}, "atendimento": {},
		"suporte": {}, "vendas": {}, "oficial": {}, "promoção": {}, "promocao": {},
	}
	for _, part := range parts {
		if _, found := rejected[strings.ToLower(strings.Trim(part, "'’-"))]; found {
			return "", false
		}
	}
	return name, true
}
