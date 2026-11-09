package accounting

import (
	"context"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/knowledge"
)

// AuthorizeKnowledge is the consumer policy. No attendance role or fake agent ID.
func (s *Service) AuthorizeKnowledge(ctx context.Context, sc knowledge.Scope, write bool) (err error) {
	defer func() {
		var domain *Error
		if errors.As(err, &domain) {
			err = &knowledge.Error{Code: domain.Code, Status: domain.Status}
		}
	}()
	scope := Scope{AccountID: sc.AccountID, ActorID: sc.ActorID, IssuerID: sc.ResourceID}
	permission, grant := "accounting.knowledge.view", "knowledge_view"
	if write {
		permission, grant = "accounting.knowledge.manage", "knowledge_manage"
	}
	if e := s.check(ctx, scope, permission); e != nil {
		return e
	}
	tx, e := s.store.begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, e = issuerLock(ctx, tx, scope, grant)
	return e
}
func (s *Service) Knowledge() *knowledge.Service { return s.knowledge }
