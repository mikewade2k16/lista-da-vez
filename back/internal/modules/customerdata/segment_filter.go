package customerdata

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	segmentFilterSchema = "segment.filter.v1"
	maxFilterDepth      = 6
	maxFilterNodes      = 64
	maxFilterChildren   = 24
	maxFilterList       = 50
	maxFilterString     = 256
)

type filterValidationState struct {
	nodes int
}

func ValidateSegmentDraft(draft SegmentDraftInput) (SegmentFilter, string, int, error) {
	if draft.FilterSchemaVersion != segmentFilterSchema {
		return SegmentFilter{}, "", 0, invalid("filterSchemaVersion", "unsupported")
	}
	if draft.FieldCatalogVersion != SegmentFieldCatalogVersion {
		return SegmentFilter{}, "", 0, invalid("fieldCatalogVersion", "stale_or_unknown")
	}
	if len(draft.FilterAST) == 0 || len(draft.FilterAST) > 64*1024 {
		return SegmentFilter{}, "", 0, invalid("filterAst", "invalid_size")
	}
	filter, cost, err := decodeFilterStrict(draft.FilterAST)
	if err != nil {
		return SegmentFilter{}, "", 0, err
	}
	policy, err := normalizeObject(draft.EvaluationPolicy, "evaluationPolicy", 16*1024)
	if err != nil {
		return SegmentFilter{}, "", 0, err
	}
	normalizedFilter, err := json.Marshal(filter)
	if err != nil {
		return SegmentFilter{}, "", 0, err
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(SegmentFieldCatalogVersion))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(normalizedFilter)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(policy)
	return filter, hex.EncodeToString(hash.Sum(nil)), cost, nil
}

func decodeFilterStrict(raw json.RawMessage) (SegmentFilter, int, error) {
	root, err := decodeObject(raw)
	if err != nil {
		return SegmentFilter{}, 0, invalid("filterAst", "must_be_object")
	}
	if err := exactKeys(root, "schemaVersion", "root"); err != nil {
		return SegmentFilter{}, 0, err
	}
	var schemaVersion string
	if err := json.Unmarshal(root["schemaVersion"], &schemaVersion); err != nil || schemaVersion != segmentFilterSchema {
		return SegmentFilter{}, 0, invalid("filterAst.schemaVersion", "unsupported")
	}
	state := &filterValidationState{}
	node, err := decodeNode(root["root"], 1, state)
	if err != nil {
		return SegmentFilter{}, 0, err
	}
	return SegmentFilter{SchemaVersion: schemaVersion, Root: node}, state.nodes, nil
}

func decodeNode(raw json.RawMessage, depth int, state *filterValidationState) (FilterNode, error) {
	if depth > maxFilterDepth {
		return FilterNode{}, invalid("filterAst.root", "max_depth")
	}
	state.nodes++
	if state.nodes > maxFilterNodes {
		return FilterNode{}, invalid("filterAst.root", "max_nodes")
	}
	object, err := decodeObject(raw)
	if err != nil {
		return FilterNode{}, invalid("filterAst.node", "must_be_object")
	}
	var kind string
	if err := json.Unmarshal(object["type"], &kind); err != nil {
		return FilterNode{}, invalid("filterAst.node.type", "required")
	}
	switch kind {
	case "group":
		if err := exactKeys(object, "type", "operator", "children"); err != nil {
			return FilterNode{}, err
		}
		var operator string
		if err := json.Unmarshal(object["operator"], &operator); err != nil || (operator != "and" && operator != "or") {
			return FilterNode{}, invalid("filterAst.group.operator", "unsupported")
		}
		var childrenRaw []json.RawMessage
		if err := json.Unmarshal(object["children"], &childrenRaw); err != nil ||
			len(childrenRaw) == 0 || len(childrenRaw) > maxFilterChildren {
			return FilterNode{}, invalid("filterAst.group.children", "invalid_count")
		}
		children := make([]FilterNode, 0, len(childrenRaw))
		for _, childRaw := range childrenRaw {
			child, err := decodeNode(childRaw, depth+1, state)
			if err != nil {
				return FilterNode{}, err
			}
			children = append(children, child)
		}
		return FilterNode{Type: "group", Operator: operator, Children: children}, nil
	case "predicate":
		allowed := []string{"type", "fieldKey", "operator", "value"}
		operatorRaw := object["operator"]
		var operator string
		if err := json.Unmarshal(operatorRaw, &operator); err != nil {
			return FilterNode{}, invalid("filterAst.predicate.operator", "required")
		}
		if operator == "exists" || operator == "not_exists" || operator == "is_true" || operator == "is_false" {
			allowed = []string{"type", "fieldKey", "operator"}
		}
		if err := exactKeys(object, allowed...); err != nil {
			return FilterNode{}, err
		}
		var fieldKey string
		if err := json.Unmarshal(object["fieldKey"], &fieldKey); err != nil {
			return FilterNode{}, invalid("filterAst.predicate.fieldKey", "required")
		}
		field, ok := segmentFields[fieldKey]
		if !ok {
			return FilterNode{}, invalid("filterAst.predicate.fieldKey", "unknown")
		}
		if !contains(field.Operators, operator) {
			return FilterNode{}, invalid("filterAst.predicate.operator", "not_allowed_for_field")
		}
		value := object["value"]
		if err := validatePredicateValue(field, operator, value); err != nil {
			return FilterNode{}, err
		}
		return FilterNode{Type: "predicate", FieldKey: fieldKey, Operator: operator, Value: normalizeRaw(value)}, nil
	default:
		return FilterNode{}, invalid("filterAst.node.type", "unsupported")
	}
}

