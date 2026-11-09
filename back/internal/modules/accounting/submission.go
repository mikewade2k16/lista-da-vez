package accounting

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

func (s *Service) authorize(ctx context.Context, tx pgx.Tx, sc Scope, d Document, in Command) (any, error) {
	if d.State != "reviewed" || !in.Confirmed || !validID(in.ReviewID) {
		return nil, failure("state_conflict", 409)
	}
	r, e := s.review(ctx, tx, sc, d, in.ConnectorID, in.Environment)
	if e != nil {
		return nil, e
	}
	if len(r.Pending) > 0 || r.Hash != in.SnapshotHash {
		return nil, failure("snapshot_stale", 409)
	}
	if !r.EmissionAvailable {
		return nil, failure("coverage_unavailable", 422)
	}
	var exists bool
	e = repositoryAuthorizeQueryRowDocumentReviews(ctx, tx, sc.AccountID, sc.IssuerID, d.ID, in.ReviewID, r.Hash, &exists)
	if e != nil {
		return nil, e
	}
	if !exists {
		return nil, failure("snapshot_stale", 409)
	}
	if e = invalidateAuthorizations(ctx, tx, sc, d.ID); e != nil {
		return nil, e
	}
	access, e := issuerLock(ctx, tx, sc, "issue")
	if e != nil {
		return nil, e
	}
	var id string
	var expires time.Time
	e = repositoryAuthorizeQueryRowSubmissionAuthorizations(ctx, tx, sc.AccountID, sc.IssuerID, d.ID, in.ReviewID, sc.ActorID, r.Hash, in.ConnectorID, in.Environment, access, &id, &expires)
	if e != nil {
		return nil, e
	}
	if e = setDocumentState(ctx, tx, sc, d.ID, "authorized_for_submission"); e != nil {
		return nil, e
	}
	return map[string]any{"authorizationId": id, "documentId": d.ID, "version": d.Version + 1, "authorizationVersion": 1, "state": "authorized_for_submission", "expiresAt": expires.UTC(), "environment": in.Environment, "simulated": in.Environment == "fake_local"}, nil
}
func invalidateAuthorizations(ctx context.Context, tx pgx.Tx, s Scope, id string) error {
	_, e := repositoryInvalidateAuthorizationsExecSubmissionAuthorizations(ctx, tx, s.AccountID, s.IssuerID, id)
	return e
}
func (s *Service) execute(ctx context.Context, tx pgx.Tx, sc Scope, d Document, in Command) (any, error) {
	if d.State != "authorized_for_submission" || !validID(in.AuthorizationID) || in.Operation != "issue" || len(in.BusinessKey) < 8 || len(in.BusinessKey) > 200 {
		return nil, failure("state_conflict", 409)
	}
	var active bool
	e := repositoryExecuteQueryRowExecutions(ctx, tx, sc.AccountID, sc.IssuerID, d.ID, &active)
	if e != nil {
		return nil, e
	}
	if active {
		return nil, failure("execution_in_progress", 409)
	}
	if _, e = authorizationVersion(ctx, tx, sc, in.AuthorizationID, d.ID); e != nil {
		return nil, e
	}
	var connector, env, snapshot string
	e = repositoryExecuteQueryRowSubmissionAuthorizations(ctx, tx, sc.AccountID, sc.IssuerID, d.ID, in.AuthorizationID, sc.ActorID, &connector, &env, &snapshot)
	if e != nil {
		return nil, failure("authorization_expired", 409)
	}
	r, e := s.review(ctx, tx, sc, d, connector, env)
	if e != nil {
		return nil, e
	}
	if r.Hash != snapshot || len(r.Pending) > 0 {
		return nil, failure("snapshot_stale", 409)
	}
	if !r.EmissionAvailable {
		return nil, failure("coverage_unavailable", 422)
	}
	var id string
	e = repositoryExecuteQueryRowExecutions2(ctx, tx, sc.AccountID, sc.IssuerID, d.ID, in.AuthorizationID, sc.ActorID, env, connector, hash(in), in.BusinessKey, &id)
	if e != nil {
		return nil, failure("idempotency_conflict", 409)
	}
	if s.queue == nil {
		return nil, failure("capability_unavailable", 503)
	}
	payload, e := json.Marshal(map[string]string{"executionId": id})
	if e != nil {
		return nil, e
	}
	job, created, e := s.queue.EnqueueTx(ctx, tx, jobs.NewJob{AccountID: sc.AccountID, OrderingKey: sc.IssuerID + ":" + d.ID, IdempotencyKey: "accounting.issue:" + id, Kind: "accounting.issue.v1", Payload: payload, MaxAttempts: 3})
	if e != nil {
		return nil, e
	}
	if !created {
		return nil, failure("idempotency_conflict", 409)
	}
	_, e = repositoryExecuteExecExecutions(ctx, tx, sc.AccountID, sc.IssuerID, id, job)
	if e != nil {
		return nil, e
	}
	if e = event(ctx, tx, sc, id, "queued"); e != nil {
		return nil, e
	}
	if e = setDocumentState(ctx, tx, sc, d.ID, "authorized_for_submission"); e != nil {
		return nil, e
	}
	return loadExecution(ctx, tx, sc, id)
}
func event(ctx context.Context, tx pgx.Tx, s Scope, id, code string) error {
	_, e := repositoryEventExecExecutionEvents(ctx, tx, s.AccountID, s.IssuerID, id, code, s.ActorID)
	return e
}
func loadExecution(ctx context.Context, tx pgx.Tx, s Scope, id string) (Execution, error) {
	var x Execution
	if !validID(id) {
		return x, failure("not_found", 404)
	}
	e := repositoryLoadExecutionQueryRowExecutions(ctx, tx, s.AccountID, s.IssuerID, id, &x.ID, &x.JobID, &x.IssuerID, &x.DocumentID, &x.ActorID, &x.AuthorizationID, &x.State, &x.Version, &x.ConnectorID, &x.Environment, &x.RemoteIdentity, &x.RemoteReference, &x.Protocol, &x.SafeErrorCode, &x.ArtifactsPending, &x.Fence, &x.LeaseUntil)
	x.Simulated = x.Environment == "fake_local"
	return x, dbError(e)
}
