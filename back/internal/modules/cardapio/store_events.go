package cardapio

import (
	"context"
	"encoding/json"
)

// InsertEvent grava um evento de telemetria (nome ja validado na allowlist pelo
// service). context jsonb opcional.
func (s *Store) InsertEvent(ctx context.Context, accountID, restaurantID, name, sessionID string, eventContext json.RawMessage) error {
	payload := eventContext
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}
	const q = `insert into cardapio.events (account_id, restaurant_id, name, session_id, context)
		values ($1, $2, $3, $4, $5)`
	_, err := s.pool.Exec(ctx, q, accountID, restaurantID, name, sessionID, []byte(payload))
	return err
}

// ListEvents retorna eventos crus paginados de um restaurante (mais recentes
// primeiro). Para o dashboard de analytics (fase futura) e listagem do painel.
func (s *Store) ListEvents(ctx context.Context, accountID, restaurantID string, limit, offset int) ([]EventView, int, error) {
	const countQ = `select count(*) from cardapio.events
		where restaurant_id = $1 and account_id = $2`
	var total int
	if err := s.pool.QueryRow(ctx, countQ, restaurantID, accountID).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `select id, name, session_id, context, created_at
		from cardapio.events
		where restaurant_id = $1 and account_id = $2
		order by created_at desc
		limit $3 offset $4`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]EventView, 0)
	for rows.Next() {
		var e EventView
		var eventContext []byte
		if err := rows.Scan(&e.ID, &e.Name, &e.SessionID, &eventContext, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		if len(eventContext) == 0 {
			eventContext = []byte("{}")
		}
		e.Context = eventContext
		out = append(out, e)
	}
	return out, total, rows.Err()
}
