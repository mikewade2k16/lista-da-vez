package llm

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Validacao de structured output contra o Schema (OMNI-F3.3).
//
// POR QUE VALIDAR NO GO: o `response_format` do provedor e uma DICA, nao um contrato.
// Modelo alucina campo, troca tipo, devolve string onde o schema pede numero — e o
// strict mode do provedor NAO e prova (o proprio provedor pode degradar para
// best-effort). Entregar JSON nao validado ao dominio e o mesmo que confiar em input
// de cliente. Violacao => ErrSchemaViolation, e o caller decide (repetir, cair para
// regra default, registrar).
//
// ESCOPO SUPORTADO (subconjunto do JSON Schema, deliberado e suficiente para saida de
// LLM): type (inclusive lista de tipos), properties, required, items, enum,
// additionalProperties (bool), minItems/maxItems, minLength/maxLength, minimum/maximum.
// Keyword FORA desta lista e IGNORADA — nao ha validacao silenciosamente falsa: o que
// nao esta aqui simplesmente nao e checado. Precisou de mais, some aqui e suba a
// Version do Schema.
//
// Nao ha lib de JSON Schema no go.mod (verificado) e a F3 nao adiciona dependencia.

// Validate checa o JSON cru contra o Schema. Erro sempre embrulha ErrSchemaViolation.
//
// O erro cita o CAMINHO e a expectativa (ex.: `campo "items[0].name": esperado string,
// veio number`), NUNCA o valor recebido: a resposta do modelo pode conter dado do
// cliente e o erro vai para log (canonico §10).
func Validate(schema *Schema, raw json.RawMessage) error {
	if schema == nil {
		return nil
	}
	if len(schema.Definition) == 0 {
		return fmt.Errorf("%w: schema %q v%d sem definicao", ErrSchemaViolation, schema.Name, schema.Version)
	}

	var definition map[string]any
	if err := json.Unmarshal(schema.Definition, &definition); err != nil {
		return fmt.Errorf("%w: schema %q v%d ilegivel", ErrSchemaViolation, schema.Name, schema.Version)
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("%w: resposta nao e JSON valido", ErrSchemaViolation)
	}

	if err := validateNode(definition, value, ""); err != nil {
		return fmt.Errorf("%w (schema %q v%d): %s", ErrSchemaViolation, schema.Name, schema.Version, err.Error())
	}
	return nil
}

// validateNode valida um valor contra um nó do schema. path e o caminho legivel para o
// erro ("" = raiz).
func validateNode(definition map[string]any, value any, path string) error {
	if err := checkType(definition, value, path); err != nil {
		return err
	}
	if err := checkEnum(definition, value, path); err != nil {
		return err
	}
	switch typed := value.(type) {
	case map[string]any:
		return checkObject(definition, typed, path)
	case []any:
		return checkArray(definition, typed, path)
	case string:
		return checkString(definition, typed, path)
	case float64:
		return checkNumber(definition, typed, path)
	}
	return nil
}

// checkType valida "type". Aceita string ou lista de strings (union).
func checkType(definition map[string]any, value any, path string) error {
	rawType, ok := definition["type"]
	if !ok {
		return nil
	}
	var wanted []string
	switch t := rawType.(type) {
	case string:
		wanted = []string{t}
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok {
				wanted = append(wanted, s)
			}
		}
	default:
		return nil
	}
	for _, want := range wanted {
		if matchesType(want, value) {
			return nil
		}
	}
	return fmt.Errorf("%s: esperado %s, veio %s", label(path), strings.Join(wanted, "|"), typeName(value))
}

// matchesType diz se o valor bate com um tipo do JSON Schema.
func matchesType(want string, value any) bool {
	switch want {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		// JSON nao tem inteiro: o unmarshal traz float64. Integer = sem parte fracionaria.
		n, ok := value.(float64)
		return ok && n == math.Trunc(n) && !math.IsInf(n, 0)
	case "null":
		return value == nil
	default:
		return true // tipo desconhecido: nao inventamos regra
	}
}

