package settings

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type gamificationSettingsScanRow struct {
	TenantID   string    `db:"tenant_id"`
	BadgeRules []byte    `db:"badge_rules"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// GetGamificationSection le a secao de gamificacao de tenant_gamification_settings.
// Retorna (record, false, nil) quando a linha ainda nao existe; o service usa o default.
func (repository *PostgresRepository) GetGamificationSection(ctx context.Context, tenantID string) (GamificationSectionRecord, bool, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			tenant_id::text,
			badge_rules,
			updated_at
		from public.tenant_gamification_settings
		where tenant_id = $1::uuid
		limit 1;
	`, tenantID)
	if err != nil {
		return GamificationSectionRecord{}, false, err
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[gamificationSettingsScanRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GamificationSectionRecord{}, false, nil
		}
		return GamificationSectionRecord{}, false, err
	}

	var badgeRules []BadgeRule
	if len(row.BadgeRules) > 0 {
		if unmarshalErr := json.Unmarshal(row.BadgeRules, &badgeRules); unmarshalErr != nil {
			return GamificationSectionRecord{}, false, unmarshalErr
		}
	}

	if badgeRules == nil {
		badgeRules = defaultBadgeRules()
	}

	return GamificationSectionRecord{
		TenantID: row.TenantID,
		Config: GamificationConfig{
			BadgeRules: badgeRules,
		},
		UpdatedAt: row.UpdatedAt,
	}, true, nil
}

// UpsertGamificationSection grava as badge rules do tenant em tenant_gamification_settings.
func (repository *PostgresRepository) UpsertGamificationSection(ctx context.Context, section GamificationSectionRecord) (GamificationSectionRecord, error) {
	badgeRulesJSON, err := json.Marshal(section.Config.BadgeRules)
	if err != nil {
		return GamificationSectionRecord{}, err
	}

	_, err = repository.pool.Exec(ctx, `
		insert into public.tenant_gamification_settings (
			tenant_id,
			badge_rules,
			updated_at
		)
		values ($1::uuid, $2::jsonb, now())
		on conflict (tenant_id) do update
		set
			badge_rules = excluded.badge_rules,
			updated_at  = now();
	`, section.TenantID, string(badgeRulesJSON))
	if err != nil {
		return GamificationSectionRecord{}, err
	}

	saved, found, err := repository.GetGamificationSection(ctx, section.TenantID)
	if err != nil {
		return GamificationSectionRecord{}, err
	}
	if !found {
		return GamificationSectionRecord{}, ErrTenantNotFound
	}
	return saved, nil
}
