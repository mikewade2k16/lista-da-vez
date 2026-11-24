package background

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestGoRecoversPanicAndWaitReturns(t *testing.T) {
	g := New(quiet())
	g.Go(context.Background(), "boom", func(context.Context) { panic("x") })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !g.Wait(ctx) {
		t.Fatal("Wait nao terminou apos panic recuperado")
	}
}

func TestLoopRestartsAfterPanicAndStopsOnCancel(t *testing.T) {
	g := New(quiet())
	var runs atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	g.Loop(ctx, "flaky", func(c context.Context) {
		if runs.Add(1) == 1 {
			panic("primeira")
		}
		<-c.Done()
	})
	deadline := time.Now().Add(3 * time.Second)
	for runs.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if runs.Load() < 2 {
		t.Fatal("loop nao reiniciou apos panic")
	}
	cancel()
	wctx, wcancel := context.WithTimeout(context.Background(), time.Second)
	defer wcancel()
	if !g.Shutdown(wctx) {
		t.Fatal("shutdown nao terminou apos cancelar")
	}
}

func TestShutdownRunsClosersAndTimesOut(t *testing.T) {
	g := New(quiet())
	var closed atomic.Bool
	g.AddCloser("c", func() error { closed.Store(true); return nil })
	g.Go(context.Background(), "stuck", func(context.Context) { time.Sleep(500 * time.Millisecond) })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if g.Shutdown(ctx) {
		t.Fatal("esperava timeout com goroutine presa")
	}
	if !closed.Load() {
		t.Fatal("closer nao rodou")
	}
}
