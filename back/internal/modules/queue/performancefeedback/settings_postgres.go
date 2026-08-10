package performancefeedback

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (repository *PostgresRepository) FindSettings(ctx context.Context, tenantID string) (Settings, error) {
	var settings Settings
	var sectionsJSON []byte
	err := repository.pool.QueryRow(ctx, `
		select
			tenant_id::text,
			cadence,
			default_sections,
			coalesce(updated_by_user_id::text, ''),
			created_at,
			updated_at,
			version
		from queue.performance_feedback_settings
		where tenant_id = $1::uuid
		limit 1;
	`, tenantID).Scan(
		&settings.TenantID,
		&settings.Cadence,
		&sectionsJSON,
		&settings.UpdatedByUserID,
		&settings.CreatedAt,
		&settings.UpdatedAt,
		&settings.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, ErrNotFound
	}
	if err != nil {
		return Settings{}, err
	}
	if err := json.Unmarshal(sectionsJSON, &settings.DefaultSections); err != nil {
		return Settings{}, err
	}
	settings.Configured = true
	return settings, nil
}

func (repository *PostgresRepository) UpsertSettings(ctx context.Context, settings Settings, expectedVersion int) (Settings, error) {
	sectionsJSON, err := json.Marshal(settings.DefaultSections)
	if err != nil {
		return Settings{}, err
	}

	if expectedVersion == 0 {
		_, err = repository.pool.Exec(ctx, `
			insert into queue.performance_feedback_settings (
				tenant_id, cadence, default_sections, updated_by_user_id
			)
			values ($1::uuid, $2, $3::jsonb, $4::uuid);
		`, settings.TenantID, settings.Cadence, sectionsJSON, nullableID(settings.UpdatedByUserID))
		if err != nil {
			if isUniqueViolation(err) {
				return Settings{}, ErrConflict
			}
			return Settings{}, err
		}
		return repository.FindSettings(ctx, settings.TenantID)
	}

	command, err := repository.pool.Exec(ctx, `
		update queue.performance_feedback_settings
		set cadence = $3,
			default_sections = $4::jsonb,
			updated_by_user_id = $5::uuid,
			updated_at = now(),
			version = version + 1
		where tenant_id = $1::uuid
		  and version = $2;
	`, settings.TenantID, expectedVersion, settings.Cadence, sectionsJSON, nullableID(settings.UpdatedByUserID))
	if err != nil {
		return Settings{}, err
	}
	if command.RowsAffected() != 1 {
		return Settings{}, ErrConflict
	}
	return repository.FindSettings(ctx, settings.TenantID)
}
