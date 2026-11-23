package obsagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const insertContainerSQL = `insert into obs.container_samples
  (occurred_at, host, container, service, state, health, restart_count, started_at, cpu_pct, mem_bytes, mem_limit_bytes, restart_policy, exit_code)
  values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`

const insertSystemSQL = `insert into obs.system_samples (occurred_at, source, host, metrics) values ($1,'agent',$2,$3)`

// Store grava snapshots em lote (uma ida ao banco por ciclo).
type Store struct {
	Pool *pgxpool.Pool
}

// Save grava todas as linhas do ciclo numa unica transacao implicita do batch.
func (s *Store) Save(ctx context.Context, snap Snapshot) error {
	metrics, err := json.Marshal(snap.System)
	if err != nil {
		return err
	}
	batch := &pgx.Batch{}
	for _, c := range snap.Containers {
		batch.Queue(insertContainerSQL, snap.At, snap.Host, c.Container, c.Service, c.State, c.Health,
			c.RestartCount, c.StartedAt, c.CPUPct, c.MemBytes, c.MemLimitBytes, c.RestartPolicy, c.ExitCode)
	}
	batch.Queue(insertSystemSQL, snap.At, snap.Host, metrics)

	results := s.Pool.SendBatch(ctx, batch)
	defer results.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("insert %d/%d: %w", i+1, batch.Len(), err)
		}
	}
	return results.Close()
}
