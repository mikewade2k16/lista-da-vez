package cardapio

import (
	"reflect"
	"testing"
)

func TestValidateProductTags(t *testing.T) {
	for _, tc := range []struct {
		name string
		tags []string
		want []string
	}{
		{"zero", nil, []string{}},
		{"clear", []string{}, []string{}},
		{"multiple", []string{" novo ", "Picante", "NOVO", "", "  "}, []string{"novo", "Picante"}},
		{"commas in label", []string{"Novo, especial"}, []string{"Novo, especial"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := ProductInput{Slug: "produto", Name: "Produto", Tags: tc.tags}
			if err := validateProductInput(&in); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(in.Tags, tc.want) {
				t.Fatalf("tags = %#v, want %#v", in.Tags, tc.want)
			}
		})
	}
}
