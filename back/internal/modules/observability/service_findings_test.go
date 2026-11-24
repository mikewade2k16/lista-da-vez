package observability

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFindingIDPattern(t *testing.T) {
	for id, want := range map[string]bool{"C-1": true, "H-12": true, "c-1": false, "C-": false, "C-1x": false, "CC-1": false, "": false} {
		if findingIDPattern.MatchString(id) != want {
			t.Errorf("id %q: esperado %v", id, want)
		}
	}
}

func TestValidateFindingPatch(t *testing.T) {
	long := func(n int) *string { return ptr(strings.Repeat("a", n)) }
	cases := []struct {
		name  string
		patch findingPatch
		ok    bool
	}{
		{"vazio", findingPatch{}, false},
		{"status ok", findingPatch{Status: ptr("accepted")}, true},
		{"status invalido", findingPatch{Status: ptr("closed")}, false},
		{"owner 120", findingPatch{Owner: long(120)}, true},
		{"owner 121", findingPatch{Owner: long(121)}, false},
		{"owner vazio limpa", findingPatch{Owner: ptr("")}, true},
		{"nota 2000", findingPatch{Note: long(2000)}, true},
		{"nota 2001", findingPatch{Note: long(2001)}, false},
		{"data ok", findingPatch{DueDate: optionalDate{Set: true, Value: ptr("2026-12-31")}}, true},
		{"data null limpa", findingPatch{DueDate: optionalDate{Set: true}}, true},
		{"data formato", findingPatch{DueDate: optionalDate{Set: true, Value: ptr("31/12/2026")}}, false},
		{"data inexistente", findingPatch{DueDate: optionalDate{Set: true, Value: ptr("2026-02-30")}}, false},
	}
	for _, tc := range cases {
		_, err := validateFindingPatch(tc.patch)
		if (err == nil) != tc.ok {
			t.Errorf("%s: err=%v ok=%v", tc.name, err, tc.ok)
		}
	}
}

func TestFindingPatchJSON(t *testing.T) {
	var p findingPatch
	if err := json.Unmarshal([]byte(`{"dueDate":null,"owner":" Ana "}`), &p); err != nil {
		t.Fatal(err)
	}
	if !p.DueDate.Set || p.DueDate.Value != nil || p.Status != nil {
		t.Errorf("null deve marcar Set sem valor: %+v", p)
	}
	clean, err := validateFindingPatch(p)
	if err != nil || *clean.Owner != "Ana" {
		t.Errorf("owner deve ser aparado: %+v %v", clean, err)
	}
	var absent findingPatch
	_ = json.Unmarshal([]byte(`{"note":"x"}`), &absent)
	if absent.DueDate.Set {
		t.Error("dueDate ausente nao deve marcar Set")
	}
}

func TestFindingOverdue(t *testing.T) {
	past, future, today := ptr("2026-11-23"), ptr("2026-11-25"), "2026-11-24"
	cases := []struct {
		status string
		due    *string
		want   bool
	}{
		{"open", past, true},
		{"open", ptr(today), false},
		{"open", future, false},
		{"open", nil, false},
		{"fixed", past, false},
		{"accepted", past, false},
	}
	for _, tc := range cases {
		if got := findingOverdue(tc.status, tc.due, today); got != tc.want {
			t.Errorf("%s %v: got %v", tc.status, tc.due, got)
		}
	}
}
