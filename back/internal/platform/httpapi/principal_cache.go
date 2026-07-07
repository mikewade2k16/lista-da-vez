package httpapi

import (
	"sync"
	"sync/atomic"
	"time"
)

// PrincipalCached e o tipo armazenado no cache. Generico para evitar importar o
// pacote auth (que importaria httpapi de volta — ciclo). O caller faz o cast.
type PrincipalCached[T any] struct {
	Value     T
	ExpiresAt time.Time
}

// PrincipalCache armazena Principals em memoria com TTL para reduzir round-trips
// ao banco no hot-path de autenticacao.
//
// Estrutura de dados:
//   - bySession[sessionID] → entrada do Principal (cache primario)
//   - byUser[userID]       → lista de sessionIDs (para invalidacao por usuario)
//
// Quando o token nao carrega sessionID (tokens legados, issuedBefore da C6),
// o cache nao e consultado e o auth segue o caminho legado de DB.
type PrincipalCache[T any] struct {
	mu        sync.RWMutex
	bySession map[string]*PrincipalCached[T]
	byUser    map[string][]string // userID -> []sessionID
	ttl       time.Duration
	hits      atomic.Int64 // contadores cumulativos para telemetria de hit rate (AC-01)
	misses    atomic.Int64
}

// NewPrincipalCache cria um cache com TTL especificado.
func NewPrincipalCache[T any](ttl time.Duration) *PrincipalCache[T] {
	return &PrincipalCache[T]{
		bySession: make(map[string]*PrincipalCached[T]),
		byUser:    make(map[string][]string),
		ttl:       ttl,
	}
}

// Get retorna o Principal cacheado para a sessionID informada.
// Retorna false se nao encontrado ou se a entrada expirou.
func (c *PrincipalCache[T]) Get(sessionID string) (T, bool) {
	if sessionID == "" {
		c.misses.Add(1)
		var zero T
		return zero, false
	}

	c.mu.RLock()
	entry, ok := c.bySession[sessionID]
	c.mu.RUnlock()

	if !ok || time.Now().After(entry.ExpiresAt) {
		c.misses.Add(1)
		var zero T
		return zero, false
	}
	c.hits.Add(1)
	return entry.Value, true
}

// Set armazena um Principal no cache. userID e necessario para permitir
// invalidacao por usuario (ex: quando role muda).
func (c *PrincipalCache[T]) Set(sessionID, userID string, value T) {
	if sessionID == "" {
		return
	}

	entry := &PrincipalCached[T]{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}

	c.mu.Lock()
	c.bySession[sessionID] = entry
	c.byUser[userID] = appendUniq(c.byUser[userID], sessionID)
	c.mu.Unlock()
}

// InvalidateSession remove uma sessao especifica do cache (ex: logout, revogacao).
func (c *PrincipalCache[T]) InvalidateSession(sessionID string) {
	if sessionID == "" {
		return
	}
	c.mu.Lock()
	delete(c.bySession, sessionID)
	c.mu.Unlock()
}

// InvalidateUser remove todas as sessoes cacheadas de um usuario.
// Usado quando role ou permissoes do usuario mudam.
func (c *PrincipalCache[T]) InvalidateUser(userID string) {
	if userID == "" {
		return
	}
	c.mu.Lock()
	for _, sid := range c.byUser[userID] {
		delete(c.bySession, sid)
	}
	delete(c.byUser, userID)
	c.mu.Unlock()
}

// InvalidateAll limpa todo o cache. Usado para eventos de account-level
// (ex: account.modules.changed, role.permissions.changed para um role amplo).
func (c *PrincipalCache[T]) InvalidateAll() {
	c.mu.Lock()
	c.bySession = make(map[string]*PrincipalCached[T])
	c.byUser = make(map[string][]string)
	c.mu.Unlock()
}

// Cleanup remove entradas expiradas. Chamar periodicamente (ex: a cada 60s) para
// evitar crescimento ilimitado. Nao e obrigatorio para corretude — Get ja checa expiry.
func (c *PrincipalCache[T]) Cleanup() {
	now := time.Now()
	c.mu.Lock()
	for sid, entry := range c.bySession {
		if now.After(entry.ExpiresAt) {
			delete(c.bySession, sid)
		}
	}
	// Limpa entradas vazias de byUser
	for uid, sessions := range c.byUser {
		filtered := sessions[:0]
		for _, sid := range sessions {
			if _, ok := c.bySession[sid]; ok {
				filtered = append(filtered, sid)
			}
		}
		if len(filtered) == 0 {
			delete(c.byUser, uid)
		} else {
			c.byUser[uid] = filtered
		}
	}
	c.mu.Unlock()
}

// Stats retorna contadores cumulativos desde o boot (nao resetam).
func (c *PrincipalCache[T]) Stats() (hits, misses int64) {
	return c.hits.Load(), c.misses.Load()
}

// Len retorna o numero de entradas atualmente no cache (inclui expiradas
// ainda nao varridas pelo Cleanup).
func (c *PrincipalCache[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.bySession)
}

func appendUniq(slice []string, s string) []string {
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}
