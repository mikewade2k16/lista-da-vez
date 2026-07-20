package jobs

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Classificacao do retry (canonico §8; herdada do legado, SPECS_PORT_OMNICHANNEL F5).
// Retry cego queima tentativa em erro que NUNCA vai passar (401 nao vira 200 na 5a vez)
// e desiste cedo demais no que so precisava de tempo (429/5xx). Dai classificar.
//
//	| Classe                                    | max_attempts        |
//	|-------------------------------------------|---------------------|
//	| Transitorio (rede/timeout/conexao)        | 5                   |
//	| 401 · 403 · 404 · 405 e 400/422 conhecidos| 1 (unrecoverable)   |
//	| 429                                       | 5                   |
//	| 5xx                                       | 4                   |
//	| Sem status (resposta nao-HTTP)            | 4                   |
//	| Outros (demais status)                    | 3                   |
const (
	attemptsTransient     = 5
	attemptsUnrecoverable = 1
	attemptsRateLimited   = 5
	attemptsServerError   = 4
	attemptsNoStatus      = 4
	attemptsOther         = 3
)

// Backoff exponencial com jitter: run_after = now() + min(5s * 2^(attempts-1), 5min) ±20%.
// O jitter evita thundering herd — N jobs que falharam juntos nao voltam no mesmo instante.
const (
	backoffBase   = 5 * time.Second
	backoffCap    = 5 * time.Minute
	backoffJitter = 0.20
)

// maxLastErrorLen limita o texto persistido em last_error.
const maxLastErrorLen = 500

// Class e o resultado da classificacao de um erro de handler.
type Class struct {
	// Name identifica a classe no log (nunca o payload).
	Name string

	// MaxAttempts e o teto de tentativas DAQUELA classe.
	MaxAttempts int

	// Unrecoverable = nao adianta repetir: vai direto para dead-letter.
	Unrecoverable bool
}

// unrecoverableStatuses sao os status em que repetir o mesmo request da o mesmo erro:
// 401/403 (credencial/permissao), 404/405 (rota/metodo errado) e 400/422 (payload
// rejeitado por regra do provider). Todos exigem correcao humana, nao tempo.
var unrecoverableStatuses = map[int]bool{
	400: true, 401: true, 403: true, 404: true, 405: true, 422: true,
}

// Classify decide o teto de tentativas a partir do erro do handler.
//
// Ordem importa: transitorio e checado ANTES do status porque um timeout de rede pode
// vir embrulhado num StatusError com StatusCode 0.
func Classify(err error) Class {
	if err == nil {
		return Class{Name: "none", MaxAttempts: attemptsOther}
	}
	if errors.Is(err, ErrNoHandler) {
		// Kind sem handler: so codigo novo resolve. Repetir e desperdicio.
		return Class{Name: "no_handler", MaxAttempts: attemptsUnrecoverable, Unrecoverable: true}
	}
	if isTransient(err) {
		return Class{Name: "transient", MaxAttempts: attemptsTransient}
	}

	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		// Erro de dominio, sem resposta HTTP.
		return Class{Name: "no_status", MaxAttempts: attemptsNoStatus}
	}
	if statusErr.Unrecoverable {
		return Class{Name: "unrecoverable", MaxAttempts: attemptsUnrecoverable, Unrecoverable: true}
	}
	switch code := statusErr.StatusCode; {
	case code == 0:
		return Class{Name: "no_status", MaxAttempts: attemptsNoStatus}
	case unrecoverableStatuses[code]:
		return Class{Name: "unrecoverable", MaxAttempts: attemptsUnrecoverable, Unrecoverable: true}
	case code == 429:
		return Class{Name: "rate_limited", MaxAttempts: attemptsRateLimited}
	case code >= 500 && code <= 599:
		return Class{Name: "server_error", MaxAttempts: attemptsServerError}
	default:
		return Class{Name: "other", MaxAttempts: attemptsOther}
	}
}

// isTransient reconhece falha de infra que tende a passar sozinha: timeout, conexao
// recusada/resetada, DNS, EOF no meio da resposta. context.Canceled NAO entra: e
// shutdown do worker, e o job volta para pending pelo monitor de presas.
func isTransient(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

// Backoff devolve o intervalo ate a proxima tentativa: min(5s * 2^(attempts-1), 5min)
// com jitter de ±20%. attempts e a tentativa JA consumida (1 = primeira falha).
func Backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := backoffCap
	// Limita o shift: 2^62 ja estoura qualquer teto, e shift >=63 e UB em int64.
	if shift := attempts - 1; shift < 62 {
		if scaled := backoffBase << shift; scaled < backoffCap && scaled > 0 {
			delay = scaled
		}
	}
	// Jitter simetrico: delay * (1 ± 0.20).
	factor := 1 + backoffJitter*(2*rand.Float64()-1) //nolint:gosec // jitter nao e cripto
	return time.Duration(float64(delay) * factor)
}

// secretPattern redige o que parece credencial no texto do erro antes de ele virar
// last_error (que e persistido e pode ser lido no painel). Cobre `Bearer <token>`,
// chaves estilo `sk-...`/`AIza...` e `api_key=<valor>` / `"token": "<valor>"` (a aspa
// opcional antes do separador cobre a forma JSON, que e como o erro do provider vem).
var secretPattern = regexp.MustCompile(
	`(?i)(bearer\s+[a-z0-9._\-]+|sk-[a-z0-9._\-]{8,}|AIza[a-z0-9._\-]{8,}|` +
		`(api[_-]?key|token|password|secret)"?\s*[=:]\s*"?[^\s",}]+)`)

// MaskError transforma o erro no texto que vai para last_error: credencial redigida e
// tamanho limitado. O payload cru NUNCA entra aqui (canonico §10) — quem constroi o
// erro no handler e responsavel por nao interpolar payload nele.
func MaskError(err error) string {
	if err == nil {
		return ""
	}
	msg := secretPattern.ReplaceAllString(err.Error(), "[REDACTED]")
	msg = strings.Join(strings.Fields(msg), " ")
	if len(msg) > maxLastErrorLen {
		msg = msg[:maxLastErrorLen] + "…"
	}
	return msg
}
