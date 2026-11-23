package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// deployStage é uma etapa do deploy, já reduzida pelo registrador (scripts/deploy).
type deployStage struct {
	Name       string   `json:"name"`
	Depth      int      `json:"depth"`
	Status     string   `json:"status"`
	DurationMs *float64 `json:"durationMs"`
}

// deployRow é uma linha de obs.deploys mais o veredito calculado no serviço.
type deployRow struct {
	ID          string        `json:"id"`
	Command     string        `json:"command"`
	Environment string        `json:"environment"`
	StartedAt   time.Time     `json:"startedAt"`
	FinishedAt  *time.Time    `json:"finishedAt"`
	DurationMs  *float64      `json:"durationMs"`
	ExitCode    *int          `json:"exitCode"`
	Status      string        `json:"status"`
	GitSha      string        `json:"gitSha"`
	GitBranch   string        `json:"gitBranch"`
	ImageTag    string        `json:"imageTag"`
	Verdict     string        `json:"verdict"`
	Stages      []deployStage `json:"stages"`
}

// RecentDeploys devolve os últimos deploys (mais recente primeiro).
func (s *Store) RecentDeploys(ctx context.Context, limit int) ([]deployRow, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select id, command, environment, started_at, finished_at, duration_ms, exit_code, status,
		       git_sha, git_branch, image_tag, stages
		from obs.deploys order by started_at desc limit $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent deploys: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (deployRow, error) {
		var d deployRow
		var raw []byte
		err := row.Scan(&d.ID, &d.Command, &d.Environment, &d.StartedAt, &d.FinishedAt, &d.DurationMs, &d.ExitCode,
			&d.Status, &d.GitSha, &d.GitBranch, &d.ImageTag, &raw)
		// JSON de etapas inesperado vira lista vazia: não derruba a listagem.
		_ = json.Unmarshal(raw, &d.Stages)
		if d.Stages == nil {
			d.Stages = []deployStage{}
		}
		return d, err
	})
}
