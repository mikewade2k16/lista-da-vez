package knowledge

import (
	"context"
)

// ValidateReferences rechecks a whole history page in one bounded SQL query.
func (s *Service) ValidateReferences(ctx context.Context, sc Scope, ids []string) error {
	if e := s.authorize(ctx, sc, false); e != nil {
		return e
	}
	if len(ids) > 500 {
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
	count, e := s.store.validReferenceCount(ctx, sc, ids)
	if e != nil {
		return e
	}
	if count != len(unique) {
		return fail("stale_evidence", 409)
	}
	return s.authorize(ctx, sc, false)
}
func (s *Store) validReferenceCount(ctx context.Context, sc Scope, ids []string) (int, error) {
	var count int
	e := s.pool.QueryRow(ctx, `select count(distinct c.id) from messaging.knowledge_chunks c
 join messaging.knowledge_documents d on d.account_id=c.account_id and d.id=c.document_id
 join messaging.knowledge_bases k on k.account_id=d.account_id and k.id=d.knowledge_base_id
 join accounting.knowledge_bindings b on b.account_id=k.account_id and b.knowledge_base_id=k.id
 join messaging.knowledge_document_applicability a on a.account_id=d.account_id and a.document_id=d.id and a.version=d.version
 where c.account_id=$1::uuid and b.issuer_id=$2::uuid and c.id=any($3::uuid[]) and b.is_enabled and k.is_enabled and d.status='published' and a.status='reviewed'`, sc.AccountID, sc.ResourceID, ids).Scan(&count)
	return count, e
}
