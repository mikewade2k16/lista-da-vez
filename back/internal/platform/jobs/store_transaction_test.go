package jobs_test

import (
	"context"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

func TestEnqueueTxRollbackAndLegacyCompatibility(t *testing.T) {
	pool := requireTestPool(t)
	account := setupEphemeralOutbox(t, pool)
	ctx := context.Background()
	store, e := jobs.NewPostgresStore(pool, fifoTestTable)
	if e != nil {
		t.Fatal(e)
	}
	job := jobs.NewJob{AccountID: account, OrderingKey: "synthetic", IdempotencyKey: "synthetic-transaction", Kind: "synthetic"}
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, created, e := store.EnqueueTx(ctx, tx, job); e != nil || !created {
		t.Fatalf("enqueue: %v", e)
	}
	var count int
	if e = pool.QueryRow(ctx, `select count(*) from jobs_fifo_test.outbox where account_id=$1`, account).Scan(&count); e != nil || count != 0 {
		t.Fatal("uncommitted job visible")
	}
	if e = tx.Rollback(ctx); e != nil {
		t.Fatal(e)
	}
	if _, created, e := store.Enqueue(ctx, job); e != nil || !created {
		t.Fatal("rollback retained job or changed legacy enqueue")
	}
	tx, e = pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	job.IdempotencyKey = "synthetic-commit"
	if _, created, e := store.EnqueueTx(ctx, tx, job); e != nil || !created {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if _, created, e := store.Enqueue(ctx, job); e != nil || created {
		t.Fatal("legacy dedupe changed")
	}
}
