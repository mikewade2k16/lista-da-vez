package observability

import (
	"context"
	"errors"
	"time"
)

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
	now := time.Now().UTC()
	for index := range rows {
		delete(rows[index].Detail, "_notify")
		if !silencedAt(rows[index], now) {
			// Silêncio vencido não é estado: a API só devolve silêncio em vigor.
			rows[index].SilencedUntil, rows[index].SilencedBy = nil, nil
		}
	}
	if rows == nil {
		rows = []alertRow{}
	}
	return rows, nil
}

func (s *Service) SendTestAlert(ctx context.Context) error {
	return s.alerts.SendTest(ctx)
}

// MinSilenceHours e MaxSilenceHours limitam POST /alerts/{id}/silence.
const (
	MinSilenceHours = 1
	MaxSilenceHours = 168
)

// ErrInvalidSilenceHours: hours fora de 1..168.
var ErrInvalidSilenceHours = errors.New("invalid silence hours")

// alertActor é quem executou a ação (id e nome do token do platform_admin).
type alertActor struct {
	UserID string
	Name   string
}

func (s *Service) AcknowledgeAlert(ctx context.Context, id int64, actor alertActor) error {
	return s.store.AcknowledgeAlert(ctx, id, actor.UserID, actor.Name, time.Now().UTC())
}

func (s *Service) SilenceAlert(ctx context.Context, id int64, hours int, actor alertActor) error {
	if hours < MinSilenceHours || hours > MaxSilenceHours {
		return ErrInvalidSilenceHours
	}
	until := time.Now().UTC().Add(time.Duration(hours) * time.Hour)
	return s.store.SilenceAlert(ctx, id, actor.UserID, actor.Name, until)
}

func (s *Service) UnsilenceAlert(ctx context.Context, id int64) error {
	return s.store.UnsilenceAlert(ctx, id)
}
