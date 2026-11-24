package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Politica anti brute-force dos fluxos publicos de auth (A-01). Tudo em memoria, por processo.
//
//	login (falhas)        e-mail: 5 em 15 min -> bloqueio 1 min, dobrando a cada falha extra (max 15 min)
//	                      IP:     20 em 15 min -> bloqueio 1 min, dobrando (max 15 min)
//	reset/request         e-mail: 5 pedidos em 15 min; IP: 20 (conta TODO pedido, evita e-mail bombing)
//	reset/confirm         e-mail: 5 codigos errados em 15 min; IP: 20 (o codigo tem poucos digitos)
//	convite (accept/GET)  IP: 20 falhas em 15 min
//
// Sucesso zera so o contador do e-mail (nunca o do IP: um atacante com conta propria nao pode
// limpar a propria contagem). A chave do e-mail e' o texto normalizado, exista a conta ou nao,
// e a resposta 429 e' identica nos dois casos: nao revela se o e-mail existe.
const (
	attemptWindow     = 15 * time.Minute
	attemptEmailMax   = 5
	attemptIPMax      = 20
	attemptBaseLock   = time.Minute
	attemptMaxLock    = 15 * time.Minute
	attemptMaxTracked = 50000
)

// attemptGuard combina o limitador por e-mail e por IP de um fluxo.
type attemptGuard struct {
	byEmail *httpapi.AttemptLimiter
	byIP    *httpapi.AttemptLimiter
}

func newAttemptLimiter(maxFailures int) *httpapi.AttemptLimiter {
	return httpapi.NewAttemptLimiter(httpapi.AttemptLimiterOptions{
		MaxFailures: maxFailures,
		Window:      attemptWindow,
		BaseLock:    attemptBaseLock,
		MaxLock:     attemptMaxLock,
		MaxEntries:  attemptMaxTracked,
	})
}

func newAttemptGuard() *attemptGuard {
	return &attemptGuard{byEmail: newAttemptLimiter(attemptEmailMax), byIP: newAttemptLimiter(attemptIPMax)}
}

func normalizeAttemptEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// blocked devolve true (e escreve 429 generico) quando o e-mail ou o IP estao bloqueados.
func (guard *attemptGuard) blocked(w http.ResponseWriter, r *http.Request, email string) bool {
	ok, retryAfter := guard.byIP.Check(httpapi.ClientIP(r))
	if emailKey := normalizeAttemptEmail(email); emailKey != "" {
		if emailOK, emailRetry := guard.byEmail.Check(emailKey); !emailOK {
			ok = false
			if emailRetry > retryAfter {
				retryAfter = emailRetry
			}
		}
	}
	if ok {
		return false
	}
	httpapi.WriteTooManyAttempts(w, r, retryAfter)
	return true
}

func (guard *attemptGuard) fail(r *http.Request, email string) {
	guard.byIP.Fail(httpapi.ClientIP(r))
	if emailKey := normalizeAttemptEmail(email); emailKey != "" {
		guard.byEmail.Fail(emailKey)
	}
}

func (guard *attemptGuard) succeed(email string) {
	if emailKey := normalizeAttemptEmail(email); emailKey != "" {
		guard.byEmail.Reset(emailKey)
	}
}

// authAttemptGuards agrupa um guard por fluxo, para que falhas de um nao bloqueiem outro.
type authAttemptGuards struct {
	login        *attemptGuard
	resetRequest *attemptGuard
	resetConfirm *attemptGuard
	invitation   *httpapi.AttemptLimiter
}

func newAuthAttemptGuards() *authAttemptGuards {
	return &authAttemptGuards{
		login:        newAttemptGuard(),
		resetRequest: newAttemptGuard(),
		resetConfirm: newAttemptGuard(),
		invitation:   newAttemptLimiter(attemptIPMax),
	}
}

// RateLimitIdentity devolve (userID, tenantID) do bearer/access_token com ASSINATURA valida.
// Usado pelo rate limit global, que roda antes do RequireAuth: token invalido, expirado ou
// ausente devolve vazio (cai para IP). Nao toca no banco.
func (service *Service) RateLimitIdentity(r *http.Request) (string, string) {
	if service == nil || service.tokens == nil {
		return "", ""
	}
	token := ""
	if parsed, err := ExtractBearerToken(r.Header.Get("Authorization")); err == nil {
		token = parsed
	} else {
		token = strings.TrimSpace(r.URL.Query().Get("access_token"))
	}
	if token == "" {
		return "", ""
	}
	principal, err := service.tokens.Parse(token)
	if err != nil {
		return "", ""
	}
	return principal.UserID, principal.TenantID
}

// blockedByIP escreve 429 generico quando o IP de origem esta bloqueado no limitador.
func blockedByIP(w http.ResponseWriter, r *http.Request, limiter *httpapi.AttemptLimiter) bool {
	ok, retryAfter := limiter.Check(httpapi.ClientIP(r))
	if ok {
		return false
	}
	httpapi.WriteTooManyAttempts(w, r, retryAfter)
	return true
}

// failOnInvitationNotFound conta como falha (por IP) somente token de convite inexistente,
// que e' o sinal de adivinhacao de token; convite expirado/usado/revogado exige token valido.
func failOnInvitationNotFound(r *http.Request, limiter *httpapi.AttemptLimiter, err error) {
	if errors.Is(err, ErrInvitationNotFound) {
		limiter.Fail(httpapi.ClientIP(r))
	}
}
