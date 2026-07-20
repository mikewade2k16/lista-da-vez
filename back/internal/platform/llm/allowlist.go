package llm

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
)

// Allowlist de BaseURL — mitigacao OBRIGATORIA de SSRF (OMNI-F3.3).
//
// O risco: a BaseURL vem do PAINEL. Com o LLM no Go, quem faz o request de saida e o
// container da api — que enxerga a rede interna (postgres, n8n, outros containers,
// metadata da VPS). Uma base apontando para host interno transformaria o painel em
// porta de entrada para varrer a rede de dentro. E exatamente o cuidado que
// calendar/ai_models.go:35-39 ja documenta para a listagem; agora vale para o dispatch.
//
// Mecanismo: a base so passa se bater com o mapa canonico SERVER-SIDE (espelho de
// ai_models.go:40-44) ou com um host explicitamente liberado via RegisterAllowedHost
// (o gancho para a allowlist de core.platform_settings — quem le a tabela e o caller,
// este pacote nao acessa banco). Fora disso => ErrBaseURLNotAllowed.

// providerDefaultBaseURL espelha calendar/ai_models.go:40-44 (e o AI_PROVIDER_BASE_URL
// do front). BaseURL vazia na Request => o default canonico do provider.
var providerDefaultBaseURL = map[string]string{
	"openai": "https://api.openai.com/v1",
	"gemini": "https://generativelanguage.googleapis.com/v1beta/openai",
	"glm":    "https://api.z.ai/api/paas/v4",
}

// providerSet e o enum de providers com adapter.
var providerSet = map[string]bool{"openai": true, "gemini": true, "glm": true}

// allowedHosts sao os hosts liberados alem dos canonicos. Populado por
// RegisterAllowedHost no boot, a partir de core.platform_settings (F10 escreve; este
// pacote nao le banco). Protegido por mutex: o boot escreve, os handlers leem.
var (
	allowedHostsMu sync.RWMutex
	allowedHosts   = map[string]bool{}
)

// normalizeProvider devolve o provider em minusculas se estiver no enum; senao "".
func normalizeProvider(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	if providerSet[p] {
		return p
	}
	return ""
}

// DefaultBaseURL devolve a base canonica do provider (vazio se fora do enum).
func DefaultBaseURL(provider string) string {
	return providerDefaultBaseURL[normalizeProvider(provider)]
}

// RegisterAllowedHost libera um host extra na allowlist (ex.: gateway proprio do
// cliente, vindo de core.platform_settings). Host vazio e ignorado. Chamar no BOOT,
// antes de servir request.
//
// CUIDADO: o que entra aqui e destino de request feito pelo container da api. So
// hosts publicos e confiaveis — nunca um host interno.
func RegisterAllowedHost(host string) {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return
	}
	allowedHostsMu.Lock()
	defer allowedHostsMu.Unlock()
	allowedHosts[h] = true
}

// ResetAllowedHosts limpa a allowlist extra (usado em teste e em reload de config).
func ResetAllowedHosts() {
	allowedHostsMu.Lock()
	defer allowedHostsMu.Unlock()
	allowedHosts = map[string]bool{}
}

// isHostAllowed diz se o host esta na allowlist extra.
func isHostAllowed(host string) bool {
	allowedHostsMu.RLock()
	defer allowedHostsMu.RUnlock()
	return allowedHosts[host]
}

// ResolveBaseURL valida a base do painel e devolve a base efetiva.
//
// base vazia => default canonico do provider. base preenchida => so passa se:
//   - o esquema for https (http em claro mandaria a chave sem TLS); e
//   - o host bater com o host canonico DAQUELE provider, ou estiver na allowlist extra.
//
// Qualquer outra coisa => ErrBaseURLNotAllowed. Nao ha "so desta vez": uma base livre
// e SSRF a partir da api.
func ResolveBaseURL(provider, base string) (string, error) {
	prov := normalizeProvider(provider)
	if prov == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidProvider, provider)
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return providerDefaultBaseURL[prov], nil
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("%w: base URL ilegivel", ErrBaseURLNotAllowed)
	}
	// So https: o request carrega a API key no header.
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: esquema %q (so https)", ErrBaseURLNotAllowed, parsed.Scheme)
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", fmt.Errorf("%w: base URL sem host", ErrBaseURLNotAllowed)
	}
	// Defesa extra: mesmo que alguem libere um host por engano, endereco interno/loopback
	// nunca passa. Nome que RESOLVE para IP interno nao e coberto aqui (exigiria resolver
	// DNS no momento da validacao e ainda haveria TOCTOU) — por isso a allowlist de host
	// e o mecanismo principal, e esta checagem e so a rede de seguranca.
	if isInternalHost(host) {
		return "", fmt.Errorf("%w: host interno %q", ErrBaseURLNotAllowed, host)
	}

	canonical, err := url.Parse(providerDefaultBaseURL[prov])
	if err == nil && strings.EqualFold(canonical.Hostname(), host) {
		return strings.TrimRight(base, "/"), nil
	}
	if isHostAllowed(host) {
		return strings.TrimRight(base, "/"), nil
	}
	return "", fmt.Errorf("%w: host %q nao esta na allowlist do provider %q", ErrBaseURLNotAllowed, host, prov)
}

// isInternalHost reconhece loopback, link-local, IP privado e nomes de servico
// internos (sem ponto = nome de container/servico na rede do docker).
func isInternalHost(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified()
	}
	if !strings.Contains(host, ".") {
		// "postgres", "api", "n8n": nome de servico na rede interna.
		return true
	}
	return host == "localhost" ||
		strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".local")
}
