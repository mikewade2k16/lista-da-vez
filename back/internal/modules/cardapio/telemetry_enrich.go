package cardapio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
)

// Helpers de enriquecimento e sanitizacao server-side da telemetria (sem dependencia
// externa). Defesa em profundidade: o front PROMETE nao mandar PII, mas o back nao
// confia — remove chaves sensiveis do context e descarta termos de busca crus.

// contextDenyKeys e a deny-list de chaves de PII removidas de QUALQUER context antes
// de gravar. Comparada em lowercase. Cobre nome/telefone/email/cpf e variacoes pt/en.
var contextDenyKeys = map[string]struct{}{
	"name":     {},
	"phone":    {},
	"email":    {},
	"cpf":      {},
	"telefone": {},
	"endereco": {},
	"address":  {},
}

// parseUserAgent classifica o User-Agent em (deviceType, browser, os) por substrings
// comuns, sem dependencia pesada. Bots tem precedencia (sao excluidos do analytics).
func parseUserAgent(ua string) (deviceType, browser, os string) {
	low := strings.ToLower(ua)
	if low == "" {
		return "unknown", "unknown", "unknown"
	}

	deviceType = "desktop"
	switch {
	case containsAny(low, "bot", "crawl", "spider", "headless", "slurp", "bingpreview", "facebookexternalhit", "preview"):
		deviceType = "bot"
	case strings.Contains(low, "ipad") || (strings.Contains(low, "tablet") && !strings.Contains(low, "mobile")):
		deviceType = "tablet"
	case containsAny(low, "mobi", "iphone", "ipod", "android", "windows phone"):
		// "android" sem "mobile" costuma ser tablet; refina abaixo.
		if strings.Contains(low, "android") && !strings.Contains(low, "mobile") {
			deviceType = "tablet"
		} else {
			deviceType = "mobile"
		}
	}

	browser = "other"
	switch {
	case strings.Contains(low, "edg"):
		browser = "edge"
	case strings.Contains(low, "opr") || strings.Contains(low, "opera"):
		browser = "opera"
	case strings.Contains(low, "samsungbrowser"):
		browser = "samsung"
	case strings.Contains(low, "firefox") || strings.Contains(low, "fxios"):
		browser = "firefox"
	case strings.Contains(low, "chrome") || strings.Contains(low, "crios"):
		browser = "chrome"
	case strings.Contains(low, "safari"):
		browser = "safari"
	}

	os = "other"
	switch {
	case strings.Contains(low, "windows"):
		os = "windows"
	case strings.Contains(low, "android"):
		os = "android"
	case containsAny(low, "iphone", "ipad", "ipod", "ios"):
		os = "ios"
	case strings.Contains(low, "mac os") || strings.Contains(low, "macintosh"):
		os = "macos"
	case strings.Contains(low, "linux"):
		os = "linux"
	}
	return deviceType, browser, os
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// ipHashHex devolve sha256(ip + salt) em hex. Retorna "" quando o salt e vazio (sem
// salt, nao grava hash — evita correlacao por IP cru). Nunca grava o IP em claro.
func ipHashHex(ip, salt string) string {
	ip = strings.TrimSpace(ip)
	if salt == "" || ip == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(ip + salt))
	return hex.EncodeToString(sum[:])
}

// referrerHostOf extrai o host normalizado de um Referer (lower, sem www./porta).
// Reusa normalizeHost (que tambem remove esquema/path). "" se nao parseavel.
func referrerHostOf(referer string) string {
	referer = strings.TrimSpace(referer)
	if referer == "" {
		return ""
	}
	if u, err := url.Parse(referer); err == nil && u.Host != "" {
		return normalizeHost(u.Host)
	}
	return normalizeHost(referer)
}

// sanitizeContext aplica a deny-list de PII sobre o context (top-level) e, para
// menu_search, descarta o termo cru — mantendo so {length,hasResults} derivados. Para
// outros eventos, remove apenas as chaves da deny-list e devolve o restante. context
// invalido/vazio vira "{}".
func sanitizeContext(name string, ctx json.RawMessage) json.RawMessage {
	if name == "menu_search" {
		return searchContext(ctx)
	}
	if len(ctx) == 0 {
		return json.RawMessage("{}")
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(ctx, &obj); err != nil || obj == nil {
		// Nao e objeto JSON (array/escalar) => sem chaves para filtrar; mantem como veio.
		return ctx
	}
	for k := range obj {
		if _, deny := contextDenyKeys[strings.ToLower(strings.TrimSpace(k))]; deny {
			delete(obj, k)
		}
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return json.RawMessage("{}")
	}
	return out
}

// searchContext deriva {length,hasResults} do context de menu_search, descartando o
// termo de busca cru (query/term/q/value/text). length sai do termo (se houver);
// hasResults e preservado quando booleano.
func searchContext(ctx json.RawMessage) json.RawMessage {
	type derived struct {
		Length     int  `json:"length"`
		HasResults bool `json:"hasResults"`
	}
	d := derived{}
	if len(ctx) > 0 {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(ctx, &obj); err == nil {
			for _, key := range []string{"query", "term", "q", "value", "text", "search"} {
				if raw, ok := obj[key]; ok {
					var term string
					if json.Unmarshal(raw, &term) == nil {
						d.Length = len([]rune(strings.TrimSpace(term)))
						break
					}
				}
			}
			if raw, ok := obj["length"]; ok {
				var n int
				if json.Unmarshal(raw, &n) == nil && n >= 0 {
					d.Length = n
				}
			}
			if raw, ok := obj["hasResults"]; ok {
				var b bool
				if json.Unmarshal(raw, &b) == nil {
					d.HasResults = b
				}
			}
		}
	}
	out, err := json.Marshal(d)
	if err != nil {
		return json.RawMessage("{}")
	}
	return out
}
