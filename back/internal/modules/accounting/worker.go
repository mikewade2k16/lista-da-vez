package accounting

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// Handle receives only durable execution IDs; business state, never the job retry,
// decides whether a POST is still safe. Reclaimed submissions are lookup-only.
func (s *Service) Handle(ctx context.Context, job jobs.Job) error {
	var input struct {
		ExecutionID string `json:"executionId"`
	}
	if e := json.Unmarshal(job.Payload, &input); e != nil || !validID(input.ExecutionID) {
		return failure("invalid_job", 400)
	}
	sc, x, d, snapshot, lookup, e := s.claimExecution(ctx, job.AccountID, input.ExecutionID)
	if e != nil {
		return e
	}
	if x.ID == "" {
		return nil
	}
	c, ok := s.connectors[x.ConnectorID]
	if !ok {
		return s.finish(ctx, sc, x, Outcome{State: "unknown"}, "connector_unavailable")
	}
	submission := Submission{ExecutionID: x.ID, RemoteIdentity: x.RemoteIdentity, Environment: x.Environment, SnapshotHash: snapshot, Document: d}
	if x.State == "authorized" {
		if !x.ArtifactsPending {
			return nil
		}
		return s.recoverArtifacts(ctx, sc, x, c, submission)
	}
	call, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	var outcome Outcome
	if lookup {
		outcome, e = c.Lookup(call, sc, submission)
	} else {
		outcome, e = c.Submit(call, sc, submission)
	}
	if e != nil || outcome.State == "" {
		outcome = Outcome{State: "unknown"}
	}
	// Even not_found/not_submitted after a timeout cannot prove the old worker won't finish.
	if outcome.State != "authorized" && outcome.State != "rejected" {
		outcome = Outcome{State: "unknown"}
	}
	if e = s.finish(ctx, sc, x, outcome, ""); e != nil {
		return e
	}
	if outcome.State == "authorized" {
		return s.recoverArtifacts(ctx, sc, x, c, submission)
	}
	return nil
}
func (s *Service) claimExecution(ctx context.Context, account, id string) (Scope, Execution, Document, string, bool, error) {
	var sc Scope
	var x Execution
	var d Document
	sc.AccountID = account
	// Discovery is account-filtered; the worker revalidates actor and issuer before effects.
	e := s.store.repositoryClaimExecutionQueryRowExecutions(ctx, account, id, &sc.IssuerID, &sc.ActorID)
	if e != nil {
		return sc, x, d, "", false, dbError(e)
	}
	tx, e := s.store.begin(ctx)
	if e != nil {
		return sc, x, d, "", false, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize grants/profile edits against the pre-send claim.
	var access int64
	e = repositoryClaimExecutionQueryRowIssuers(ctx, tx, account, sc.IssuerID, &access)
	if e != nil {
		return sc, x, d, "", false, e
	}
	x, e = loadExecution(ctx, tx, sc, id)
	if e != nil {
		return sc, x, d, "", false, e
	}
	if x.State == "authorized" {
		d, e = loadDocument(ctx, tx, sc, x.DocumentID)
		return sc, x, d, "", true, e
	}
	if x.State == "rejected" || x.State == "cancelled" {
		return sc, Execution{}, d, "", false, nil
	}
	if x.LeaseUntil != nil && x.LeaseUntil.After(time.Now()) {
		return sc, Execution{}, d, "", false, nil
	}
	d, e = loadDocument(ctx, tx, sc, x.DocumentID)
	if e != nil {
		return sc, x, d, "", false, e
	}
	lookup := x.State != "queued"
	var snapshot string
	var valid bool
	e = repositoryClaimExecutionQueryRowSubmissionAuthorizations(ctx, tx, account, sc.IssuerID, d.ID, x.AuthorizationID, access, &snapshot, &valid)
	if e != nil {
		return sc, x, d, "", false, e
	}
	if !lookup {
		if _, err := issuerLock(ctx, tx, sc, "issue"); err != nil {
			valid = false
		}
		if e = s.check(ctx, sc, "accounting.documents.issue"); e != nil {
			valid = false
		}
		r, err := s.review(ctx, tx, sc, d, x.ConnectorID, x.Environment)
		if err != nil {
			return sc, x, d, "", false, err
		}
		if !valid || r.Hash != snapshot || len(r.Pending) > 0 || !r.EmissionAvailable {
			_, e = repositoryClaimExecutionExecExecutions(ctx, tx, account, sc.IssuerID, id)
			if e == nil {
				e = event(ctx, tx, sc, id, "cancelled")
			}
			if e == nil {
				e = setDocumentState(ctx, tx, sc, d.ID, "draft")
			}
			if e != nil {
				return sc, x, d, "", false, e
			}
			return sc, Execution{}, d, "", false, tx.Commit(ctx)
		}
		_, e = repositoryClaimExecutionExecSubmissionAuthorizations(ctx, tx, account, sc.IssuerID, x.AuthorizationID)
		if e != nil {
			return sc, x, d, "", false, e
		}
	}
	e = repositoryClaimExecutionQueryRowExecutions2(ctx, tx, account, sc.IssuerID, id, id, &x.Fence)
	if e != nil {
		return sc, x, d, "", false, e
	}
	if e = setDocumentState(ctx, tx, sc, d.ID, "submitting"); e != nil {
		return sc, x, d, "", false, e
	}
	if e = event(ctx, tx, sc, id, "claimed"); e != nil {
		return sc, x, d, "", false, e
	}
	return sc, x, d, snapshot, lookup, tx.Commit(ctx)
}
func (s *Service) finish(ctx context.Context, sc Scope, x Execution, o Outcome, code string) error {
	tx, e := s.store.begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, e := loadExecution(ctx, tx, sc, x.ID)
	if e != nil {
		return e
	}
	if current.Fence != x.Fence || current.State != "processing" || current.LeaseUntil == nil || !current.LeaseUntil.After(time.Now()) {
		return nil
	}
	tag, e := repositoryFinishExecExecutions(ctx, tx, sc.AccountID, sc.IssuerID, x.ID, o.State, o.RemoteReference, o.Protocol, code, x.Fence)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return nil
	}
	if e = setDocumentState(ctx, tx, sc, x.DocumentID, o.State); e != nil {
		return e
	}
	if e = event(ctx, tx, sc, x.ID, o.State); e != nil {
		return e
	}
	if (o.State == "unknown" && x.Fence < 10) || o.State == "authorized" {
		b, e := json.Marshal(map[string]string{"executionId": x.ID})
		if e != nil {
			return e
		}
		_, _, e = s.queue.EnqueueTx(ctx, tx, jobs.NewJob{AccountID: sc.AccountID, OrderingKey: sc.IssuerID + ":" + x.DocumentID, IdempotencyKey: fmt.Sprintf("accounting.recover:%s:%s:%d", o.State, x.ID, x.Fence), Kind: "accounting.issue.v1", Payload: b, RunAfter: time.Now().Add(5 * time.Second)})
		if e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (s *Service) RevokeAuthorization(ctx context.Context, sc Scope, id, key string, in Command) (json.RawMessage, error) {
	if e := s.check(ctx, sc, "accounting.documents.issue"); e != nil {
		return nil, e
	}
	if !validID(id) {
		return nil, failure("not_found", 404)
	}
	return s.store.command(ctx, sc, "authorization.revoke."+id, key, in, "issue", func(tx pgx.Tx) (any, error) {
		current, e := authorizationVersion(ctx, tx, sc, id, "")
		if e != nil {
			return nil, e
		}
		if current != in.ExpectedVersion {
			return nil, failure("version_conflict", 409)
		}
		var doc string
		var v int64
		e = repositoryRevokeAuthorizationQueryRowSubmissionAuthorizations(ctx, tx, sc.AccountID, sc.IssuerID, id, in.ExpectedVersion, &doc, &v)
		if e != nil {
			return nil, failure("version_conflict", 409)
		}
		d, e := loadDocument(ctx, tx, sc, doc)
		if e != nil {
			return nil, e
		}
		if d.State == "authorized_for_submission" {
			_, e = repositoryRevokeAuthorizationExecExecutions(ctx, tx, sc.AccountID, sc.IssuerID, doc)
			if e != nil {
				return nil, e
			}
			if e = setDocumentState(ctx, tx, sc, doc, "reviewed"); e != nil {
				return nil, e
			}
		}
		return map[string]any{"authorizationId": id, "version": v, "revoked": true}, nil
	})
}
