package bio

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// slugPattern valida o slug publico: minusculas, digitos e hifen.
var slugPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// uniqueSlug devolve um slug livre a partir de uma base ja normalizavel: tenta a
// base; se colidir, adiciona sufixo numerico (-2, -3, ...) ate ser unico.
func (s *Service) uniqueSlug(ctx context.Context, base string) (string, error) {
	normalized, err := normalizeSlug(base)
	if err != nil {
		return "", err
	}
	candidate := normalized
	for i := 2; ; i++ {
		taken, err := s.store.SlugExists(ctx, candidate, "")
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", normalized, i)
	}
}

// normalizeSlug baixa para lowercase, valida o padrao e devolve o slug limpo.
func normalizeSlug(raw string) (string, error) {
	slug := strings.ToLower(strings.TrimSpace(raw))
	if slug == "" || !slugPattern.MatchString(slug) {
		return "", ErrInvalidSlug
	}
	return slug, nil
}

// slugify deriva um slug valido (`^[a-z0-9-]+$`) de um texto livre (ex.: o nome
// da bio): remove acentos (NFKD + descarta marcas), baixa para minusculo e troca
// qualquer caractere fora de [a-z0-9] por hifen, colapsando hifens repetidos.
func slugify(raw string) string {
	decomposed := norm.NFKD.String(strings.ToLower(strings.TrimSpace(raw)))
	var b strings.Builder
	prevHyphen := false
	for _, r := range decomposed {
		switch {
		case unicode.Is(unicode.Mn, r):
			// marca de combinacao (acento) — descarta
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
