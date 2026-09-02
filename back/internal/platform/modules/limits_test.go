package modules

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestLookupInt(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		key     string
		want    int64
		wantSet bool
	}{
		{"presente", `{"max_whatsapp_numbers":2}`, "max_whatsapp_numbers", 2, true},
		{"ausente", `{"outra":9}`, "max_whatsapp_numbers", 0, false},
		{"config vazia", ``, "x", 0, false},
		{"objeto vazio", `{}`, "x", 0, false},
		{"negativo tratado como nao-configurado", `{"x":-1}`, "x", 0, false},
		{"json invalido", `{nao é json`, "x", 0, false},
		{"zero e valido (bloqueia tudo)", `{"x":0}`, "x", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := lookupInt([]byte(tc.raw), tc.key)
			if ok != tc.wantSet || got != tc.want {
				t.Fatalf("lookupInt = (%d,%v), queria (%d,%v)", got, ok, tc.want, tc.wantSet)
			}
		})
	}
}

func TestLookupModuleInt(t *testing.T) {
	raw := []byte(`{"omnichannel":{"monthly_ai_runs":5000},"crm":{"x":1}}`)

	if v, ok := lookupModuleInt(raw, "omnichannel", "monthly_ai_runs"); !ok || v != 5000 {
		t.Errorf("omnichannel.monthly_ai_runs = (%d,%v), queria (5000,true)", v, ok)
	}
	if _, ok := lookupModuleInt(raw, "omnichannel", "inexistente"); ok {
		t.Error("chave inexistente deveria dar ok=false")
	}
	if _, ok := lookupModuleInt(raw, "modulo_desconhecido", "x"); ok {
		t.Error("modulo inexistente deveria dar ok=false")
	}
}

func TestErrLimitExceeded(t *testing.T) {
	err := error(&ErrLimitExceeded{Key: "max_whatsapp_numbers", Limit: 2, Current: 2})

	if !IsLimitExceeded(err) {
		t.Fatal("IsLimitExceeded deveria reconhecer o erro")
	}
	if IsLimitExceeded(errors.New("outro")) {
		t.Fatal("IsLimitExceeded nao deveria casar com erro qualquer")
	}

	var le *ErrLimitExceeded
	if !errors.As(err, &le) || le.Key != "max_whatsapp_numbers" || le.Limit != 2 || le.Current != 2 {
		t.Fatalf("campos do erro nao preservados: %+v", le)
	}
	// A mensagem tem que citar chave, atual e teto — 409 acionavel (principio 5).
	msg := err.Error()
	for _, sub := range []string{"max_whatsapp_numbers", "2"} {
		if !contains(msg, sub) {
			t.Errorf("mensagem %q nao cita %q", msg, sub)
		}
	}
}

// checkThreshold espelha a regra de Check sem tocar o banco: estoura quando current ja
// alcancou o teto (o proximo recurso passaria do limite).
func checkThreshold(limit Limit, current int64) error {
	if !limit.Set {
		return nil
	}
	if current >= limit.Value {
		return &ErrLimitExceeded{Limit: limit.Value, Current: current}
	}
	return nil
}

func TestCheckThreshold(t *testing.T) {
	semLimite := Limit{Set: false}
	if err := checkThreshold(semLimite, 999999); err != nil {
		t.Errorf("sem limite nunca estoura: %v", err)
	}

	lim := Limit{Value: 2, Set: true, Source: "account"}
	if err := checkThreshold(lim, 1); err != nil {
		t.Errorf("1 de 2 nao deveria estourar: %v", err)
	}
	if err := checkThreshold(lim, 2); !IsLimitExceeded(err) {
		t.Errorf("2 de 2 deveria estourar (o 3o passaria): %v", err)
	}

	zero := Limit{Value: 0, Set: true, Source: "account"}
	if err := checkThreshold(zero, 0); !IsLimitExceeded(err) {
		t.Errorf("limite zero deve bloquear a primeira criacao: %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// garante que o formato do default de plataforma parseia como esperado.
func TestPlatformDefaultShape(t *testing.T) {
	raw, _ := json.Marshal(map[string]map[string]int{
		"omnichannel": {"max_whatsapp_numbers": 2, "monthly_ai_runs": 5000},
	})
	if v, ok := lookupModuleInt(raw, "omnichannel", "max_whatsapp_numbers"); !ok || v != 2 {
		t.Fatalf("shape do default nao bate: (%d,%v)", v, ok)
	}
}
