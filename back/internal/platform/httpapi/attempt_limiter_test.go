package httpapi

import (
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestLimiter(clock *fakeClock, max int) *AttemptLimiter {
	return NewAttemptLimiter(AttemptLimiterOptions{MaxFailures: max, Window: 15 * time.Minute, BaseLock: time.Minute, MaxLock: 8 * time.Minute, Now: clock.Now})
}

func TestAttemptLimiter_BlocksAtThresholdAndBacksOff(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1000, 0)}
	limiter := newTestLimiter(clock, 5)

	for index := 0; index < 4; index++ {
		limiter.Fail("a")
		if ok, _ := limiter.Check("a"); !ok {
			t.Fatalf("falha %d ainda nao deveria bloquear", index+1)
		}
	}
	limiter.Fail("a") // 5a falha
	ok, retry := limiter.Check("a")
	if ok || retry != time.Minute {
		t.Fatalf("5a falha deveria bloquear por 1min; ok=%v retry=%v", ok, retry)
	}

	clock.Advance(61 * time.Second)
	if ok, _ := limiter.Check("a"); !ok {
		t.Fatal("bloqueio deveria expirar")
	}
	limiter.Fail("a") // 6a falha: backoff dobra
	if _, retry := limiter.Check("a"); retry != 2*time.Minute {
		t.Errorf("backoff esperado 2min; got %v", retry)
	}
	for index := 0; index < 10; index++ {
		clock.Advance(10 * time.Minute)
		limiter.Fail("a")
	}
	if _, retry := limiter.Check("a"); retry > 8*time.Minute {
		t.Errorf("bloqueio deve respeitar MaxLock; got %v", retry)
	}
}

func TestAttemptLimiter_ResetAndWindow(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1000, 0)}
	limiter := newTestLimiter(clock, 3)

	for index := 0; index < 3; index++ {
		limiter.Fail("a")
	}
	limiter.Reset("a")
	if ok, _ := limiter.Check("a"); !ok {
		t.Fatal("Reset deveria zerar o bloqueio")
	}

	limiter.Fail("b")
	limiter.Fail("b")
	clock.Advance(16 * time.Minute) // janela vencida: contagem recomeca
	limiter.Fail("b")
	if ok, _ := limiter.Check("b"); !ok {
		t.Fatal("falhas fora da janela nao devem acumular")
	}
}

func TestAttemptLimiter_KeysAreIndependent(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1000, 0)}
	limiter := newTestLimiter(clock, 2)
	limiter.Fail("ip:1")
	limiter.Fail("ip:1")
	if ok, _ := limiter.Check("ip:1"); ok {
		t.Fatal("ip:1 deveria estar bloqueado")
	}
	if ok, _ := limiter.Check("ip:2"); !ok {
		t.Fatal("ip:2 nao deve ser afetado")
	}
}

func TestAttemptLimiter_BoundedMemoryAndSweep(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1000, 0)}
	limiter := NewAttemptLimiter(AttemptLimiterOptions{MaxFailures: 5, MaxEntries: 3, Now: clock.Now})
	for _, key := range []string{"a", "b", "c", "d", "e"} {
		clock.Advance(time.Second)
		limiter.Fail(key)
	}
	if limiter.Len() > 3 {
		t.Errorf("mapa deve respeitar MaxEntries; got %d", limiter.Len())
	}
	clock.Advance(time.Hour)
	limiter.Fail("z")
	if limiter.Len() != 1 {
		t.Errorf("varredura deve remover entradas vencidas; got %d", limiter.Len())
	}
}
