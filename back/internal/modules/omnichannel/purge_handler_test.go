package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

func TestParsePurgePayload(t *testing.T) {
	p, err := parsePurgePayload(json.RawMessage(`{"accountId":"a1","date":"2026-07-17","dryRun":true,"seq":2}`))
	if err != nil {
		t.Fatalf("payload valido: %v", err)
	}
	if p.AccountID != "a1" || p.Date != "2026-07-17" || !p.DryRun || p.Seq != 2 {
		t.Fatalf("payload mal desserializado: %+v", p)
	}
	if _, err := parsePurgePayload(nil); err == nil {
		t.Error("payload vazio devia dar erro")
	}
	if _, err := parsePurgePayload(json.RawMessage(`{`)); err == nil {
		t.Error("payload ilegivel devia dar erro")
	}
}

// A checagem de isolamento roda ANTES de tocar qualquer dependencia — por isso um handler
// com deps nil ja prova a regra: payload apontando outra conta que a do job = rejeitado.
func TestHandleRejectsAccountMismatch(t *testing.T) {
	h := &PurgeHandler{}
	job := jobs.Job{AccountID: "A", Kind: PurgeAccountJobKind, Payload: json.RawMessage(`{"accountId":"B"}`)}
	if err := h.Handle(context.Background(), job); !errors.Is(err, errPurgeAccountMismatch) {
		t.Fatalf("conta divergente devia ser rejeitada, veio %v", err)
	}
	// accountId vazio no payload tambem nao pode passar.
	job.Payload = json.RawMessage(`{"accountId":""}`)
	if err := h.Handle(context.Background(), job); !errors.Is(err, errPurgeAccountMismatch) {
		t.Fatalf("accountId vazio devia ser rejeitado, veio %v", err)
	}
}

func TestCutoffFor(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	if got := cutoffFor(now, 30); !got.Equal(now.AddDate(0, 0, -30)) {
		t.Errorf("cutoff 30d = %v, quer %v", got, now.AddDate(0, 0, -30))
	}
	// days invalido (0) e clampado para 1: NUNCA um cutoff no futuro/agora (que apagaria tudo).
	if got := cutoffFor(now, 0); !got.Equal(now.AddDate(0, 0, -1)) {
		t.Errorf("cutoff 0d devia clampar para 1d, veio %v", got)
	}
	if got := cutoffFor(now, 90); got.After(now) {
		t.Error("cutoff nunca pode cair no futuro")
	}
}

func TestPurgeKeys(t *testing.T) {
	if got := dailyPurgeKey("a1", "2026-07-17"); got != "purge:a1:2026-07-17" {
		t.Errorf("dailyPurgeKey = %q", got)
	}
	if got := continuationKey("a1", "2026-07-17", 3); got != "purge:a1:2026-07-17:cont:3" {
		t.Errorf("continuationKey = %q", got)
	}
	if got := purgeOrderingKey("a1"); got != "purge:a1" {
		t.Errorf("purgeOrderingKey = %q", got)
	}
}
