package core

import (
	"strings"
	"unicode"
)

// BuildNickname gera o nick padrao do produto a partir do display_name.
// Replica `web/app/domain/utils/person-display.ts > buildNickname`:
//
//	"Acilene dos Santos Teste" → "Acilene D."
//	"Mike"                     → "Mike"
//	""                         → ""
//
// Regra: primeiro nome + inicial do segundo token + ponto. Se nome ultrapassa
// maxLength, encurta primeiro nome com "...". Mantida 1-para-1 com o front
// para evitar drift entre nicks gerados em camadas diferentes.
func BuildNickname(displayName string, maxLength int) string {
	if maxLength <= 0 {
		maxLength = 18
	}
	normalized := strings.TrimSpace(displayName)
	if normalized == "" {
		return ""
	}
	parts := strings.Fields(normalized)
	if len(parts) == 0 {
		return ""
	}
	first := parts[0]
	var nickname string
	if len(parts) > 1 {
		secondInitial := upperFirstRune(parts[1])
		nickname = first + " " + secondInitial + "."
	} else {
		nickname = first
	}
	if len(nickname) > maxLength {
		cut := maxLength - 3
		if cut < 1 {
			cut = 1
		}
		if cut > len(first) {
			cut = len(first)
		}
		return first[:cut] + "..."
	}
	return nickname
}

func upperFirstRune(s string) string {
	for _, r := range s {
		return string(unicode.ToUpper(r))
	}
	return ""
}
