// Package stringsx reune helpers de string compartilhados entre modulos.
//
// Cada funcao aqui consolida copias identicas que estavam espalhadas pelos
// modulos (queue, crm, cardapio, site, users, etc.). O objetivo e fonte unica
// sem alterar o comportamento OBSERVAVEL de nenhum call-site original.
package stringsx

import (
	"encoding/json"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// FirstNonEmpty devolve o PRIMEIRO valor nao-vazio (apos TrimSpace), ja
// trimado. Strings que so contem espacos sao tratadas como vazias. Quando todos
// os valores sao vazios, devolve "".
//
// Reproduz o comportamento observavel de todas as copias antigas: as variantes
// "trimmed" (queue/reports|operations|analytics, queue/settings, crm/erp,
// site/tracking) ja retornavam o valor trimado; as variantes "raw"
// (crm/catalog, cardapio/service_public) eram sempre consumidas dentro de um
// strings.TrimSpace(...) externo, portanto retornar o valor trimado e
// equivalente para esses call-sites.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// NormalizeIDs trima, descarta vazios e remove duplicatas preservando a ordem
// de primeira aparicao. Sempre devolve um slice nao-nil (vazio quando nao ha
// itens validos), espelhando as copias de normalizeStoreIDs.
func NormalizeIDs(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

// Slugify deriva um slug valido (`^[a-z0-9-]+$`) de um texto livre.
//
// Regra canonica (identica no Go e no TS):
//  1. Trim + lowercase.
//  2. NFD: decompoe caracteres acentuados em letra-base + marca de combinacao.
//  3. Descarta todas as marcas de combinacao (categoria Unicode Mn).
//  4. Troca qualquer caractere fora de [a-z0-9] por hifen.
//  5. Colapsa hifens repetidos.
//  6. Remove hifens nas pontas.
//
// Exemplos: "Acao" -> "acao", "Pérola@RioMar!" -> "perola-riomar",
// "  Loja  da  Esquina  " -> "loja-da-esquina".
//
// Mudanca deliberada vs. copias antigas:
//   - cardapio: era so ToLower+Trim (sem normalizar acentos). Agora normaliza.
//   - bio: usava NFKD em vez de NFD (resultado identico para os acentos pt-BR,
//     mas NFD e o padrao canonico e mais previsivel para outros idiomas).
//   - site.perolaSlug e uma logica diferente (usa "_", para o crow-notion) e
//     NAO usa esta funcao — permanece inalterada.
func Slugify(raw string) string {
	// NFD decompoe letras acentuadas; Mn descarta as marcas de combinacao.
	decomposed := norm.NFD.String(strings.ToLower(strings.TrimSpace(raw)))
	var b strings.Builder
	prevHyphen := false
	for _, r := range decomposed {
		switch {
		case unicode.Is(unicode.Mn, r):
			// marca de combinacao (acento) — descarta
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		default:
			// qualquer outro caractere vira hifen (colapsa repeticoes)
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// DecodeJSONStringSlice decodifica um jsonb (coluna text[]/json serializada)
// para []string. Entrada vazia ou JSON invalido devolve um slice nao-nil vazio
// (nunca nil), assim como as copias de decodeStringSlice nos stores.
func DecodeJSONStringSlice(raw []byte) []string {
	out := []string{}
	if len(raw) == 0 {
		return out
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		return out
	}
	if items == nil {
		return out
	}
	return items
}
