package accounting

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// ReconcileStale repairs a worker crash after claim without depending on the old
// job's status. Discovery is bounded; all subsequent commands are account-filtered.
func (s *Service) ReconcileStale(ctx context.Context) error {
	rows, e := s.store.repositoryReconcileStaleQueryExecutions(ctx)
	if e != nil {
		return e
	}
	type candidate struct {
		account, issuer, id, document string
		fence                         int64
	}
	candidates := []candidate{}
	for rows.Next() {
		var c candidate
		if e = rows.Scan(&c.account, &c.issuer, &c.id, &c.document, &c.fence); e != nil {
			rows.Close()
			return e
		}
		candidates = append(candidates, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, c := range candidates {
		tx, e := s.store.begin(ctx)
		if e != nil {
			return e
		}
		payload, _ := json.Marshal(map[string]string{"executionId": c.id})
		_, _, e = s.queue.EnqueueTx(ctx, tx, jobs.NewJob{AccountID: c.account, OrderingKey: c.issuer + ":" + c.document, IdempotencyKey: fmt.Sprintf("accounting.reconcile:%s:%d", c.id, c.fence), Kind: "accounting.issue.v1", Payload: payload, RunAfter: time.Now()})
		if e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
	}
	return nil
}
