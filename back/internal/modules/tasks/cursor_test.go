package tasks

import (
	"testing"
	"time"
)

func TestListTasksCursor_RoundTrip(t *testing.T) {
	original := listTasksCursor{
		SortOrder: 1024.5,
		CreatedAt: time.Date(2026, 5, 15, 12, 30, 45, 0, time.UTC),
		ID:        "00000000-0000-0000-0000-000000000abc",
	}

	encoded := encodeListTasksCursor(original)
	if encoded == "" {
		t.Fatal("encode deveria retornar uma string nao-vazia para cursor valido")
	}

	decoded, ok := decodeListTasksCursor(encoded)
	if !ok {
		t.Fatalf("decode deveria aceitar o cursor recem-encodado")
	}
	if decoded.SortOrder != original.SortOrder {
		t.Errorf("SortOrder diff: got %v, want %v", decoded.SortOrder, original.SortOrder)
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt diff: got %v, want %v", decoded.CreatedAt, original.CreatedAt)
	}
	if decoded.ID != original.ID {
		t.Errorf("ID diff: got %q, want %q", decoded.ID, original.ID)
	}
}

func TestListTasksCursor_DecodeEmptyReturnsFalse(t *testing.T) {
	cases := []string{"", "   ", "\n\t"}
	for _, raw := range cases {
		if _, ok := decodeListTasksCursor(raw); ok {
			t.Errorf("decode de %q deveria retornar ok=false (sem cursor)", raw)
		}
	}
}

func TestListTasksCursor_DecodeInvalidReturnsFalse(t *testing.T) {
	cases := []string{
		"not-base64",
		"!!!@@@",
		// base64 valido, mas nao e' JSON valido
		"YWJjZGVm",
		// JSON valido, mas sem ID (campo obrigatorio)
		"eyJzIjoxLCJjIjoiMjAyNi0wNS0xNVQwMDowMDowMFoifQ",
	}
	for _, raw := range cases {
		if _, ok := decodeListTasksCursor(raw); ok {
			t.Errorf("decode de %q deveria retornar ok=false (cursor invalido)", raw)
		}
	}
}

func TestListTasksCursor_EncodeIsBase64URLSafe(t *testing.T) {
	// Cursor com IDs/timestamps reais nao pode conter caracteres que quebram URL (+, /, =).
	cursor := listTasksCursor{
		SortOrder: 99,
		CreatedAt: time.Now().UTC(),
		ID:        "abc-123-xyz",
	}
	encoded := encodeListTasksCursor(cursor)
	for _, ch := range encoded {
		if ch == '+' || ch == '/' || ch == '=' {
			t.Errorf("encode deveria ser URL-safe (sem +,/,=); got char %q em %q", ch, encoded)
		}
	}
}
