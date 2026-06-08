package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionRepository persiste e consulta sessoes em core.user_sessions.
type SessionRepository interface {
	// Create registra uma nova sessao ativa e retorna o UUID gerado.
	Create(ctx context.Context, userID, userAgent, ip string) (sessionID string, err error)

	// IsRevoked retorna true se a sessao existir e estiver revogada (revoked_at NOT NULL).
	// Retorna false sem erro se a sessao nao existir (tokens legados sem sessionId).
	IsRevoked(ctx context.Context, sessionID string) (bool, error)

	// Revoke marca a sessao como revogada. Idempotente — nao falha se ja revogada.
	Revoke(ctx context.Context, sessionID string) error

	// Touch atualiza last_seen_at. Chamado no hot-path so quando cache miss.
	Touch(ctx context.Context, sessionID string) error
}

// PostgresSessionRepository implementa SessionRepository sobre core.user_sessions.
type PostgresSessionRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresSessionRepository cria um repositorio usando o pool compartilhado.
func NewPostgresSessionRepository(pool *pgxpool.Pool) *PostgresSessionRepository {
	return &PostgresSessionRepository{pool: pool}
}

func (r *PostgresSessionRepository) Create(ctx context.Context, userID, userAgent, ip string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO core.user_sessions (user_id, user_agent, ip)
		VALUES ($1::uuid, $2, $3)
		RETURNING id::text
	`, userID, userAgent, ip).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *PostgresSessionRepository) IsRevoked(ctx context.Context, sessionID string) (bool, error) {
	var revokedAt *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT revoked_at
		FROM core.user_sessions
		WHERE id = $1::uuid
	`, sessionID).Scan(&revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Sessao nao encontrada — token legado ou sessao nunca persistida.
			return false, nil
		}
		return false, err
	}
	return revokedAt != nil, nil
}

func (r *PostgresSessionRepository) Revoke(ctx context.Context, sessionID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE core.user_sessions
		SET revoked_at = now()
		WHERE id = $1::uuid AND revoked_at IS NULL
	`, sessionID)
	return err
}

func (r *PostgresSessionRepository) Touch(ctx context.Context, sessionID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE core.user_sessions
		SET last_seen_at = now()
		WHERE id = $1::uuid
	`, sessionID)
	return err
}
