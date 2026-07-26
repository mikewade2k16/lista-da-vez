package customerdata

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func validSegmentDraft() SegmentDraftInput {
	return SegmentDraftInput{
		FilterSchemaVersion: "segment.filter.v1",
		FieldCatalogVersion: SegmentFieldCatalogVersion,
		FilterAST: json.RawMessage(`{
			"schemaVersion":"segment.filter.v1",
			"root":{"type":"group","operator":"and","children":[
				{"type":"predicate","fieldKey":"relationship.lifecycle_status","operator":"in","value":["lead","prospect"]},
				{"type":"predicate","fieldKey":"relationship.archived","operator":"is_false"}
			]}
		}`),
		EvaluationPolicy: json.RawMessage(`{}`),
	}
}

func TestSegmentFilterValidationAndScopedCompilation(t *testing.T) {
	t.Parallel()
	filter, hash, cost, err := ValidateSegmentDraft(validSegmentDraft())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(hash) != 64 || cost < 3 {
		t.Fatalf("unexpected hash/cost: %q %d", hash, cost)
	}
	compiled, err := CompileSegmentFilter(
		Scope{AccountID: "account-a", ClientAccountID: "client-a"},
		filter, time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !strings.HasPrefix(compiled.Where, "r.account_id = $1::uuid and r.client_account_id = $2::uuid") {
		t.Fatalf("scope is not mandatory in compiled SQL: %s", compiled.Where)
	}
	if len(compiled.Args) < 3 || compiled.Args[0] != "account-a" || compiled.Args[1] != "client-a" {
		t.Fatalf("unexpected args: %#v", compiled.Args)
	}
}

func TestSegmentCompilerKeepsMetacharactersInArguments(t *testing.T) {
	t.Parallel()
	draft := validSegmentDraft()
	draft.FilterAST = json.RawMessage(`{
		"schemaVersion":"segment.filter.v1",
		"root":{"type":"predicate","fieldKey":"relationship.display_name","operator":"contains","value":"x%' OR 1=1 --"}
	}`)
	filter, _, _, err := ValidateSegmentDraft(draft)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	compiled, err := CompileSegmentFilter(
		Scope{AccountID: "a", ClientAccountID: "c"}, filter, time.Now(),
	)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if strings.Contains(compiled.Where, "OR 1=1") {
		t.Fatalf("predicate value leaked into SQL: %s", compiled.Where)
	}
	if got := compiled.Args[len(compiled.Args)-1].(string); !strings.Contains(got, "or 1=1") {
		t.Fatalf("predicate was not retained as a bound argument: %q", got)
	}
}

func TestSegmentFilterRejectsOpenGrammar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "unknown field",
			raw:  `{"schemaVersion":"segment.filter.v1","root":{"type":"predicate","fieldKey":"relationship.drop_table","operator":"eq","value":"x"}}`,
		},
		{
			name: "sql field",
			raw:  `{"schemaVersion":"segment.filter.v1","root":{"type":"predicate","fieldKey":"r.name; drop table users","operator":"eq","value":"x"}}`,
		},
		{
			name: "unknown node key",
			raw:  `{"schemaVersion":"segment.filter.v1","root":{"type":"predicate","fieldKey":"relationship.lifecycle_status","operator":"eq","value":"lead","sql":"true"}}`,
		},
		{
			name: "operator not allowlisted",
			raw:  `{"schemaVersion":"segment.filter.v1","root":{"type":"predicate","fieldKey":"relationship.display_name","operator":"regex","value":".*"}}`,
		},
		{
			name: "value on value-less operator",
			raw:  `{"schemaVersion":"segment.filter.v1","root":{"type":"predicate","fieldKey":"relationship.archived","operator":"is_false","value":false}}`,
		},
		{
			name: "top-level extra",
			raw:  `{"schemaVersion":"segment.filter.v1","root":{"type":"predicate","fieldKey":"relationship.archived","operator":"is_false"},"url":"https://example.invalid"}`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			draft := validSegmentDraft()
			draft.FilterAST = json.RawMessage(test.raw)
			_, _, _, err := ValidateSegmentDraft(draft)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestSegmentFilterRejectsDepthAndListCaps(t *testing.T) {
	t.Parallel()
	node := map[string]any{
		"type": "predicate", "fieldKey": "relationship.lifecycle_status",
		"operator": "eq", "value": "lead",
	}
	for i := 0; i < maxFilterDepth+1; i++ {
		node = map[string]any{"type": "group", "operator": "and", "children": []any{node}}
	}
	raw, _ := json.Marshal(map[string]any{"schemaVersion": segmentFilterSchema, "root": node})
	draft := validSegmentDraft()
	draft.FilterAST = raw
	if _, _, _, err := ValidateSegmentDraft(draft); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected depth rejection, got %v", err)
	}

	values := make([]string, maxFilterList+1)
	for i := range values {
		values[i] = "lead"
	}
	raw, _ = json.Marshal(map[string]any{
		"schemaVersion": segmentFilterSchema,
		"root": map[string]any{
			"type": "predicate", "fieldKey": "relationship.lifecycle_status",
			"operator": "in", "value": values,
		},
	})
	draft.FilterAST = raw
	if _, _, _, err := ValidateSegmentDraft(draft); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected list rejection, got %v", err)
	}
}
