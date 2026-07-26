package secretbox_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// testKey e uma chave de 32 bytes SO para teste (nunca reusar fora daqui).
func testKey(t *testing.T, fill byte) []byte {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill + byte(i)
	}
	return key
}

func newBox(t *testing.T, fill byte) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(testKey(t, fill))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return box
}

// TestRoundTrip prova o basico: o que cifra, decifra igual — inclusive vazio,
// unicode e texto longo.
func TestRoundTrip(t *testing.T) {
	box := newBox(t, 1)
	cases := []string{"", "sk-abc1234", "chave com acento: ção", strings.Repeat("x", 4096)}
	for _, plaintext := range cases {
		encoded, err := box.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plaintext, err)
		}
		got, err := box.Decrypt(encoded)
		if err != nil {
			t.Fatalf("Decrypt(%q): %v", plaintext, err)
		}
		if got != plaintext {
			t.Fatalf("round-trip: esperado %q, veio %q", plaintext, got)
		}
	}
}

// TestEncryptHasVersionPrefix: sem prefixo nao ha rotacao (armadilha da spec).
func TestEncryptHasVersionPrefix(t *testing.T) {
	box := newBox(t, 1)
	encoded, err := box.Encrypt("sk-abc1234")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !strings.HasPrefix(encoded, "v1:") {
		t.Fatalf("ciphertext sem prefixo v1: %q", encoded)
	}
	if strings.Contains(encoded, "sk-abc1234") {
		t.Fatal("ciphertext contem o plaintext — nao cifrou")
	}
}

// TestNonceIsRandomPerEncrypt e BLOQUEANTE: nonce reusado em GCM quebra a cifra.
// Duas cifragens do mesmo texto TEM de sair diferentes.
func TestNonceIsRandomPerEncrypt(t *testing.T) {
	box := newBox(t, 1)
	const plaintext = "sk-mesma-chave"
	first, err := box.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	second, err := box.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if first == second {
		t.Fatal("mesmo texto gerou ciphertext identico — nonce esta sendo reusado")
	}
	// Ambos ainda decifram para o mesmo texto.
	for _, encoded := range []string{first, second} {
		got, err := box.Decrypt(encoded)
		if err != nil || got != plaintext {
			t.Fatalf("Decrypt(%q) = (%q,%v)", encoded, got, err)
		}
	}
}

// TestDecryptWrongKeyFails: chave errada FALHA, nunca devolve lixo.
func TestDecryptWrongKeyFails(t *testing.T) {
	encoded, err := newBox(t, 1).Encrypt("sk-abc1234")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := newBox(t, 9).Decrypt(encoded)
	if !errors.Is(err, secretbox.ErrDecrypt) {
		t.Fatalf("esperado ErrDecrypt, veio (%q, %v)", got, err)
	}
	if got != "" {
		t.Fatalf("decrypt com chave errada devolveu conteudo: %q", got)
	}
}

// TestDecryptTamperedFails: GCM autentica — 1 byte trocado tem de falhar.
func TestDecryptTamperedFails(t *testing.T) {
	box := newBox(t, 1)
	encoded, err := box.Encrypt("sk-abc1234")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	blob, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, "v1:"))
	if err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	blob[len(blob)-1] ^= 0xFF
	if _, err := box.Decrypt("v1:" + base64.StdEncoding.EncodeToString(blob)); !errors.Is(err, secretbox.ErrDecrypt) {
		t.Fatalf("esperado ErrDecrypt em dado adulterado, veio %v", err)
	}
}

// TestDecryptRejectsUnknownVersion cobre a rotacao: prefixo desconhecido ou ausente
// nao e adivinhado.
func TestDecryptRejectsUnknownVersion(t *testing.T) {
	box := newBox(t, 1)
	for _, encoded := range []string{"v2:abc", "sem-prefixo", "", "v1abc"} {
		if _, err := box.Decrypt(encoded); !errors.Is(err, secretbox.ErrUnknownVersion) {
			t.Fatalf("Decrypt(%q): esperado ErrUnknownVersion, veio %v", encoded, err)
		}
	}
}

// TestDecryptMalformed: base64 invalido ou curto demais => ErrMalformed.
func TestDecryptMalformed(t *testing.T) {
	box := newBox(t, 1)
	for _, encoded := range []string{"v1:!!!nao-base64!!!", "v1:" + base64.StdEncoding.EncodeToString([]byte("curto"))} {
		if _, err := box.Decrypt(encoded); !errors.Is(err, secretbox.ErrMalformed) {
			t.Fatalf("Decrypt(%q): esperado ErrMalformed, veio %v", encoded, err)
		}
	}
}

