package cardapio

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// GetPublishedLayout retorna o layout PUBLICADO (jsonb) + version. pgx.ErrNoRows
// se nao ha linha; published vazio ('{}') e tratado como "sem publicado" no service.
func (s *Store) GetPublishedLayout(ctx context.Context, accountID, restaurantID string) (json.RawMessage, int64, error) {
	const q = `select published, version from cardapio.site_layouts
		where restaurant_id = $1 and account_id = $2`
	var published []byte
	var version int64
	if err := s.pool.QueryRow(ctx, q, restaurantID, accountID).Scan(&published, &version); err != nil {
		return nil, 0, err
	}
	return json.RawMessage(published), version, nil
}

// GetDraftLayout retorna o RASCUNHO (jsonb) + version + found (false sem linha).
func (s *Store) GetDraftLayout(ctx context.Context, accountID, restaurantID string) (json.RawMessage, int64, bool, error) {
	const q = `select draft, version from cardapio.site_layouts
		where restaurant_id = $1 and account_id = $2`
	var draft []byte
	var version int64
	err := s.pool.QueryRow(ctx, q, restaurantID, accountID).Scan(&draft, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	return json.RawMessage(draft), version, true, nil
}

// PutDraftLayout faz upsert do rascunho com concorrencia otimista por version.
// expectedVersion != nil e a linha existe com version diferente => ErrVersionConflict.
// Retorna o draft salvo + a nova version.
func (s *Store) PutDraftLayout(ctx context.Context, accountID, restaurantID string, draft json.RawMessage, expectedVersion *int64) (json.RawMessage, int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var currentVersion int64
	exists := true
	err = tx.QueryRow(ctx, `select version from cardapio.site_layouts
		where restaurant_id = $1 and account_id = $2 for update`, restaurantID, accountID).Scan(&currentVersion)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		exists = false
	case err != nil:
		return nil, 0, err
	}

	if exists && expectedVersion != nil && *expectedVersion != currentVersion {
		return nil, 0, ErrVersionConflict
	}

	var newVersion int64
	if exists {
		const upd = `update cardapio.site_layouts
			set draft = $3, version = version + 1, updated_at = now()
			where restaurant_id = $1 and account_id = $2
			returning version`
		if err := tx.QueryRow(ctx, upd, restaurantID, accountID, []byte(draft)).Scan(&newVersion); err != nil {
			return nil, 0, err
		}
	} else {
		const ins = `insert into cardapio.site_layouts
			(account_id, restaurant_id, draft, published, version)
			values ($1, $2, $3, '{}'::jsonb, 1)
			returning version`
		if err := tx.QueryRow(ctx, ins, accountID, restaurantID, []byte(draft)).Scan(&newVersion); err != nil {
			return nil, 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return draft, newVersion, nil
}

// PublishLayout promove o rascunho atual para publicado (version++). pgx.ErrNoRows
// se nao ha linha (nada para publicar).
func (s *Store) PublishLayout(ctx context.Context, accountID, restaurantID string) (json.RawMessage, int64, error) {
	const q = `update cardapio.site_layouts
		set published = draft, version = version + 1, updated_at = now()
		where restaurant_id = $1 and account_id = $2
		returning published, version`
	var published []byte
	var version int64
	if err := s.pool.QueryRow(ctx, q, restaurantID, accountID).Scan(&published, &version); err != nil {
		return nil, 0, err
	}
	return json.RawMessage(published), version, nil
}
