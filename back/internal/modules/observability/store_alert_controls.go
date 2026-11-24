package observability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrAlertResolved: o alerta existe mas já foi resolvido (não aceita reconhecer/silenciar).
var ErrAlertResolved = errors.New("alert already resolved")

// actorNameSQL: nome gravado em acknowledged_by/silenced_by (core.users.display_name,
// com o nome do token como reserva se o usuário não for encontrado).
const actorNameSQL = `coalesce((select u.display_name from core.users u where u.id = $2::uuid), $3)`

// AcknowledgeAlert reconhece um alerta aberto. Reconhecer de novo preserva quem e quando.
func (s *Store) AcknowledgeAlert(ctx context.Context, id int64, userID, fallbackName string, now time.Time) error {
	tag, err := s.pool.Exec(quiet(ctx), `
		update obs.alerts set acknowledged_at = coalesce(acknowledged_at, $4),
		       acknowledged_by = coalesce(acknowledged_by, `+actorNameSQL+`)
		where id = $1 and status = 'open'`, id, userID, fallbackName, now)
	if err != nil {
		return fmt.Errorf("acknowledge alert: %w", err)
	}
	return s.controlResult(ctx, tag.RowsAffected(), id)
}

// SilenceAlert silencia os e-mails de um alerta aberto até `until`.
func (s *Store) SilenceAlert(ctx context.Context, id int64, userID, fallbackName string, until time.Time) error {
	tag, err := s.pool.Exec(quiet(ctx), `
		update obs.alerts set silenced_until = $4, silenced_by = `+actorNameSQL+`
		where id = $1 and status = 'open'`, id, userID, fallbackName, until)
	if err != nil {
		return fmt.Errorf("silence alert: %w", err)
	}
	return s.controlResult(ctx, tag.RowsAffected(), id)
}

// UnsilenceAlert reativa os avisos de um alerta aberto.
func (s *Store) UnsilenceAlert(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(quiet(ctx), `
		update obs.alerts set silenced_until = null, silenced_by = null
		where id = $1 and status = 'open'`, id)
	if err != nil {
		return fmt.Errorf("unsilence alert: %w", err)
	}
	return s.controlResult(ctx, tag.RowsAffected(), id)
}

// controlResult distingue "não existe" (404) de "já resolvido" (409) quando nada foi alterado.
func (s *Store) controlResult(ctx context.Context, affected int64, id int64) error {
	if affected > 0 {
		return nil
	}
	var status string
	err := s.pool.QueryRow(quiet(ctx), `select status from obs.alerts where id = $1`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("alert status: %w", err)
	}
	return ErrAlertResolved
}
