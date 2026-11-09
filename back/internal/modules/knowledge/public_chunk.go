package knowledge

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	publicDotLeader = regexp.MustCompile(`(\.\s?){6,}\s*\d+`)
	publicPageOnly  = regexp.MustCompile(`(?i)^(p[aá]gina\s*)?\d{1,4}(\s*(de|/)\s*\d{1,4})?$`)
	// Rodapé/compartilhamento padrão das páginas gov.br: não orienta o contribuinte.
	publicBoilerplate = []string{"compartilhe :", "compartilhe:", "links de compartilhamento em redes sociais", "todo o conteúdo deste site está publicado sob a licença"}
)

// publicNoise descarta sumário com pontilhado, número de página isolado e rodapé
// padrão. O texto descartado não some da fonte: o revisor compara com o original.
func publicNoise(paragraph string) bool {
	lower := strings.ToLower(strings.TrimSpace(paragraph))
	if len(publicDotLeader.FindAllStringIndex(lower, 3)) >= 2 || publicPageOnly.MatchString(lower) {
		return true
	}
	for _, b := range publicBoilerplate {
		if strings.HasPrefix(lower, b) {
			return true
		}
	}
	return false
}

// PublicPage é o texto de uma página (PDF) ou do documento inteiro (HTML, page=0).
type PublicPage struct {
	Page int
	Text string
}

type publicChunk struct {
	Index   int
	Page    int
	Heading string
	Text    string
}

const (
	publicChunkTarget = 1200
	publicChunkMax    = 1550 // excerpt em knowledge.public_sources aceita até 1600
	publicChunkLimit  = 3000
)

// chunkPublicPages fatia por parágrafos, sem cortar palavra, preservando o último
// título visto (linhas "## ...") e a página de origem. Só ruído (publicNoise) é
// descartado; parágrafo maior que o limite é quebrado por palavras.
func chunkPublicPages(pages []PublicPage) []publicChunk {
	out := []publicChunk{}
	heading := ""
	var current strings.Builder
	currentPage, currentHeading := 0, ""
	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			out = append(out, publicChunk{Index: len(out), Page: currentPage, Heading: currentHeading, Text: text})
		}
		current.Reset()
	}
	for _, page := range pages {
		for _, paragraph := range strings.Split(page.Text, "\n\n") {
			paragraph = strings.TrimSpace(paragraph)
			if paragraph == "" {
				continue
			}
			if publicNoise(strings.TrimPrefix(paragraph, "## ")) {
				continue
			}
			if strings.HasPrefix(paragraph, "## ") && !strings.Contains(paragraph, "\n") {
				flush()
				heading = truncateRunes(strings.TrimPrefix(paragraph, "## "), 200)
				continue
			}
			for _, piece := range splitLong(paragraph, publicChunkMax) {
				if current.Len() > 0 && (utf8.RuneCountInString(current.String())+utf8.RuneCountInString(piece)+2 > publicChunkMax || page.Page != currentPage || heading != currentHeading) {
					flush()
				}
				if current.Len() == 0 {
					currentPage, currentHeading = page.Page, heading
				} else {
					current.WriteString("\n\n")
				}
				current.WriteString(piece)
				if utf8.RuneCountInString(current.String()) >= publicChunkTarget {
					flush()
				}
			}
		}
		flush()
	}
	flush()
	return out
}

func splitLong(text string, max int) []string {
	if utf8.RuneCountInString(text) <= max {
		return []string{text}
	}
	parts := []string{}
	var b strings.Builder
	for _, word := range strings.Fields(text) {
		if utf8.RuneCountInString(word) > max {
			word = truncateRunes(word, max)
		}
		if b.Len() > 0 && utf8.RuneCountInString(b.String())+1+utf8.RuneCountInString(word) > max {
			parts = append(parts, b.String())
			b.Reset()
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(word)
	}
	if b.Len() > 0 {
		parts = append(parts, b.String())
	}
	return parts
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}