func validatePredicateValue(field SegmentFieldDefinition, operator string, raw json.RawMessage) error {
	switch operator {
	case "exists", "not_exists", "is_true", "is_false":
		if len(raw) != 0 {
			return invalid("filterAst.predicate.value", "must_be_absent")
		}
		return nil
	case "in", "not_in", "between":
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil || len(values) == 0 || len(values) > maxFilterList {
			return invalid("filterAst.predicate.value", "invalid_list")
		}
		if operator == "between" && len(values) != 2 {
			return invalid("filterAst.predicate.value", "between_requires_two")
		}
		for _, value := range values {
			if err := validateScalar(field.DataType, value); err != nil {
				return err
			}
		}
		return nil
	case "within_last":
		object, err := decodeObject(raw)
		if err != nil {
			return invalid("filterAst.predicate.value", "invalid_window")
		}
		if err := exactKeys(object, "amount", "unit"); err != nil {
			return err
		}
		var amount int
		var unit string
		if json.Unmarshal(object["amount"], &amount) != nil || amount < 1 || amount > 3650 {
			return invalid("filterAst.predicate.value.amount", "out_of_range")
		}
		if json.Unmarshal(object["unit"], &unit) != nil || (unit != "day" && unit != "hour") {
			return invalid("filterAst.predicate.value.unit", "unsupported")
		}
		return nil
	default:
		return validateScalar(field.DataType, raw)
	}
}

func validateScalar(dataType string, raw json.RawMessage) error {
	switch dataType {
	case "string":
		var value string
		if json.Unmarshal(raw, &value) != nil || strings.TrimSpace(value) == "" || len(value) > maxFilterString {
			return invalid("filterAst.predicate.value", "invalid_string")
		}
	case "datetime":
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return invalid("filterAst.predicate.value", "invalid_datetime")
		}
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return invalid("filterAst.predicate.value", "invalid_datetime")
		}
	case "boolean":
		var value bool
		if json.Unmarshal(raw, &value) != nil {
			return invalid("filterAst.predicate.value", "invalid_boolean")
		}
	default:
		return invalid("filterAst.predicate.value", "unsupported_type")
	}
	return nil
}

func CompileSegmentFilter(scope Scope, filter SegmentFilter, asOf time.Time) (CompiledFilter, error) {
	if strings.TrimSpace(scope.AccountID) == "" || strings.TrimSpace(scope.ClientAccountID) == "" {
		return CompiledFilter{}, ErrNotFound
	}
	args := []any{scope.AccountID, scope.ClientAccountID}
	where, cost, err := compileNode(filter.Root, asOf.UTC(), &args)
	if err != nil {
		return CompiledFilter{}, err
	}
	return CompiledFilter{
		Where: "r.account_id = $1::uuid and r.client_account_id = $2::uuid and (" + where + ")",
		Args:  args,
		Cost:  cost,
	}, nil
}

