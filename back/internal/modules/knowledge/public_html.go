package knowledge

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Extração de texto de páginas oficiais do catálogo público. O resultado é texto puro
// (nunca renderizado como HTML); scripts, estilos, formulários e navegação são
// descartados e links não são seguidos. Revisores conferem o texto antes de publicar.

var (
	htmlCharsetMeta = regexp.MustCompile(`(?i)<meta[^>]+charset\s*=\s*["']?([a-z0-9_-]+)`)
	htmlDropBlocks  = regexp.MustCompile(`(?is)<(script|style|noscript|svg|template|iframe|form|select|button|head)\b.*?</(script|style|noscript|svg|template|iframe|form|select|button|head)\s*>`)
	htmlComments    = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlNoiseBlocks = regexp.MustCompile(`(?is)<(nav|header|footer|aside)\b.*?</(nav|header|footer|aside)\s*>`)
	htmlMainOpen    = regexp.MustCompile(`(?is)<(main|article)\b[^>]*>|<div[^>]+id\s*=\s*["'](content|content-core|parent-fieldname-text)["'][^>]*>`)
	htmlHeading     = regexp.MustCompile(`(?is)<h([1-4])\b[^>]*>(.*?)</h[1-4]\s*>`)
	htmlBlockTag    = regexp.MustCompile(`(?i)</?(p|div|br|li|ul|ol|tr|table|section|article|main|h[1-6]|blockquote|dd|dt|pre)\b[^>]*>`)
	htmlCellTag     = regexp.MustCompile(`(?i)</(td|th)\s*>`)
	htmlAnyTag      = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlBlankLines  = regexp.MustCompile(`\n[ \t]*(\n[ \t]*)+`)
	htmlSpaces      = regexp.MustCompile(`[ \t\x{00a0}]+`)
)

// cp1252High mapeia 0x80–0x9F do Windows-1252 (Planalto e páginas antigas).
var cp1252High = [32]rune{'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0x8D, 'Ž', 0x8F, 0x90, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0x9D, 'ž', 'Ÿ'}

// decodeHTMLBytes converte para UTF-8. UTF-8 válido é mantido; senão usa o charset
// declarado (latin1/windows-1252) e, sem declaração, windows-1252.
func decodeHTMLBytes(raw []byte, headerCharset string) string {
	charset := strings.ToLower(strings.TrimSpace(headerCharset))
	if charset == "" {
		if m := htmlCharsetMeta.FindSubmatch(raw[:min(len(raw), 4096)]); m != nil {
			charset = strings.ToLower(string(m[1]))
		}
	}
	if (charset == "" || charset == "utf-8" || charset == "utf8") && utf8.Valid(raw) {
		return string(raw)
	}
	var b strings.Builder
	b.Grow(len(raw) + len(raw)/8)
	for _, c := range raw {
		switch {
		case c < 0x80:
			b.WriteByte(c)
		case c < 0xA0:
			b.WriteRune(cp1252High[c-0x80])
		default:
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}

// htmlToText devolve o texto principal em parágrafos e a lista de títulos (h1–h4)
// marcados como linhas "## título" para o fatiamento manter o contexto.
func htmlToText(doc string) string {
	doc = htmlComments.ReplaceAllString(doc, " ")
	doc = htmlDropBlocks.ReplaceAllString(doc, " ")
	if loc := htmlMainOpen.FindStringIndex(doc); loc != nil {
		doc = doc[loc[0]:]
	}
	doc = htmlNoiseBlocks.ReplaceAllString(doc, " ")
	doc = htmlHeading.ReplaceAllStringFunc(doc, func(m string) string {
		inner := htmlHeading.FindStringSubmatch(m)[2]
		title := strings.TrimSpace(htmlSpaces.ReplaceAllString(html.UnescapeString(htmlAnyTag.ReplaceAllString(inner, " ")), " "))
		if title == "" {
			return "\n"
		}
		return "\n\n## " + title + "\n\n"
	})
	doc = htmlBlockTag.ReplaceAllString(doc, "\n")
	doc = htmlCellTag.ReplaceAllString(doc, " | ")
	doc = htmlAnyTag.ReplaceAllString(doc, " ")
	doc = html.UnescapeString(doc)
	doc = strings.ReplaceAll(doc, "\r", "")
	doc = htmlSpaces.ReplaceAllString(doc, " ")
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "|"))
	}
	doc = strings.Join(lines, "\n")
	doc = htmlBlankLines.ReplaceAllString(doc, "\n\n")
	return strings.TrimSpace(doc)
}
