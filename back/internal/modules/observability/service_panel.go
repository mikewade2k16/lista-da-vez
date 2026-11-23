package observability

import "context"

// Security devolve o catálogo de checagens e os achados.
func (s *Service) Security(ctx context.Context) (map[string]any, error) {
	checks, err := s.store.SecurityChecks(ctx)
	if err != nil {
		return nil, err
	}
	findings, err := s.store.SecurityFindings(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"checks": checks, "findings": findings}, nil
}

// System devolve as séries de amostras e a última amostra de cada container.
func (s *Service) System(ctx context.Context, hours int) (map[string]any, error) {
	api, err := s.store.SystemSeries(ctx, "api", hours)
	if err != nil {
		return nil, err
	}
	agent, err := s.store.SystemSeries(ctx, "agent", hours)
	if err != nil {
		return nil, err
	}
	containers, err := s.store.LatestContainerViews(ctx, hours)
	if err != nil {
		return nil, err
	}
	return map[string]any{"api": api, "agent": agent, "containers": containers}, nil
}

// Alerts lista os alertas por status (open|resolved|all), sem o estado interno.
func (s *Service) Alerts(ctx context.Context, status string) ([]alertRow, error) {
	rows, err := s.store.ListAlerts(ctx, status, 200)
	if err != nil {
		return nil, err
	}
	for index := range rows {
		delete(rows[index].Detail, "_notify")
	}
	if rows == nil {
		rows = []alertRow{}
	}
	return rows, nil
}

func (s *Service) SendTestAlert(ctx context.Context) error {
	return s.alerts.SendTest(ctx)
}