// checkEnum valida "enum" comparando o valor serializado (cobre string/number/bool).
func checkEnum(definition map[string]any, value any, path string) error {
	rawEnum, ok := definition["enum"]
	if !ok {
		return nil
	}
	options, ok := rawEnum.([]any)
	if !ok || len(options) == 0 {
		return nil
	}
	got, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("%s: valor nao serializavel", label(path))
	}
	for _, option := range options {
		if candidate, err := json.Marshal(option); err == nil && string(candidate) == string(got) {
			return nil
		}
	}
	// Lista os valores PERMITIDOS (do schema, nao do modelo) — nao ecoa o recebido.
	allowed, _ := json.Marshal(options)
	return fmt.Errorf("%s: valor fora do enum %s", label(path), string(allowed))
}

// checkObject valida required, properties e additionalProperties.
func checkObject(definition map[string]any, value map[string]any, path string) error {
	if required, ok := definition["required"].([]any); ok {
		for _, item := range required {
			name, ok := item.(string)
			if !ok {
				continue
			}
			if _, present := value[name]; !present {
				return fmt.Errorf("%s: campo obrigatorio %q ausente", label(path), name)
			}
		}
	}

	properties, _ := definition["properties"].(map[string]any)
	for name, child := range value {
		childDef, known := properties[name].(map[string]any)
		if !known {
			// additionalProperties: false => campo extra e violacao (modelo alucinou).
			if allowed, ok := definition["additionalProperties"].(bool); ok && !allowed {
				return fmt.Errorf("%s: campo %q nao previsto no schema", label(path), name)
			}
			continue
		}
		if err := validateNode(childDef, child, join(path, name)); err != nil {
			return err
		}
	}
	return nil
}

// checkArray valida items, minItems e maxItems.
func checkArray(definition map[string]any, value []any, path string) error {
	if minItems, ok := numberOf(definition, "minItems"); ok && float64(len(value)) < minItems {
		return fmt.Errorf("%s: esperado ao menos %d itens, veio %d", label(path), int(minItems), len(value))
	}
	if maxItems, ok := numberOf(definition, "maxItems"); ok && float64(len(value)) > maxItems {
		return fmt.Errorf("%s: esperado no maximo %d itens, veio %d", label(path), int(maxItems), len(value))
	}
	itemDef, ok := definition["items"].(map[string]any)
	if !ok {
		return nil
	}
	for i, item := range value {
		if err := validateNode(itemDef, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

// checkString valida minLength/maxLength (em runes, nao bytes).
func checkString(definition map[string]any, value string, path string) error {
	length := float64(len([]rune(value)))
	if minLength, ok := numberOf(definition, "minLength"); ok && length < minLength {
		return fmt.Errorf("%s: esperado ao menos %d caracteres", label(path), int(minLength))
	}
	if maxLength, ok := numberOf(definition, "maxLength"); ok && length > maxLength {
		return fmt.Errorf("%s: esperado no maximo %d caracteres", label(path), int(maxLength))
	}
	return nil
}

// checkNumber valida minimum/maximum.
func checkNumber(definition map[string]any, value float64, path string) error {
	if minimum, ok := numberOf(definition, "minimum"); ok && value < minimum {
		return fmt.Errorf("%s: esperado >= %v", label(path), minimum)
	}
	if maximum, ok := numberOf(definition, "maximum"); ok && value > maximum {
		return fmt.Errorf("%s: esperado <= %v", label(path), maximum)
	}
	return nil
}

// numberOf le uma keyword numerica do schema.
func numberOf(definition map[string]any, key string) (float64, bool) {
	n, ok := definition[key].(float64)
	return n, ok
}

// typeName nomeia o tipo recebido para a mensagem de erro (sem ecoar o valor).
func typeName(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		if v == math.Trunc(v) {
			return "number(integer)"
		}
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "desconhecido"
	}
}

// label nomeia o caminho na mensagem de erro.
func label(path string) string {
	if path == "" {
		return "raiz"
	}
	return fmt.Sprintf("campo %q", path)
}

// join monta o caminho do filho.
func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}
