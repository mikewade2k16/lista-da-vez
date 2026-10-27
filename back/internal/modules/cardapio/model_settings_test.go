package cardapio

import (
	"encoding/json"
	"testing"
)

func TestSettingsProductPriceDecimals(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"existing menu defaults to whole amounts", `{}`, false},
		{"decimals enabled", `{"productPriceDecimals":true}`, true},
		{"decimals disabled", `{"productPriceDecimals":false}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var settings Settings
			if err := json.Unmarshal([]byte(tc.body), &settings); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(settings)
			if err != nil {
				t.Fatal(err)
			}
			var restored Settings
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			if restored.ProductPriceDecimals != tc.want {
				t.Fatalf("productPriceDecimals = %v, want %v", restored.ProductPriceDecimals, tc.want)
			}
		})
	}
}
