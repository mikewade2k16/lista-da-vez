package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresPlatformSettingsRepository persiste a config GLOBAL da plataforma em
// core.platform_settings (key-value singleton por chave). Reaproveita o mesmo
// *pgxpool.Pool dos demais repositórios do módulo core.
type PostgresPlatformSettingsRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresPlatformSettingsRepository cria a implementação Postgres do
// repositório de platform_settings.
func NewPostgresPlatformSettingsRepository(pool *pgxpool.Pool) *PostgresPlatformSettingsRepository {
	return &PostgresPlatformSettingsRepository{pool: pool}
}

// GetByKey lê a config (jsonb cru) de uma chave singleton. Quando a linha ainda
// não existe, retorna config=nil e updatedAt/updatedBy=nil sem erro — o service
// traduz isso para o default vazio. updated_at e updated_by são scaneados como
// ponteiros (updated_by é nullable na coluna; ambos nil quando não há linha).
func (r *PostgresPlatformSettingsRepository) GetByKey(ctx context.Context, key string) (config []byte, updatedAt *string, updatedBy *string, err error) {
	const query = `
		select config, updated_at, updated_by
		from core.platform_settings
		where key = $1
	`
	var raw []byte
	var ts *time.Time
	var by *string
	if scanErr := r.pool.QueryRow(ctx, query, key).Scan(&raw, &ts, &by); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, scanErr
	}
	return raw, formatTimestamp(ts), by, nil
}

// Upsert insere ou atualiza a config de uma chave singleton e devolve o
// updated_at resultante (RFC3339). updatedBy é o userID de quem escreveu.
func (r *PostgresPlatformSettingsRepository) Upsert(ctx context.Context, key string, config []byte, updatedBy string) (updatedAt string, err error) {
	const query = `
		insert into core.platform_settings (key, config, updated_at, updated_by)
		values ($1, $2, now(), $3::uuid)
		on conflict (key) do update set
			config = excluded.config,
			updated_at = now(),
			updated_by = excluded.updated_by
		returning updated_at
	`
	var ts time.Time
	if scanErr := r.pool.QueryRow(ctx, query, key, config, updatedBy).Scan(&ts); scanErr != nil {
		return "", scanErr
	}
	return ts.UTC().Format(time.RFC3339), nil
}

// formatTimestamp converte um *time.Time nullable para *string RFC3339 (UTC),
// preservando nil quando a coluna é NULL.
func formatTimestamp(ts *time.Time) *string {
	if ts == nil {
		return nil
	}
	s := ts.UTC().Format(time.RFC3339)
	return &s
}
