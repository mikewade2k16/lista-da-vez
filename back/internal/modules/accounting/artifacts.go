package accounting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

func (s *Service) recoverArtifacts(ctx context.Context, sc Scope, x Execution, c Connector, submission Submission) error {
	// Remote authorization is already durable. Storage errors must never cause Submit again.
	if s.artifacts == nil {
		return nil
	}
	artifacts, e := c.FetchArtifacts(ctx, sc, submission)
	if e != nil {
		return nil
	}
	if len(artifacts) == 0 || len(artifacts) > 10 {
		return nil
	}
	for _, a := range artifacts {
		if len(a.Content) > 16*1024*1024 || len(a.Content) == 0 || len(a.Kind) > 80 {
			return nil
		}
		sum := sha256.Sum256(a.Content)
		checksum := hex.EncodeToString(sum[:])
		if a.SHA256 != "" && a.SHA256 != checksum {
			return nil
		}
		a.SHA256 = checksum
		object, e := s.artifacts.PutPrivate(ctx, sc, x.ID, a)
		if e != nil {
			return nil
		}
		if !validID(object) {
			return failure("invalid_storage_reference", 500)
		}
		_, e = s.store.repositoryRecoverArtifactsExecArtifacts(ctx, sc.AccountID, sc.IssuerID, x.ID, object, a.Kind, a.SHA256, len(a.Content))
		if e != nil {
			return e
		}
	}
	_, e = s.store.repositoryRecoverArtifactsExecExecutions(ctx, sc.AccountID, sc.IssuerID, x.ID)
	return e
}
func (s *Service) Recover(ctx context.Context, sc Scope, id, key string, in Command) (json.RawMessage, error) {
	if e := s.check(ctx, sc, "accounting.documents.issue"); e != nil {
		return nil, e
	}
	return s.store.command(ctx, sc, "execution.recover."+id, key, in, "issue", func(tx pgx.Tx) (any, error) {
		x, e := loadExecution(ctx, tx, sc, id)
		if e != nil {
			return nil, e
		}
		if x.Version != in.ExpectedVersion {
			return nil, failure("version_conflict", 409)
		}
		if x.State != "authorized" || !x.ArtifactsPending {
			return nil, failure("state_conflict", 409)
		}
		payload, _ := json.Marshal(map[string]string{"executionId": x.ID})
		job, _, e := s.queue.EnqueueTx(ctx, tx, jobs.NewJob{AccountID: sc.AccountID, OrderingKey: sc.IssuerID + ":" + x.DocumentID, IdempotencyKey: "accounting.artifacts:" + x.ID + ":" + key, Kind: "accounting.issue.v1", Payload: payload})
		if e != nil {
			return nil, e
		}
		if e = event(ctx, tx, sc, id, "artifact_recovery_requested"); e != nil {
			return nil, e
		}
		return map[string]any{"executionId": id, "jobId": job, "state": "queued"}, nil
	})
}
