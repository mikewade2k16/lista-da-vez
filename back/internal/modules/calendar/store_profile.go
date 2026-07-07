package calendar

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// Perfil estrategico do cliente (Fase 4) — calendar.client_profiles
// ============================================================================

// profileCols e a ordem esperada por scanProfile. extra sai com coalesce '{}'.
const profileCols = `client_id::text, segment, positioning, description, history,
	site_url, instagram, address, objectives, brand_voice,
	coalesce(extra, '{}'::jsonb), updated_at`

func scanProfile(row rowScanner) (ClientProfile, error) {
	var p ClientProfile
	var extra json.RawMessage
	err := row.Scan(&p.ClientID, &p.Segment, &p.Positioning, &p.Description, &p.History,
		&p.SiteURL, &p.Instagram, &p.Address, &p.Objectives, &p.BrandVoice,
		&extra, &p.UpdatedAt)
	if err != nil {
		return p, err
	}
	p.Extra = decodeExtra(extra)
	return p, nil
}

// GetClientProfile le o perfil (account, cliente). Segundo retorno = achou linha.
// WHERE por account_id (defesa em profundidade): perfil de outra account nunca sai.
func (s *Store) GetClientProfile(ctx context.Context, accountID, clientID string) (ClientProfile, bool, error) {
	const q = `select ` + profileCols + `
		from calendar.client_profiles
		where account_id = $1::uuid and client_id = $2::uuid`
	p, err := scanProfile(s.pool.QueryRow(ctx, q, accountID, clientID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ClientProfile{ClientID: clientID}, false, nil
	}
	if err != nil {
		return ClientProfile{}, false, err
	}
	return p, true, nil
}

// PutClientProfile faz upsert (full replace) do perfil no escopo da account.
func (s *Store) PutClientProfile(ctx context.Context, accountID string, p ClientProfile, updatedBy string) (ClientProfile, error) {
	extra, err := json.Marshal(p.Extra)
	if err != nil {
		return ClientProfile{}, err
	}
	const q = `
		insert into calendar.client_profiles
			(account_id, client_id, segment, positioning, description, history,
			 site_url, instagram, address, objectives, brand_voice, extra,
			 updated_by, updated_at)
		values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13, now())
		on conflict (account_id, client_id) do update set
			segment = excluded.segment, positioning = excluded.positioning,
			description = excluded.description, history = excluded.history,
			site_url = excluded.site_url, instagram = excluded.instagram,
			address = excluded.address, objectives = excluded.objectives,
			brand_voice = excluded.brand_voice, extra = excluded.extra,
			updated_by = excluded.updated_by, updated_at = now()
		returning ` + profileCols
	return scanProfile(s.pool.QueryRow(ctx, q,
		accountID, p.ClientID, p.Segment, p.Positioning, p.Description, p.History,
		p.SiteURL, p.Instagram, p.Address, p.Objectives, p.BrandVoice, extra, updatedBy))
}

// ListClientProfiles devolve o indice lean (clientId, filled, updatedAt) da account.
func (s *Store) ListClientProfiles(ctx context.Context, accountID string) ([]ProfileIndexItem, error) {
	const q = `select ` + profileCols + `
		from calendar.client_profiles
		where account_id = $1::uuid
		order by updated_at desc`
	rows, err := s.pool.Query(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProfileIndexItem, 0)
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ProfileIndexItem{
			ClientID:  p.ClientID,
			Filled:    profileFilled(p),
			UpdatedAt: p.UpdatedAt,
		})
	}
	return out, rows.Err()
}

// decodeExtra desserializa o jsonb do brief; falha/nulo -> struct zero.
func decodeExtra(raw json.RawMessage) ProfileExtra {
	var e ProfileExtra
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &e)
	}
	return e
}
