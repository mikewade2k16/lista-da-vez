package httpapi

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// AttemptLimiterOptions configura o limitador de tentativas falhas (login, reset, convite).
//
// Politica: apos MaxFailures falhas dentro de Window (contada desde a ultima falha), a chave fica
// bloqueada por BaseLock; cada falha adicional dobra o bloqueio (backoff progressivo) ate MaxLock.
// A janela reinicia quando a chave fica Window sem falhar. MaxEntries limita a memoria.
type AttemptLimiterOptions struct {
	MaxFailures int
	Window      time.Duration
	BaseLock    time.Duration
	MaxLock     time.Duration
	MaxEntries  int
	// Now permite injetar relogio em testes; nil usa time.Now.
	Now func() time.Time
}

type attemptEntry struct {
	failures    int
	lastFailAt  time.Time
	lockedUntil time.Time
}

// AttemptLimiter conta falhas por chave (e-mail normalizado ou IP) em memoria. Nao serve para
// multi-instancia sem broker. A limpeza e' preguicosa (varredura a cada minuto dentro de Fail),
// sem goroutine — nada para parar no shutdown.
type AttemptLimiter struct {
	mu        sync.Mutex
	options   AttemptLimiterOptions
	entries   map[string]*attemptEntry
	lastSweep time.Time
}

// NewAttemptLimiter cria o limitador aplicando padroes seguros aos campos zerados.
func NewAttemptLimiter(options AttemptLimiterOptions) *AttemptLimiter {
	if options.MaxFailures <= 0 {
		options.MaxFailures = 5
	}
	if options.Window <= 0 {
		options.Window = 15 * time.Minute
	}
	if options.BaseLock <= 0 {
		options.BaseLock = time.Minute
	}
	if options.MaxLock <= 0 {
		options.MaxLock = 15 * time.Minute
	}
	if options.MaxEntries <= 0 {
		options.MaxEntries = 50000
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &AttemptLimiter{options: options, entries: map[string]*attemptEntry{}}
}

// Check informa se a chave esta bloqueada e, se sim, por quanto tempo.
func (limiter *AttemptLimiter) Check(key string) (bool, time.Duration) {
	if key == "" {
		return true, 0
	}
	now := limiter.options.Now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	entry, ok := limiter.entries[key]
	if !ok {
		return true, 0
	}
	if now.After(entry.lockedUntil) && now.Sub(entry.lastFailAt) > limiter.options.Window {
		delete(limiter.entries, key)
		return true, 0
	}
	if now.Before(entry.lockedUntil) {
		return false, entry.lockedUntil.Sub(now)
	}
	return true, 0
}

// Fail registra uma falha e aplica o bloqueio quando o limite e' atingido.
func (limiter *AttemptLimiter) Fail(key string) {
	if key == "" {
		return
	}
	now := limiter.options.Now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	limiter.sweepLocked(now)

	entry, ok := limiter.entries[key]
	if ok && now.After(entry.lockedUntil) && now.Sub(entry.lastFailAt) > limiter.options.Window {
		ok = false
	}
	if !ok {
		limiter.makeRoomLocked()
		entry = &attemptEntry{}
		limiter.entries[key] = entry
	}
	entry.failures++
	entry.lastFailAt = now

	if entry.failures >= limiter.options.MaxFailures {
		lock := limiter.options.BaseLock
		for step := limiter.options.MaxFailures; step < entry.failures && lock < limiter.options.MaxLock; step++ {
			lock *= 2
		}
		if lock > limiter.options.MaxLock {
			lock = limiter.options.MaxLock
		}
		entry.lockedUntil = now.Add(lock)
	}
}

// Reset zera o contador da chave (ex.: login bem-sucedido do e-mail).
func (limiter *AttemptLimiter) Reset(key string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	delete(limiter.entries, key)
}

// Len devolve o numero de chaves rastreadas (testes/telemetria).
func (limiter *AttemptLimiter) Len() int {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return len(limiter.entries)
}

func (limiter *AttemptLimiter) sweepLocked(now time.Time) {
	if now.Sub(limiter.lastSweep) < time.Minute {
		return
	}
	limiter.lastSweep = now
	for key, entry := range limiter.entries {
		if now.After(entry.lockedUntil) && now.Sub(entry.lastFailAt) > limiter.options.Window {
			delete(limiter.entries, key)
		}
	}
}

// makeRoomLocked garante espaco para uma chave nova: descarta a mais antiga quando cheio.
func (limiter *AttemptLimiter) makeRoomLocked() {
	if len(limiter.entries) < limiter.options.MaxEntries {
		return
	}
	var oldestKey string
	var oldest time.Time
	for key, entry := range limiter.entries {
		if oldestKey == "" || entry.lastFailAt.Before(oldest) {
			oldestKey, oldest = key, entry.lastFailAt
		}
	}
	delete(limiter.entries, oldestKey)
}

// WriteTooManyAttempts responde 429 generico com Retry-After. A mensagem e' identica para qualquer
// chave (e-mail existente ou nao, IP), para nao vazar a existencia da conta.
func WriteTooManyAttempts(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
	seconds := int(retryAfter.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	WriteError(w, r, http.StatusTooManyRequests, "too_many_attempts", "Muitas tentativas. Aguarde alguns instantes e tente novamente.")
}
