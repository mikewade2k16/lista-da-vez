package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// TestClassify cobre a tabela do canonico §8 linha a linha.
func TestClassify(t *testing.T) {
	cases := []struct {
		name              string
		err               error
		wantAttempts      int
		wantUnrecoverable bool
	}{
		{"timeout de rede => transitorio 5", context.DeadlineExceeded, 5, false},
		{"conexao recusada => transitorio 5", syscall.ECONNREFUSED, 5, false},
		{"conexao resetada => transitorio 5", syscall.ECONNRESET, 5, false},
		{"EOF => transitorio 5", io.EOF, 5, false},
		{"DNS => transitorio 5", &net.DNSError{Err: "no such host"}, 5, false},
		{"400 => unrecoverable 1", &jobs.StatusError{StatusCode: 400}, 1, true},
		{"401 => unrecoverable 1", &jobs.StatusError{StatusCode: 401}, 1, true},
		{"403 => unrecoverable 1", &jobs.StatusError{StatusCode: 403}, 1, true},
		{"404 => unrecoverable 1", &jobs.StatusError{StatusCode: 404}, 1, true},
		{"405 => unrecoverable 1", &jobs.StatusError{StatusCode: 405}, 1, true},
		{"422 => unrecoverable 1", &jobs.StatusError{StatusCode: 422}, 1, true},
		{"429 => rate limited 5", &jobs.StatusError{StatusCode: 429}, 5, false},
		{"500 => 5xx 4", &jobs.StatusError{StatusCode: 500}, 4, false},
		{"503 => 5xx 4", &jobs.StatusError{StatusCode: 503}, 4, false},
		{"599 => 5xx 4", &jobs.StatusError{StatusCode: 599}, 4, false},
		{"sem status => 4", &jobs.StatusError{StatusCode: 0, Err: errors.New("resposta nao-HTTP")}, 4, false},
		{"erro de dominio sem status => 4", errors.New("falha qualquer"), 4, false},
		{"402 => outros 3", &jobs.StatusError{StatusCode: 402}, 3, false},
		{"418 => outros 3", &jobs.StatusError{StatusCode: 418}, 3, false},
		{"301 => outros 3", &jobs.StatusError{StatusCode: 301}, 3, false},
		{"unrecoverable explicito => 1", &jobs.StatusError{StatusCode: 409, Unrecoverable: true}, 1, true},
		{"kind sem handler => unrecoverable 1", jobs.ErrNoHandler, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := jobs.Classify(tc.err)
			if got.MaxAttempts != tc.wantAttempts {
				t.Fatalf("MaxAttempts = %d, esperado %d (classe %q)", got.MaxAttempts, tc.wantAttempts, got.Name)
			}
			if got.Unrecoverable != tc.wantUnrecoverable {
				t.Fatalf("Unrecoverable = %v, esperado %v (classe %q)", got.Unrecoverable, tc.wantUnrecoverable, got.Name)
			}
		})
	}
}

// TestClassifyWrappedError prova que a classificacao atravessa fmt.Errorf("%w").
func TestClassifyWrappedError(t *testing.T) {
	wrapped := fmt.Errorf("enviar ao provider: %w", &jobs.StatusError{StatusCode: 429})
	if got := jobs.Classify(wrapped); got.MaxAttempts != 5 || got.Name != "rate_limited" {
		t.Fatalf("erro embrulhado: classe %q com %d tentativas", got.Name, got.MaxAttempts)
	}
}

// TestBackoff prova a curva min(5s * 2^(attempts-1), 5min) com jitter de ±20%.
func TestBackoff(t *testing.T) {
	cases := []struct {
		attempts int
		base     time.Duration
	}{
		{1, 5 * time.Second},
		{2, 10 * time.Second},
		{3, 20 * time.Second},
		{4, 40 * time.Second},
		{5, 80 * time.Second},
		{7, 5 * time.Minute},   // 320s estoura o teto
		{50, 5 * time.Minute},  // teto
		{200, 5 * time.Minute}, // sem overflow de shift
	}
	for _, tc := range cases {
		min := time.Duration(float64(tc.base) * 0.8)
		max := time.Duration(float64(tc.base) * 1.2)
		for i := 0; i < 50; i++ {
			got := jobs.Backoff(tc.attempts)
			if got < min || got > max {
				t.Fatalf("Backoff(%d) = %v, fora de [%v, %v]", tc.attempts, got, min, max)
			}
		}
	}
}

// TestBackoffHasJitter: sem jitter, N jobs que falharam juntos voltam no mesmo
// instante (thundering herd).
func TestBackoffHasJitter(t *testing.T) {
	seen := map[time.Duration]bool{}
	for i := 0; i < 50; i++ {
		seen[jobs.Backoff(3)] = true
	}
	if len(seen) < 2 {
		t.Fatal("Backoff sempre devolveu o mesmo valor — sem jitter")
	}
}

// TestMaskError prova que credencial nao vaza para last_error (que e persistido).
func TestMaskError(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		leaking string
	}{
		{"bearer", errors.New("401 com header Authorization: Bearer abc123def456"), "abc123def456"},
		{"sk key", errors.New("chave sk-proj-abcdef123456 rejeitada"), "sk-proj-abcdef123456"},
		{"gemini key", errors.New("AIzaSyD1234567890abc invalida"), "AIzaSyD1234567890abc"},
		{"api_key=", errors.New(`falhou com api_key=segredo123`), "segredo123"},
		{"token:", errors.New(`{"token": "xyz789abc"}`), "xyz789abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := jobs.MaskError(tc.err)
			if got == "" {
				t.Fatal("MaskError apagou tudo")
			}
			if strings.Contains(got, tc.leaking) {
				t.Fatalf("MaskError vazou credencial: %q", got)
			}
		})
	}

	t.Run("nil => vazio", func(t *testing.T) {
		if got := jobs.MaskError(nil); got != "" {
			t.Fatalf("MaskError(nil) = %q", got)
		}
	})

	t.Run("truncado", func(t *testing.T) {
		long := make([]byte, 5000)
		for i := range long {
			long[i] = 'x'
		}
		if got := jobs.MaskError(errors.New(string(long))); len(got) > 520 {
			t.Fatalf("MaskError nao truncou: %d chars", len(got))
		}
	})
}

// TestStatusErrorMessage: o Error() nao interpola struct.
func TestStatusErrorMessage(t *testing.T) {
	withCause := &jobs.StatusError{StatusCode: 500, Err: errors.New("boom")}
	if got := withCause.Error(); got != "status 500: boom" {
		t.Fatalf("Error() = %q", got)
	}
	noCause := &jobs.StatusError{StatusCode: 503}
	if got := noCause.Error(); got != "jobs: provider respondeu status 503" {
		t.Fatalf("Error() = %q", got)
	}
	noStatus := &jobs.StatusError{Err: errors.New("sem resposta")}
	if got := noStatus.Error(); got != "sem resposta" {
		t.Fatalf("Error() = %q", got)
	}
}
