// Package htmlsanitize limpa HTML vindo de usuário (editor rico) antes de gravar
// e antes de devolver ao front, que renderiza com v-html. Allowlist: só formatação,
// listas, links http(s)/mailto e imagens http(s); sem script, estilo, eventos nem
// iframes. Defesa contra XSS armazenado (achado C-01 da auditoria).
package htmlsanitize

import (
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

var (
	policyOnce sync.Once
	policy     *bluemonday.Policy
)

func richTextPolicy() *bluemonday.Policy {
	policyOnce.Do(func() {
		p := bluemonday.UGCPolicy()
		p.AllowURLSchemes("http", "https", "mailto")
		p.RequireNoReferrerOnLinks(true)
		p.AddTargetBlankToFullyQualifiedLinks(true)
		// Mencoes e marcacoes do editor (Tiptap) usam data-* e class simples.
		p.AllowDataAttributes()
		p.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).Globally()
		policy = p
	})
	return policy
}

// RichText devolve o HTML seguro para renderizar com v-html.
func RichText(html string) string {
	if html == "" {
		return ""
	}
	return richTextPolicy().Sanitize(html)
}
