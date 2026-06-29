package stringsx

import (
	"reflect"
	"testing"
)

func TestFirstNonEmpty(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   string
	}{
		{"todos vazios", []string{"", "  ", "\t"}, ""},
		{"sem argumentos", nil, ""},
		{"primeiro valido", []string{"alpha", "beta"}, "alpha"},
		{"pula vazios e trima", []string{"", "  ", "  bravo  "}, "bravo"},
		{"trima o escolhido", []string{"  charlie "}, "charlie"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FirstNonEmpty(tc.values...); got != tc.want {
				t.Fatalf("FirstNonEmpty(%q) = %q, quer %q", tc.values, got, tc.want)
			}
		})
	}
}

func TestNormalizeIDs(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   []string
	}{
		{"nil vira slice vazio", nil, []string{}},
		{"so vazios", []string{"", "  "}, []string{}},
		{"trim e dedup preserva ordem", []string{" a ", "b", "a", " ", "c", "b"}, []string{"a", "b", "c"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeIDs(tc.values)
			if got == nil {
				t.Fatalf("NormalizeIDs deve devolver slice nao-nil")
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NormalizeIDs(%q) = %q, quer %q", tc.values, got, tc.want)
			}
		})
	}
}

func TestDecodeJSONStringSlice(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"vazio", "", []string{}},
		{"json array", `["x","y"]`, []string{"x", "y"}},
		{"array vazio", `[]`, []string{}},
		{"null vira vazio", `null`, []string{}},
		{"invalido vira vazio", `{nao-json`, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecodeJSONStringSlice([]byte(tc.raw))
			if got == nil {
				t.Fatalf("DecodeJSONStringSlice deve devolver slice nao-nil")
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("DecodeJSONStringSlice(%q) = %q, quer %q", tc.raw, got, tc.want)
			}
		})
	}
}
