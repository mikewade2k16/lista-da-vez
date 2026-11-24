package obsagent

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	tlsCheckInterval = 6 * time.Hour
	tlsDialTimeout   = 5 * time.Second
)

// TLSCheck e o resultado da checagem de um host (campo "tls" da amostra do agente).
type TLSCheck struct {
	Host      string `json:"host"`
	ExpiresAt string `json:"expiresAt"`
	DaysLeft  int    `json:"daysLeft"`
	Error     string `json:"error"`
}

// ParseTLSHosts le OBS_AGENT_TLS_HOSTS: "host[:porta]" separados por virgula; sem
// porta usa 443. Duplicados e vazios saem.
func ParseTLSHosts(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		h := strings.TrimSpace(part)
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

// tlsChecker checa os certificados a cada tlsCheckInterval e devolve o cache nos
// ciclos intermediarios.
type tlsChecker struct {
	hosts []string
	// config permite trocar a CA nos testes.
	config *tls.Config
	now    func() time.Time

	mu      sync.Mutex
	lastRun time.Time
	cached  []TLSCheck
}

func newTLSChecker(hosts []string) *tlsChecker {
	return &tlsChecker{hosts: hosts, now: time.Now}
}

// Results devolve nil sem hosts configurados.
func (t *tlsChecker) Results(ctx context.Context) []TLSCheck {
	if t == nil || len(t.hosts) == 0 {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	if t.cached != nil && now.Sub(t.lastRun) < tlsCheckInterval {
		return t.cached
	}
	// Em paralelo e sem herdar o cancelamento do ciclo: um timeout do ciclo nao deve
	// gravar erro falso no cache por 6 h (cada dial tem o proprio timeout).
	detached := context.WithoutCancel(ctx)
	out := make([]TLSCheck, len(t.hosts))
	var wg sync.WaitGroup
	for i, h := range t.hosts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = checkTLS(detached, h, t.config, now)
		}()
	}
	wg.Wait()
	t.cached, t.lastRun = out, now
	return out
}

// checkTLS conecta, le o NotAfter do certificado folha e calcula os dias restantes.
// Qualquer falha vira Error (sem derrubar o ciclo).
func checkTLS(ctx context.Context, hostPort string, cfg *tls.Config, now time.Time) TLSCheck {
	res := TLSCheck{Host: hostPort}
	host, addr := hostPort, hostPort
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		host = h
	} else {
		addr = net.JoinHostPort(hostPort, "443")
	}
	conf := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg != nil {
		conf = cfg.Clone()
	}
	conf.ServerName = host
	dctx, cancel := context.WithTimeout(ctx, tlsDialTimeout)
	defer cancel()
	d := &tls.Dialer{NetDialer: &net.Dialer{Timeout: tlsDialTimeout}, Config: conf}
	conn, err := d.DialContext(dctx, "tcp", addr)
	if err != nil {
		res.Error = truncate(err.Error(), 200)
		return res
	}
	defer conn.Close()
	certs := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certs) == 0 {
		res.Error = "sem certificado"
		return res
	}
	notAfter := certs[0].NotAfter.UTC()
	res.ExpiresAt = notAfter.Format(time.RFC3339)
	res.DaysLeft = int(notAfter.Sub(now).Hours() / 24)
	if notAfter.Before(now) {
		res.DaysLeft = -int(now.Sub(notAfter).Hours()/24) - 1
	}
	return res
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
