package accounting

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func (s *Service) Delete(ctx context.Context, sc Scope, id, key string, in Command) (json.RawMessage, error) {
	perm := "accounting.documents.manage"
	op := "document.delete." + id
	if id == "" {
		perm = "accounting.issuers.manage"
		op = "issuer.delete"
	}
	if e := s.check(ctx, sc, perm); e != nil {
		return nil, e
	}
	return s.store.command(ctx, sc, op, key, in, "manage", func(tx pgx.Tx) (any, error) {
		if id == "" {
			var active bool
			e := repositoryDeleteQueryRowExecutions(ctx, tx, sc.AccountID, sc.IssuerID, &active)
			if e != nil {
				return nil, e
			}
			if active {
				return nil, failure("execution_in_progress", 409)
			}
			var v int64
			e = repositoryDeleteQueryRowIssuers(ctx, tx, sc.AccountID, sc.IssuerID, in.ExpectedVersion, &v)
			if e != nil {
				return nil, failure("version_conflict", 409)
			}
			return map[string]any{"id": sc.IssuerID, "version": v, "status": "inactive"}, nil
		}
		d, e := loadDocument(ctx, tx, sc, id)
		if e != nil {
			return nil, e
		}
		if d.Version != in.ExpectedVersion {
			return nil, failure("version_conflict", 409)
		}
		if d.State != "draft" && d.State != "rejected" {
			return nil, failure("state_conflict", 409)
		}
		if e = invalidate(ctx, tx, sc, id); e != nil {
			return nil, e
		}
		_, e = repositoryDeleteExecDocuments(ctx, tx, sc.AccountID, sc.IssuerID, id)
		return map[string]any{"id": id, "deleted": true, "version": d.Version + 1}, e
	})
}
