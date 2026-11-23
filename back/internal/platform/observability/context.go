package observability

import (
	"context"
	"sync"
	"time"
)

type contextKey int

const (
	stateKey contextKey = iota
	skipTraceKey
	queryStartKey
)

// requestState acumula o que acontece durante uma requisição. O ponteiro vive no
// contexto, então middlewares internos (auth, Recover, WriteError) e o tracer do
// pgx enriquecem a mesma struct que o middleware externo grava no fim.
type requestState struct {
	mu            sync.Mutex
	correlationID string
	route         string
	source        string
	userID        string
	accountID     string
	queryCount    int
	queryDuration time.Duration
	externalCount int
	fingerprints  map[string]*fingerprintCount
	errorCode     string
	errorMessage  string
	errorStack    string
}

type fingerprintCount struct {
	count     int
	statement string
}

func withState(ctx context.Context, state *requestState) context.Context {
	return context.WithValue(ctx, stateKey, state)
}

func stateFrom(ctx context.Context) *requestState {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(stateKey).(*requestState)
	return state
}

// CorrelationID devolve o correlation ID da requisição corrente ("" fora de uma).
func CorrelationID(ctx context.Context) string {
	state := stateFrom(ctx)
	if state == nil {
		return ""
	}
	return state.correlationID
}

// SetPrincipal registra usuário e conta autenticados (só IDs).
func SetPrincipal(ctx context.Context, userID, accountID string) {
	state := stateFrom(ctx)
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if userID != "" {
		state.userID = userID
	}
	if accountID != "" {
		state.accountID = accountID
	}
}

// NoteError registra o código e a mensagem de erro devolvidos ao cliente.
func NoteError(ctx context.Context, code, message string) {
	state := stateFrom(ctx)
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.errorCode = truncate(code, 120)
	state.errorMessage = SanitizeText(message, 1000)
}

// NotePanic guarda a stack do panic recuperado. Vai só para log e schema obs,
// nunca para a resposta.
func NotePanic(ctx context.Context, message string, stack []byte) {
	state := stateFrom(ctx)
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.errorCode = "panic"
	state.errorMessage = SanitizeText(message, 1000)
	state.errorStack = SanitizeText(string(stack), 8000)
}

// WithoutTracing marca o contexto para o tracer ignorar as consultas (usado pelo
// próprio gravador e pelas leituras do painel, para não medir a si mesmo).
func WithoutTracing(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipTraceKey, true)
}

func tracingSkipped(ctx context.Context) bool {
	skip, _ := ctx.Value(skipTraceKey).(bool)
	return skip
}

func (s *requestState) addQuery(fingerprint, statement string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queryCount++
	s.queryDuration += duration
	if s.fingerprints == nil {
		s.fingerprints = make(map[string]*fingerprintCount, 8)
	}
	entry, ok := s.fingerprints[fingerprint]
	if !ok {
		if len(s.fingerprints) >= 500 {
			return
		}
		entry = &fingerprintCount{statement: statement}
		s.fingerprints[fingerprint] = entry
	}
	entry.count++
}

func (s *requestState) addExternal() {
	s.mu.Lock()
	s.externalCount++
	s.mu.Unlock()
}

func (s *requestState) routeName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.route
}
