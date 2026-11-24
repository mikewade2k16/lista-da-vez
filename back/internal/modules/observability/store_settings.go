package observability

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GetSetting devolve o JSON gravado em obs.settings (found=false se a chave não existe).
func (s *Store) GetSetting(ctx context.Context, key string) (raw []byte, found bool, err error) {
	err = s.pool.QueryRow(quiet(ctx), `select value from obs.settings where key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get setting %s: %w", key, err)
	}
	return raw, true, nil
}

// PutSetting grava (upsert) o valor de uma chave e quem alterou.
func (s *Store) PutSetting(ctx context.Context, key string, value []byte, updatedBy string) error {
	_, err := s.pool.Exec(quiet(ctx), `
		insert into obs.settings (key, value, updated_by, updated_at)
		values ($1, $2::jsonb, $3, now())
		on conflict (key) do update
		   set value = excluded.value, updated_by = excluded.updated_by, updated_at = now()`,
		key, value, updatedBy)
	if err != nil {
		return fmt.Errorf("put setting %s: %w", key, err)
	}
	return nil
}
