package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"strconv"
	"time"
)

func ingestionDenied(err error) bool {
	var domain *Error
	return errors.As(err, &domain) && (domain.Status == 401 || domain.Status == 403 || domain.Status == 404)
}
func stringFence(v int64) string { return strconv.FormatInt(v, 10) }
func (s *Service) finishIngestion(ctx context.Context, r IngestionRun, state, code string, doc *Document, checksum string) error {
	tx, e := s.store.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var enabled bool
	var version int64
	e = tx.QueryRow(ctx, `select enabled,version from messaging.knowledge_source_subscriptions where account_id=$1::uuid and id=$2::uuid for update`, r.Scope.AccountID, r.SubscriptionID).Scan(&enabled, &version)
	if e != nil {
		return e
	}
	if !enabled || version != r.SubscriptionVersion {
		state, code = "cancelled", "subscription_changed"
	}
	var documentID *string
	var digest *string
	if doc != nil {
		documentID = &doc.ID
		digest = &checksum
	}
	tag, e := tx.Exec(ctx, `update messaging.knowledge_ingestion_runs set state=$4,error_code=nullif($5,''),document_id=$6::uuid,checksum=$7,lease_until=null,updated_at=now()where account_id=$1::uuid and id=$2::uuid and fence=$3 and state='fetching'`, r.Scope.AccountID, r.ID, r.Fence, state, code, documentID, digest)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 1 && doc != nil && state != "cancelled" {
		_, e = tx.Exec(ctx, `update messaging.knowledge_source_subscriptions set last_checksum=$3,last_document_id=$4::uuid,updated_at=now()where account_id=$1::uuid and id=$2::uuid and version=$5`, r.Scope.AccountID, r.SubscriptionID, checksum, doc.ID, r.SubscriptionVersion)
		if e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (s *Service) Handle(ctx context.Context, job jobs.Job) error {
	var payload struct {
		RunID string `json:"runId"`
	}
	if job.Kind != IngestionJobKind || json.Unmarshal(job.Payload, &payload) != nil || !idPattern.MatchString(job.AccountID) || !idPattern.MatchString(payload.RunID) {
		return &jobs.StatusError{StatusCode: 422, Unrecoverable: true, Err: fail("invalid_ingestion_job", 422)}
	}
	r, e := scanIngestionRun(s.store.pool.QueryRow(ctx, `select `+ingestionRunColumns+` from messaging.knowledge_ingestion_runs r where r.account_id=$1::uuid and r.id=$2::uuid`, job.AccountID, payload.RunID))
	if e != nil {
		return fail("ingestion_run_unavailable", 503)
	}
	if r.State != "queued" && r.State != "retry_wait" && r.State != "fetching" {
		return nil
	}
	var fence int64
	var attempts int
	e = s.store.pool.QueryRow(ctx, `update messaging.knowledge_ingestion_runs set state='fetching',attempts=attempts+1,fence=fence+1,lease_until=now()+interval '60 seconds',updated_at=now() where account_id=$1::uuid and id=$2::uuid and (state in ('queued','retry_wait') or (state='fetching' and lease_until<=now())) returning fence,attempts`, job.AccountID, r.ID).Scan(&fence, &attempts)
	if e != nil {
		return &jobs.StatusError{StatusCode: 503, Err: fail("ingestion_busy", 503)}
	}
	r.Fence, r.Attempts = fence, attempts
	finish := func(state, code string, d *Document, digest string) error {
		if e := s.finishIngestion(ctx, r, state, code, d, digest); e != nil {
			return fail("ingestion_checkpoint_unavailable", 503)
		}
		return nil
	}
	if attempts > 4 {
		return finish("failed", "attempts_exhausted", nil, "")
	}
	authorize := func() error {
		err := s.authorize(ctx, r.Scope, true)
		if err == nil {
			return nil
		}
		if ingestionDenied(err) {
			if e := finish("cancelled", "permission_revoked", nil, ""); e != nil {
				return e
			}
			return fail("ingestion_cancelled", 409)
		}
		state := "retry_wait"
		if attempts >= 4 {
			state = "failed"
		}
		if e := finish(state, "authorization_unavailable", nil, ""); e != nil {
			return e
		}
		return fail("authorization_unavailable", 503)
	}
	if err := authorize(); err != nil {
		var code *Error
		if errors.As(err, &code) && code.Status == 409 {
			return nil
		}
		return err
	}
	source, e := s.ingestionSource(r.SourceID)
	if e != nil || source.Revision != r.SourceRevision {
		return finish("cancelled", "source_configuration_changed", nil, "")
	}
	var valid bool
	e = s.store.pool.QueryRow(ctx, `select exists(select 1 from messaging.knowledge_source_subscriptions where account_id=$1::uuid and id=$2::uuid and enabled and version=$3)`, r.Scope.AccountID, r.SubscriptionID, r.SubscriptionVersion).Scan(&valid)
	if e != nil {
		return fail("ingestion_store_unavailable", 503)
	}
	if !valid || s.store.ingestionBase(ctx, r.Scope, r.BaseID) != nil {
		return finish("cancelled", "subscription_or_base_changed", nil, "")
	}
	fetch := s.ports.FetchSource
	if fetch == nil {
		fetch = fetchIngestionSource
	}
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	content, e := fetch(call, source)
	if e == nil {
		if err := authorize(); err != nil {
			var code *Error
			if errors.As(err, &code) && code.Status == 409 {
				return nil
			}
			return err
		}
		hash := sha256.Sum256(content)
		checksum := hex.EncodeToString(hash[:])
		var old *string
		_ = s.store.pool.QueryRow(ctx, `select last_checksum from messaging.knowledge_source_subscriptions where account_id=$1::uuid and id=$2::uuid`, r.Scope.AccountID, r.SubscriptionID).Scan(&old)
		in := Import{ingestion: &ingestionImportGuard{RunID: r.ID, Fence: r.Fence, SubscriptionVersion: r.SubscriptionVersion}, BaseID: r.BaseID, Title: source.Title, SourceRef: source.URL, FileName: source.ID, ContentType: source.ContentType, Content: content, Metadata: Metadata{Authority: source.Authority, Jurisdiction: "unknown", Regime: "unknown", Classification: "informational_unverified"}}
		var doc Document
		doc, e = s.Import(call, r.Scope, in)
		if e == nil {
			state := "review_pending"
			if old != nil && *old == checksum {
				state = "unchanged"
			}
			return finish(state, "", &doc, checksum)
		}
	}
	code := "ingestion_unavailable"
	status := 503
	var domain *Error
	if errors.As(e, &domain) {
		code, status = domain.Code, domain.Status
	}
	retry := status >= 500 && attempts < 4
	state := "failed"
	if retry {
		state = "retry_wait"
	}
	if e = finish(state, code, nil, ""); e != nil {
		return e
	}
	if retry {
		return &jobs.StatusError{StatusCode: 503, Err: fail(code, 503)}
	}
	return nil
}
