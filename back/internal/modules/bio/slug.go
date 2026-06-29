package bio

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/stringsx"
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
// Usado para validar slugs ja fornecidos pelo usuario (nao deriva do texto livre).
func normalizeSlug(raw string) (string, error) {
	slug := strings.ToLower(strings.TrimSpace(raw))
	if slug == "" || !slugPattern.MatchString(slug) {
		return "", ErrInvalidSlug
	}
	return slug, nil
}

// slugify deriva um slug valido (`^[a-z0-9-]+$`) de um texto livre (ex.: o nome
// da bio). Delega para a regra canonica unica em stringsx.Slugify (NFD, sem
// acentos, hifen como separador, sem hifens repetidos nem nas pontas).
func slugify(raw string) string {
	return stringsx.Slugify(raw)
}
