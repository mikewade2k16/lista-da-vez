package observability

import (
	"context"
	"math"
	"time"
)

// integrationRow é a agregação de obs.external_calls por integração no período.
type integrationRow struct {
	Name        string
	Calls       int64
	Errors      int64
	AvgMs       float64
	P95Ms       float64
	LastErrorAt *time.Time
	LastError   string
}

// IntegrationStat é um item de GET /integrations. `host` é o nome da integração
// gravado pelo cliente HTTP instrumentado (waha, meta, google, r2...).
type IntegrationStat struct {
	Host        string     `json:"host"`
	Calls       int64      `json:"calls"`
	Errors      int64      `json:"errors"`
	ErrorRate   float64    `json:"errorRate"`
	AvgMs       float64    `json:"avgMs"`
	P95Ms       float64    `json:"p95Ms"`
	LastErrorAt *time.Time `json:"lastErrorAt"`
	LastError   string     `json:"lastError"`
}

func toIntegrationStat(row integrationRow) IntegrationStat {
	stat := IntegrationStat{Host: row.Name, Calls: row.Calls, Errors: row.Errors,
		AvgMs: round1(row.AvgMs), P95Ms: round1(row.P95Ms), LastErrorAt: row.LastErrorAt, LastError: row.LastError}
	if row.Calls > 0 {
		stat.ErrorRate = math.Round(float64(row.Errors)/float64(row.Calls)*10000) / 10000
	}
	return stat
}

// Integrations lista tempo, erro e volume por integração externa (mais chamadas primeiro).
func (s *Service) Integrations(ctx context.Context, days int) ([]IntegrationStat, error) {
	rows, err := s.store.IntegrationStats(ctx, days)
	if err != nil {
		return nil, err
	}
	items := make([]IntegrationStat, 0, len(rows))
	for _, row := range rows {
		items = append(items, toIntegrationStat(row))
	}
	return items, nil
}
