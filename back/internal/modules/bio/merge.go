package bio

import (
	"encoding/json"
	"strings"
)

// deepMerge funde base e override com a mesma semantica do
// server/utils/deepMerge.ts do front bio:
//   - objetos: merge recursivo (chaves do override sobrescrevem/ adicionam).
//   - arrays e primitivos: substituicao TOTAL (override vence).
//   - chave so no base: preservada.
//
// Recebe e devolve json.RawMessage. base ou override vazios sao tratados como
// objeto vazio. Em JSON invalido, devolve o override cru (fail-safe: o que veio
// do banco da bio prevalece).
func deepMerge(base, override json.RawMessage) (json.RawMessage, error) {
	baseVal, baseOK := decodeJSON(base)
	overrideVal, overrideOK := decodeJSON(override)

	if !overrideOK {
		return normalizeRaw(base), nil
	}
	if !baseOK {
		return normalizeRaw(override), nil
	}

	merged := mergeValues(baseVal, overrideVal)
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// mergeValues aplica a regra: so funde quando AMBOS sao objetos; caso contrario
// o override substitui por inteiro.
func mergeValues(base, override any) any {
	baseObj, baseIsObj := base.(map[string]any)
	overrideObj, overrideIsObj := override.(map[string]any)
	if !baseIsObj || !overrideIsObj {
		return override
	}

	out := make(map[string]any, len(baseObj)+len(overrideObj))
	for k, v := range baseObj {
		out[k] = v
	}
	for k, v := range overrideObj {
		if existing, ok := out[k]; ok {
			out[k] = mergeValues(existing, v)
			continue
		}
		out[k] = v
	}
	return out
}

func decodeJSON(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false
	}
	return v, true
}

// absolutizeUploads percorre o JSON e troca toda string que comeca com
// "/uploads/" pela URL absoluta base+path. base e a PUBLIC_API_BASE_URL (sem "/"
// final). Quando base e vazia, devolve o caminho relativo intacto.
func absolutizeUploads(raw json.RawMessage, base string) (json.RawMessage, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return normalizeRaw(raw), nil
	}
	val, ok := decodeJSON(raw)
	if !ok {
		return normalizeRaw(raw), nil
	}
	walked := walkUploads(val, base)
	out, err := json.Marshal(walked)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func walkUploads(value any, base string) any {
	switch v := value.(type) {
	case map[string]any:
		for k, child := range v {
			v[k] = walkUploads(child, base)
		}
		return v
	case []any:
		for i, child := range v {
			v[i] = walkUploads(child, base)
		}
		return v
	case string:
		if strings.HasPrefix(v, "/uploads/") {
			return base + v
		}
		return v
	default:
		return value
	}
}

// jsonHasNonEmptyPath verifica se uma chave aninhada (ex.: "branding","logo",
// "srcMobile") existe no JSON e e uma string nao-vazia. Usado na validacao de
// publish (campos minimos).
func jsonHasNonEmptyPath(raw json.RawMessage, path ...string) bool {
	val, ok := decodeJSON(raw)
	if !ok {
		return false
	}
	cur := val
	for _, key := range path {
		obj, isObj := cur.(map[string]any)
		if !isObj {
			return false
		}
		next, exists := obj[key]
		if !exists {
			return false
		}
		cur = next
	}
	str, isStr := cur.(string)
	return isStr && strings.TrimSpace(str) != ""
}
