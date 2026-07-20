package omnichannel

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Guarda anti-SSRF reutilizavel (spec F6 C5; F12 reusa a MESMA allowlist). Nao existe
// helper equivalente no back/ hoje — nasce aqui.
//
// Regra: validar o IP RESOLVIDO, nunca o hostname (DNS rebinding). A checagem final vive no
// `Control` do dialer, entao o IP verificado e o MESMO em que se conecta (fecha o TOCTOU), e
// nao seguimos redirect para destino nao validado.

var (
	// ErrSSRFBlockedHost: host interno/privado. Mapeia para 403 (spec F5 §3).
	ErrSSRFBlockedHost = errors.New("omnichannel: destino nao permitido")
	// ErrSSRFBadScheme: protocolo != http/https. Mapeia para 422 (spec F5 §3).
	ErrSSRFBadScheme = errors.New("omnichannel: protocolo nao permitido")
)

// isBlockedIP marca os ranges que NUNCA podem ser discados a partir da api: loopback,
// privados (RFC1918), link-local (inclui o metadata 169.254.169.254), unspecified,
// multicast e o CGNAT 100.64.0.0/10. IP nil = bloqueado (fail-close).
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	// CGNAT 100.64.0.0/10 nao e coberto por IsPrivate.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xC0 == 64 {
		return true
	}
	return false
}

// validatePublicURL rejeita, ANTES de qualquer fetch, esquema fora de http/https (422) e
// host que resolva para IP interno (403). E a primeira barreira; o dialer.Control repete a
// checagem no momento da conexao (o IP pode mudar entre o LookupIP e o dial).
func validatePublicURL(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ErrSSRFBadScheme
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return ErrSSRFBadScheme
	}
	host := u.Hostname()
	if host == "" {
		return ErrSSRFBlockedHost
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return ErrSSRFBlockedHost
	}
	for _, ip := range ips {
		if isBlockedIP(ip.IP) {
			return ErrSSRFBlockedHost
		}
	}
	return nil
}

// ssrfSafeClient devolve um http.Client que (a) revalida o IP no Control do dialer e (b)
// NAO segue redirect (ErrUseLastResponse) — assim um 30x nao leva a api para um destino
// nao validado. timeout limita a requisicao inteira.
func ssrfSafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: ssrfDialControl}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{DialContext: dialer.DialContext},
	}
}

// ssrfDialControl e chamado com o endereco JA resolvido, imediatamente antes do connect:
// o IP aqui e exatamente o que sera discado (fecha o DNS rebinding/TOCTOU).
func ssrfDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return ErrSSRFBlockedHost
	}
	if ip := net.ParseIP(host); isBlockedIP(ip) {
		return ErrSSRFBlockedHost
	}
	return nil
}
