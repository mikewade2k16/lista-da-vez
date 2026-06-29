package cardapio

import (
	"context"
	"log/slog"
	"time"
)

// defaultRetentionDays e a janela de retencao da telemetria (events/sessions). LGPD:
// linhas com device_id/session_id sao pseudonimos com vida curta; o rollup de longo
// prazo (futuro) sera anonimo (sem id). Sobrescrito por CARDAPIO_TELEMETRY_RETENTION_DAYS.
const defaultRetentionDays = 90

// startRetentionLoop dispara a poda diaria da telemetria em background. Para no
// fechamento do modulo (o canal stop e fechado em handle.Close). retentionDays <= 0
// desliga a poda automatica.
func (s *Service) startRetentionLoop(stop <-chan struct{}, retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	go func() {
		// Primeira poda ~5min apos o boot (fora do caminho critico de subida da api),
		// depois a cada 24h. O timer e reaproveitado para nao acumular goroutines.
		timer := time.NewTimer(5 * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-stop:
				return
			case <-timer.C:
				s.pruneTelemetryOnce(retentionDays)
				timer.Reset(24 * time.Hour)
			}
		}
	}()
}

// pruneTelemetryOnce roda uma poda com timeout proprio e loga o resultado (so quando
// apaga algo). Falha nao derruba nada — telemetria e best-effort.
func (s *Service) pruneTelemetryOnce(retentionDays int) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events, sessions, err := s.store.PruneTelemetry(ctx, retentionDays)
	if err != nil {
		slog.Warn("cardapio: poda de telemetria falhou", "error", err)
		return
	}
	if events > 0 || sessions > 0 {
		slog.Info("cardapio: poda de telemetria",
			"retentionDays", retentionDays, "eventsDeleted", events, "sessionsDeleted", sessions)
	}
}
