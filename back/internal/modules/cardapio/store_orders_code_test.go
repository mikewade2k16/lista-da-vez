package cardapio

import (
	"strings"
	"testing"
)

// TestGenerateOrderCode garante formato (tamanho + alfabeto) e baixa colisao do
// codigo de pedido voltado ao cliente (WS-G).
func TestGenerateOrderCode(t *testing.T) {
	seen := make(map[string]struct{}, 500)
	for i := 0; i < 500; i++ {
		code, err := generateOrderCode()
		if err != nil {
			t.Fatalf("generateOrderCode: %v", err)
		}
		if len(code) != orderCodeLen {
			t.Fatalf("tamanho %d, esperado %d (%q)", len(code), orderCodeLen, code)
		}
		for _, r := range code {
			if !strings.ContainsRune(orderCodeAlphabet, r) {
				t.Fatalf("char fora do alfabeto: %q em %q", r, code)
			}
		}
		seen[code] = struct{}{}
	}
	// 500 codigos num espaco de 32^6 (~1e9): colisao deve ser raríssima.
	if len(seen) < 495 {
		t.Fatalf("colisoes demais: %d unicos de 500", len(seen))
	}
}
