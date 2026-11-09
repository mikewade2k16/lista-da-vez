package knowledge

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func (s *Store) editDocument(ctx context.Context, sc Scope, id string, version int64, run func(pgx.Tx, Document) error) (Document, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return Document{}, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	d, e := scanFiscal(tx.QueryRow(ctx, `select `+fiscalDocColumns+fiscalDocFrom+`and d.id=$3::uuid for update of d,a,b`, sc.AccountID, sc.ResourceID, id))
	if e != nil {
		return d, e
	}
	if d.LockVersion != version {
		return d, fail("version_conflict", 409)
	}
	if e = run(tx, d); e != nil {
		return d, e
	}
	d, e = scanFiscal(tx.QueryRow(ctx, `select `+fiscalDocColumns+fiscalDocFrom+`and d.id=$3::uuid`, sc.AccountID, sc.ResourceID, id))
	if e != nil {
		return d, e
	}
	return d, tx.Commit(ctx)
}
func (s *Store) review(ctx context.Context, sc Scope, id string, in Review) (Document, error) {
	return s.editDocument(ctx, sc, id, in.ExpectedVersion, func(tx pgx.Tx, d Document) error {
		if d.Status != "draft" {
			return fail("immutable_version", 409)
		}
		status := "draft"
		if in.Confirmed {
			status = "reviewed"
		}
		_, e := tx.Exec(ctx, `update messaging.knowledge_document_applicability set metadata=$3::jsonb,status=$4,reviewed_by=case when $4='reviewed' then $5::uuid else null end,reviewed_at=case when $4='reviewed' then now() else null end,version_lock=version_lock+1,updated_at=now()where account_id=$1::uuid and document_id=$2::uuid`, sc.AccountID, id, raw(in.Metadata), status, sc.ActorID)
		return e
	})
}
func (s *Store) publish(ctx context.Context, sc Scope, id string, version int64) (Document, error) {
	return s.editDocument(ctx, sc, id, version, func(tx pgx.Tx, d Document) error {
		if d.Status != "draft" || d.ChunkCount < 1 || d.ReviewedBy == nil {
			return fail("review_required", 409)
		}
		var ready bool
		e := tx.QueryRow(ctx, `select status='reviewed' and reviewed_by is not null and original_storage_id is not null and checksum=$3 from messaging.knowledge_document_applicability where account_id=$1::uuid and document_id=$2::uuid`, sc.AccountID, id, d.Checksum).Scan(&ready)
		if e != nil {
			return e
		}
		if !ready {
			return fail("review_required", 409)
		}
		_, e = tx.Exec(ctx, `update messaging.knowledge_documents set status='published',updated_at=now()where account_id=$1::uuid and id=$2::uuid`, sc.AccountID, id)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `update messaging.knowledge_document_applicability set version_lock=version_lock+1,updated_at=now()where account_id=$1::uuid and document_id=$2::uuid`, sc.AccountID, id)
		return e
	})
}
func (s *Store) deactivate(ctx context.Context, sc Scope, id string, version int64) (Document, error) {
	return s.editDocument(ctx, sc, id, version, func(tx pgx.Tx, d Document) error {
		if d.Status != "published" {
			return fail("invalid_state", 409)
		}
		_, e := tx.Exec(ctx, `update messaging.knowledge_documents set status='archived',updated_at=now()where account_id=$1::uuid and id=$2::uuid`, sc.AccountID, id)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `update messaging.knowledge_document_applicability set status='inactive',version_lock=version_lock+1,updated_at=now()where account_id=$1::uuid and document_id=$2::uuid`, sc.AccountID, id)
		return e
	})
}
