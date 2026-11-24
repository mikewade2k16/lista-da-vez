// Package background centraliza as goroutines de longa duracao da API: cada uma
// roda com recover (panic vira log com stack, nunca derruba o processo) e fica
// registrada num WaitGroup para o shutdown esperar o termino (A-03).
package background

import (
	"context"
	"fmt"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/panics"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

const (
	minRestartBackoff = time.Second
	maxRestartBackoff = time.Minute
	// Execucao estavel por mais que isso zera o backoff de reinicio.
	stableRunAfter = time.Minute
)

// Group rastreia goroutines de background e closers de shutdown.
type Group struct {
	logger *slog.Logger
	wg     sync.WaitGroup

	mu      sync.Mutex
	closers []closer
}

type closer struct {
	name string
	fn   func() error
}

// New cria o Group. logger nil => slog.Default().
func New(logger *slog.Logger) *Group {
	if logger == nil {
		logger = slog.Default()
	}
	return &Group{logger: logger}
}

// Go roda fn numa goroutine rastreada, com recover. Panic e logado (com stack) e
// a goroutine termina: use para tarefas one-shot.
func (g *Group) Go(ctx context.Context, name string, fn func(ctx context.Context)) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		g.safeRun(ctx, name, fn)
	}()
}

// Loop e como Go, mas se fn voltar por panic reinicia com backoff exponencial
// (1s ate 1min) enquanto ctx estiver vivo. Retorno normal de fn encerra o loop.
func (g *Group) Loop(ctx context.Context, name string, fn func(ctx context.Context)) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		backoff := minRestartBackoff
		for ctx.Err() == nil {
			started := time.Now()
			if !g.safeRun(ctx, name, fn) {
				return
			}
			if time.Since(started) > stableRunAfter {
				backoff = minRestartBackoff
			}
			g.logger.Warn("background_restart", "name", name, "backoff", backoff.String())
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if backoff *= 2; backoff > maxRestartBackoff {
				backoff = maxRestartBackoff
			}
		}
	}()
}

// safeRun executa fn; devolve true se houve panic (recuperado).
func (g *Group) safeRun(ctx context.Context, name string, fn func(ctx context.Context)) (panicked bool) {
	defer func() {
		if rec := recover(); rec != nil {
			panicked = true
			panics.Record("background")
			g.logger.Error("background_panic",
				"name", name,
				"panic", fmt.Sprint(rec),
				"stack", string(debug.Stack()),
			)
		}
	}()
	fn(ctx)
	return false
}

// AddCloser registra uma funcao de encerramento (ex.: Handle.Close de modulo),
// executada por Shutdown depois do cancelamento do contexto raiz.
func (g *Group) AddCloser(name string, fn func() error) {
	if fn == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closers = append(g.closers, closer{name: name, fn: fn})
}

// Wait bloqueia ate todas as goroutines rastreadas terminarem ou ctx expirar.
// Devolve true se terminaram.
func (g *Group) Wait(ctx context.Context) bool {
	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

// Shutdown roda os closers (em ordem inversa de registro, cada um com recover e
// limitado por ctx) e depois espera as goroutines. Chamar APOS cancelar o
// contexto raiz. Devolve true se tudo terminou dentro do prazo.
func (g *Group) Shutdown(ctx context.Context) bool {
	g.mu.Lock()
	closers := append([]closer(nil), g.closers...)
	g.mu.Unlock()

	ok := true
	for i := len(closers) - 1; i >= 0; i-- {
		c := closers[i]
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() {
				if rec := recover(); rec != nil {
					panics.Record("background")
					g.logger.Error("background_closer_panic", "name", c.name, "panic", fmt.Sprint(rec))
				}
			}()
			if err := c.fn(); err != nil {
				g.logger.Warn("background_closer_failed", "name", c.name, "error", err.Error())
			}
		}()
		select {
		case <-done:
		case <-ctx.Done():
			g.logger.Warn("background_closer_timeout", "name", c.name)
			ok = false
		}
	}
	if !g.Wait(ctx) {
		ok = false
	}
	return ok
}