func compileNode(node FilterNode, asOf time.Time, args *[]any) (string, int, error) {
	if node.Type == "group" {
		parts := make([]string, 0, len(node.Children))
		cost := 1
		for _, child := range node.Children {
			part, childCost, err := compileNode(child, asOf, args)
			if err != nil {
				return "", 0, err
			}
			parts = append(parts, "("+part+")")
			cost += childCost
		}
		joiner := " and "
		if node.Operator == "or" {
			joiner = " or "
		}
		return strings.Join(parts, joiner), cost, nil
	}
	field, ok := segmentFields[node.FieldKey]
	if !ok || !contains(field.Operators, node.Operator) {
		return "", 0, invalid("filterAst", "unvalidated_node")
	}
	column := map[string]string{
		"relationship.lifecycle_status": "r.lifecycle_status",
		"relationship.display_name":     "r.display_name",
		"relationship.owner_user_id":    "r.owner_user_id::text",
		"relationship.last_seen_at":     "r.last_seen_at",
		"relationship.created_at":       "r.created_at",
		"relationship.archived":         "r.archived_at",
	}[node.FieldKey]
	if node.FieldKey == "relationship.tag" {
		return compileTag(node, args)
	}
	switch node.Operator {
	case "exists":
		return column + " is not null", 1, nil
	case "not_exists":
		return column + " is null", 1, nil
	case "is_true":
		return column + " is not null", 1, nil
	case "is_false":
		return column + " is null", 1, nil
	case "within_last":
		var window struct {
			Amount int    `json:"amount"`
			Unit   string `json:"unit"`
		}
		if json.Unmarshal(node.Value, &window) != nil {
			return "", 0, invalid("filterAst", "invalid_window")
		}
		duration := time.Duration(window.Amount) * 24 * time.Hour
		if window.Unit == "hour" {
			duration = time.Duration(window.Amount) * time.Hour
		}
		return column + " >= " + addArg(args, asOf.Add(-duration)), 2, nil
	case "between":
		var values []json.RawMessage
		_ = json.Unmarshal(node.Value, &values)
		first, err := scalarForArgument(field.DataType, values[0])
		if err != nil {
			return "", 0, err
		}
		second, err := scalarForArgument(field.DataType, values[1])
		if err != nil {
			return "", 0, err
		}
		return column + " between " + addArg(args, first) + " and " + addArg(args, second), 2, nil
	case "in", "not_in":
		var rawValues []json.RawMessage
		_ = json.Unmarshal(node.Value, &rawValues)
		values := make([]string, 0, len(rawValues))
		for _, raw := range rawValues {
			var value string
			_ = json.Unmarshal(raw, &value)
			values = append(values, value)
		}
		op := "= any"
		if node.Operator == "not_in" {
			op = "<> all"
		}
		return column + " " + op + "(" + addArg(args, values) + "::text[])", 1 + len(values), nil
	case "contains", "prefix":
		var value string
		_ = json.Unmarshal(node.Value, &value)
		pattern := escapeLike(strings.ToLower(value))
		if node.Operator == "contains" {
			pattern = "%" + pattern + "%"
		} else {
			pattern += "%"
		}
		return "lower(" + column + ") like " + addArg(args, pattern) + " escape '\\'", 2, nil
	default:
		value, err := scalarForArgument(field.DataType, node.Value)
		if err != nil {
			return "", 0, err
		}
		operator := map[string]string{
			"eq": "=", "neq": "<>", "before": "<", "after": ">",
		}[node.Operator]
		if operator == "" {
			return "", 0, invalid("filterAst", "unsupported_operator")
		}
		return column + " " + operator + " " + addArg(args, value), 1, nil
	}
}

func compileTag(node FilterNode, args *[]any) (string, int, error) {
	if node.Operator == "eq" || node.Operator == "neq" {
		var value string
		_ = json.Unmarshal(node.Value, &value)
		expression := "r.tags ? " + addArg(args, strings.ToLower(value))
		if node.Operator == "neq" {
			expression = "not (" + expression + ")"
		}
		return expression, 2, nil
	}
	var rawValues []json.RawMessage
	_ = json.Unmarshal(node.Value, &rawValues)
	parts := make([]string, 0, len(rawValues))
	for _, raw := range rawValues {
		var value string
		_ = json.Unmarshal(raw, &value)
		parts = append(parts, "r.tags ? "+addArg(args, strings.ToLower(value)))
	}
	joiner := " or "
	if node.Operator == "not_in" {
		for i := range parts {
			parts[i] = "not (" + parts[i] + ")"
		}
		joiner = " and "
	}
	return "(" + strings.Join(parts, joiner) + ")", len(parts) * 2, nil
}

func scalarForArgument(dataType string, raw json.RawMessage) (any, error) {
	switch dataType {
	case "string":
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return nil, invalid("filterAst", "invalid_string")
		}
		return value, nil
	case "datetime":
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return nil, invalid("filterAst", "invalid_datetime")
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, invalid("filterAst", "invalid_datetime")
		}
		return parsed, nil
	case "boolean":
		var value bool
		if json.Unmarshal(raw, &value) != nil {
			return nil, invalid("filterAst", "invalid_boolean")
		}
		return value, nil
	default:
		return nil, invalid("filterAst", "unsupported_type")
	}
}

func addArg(args *[]any, value any) string {
	*args = append(*args, value)
	return "$" + strconv.Itoa(len(*args))
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func decodeObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil || object == nil {
		return nil, err
	}
	if decoder.More() {
		return nil, fmt.Errorf("trailing value")
	}
	return object, nil
}

func exactKeys(object map[string]json.RawMessage, allowed ...string) error {
	set := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for key := range object {
		if _, ok := set[key]; !ok {
			return invalid("filterAst", "unknown_field")
		}
	}
	for _, required := range allowed {
		if _, ok := object[required]; !ok {
			return invalid("filterAst", "missing_field")
		}
	}
	return nil
}

func normalizeRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return raw
	}
	normalized, _ := json.Marshal(value)
	return normalized
}

func normalizeObject(raw json.RawMessage, field string, maxBytes int) ([]byte, error) {
	if len(raw) == 0 {
		return []byte("{}"), nil
	}
	if len(raw) > maxBytes {
		return nil, invalid(field, "too_large")
	}
	object, err := decodeObject(raw)
	if err != nil {
		return nil, invalid(field, "must_be_object")
	}
	return json.Marshal(object)
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
