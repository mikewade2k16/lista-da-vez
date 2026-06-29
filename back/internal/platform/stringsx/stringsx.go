// Package stringsx reune helpers de string compartilhados entre modulos.
//
// Cada funcao aqui consolida copias identicas que estavam espalhadas pelos
// modulos (queue, crm, cardapio, site, users, etc.). O objetivo e fonte unica
// sem alterar o comportamento OBSERVAVEL de nenhum call-site original.
package stringsx

import (
	"encoding/json"
	"strings"
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
