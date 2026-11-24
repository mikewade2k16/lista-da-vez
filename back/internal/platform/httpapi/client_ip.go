package httpapi

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
)

// DefaultTrustedProxies sao as redes de onde um proxy reverso (Caddy no Docker/VPS) fala com a
// API: loopback e faixas privadas. Qualquer conexao vinda de fora delas e' tratada como cliente
// final e seus cabecalhos X-Forwarded-For/X-Real-IP sao IGNORADOS (forjaveis).
var DefaultTrustedProxies = []string{
	"127.0.0.0/8",
	"::1/128",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fc00::/7",
}

var trustedProxyNets atomic.Pointer[[]*net.IPNet]

func init() {
	nets, _ := ParseTrustedProxies(DefaultTrustedProxies)
	trustedProxyNets.Store(&nets)
}

// ParseTrustedProxies converte CIDRs (ou IPs simples) em redes. Entradas vazias sao ignoradas.
func ParseTrustedProxies(values []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if !strings.Contains(value, "/") {
			ip := net.ParseIP(value)
			if ip == nil {
				return nil, fmt.Errorf("proxy confiavel invalido: %q", value)
			}
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			value = fmt.Sprintf("%s/%d", ip.String(), bits)
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("proxy confiavel invalido: %q: %w", value, err)
		}
		nets = append(nets, network)
	}
	return nets, nil
}

// SetTrustedProxies define as redes de proxy confiavel usadas por ClientIP (chamar no boot).
// Lista vazia significa "nenhum proxy confiavel": sempre RemoteAddr.
func SetTrustedProxies(nets []*net.IPNet) {
	copied := append([]*net.IPNet(nil), nets...)
	trustedProxyNets.Store(&copied)
}

func isTrustedProxy(ip net.IP, nets []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, network := range nets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP devolve o IP de origem do request. X-Forwarded-For e X-Real-IP so valem quando a
// conexao TCP vem de um proxy confiavel; nesse caso percorre o XFF da direita para a esquerda,
// pulando proxies confiaveis, e devolve o primeiro IP nao confiavel (o que o proxy mais proximo
// observou — o cliente nao controla essa posicao). Sem proxy confiavel, usa RemoteAddr.
func ClientIP(r *http.Request) string {
	remote := remoteHost(r)
	nets := *trustedProxyNets.Load()
	remoteIP := net.ParseIP(remote)
	if !isTrustedProxy(remoteIP, nets) {
		return remote
	}

	parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for index := len(parts) - 1; index >= 0; index-- {
		candidate := net.ParseIP(strings.TrimSpace(parts[index]))
		if candidate == nil {
			continue
		}
		if !isTrustedProxy(candidate, nets) {
			return candidate.String()
		}
	}

	if real := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); real != nil && !isTrustedProxy(real, nets) {
		return real.String()
	}
	return remote
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
