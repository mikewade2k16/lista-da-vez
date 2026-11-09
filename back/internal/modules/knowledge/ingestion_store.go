package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

const subscriptionColumns = `s.id::text,s.base_id::text,s.source_id,s.source_revision,s.version,s.enabled,s.interval_hours,s.next_check_at,s.last_checksum,s.last_document_id::text,s.account_id::text,s.actor_id::text,s.issuer_id::text`

func scanSubscription(row rowScanner) (SourceSubscription, error) {
	var s SourceSubscription
	e := row.Scan(&s.ID, &s.BaseID, &s.SourceID, &s.SourceRevision, &s.Version, &s.Enabled, &s.IntervalHours, &s.NextCheckAt, &s.LastChecksum, &s.LastDocumentID, &s.Scope.AccountID, &s.Scope.ActorID, &s.Scope.ResourceID)
	return s, fiscalError(e)
}

const ingestionRunColumns = `r.id::text,r.subscription_id::text,r.subscription_version,r.base_id::text,r.source_id,r.source_revision,r.state,r.attempts,r.recoveries,r.fence,r.document_id::text,r.checksum,r.error_code,r.created_at,r.account_id::text,r.actor_id::text,r.issuer_id::text`

func scanIngestionRun(row rowScanner) (IngestionRun, error) {
	var r IngestionRun
	e := row.Scan(&r.ID, &r.SubscriptionID, &r.SubscriptionVersion, &r.BaseID, &r.SourceID, &r.SourceRevision, &r.State, &r.Attempts, &r.Recoveries, &r.Fence, &r.DocumentID, &r.Checksum, &r.ErrorCode, &r.CreatedAt, &r.Scope.AccountID, &r.Scope.ActorID, &r.Scope.ResourceID)
	return r, fiscalError(e)
}
func (s *Store) ingestionCommand(ctx context.Context, sc Scope, key string, in any, run func(pgx.Tx) (any, error)) (json.RawMessage, error) {
	if len(key) < 8 || len(key) > 200 {
		return nil, fail("invalid_idempotency_key", 400)
	}
	for _, r := range key {
		if r < 33 || r > 126 {
			return nil, fail("invalid_idempotency_key", 400)
		}
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, e = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, sc.AccountID+sc.ResourceID+sc.ActorID+key)
	if e != nil {
		return nil, e
	}
	raw, e := json.Marshal(in)
	if e != nil {
		return nil, e
	}
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	var old string
	var result json.RawMessage
	e = tx.QueryRow(ctx, `select request_hash,result from messaging.knowledge_ingestion_receipts where account_id=$1::uuid and issuer_id=$2::uuid and actor_id=$3::uuid and operation='source.save' and request_key=$4`, sc.AccountID, sc.ResourceID, sc.ActorID, key).Scan(&old, &result)
	if e == nil {
		if old != hash {
			return nil, fail("idempotency_conflict", 409)
		}
		return result, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return nil, e
	}
	out, e := run(tx)
	if e != nil {
		return nil, e
	}
	result, e = json.Marshal(out)
	if e != nil {
		return nil, e
	}
	_, e = tx.Exec(ctx, `insert into messaging.knowledge_ingestion_receipts(account_id,issuer_id,actor_id,operation,request_key,request_hash,result)values($1::uuid,$2::uuid,$3::uuid,'source.save',$4,$5,$6)`, sc.AccountID, sc.ResourceID, sc.ActorID, key, hash, result)
	if e != nil {
		return nil, e
	}
	return result, tx.Commit(ctx)
}
func (s *Store) ingestionBase(ctx context.Context, sc Scope, base string) error {
	var ok bool
	e := s.pool.QueryRow(ctx, `select exists(select 1 from accounting.knowledge_bindings b join messaging.knowledge_bases k on k.account_id=b.account_id and k.id=b.knowledge_base_id where b.account_id=$1::uuid and b.issuer_id=$2::uuid and k.id=$3::uuid and b.is_enabled and k.is_enabled)`, sc.AccountID, sc.ResourceID, base).Scan(&ok)
	if e == nil && !ok {
		return fail("base_unavailable", 409)
	}
	return e
}
func (s *Store) enqueueIngestion(ctx context.Context, tx pgx.Tx, q IngestionQueue, sub SourceSubscription) (IngestionRun, error) {
	var id string
	e := tx.QueryRow(ctx, `insert into messaging.knowledge_ingestion_runs(account_id,issuer_id,base_id,subscription_id,subscription_version,actor_id,source_id,source_revision,state)values($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6::uuid,$7,$8,'queued')on conflict do nothing returning id::text`, sub.Scope.AccountID, sub.Scope.ResourceID, sub.BaseID, sub.ID, sub.Version, sub.Scope.ActorID, sub.SourceID, sub.SourceRevision).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return scanIngestionRun(tx.QueryRow(ctx, `select `+ingestionRunColumns+` from messaging.knowledge_ingestion_runs r where r.account_id=$1::uuid and r.subscription_id=$2::uuid and r.state in ('queued','fetching','retry_wait')`, sub.Scope.AccountID, sub.ID))
	}
	if e != nil {
		return IngestionRun{}, e
	}
	_, _, e = q.EnqueueTx(ctx, tx, jobs.NewJob{AccountID: sub.Scope.AccountID, OrderingKey: "knowledge:" + sub.ID, IdempotencyKey: "knowledge-ingest:" + id, Kind: IngestionJobKind, Payload: raw(map[string]string{"runId": id}), MaxAttempts: 4})
	if e != nil {
		return IngestionRun{}, e
	}
	return scanIngestionRun(tx.QueryRow(ctx, `select `+ingestionRunColumns+` from messaging.knowledge_ingestion_runs r where r.account_id=$1::uuid and r.id=$2::uuid`, sub.Scope.AccountID, id))
}
