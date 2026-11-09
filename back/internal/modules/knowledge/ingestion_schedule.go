package knowledge

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// IngestionTick is called by the host's existing lifecycle; bounded work only.
func (s *Service) IngestionTick(ctx context.Context) error {
	if s.ingestionQueue == nil {
		return nil
	}
	if e := s.recoverIngestion(ctx); e != nil {
		return e
	}
	rows, e := s.store.pool.Query(ctx, `select `+subscriptionColumns+` from messaging.knowledge_source_subscriptions s join accounting.knowledge_bindings b on b.account_id=s.account_id and b.issuer_id=s.issuer_id and b.knowledge_base_id=s.base_id join messaging.knowledge_bases k on k.account_id=s.account_id and k.id=s.base_id where s.enabled and b.is_enabled and k.is_enabled and s.next_check_at<=now() order by s.next_check_at,s.id limit 20`)
	if e != nil {
		return e
	}
	list := []SourceSubscription{}
	for rows.Next() {
		v, e := scanSubscription(rows)
		if e != nil {
			rows.Close()
			return e
		}
		list = append(list, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, sub := range list {
		source, e := s.ingestionSource(sub.SourceID)
		authorization := s.authorize(ctx, sub.Scope, true)
		if authorization != nil && !ingestionDenied(authorization) {
			if _, err := s.store.pool.Exec(ctx, `update messaging.knowledge_source_subscriptions set next_check_at=now()+interval '1 minute' where account_id=$1::uuid and id=$2::uuid and version=$3`, sub.Scope.AccountID, sub.ID, sub.Version); err != nil {
				return err
			}
			continue
		}
		if e != nil || source.Revision != sub.SourceRevision || authorization != nil {
			if _, e = s.store.pool.Exec(ctx, `update messaging.knowledge_source_subscriptions set enabled=false,version=version+1,updated_at=now() where account_id=$1::uuid and id=$2::uuid and version=$3`, sub.Scope.AccountID, sub.ID, sub.Version); e != nil {
				return e
			}
			continue
		}
		if s.store.ingestionBase(ctx, sub.Scope, sub.BaseID) != nil {
			continue
		}
		if e = s.scheduleIngestion(ctx, sub); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) scheduleIngestion(ctx context.Context, sub SourceSubscription) error {
	tx, e := s.store.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var ok bool
	e = tx.QueryRow(ctx, `select enabled and version=$3 and next_check_at<=now() from messaging.knowledge_source_subscriptions where account_id=$1::uuid and id=$2::uuid for update`, sub.Scope.AccountID, sub.ID, sub.Version).Scan(&ok)
	if e != nil {
		return e
	}
	if !ok {
		return nil
	}
	if _, e = s.store.enqueueIngestion(ctx, tx, s.ingestionQueue, sub); e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `update messaging.knowledge_source_subscriptions set next_check_at=now()+interval_hours*interval '1 hour',updated_at=now() where account_id=$1::uuid and id=$2::uuid`, sub.Scope.AccountID, sub.ID)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Service) recoverIngestion(ctx context.Context) error {
	tx, e := s.store.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, e := tx.Query(ctx, `select `+ingestionRunColumns+` from messaging.knowledge_ingestion_runs r where (r.state='fetching' and r.lease_until<=now()) or (r.state in ('queued','retry_wait') and r.updated_at<now()-interval '30 minutes') order by r.updated_at,r.id limit 20 for update skip locked`)
	if e != nil {
		return e
	}
	list := []IngestionRun{}
	for rows.Next() {
		r, e := scanIngestionRun(rows)
		if e != nil {
			rows.Close()
			return e
		}
		list = append(list, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, r := range list {
		state := "queued"
		if r.Attempts >= 4 || r.Recoveries >= 4 {
			state = "failed"
		}
		if authorization := s.authorize(ctx, r.Scope, true); authorization != nil {
			if !ingestionDenied(authorization) {
				if _, e = tx.Exec(ctx, `update messaging.knowledge_ingestion_runs set error_code='authorization_unavailable',updated_at=now()where account_id=$1::uuid and id=$2::uuid`, r.Scope.AccountID, r.ID); e != nil {
					return e
				}
				continue
			}
			state = "cancelled"
		}
		_, e = tx.Exec(ctx, `update messaging.knowledge_ingestion_runs set state=$3,fence=fence+1,recoveries=recoveries+1,lease_until=null,error_code='lease_expired',updated_at=now()where account_id=$1::uuid and id=$2::uuid`, r.Scope.AccountID, r.ID, state)
		if e != nil {
			return e
		}
		if state == "queued" {
			_, _, e = s.ingestionQueue.EnqueueTx(ctx, tx, jobs.NewJob{AccountID: r.Scope.AccountID, OrderingKey: "knowledge:" + r.SubscriptionID, IdempotencyKey: "knowledge-recover:" + r.ID + ":" + stringFence(r.Fence), Kind: IngestionJobKind, Payload: raw(map[string]string{"runId": r.ID}), MaxAttempts: 4})
			if e != nil {
				return e
			}
		}
	}
	return tx.Commit(ctx)
}