func TestOpaqueFingerprintIsStableScopedAndDoesNotExposeInput(t *testing.T) {
	box := newBox(t, 1)
	first := box.OpaqueFingerprint(
		"customer-intelligence.observation-provenance.v1",
		"account-a",
		"client-a",
		"erp",
		"observation-a",
	)
	repeated := box.OpaqueFingerprint(
		"customer-intelligence.observation-provenance.v1",
		"account-a",
		"client-a",
		"erp",
		"observation-a",
	)
	otherClient := box.OpaqueFingerprint(
		"customer-intelligence.observation-provenance.v1",
		"account-a",
		"client-b",
		"erp",
		"observation-a",
	)
	otherKey := newBox(t, 9).OpaqueFingerprint(
		"customer-intelligence.observation-provenance.v1",
		"account-a",
		"client-a",
		"erp",
		"observation-a",
	)
	if first == "" || first != repeated {
		t.Fatalf("fingerprint nao estavel: first=%q repeated=%q", first, repeated)
	}
	if first == otherClient || first == otherKey {
		t.Fatalf("fingerprint nao ficou escopado: %q", first)
	}
	for _, raw := range []string{"account-a", "client-a", "observation-a"} {
		if strings.Contains(first, raw) {
			t.Fatalf("fingerprint expos input %q: %q", raw, first)
		}
	}
}

// TestNewRejectsBadKeySize: so AES-256 (32 bytes).
func TestNewRejectsBadKeySize(t *testing.T) {
	for _, size := range []int{0, 16, 24, 31, 33, 64} {
		if _, err := secretbox.New(make([]byte, size)); !errors.Is(err, secretbox.ErrKeySize) {
			t.Fatalf("New(%d bytes): esperado ErrKeySize, veio %v", size, err)
		}
	}
}

// TestFromEnv cobre o fail-fast do boot: ausente, base64 invalido, tamanho errado,
// e o caminho feliz.
func TestFromEnv(t *testing.T) {
	t.Run("ausente => ErrKeyMissing", func(t *testing.T) {
		t.Setenv(secretbox.EnvKey, "")
		if _, err := secretbox.FromEnv(); !errors.Is(err, secretbox.ErrKeyMissing) {
			t.Fatalf("esperado ErrKeyMissing, veio %v", err)
		}
	})

	t.Run("base64 invalido => erro nomeando a env", func(t *testing.T) {
		t.Setenv(secretbox.EnvKey, "isso!nao!e!base64")
		_, err := secretbox.FromEnv()
		if err == nil {
			t.Fatal("esperado erro para base64 invalido")
		}
		if !strings.Contains(err.Error(), secretbox.EnvKey) {
			t.Fatalf("erro nao nomeia a env %s: %v", secretbox.EnvKey, err)
		}
	})

	t.Run("tamanho errado => erro", func(t *testing.T) {
		t.Setenv(secretbox.EnvKey, base64.StdEncoding.EncodeToString(make([]byte, 16)))
		if _, err := secretbox.FromEnv(); err == nil {
			t.Fatal("esperado erro para chave de 16 bytes")
		}
	})

	t.Run("chave valida => Box funcional", func(t *testing.T) {
		t.Setenv(secretbox.EnvKey, base64.StdEncoding.EncodeToString(testKey(t, 3)))
		box, err := secretbox.FromEnv()
		if err != nil {
			t.Fatalf("FromEnv: %v", err)
		}
		encoded, err := box.Encrypt("sk-boot")
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		if got, err := box.Decrypt(encoded); err != nil || got != "sk-boot" {
			t.Fatalf("round-trip pos-FromEnv: (%q,%v)", got, err)
		}
	})
}

// TestMask espelha a regra do calendario (secrets.go:44).
func TestMask(t *testing.T) {
	cases := []struct {
		in   string
		want secretbox.Status
	}{
		{"sk-abc1234", secretbox.Status{Set: true, Last4: "1234"}},
		{"", secretbox.Status{Set: false, Last4: ""}},
		{"   ", secretbox.Status{Set: false, Last4: ""}},
		{"1234", secretbox.Status{Set: true, Last4: "1234"}}, // <=4 => o valor todo
		{"ab", secretbox.Status{Set: true, Last4: "ab"}},
	}
	for _, tc := range cases {
		if got := secretbox.Mask(tc.in); got != tc.want {
			t.Fatalf("Mask(%q) = %+v, esperado %+v", tc.in, got, tc.want)
		}
	}
}
