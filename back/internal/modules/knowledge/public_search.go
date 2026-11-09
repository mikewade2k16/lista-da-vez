package knowledge

import (
	"context"
	"regexp"
)

var publicTerm = regexp.MustCompile(`^[a-z0-9]{2,50}$`)

// SearchPublic reads only explicitly reviewed public material. It cannot access
// the tenant-owned corpus, bindings, uploads or original files.
func (s *Service) SearchPublic(ctx context.Context, terms []string) (Result, error) {
	out := Result{Status: "no_evidence", Message: "Não encontrei uma fonte pública revisada para essa pergunta. Não é necessário conectar IA; falta conteúdo pertinente publicado na base.", Evidence: []Evidence{}}
	if len(terms) == 0 || len(terms) > 12 {
		return out, fail("invalid_query", 400)
	}
	for _, term := range terms {
		if !publicTerm.MatchString(term) {
			return out, fail("invalid_query", 400)
		}
	}
	var err error
	out.Evidence, err = s.store.searchPublic(ctx, terms)
	if err != nil {
		return out, err
	}
	if len(out.Evidence) > 0 {
		out.PotentialConflict = len(out.Evidence) > 1
		out.Status = "informational"
		out.Message = "Encontrei estes trechos em fontes públicas revisadas. Confira as datas e os cenários de cada fonte. Os trechos não confirmam seu enquadramento nem calculam seu imposto."
	}
	return out, nil
}

func (s *Service) ValidatePublicReferences(ctx context.Context, ids []string) error {
	if len(ids) > 180 {
		return fail("invalid_references", 400)
	}
	unique := map[string]bool{}
	for _, id := range ids {
		if !idPattern.MatchString(id) {
			return fail("not_found", 404)
		}
		unique[id] = true
	}
	if len(unique) == 0 {
		return nil
	}
	count, err := s.store.publicReferenceCount(ctx, ids)
	if err != nil {
		return err
	}
	if count != len(unique) {
		return fail("stale_evidence", 409)
	}
	return nil
}
