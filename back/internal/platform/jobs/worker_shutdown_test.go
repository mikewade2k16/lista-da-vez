package jobs

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeStore registra os desfechos; Claim entrega os jobs uma unica vez.
type fakeStore struct {
	mu          sync.Mutex
	pending     []Job
	done        []string
	rescheduled []string
	dead        []string
	claims      int
}

func (f *fakeStore) Enqueue(context.Context, NewJob) (string, bool, error) { return "", false, nil }
func (f *fakeStore) Claim(_ context.Context, _ string, _ int) ([]Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claims++
	jobs := f.pending
	f.pending = nil
	return jobs, nil
}
func (f *fakeStore) MarkDone(_ context.Context, _, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.done = append(f.done, id)
	return nil
}
func (f *fakeStore) Reschedule(_ context.Context, _, id string, _ time.Time, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rescheduled = append(f.rescheduled, id)
	return nil
}
func (f *fakeStore) MarkDead(_ context.Context, _, id, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dead = append(f.dead, id)
	return nil
}
func (f *fakeStore) StuckAccounts(context.Context, time.Time, int) ([]string, error) {
	return nil, nil
}
func (f *fakeStore) ReleaseStuck(context.Context, string, time.Time, int) (int, error) {
	return 0, nil
}

func TestPanicInJobBecomesFailureAndWorkerContinues(t *testing.T) {
	store := &fakeStore{pending: []Job{
		{ID: "boom", AccountID: "a", Kind: "k", Attempts: 1, MaxAttempts: 3},
		{ID: "ok", AccountID: "a", Kind: "k2", Attempts: 1, MaxAttempts: 3},
	}}
	w := New(store, Config{})
	w.Register("k", HandlerFunc(func(context.Context, Job) error { panic("handler quebrou") }))
	w.Register("k2", HandlerFunc(func(context.Context, Job) error { return nil }))

	n, err := w.RunOnce(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("RunOnce = %d, %v", n, err)
	}
	if len(store.rescheduled) != 1 || store.rescheduled[0] != "boom" {
		t.Fatalf("job com panic deveria ir para retry: %+v", store)
	}
	if len(store.done) != 1 || store.done[0] != "ok" {
		t.Fatalf("job seguinte deveria concluir: %+v", store.done)
	}
}

func TestPanicInJobExhaustedGoesDead(t *testing.T) {
	store := &fakeStore{pending: []Job{{ID: "boom", AccountID: "a", Kind: "k", Attempts: 3, MaxAttempts: 3}}}
	w := New(store, Config{})
	w.Register("k", HandlerFunc(func(context.Context, Job) error { panic("x") }))
	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.dead) != 1 {
		t.Fatalf("esperava dead-letter: %+v", store)
	}
}

func TestRunOnceDoesNotClaimWhenContextCancelled(t *testing.T) {
	store := &fakeStore{pending: []Job{{ID: "x", AccountID: "a", Kind: "k"}}}
	w := New(store, Config{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n, err := w.RunOnce(ctx)
	if err != nil || n != 0 || store.claims != 0 {
		t.Fatalf("nao deveria pegar job: n=%d err=%v claims=%d", n, err, store.claims)
	}
}

func TestInFlightJobFinishesAfterCancelAndLoopStops(t *testing.T) {
	store := &fakeStore{pending: []Job{{ID: "slow", AccountID: "a", Kind: "k", Attempts: 1, MaxAttempts: 3}}}
	w := New(store, Config{Interval: 5 * time.Millisecond})
	started := make(chan struct{})
	w.Register("k", HandlerFunc(func(ctx context.Context, _ Job) error {
		close(started)
		time.Sleep(150 * time.Millisecond)
		return ctx.Err() // o job nao pode herdar o cancelamento do shutdown
	}))

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	<-started
	cancel()

	closed := make(chan struct{})
	go func() { _ = w.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("Close nao terminou: loop nao encerrou com contexto cancelado")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.done) != 1 || store.done[0] != "slow" {
		t.Fatalf("job em andamento deveria concluir: done=%v resched=%v", store.done, store.rescheduled)
	}
}

func TestLoopSurvivesPanicInCycle(t *testing.T) {
	w := New(&fakeStore{}, Config{})
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	calls := 0
	w.wg.Add(1)
	go w.loop(ctx, 5*time.Millisecond, func(context.Context) {
		mu.Lock()
		calls++
		mu.Unlock()
		panic("ciclo")
	})
	time.Sleep(60 * time.Millisecond)
	cancel()
	w.wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Fatalf("loop deveria continuar apos panic, calls=%d", calls)
	}
}
