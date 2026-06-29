package cardapio

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

// eventInsert e a linha enriquecida (server-side) pronta para gravar em
// cardapio.events. Os campos vem do service apos sanitize + enriquecimento; o repo
// nao reinterpreta context. CreatedAt e sempre o relogio do servidor (definido aqui),
// OccurredAt e o do cliente (clampado no service).
type eventInsert struct {
	EventID      string
	Name         string
	SessionID    string
	DeviceID     string
	OccurredAt   time.Time
	PagePath     string
	ProductSlug  string
	DeviceType   string
	Browser      string
	OS           string
	ReferrerHost string
	UTMSource    string
	UTMMedium    string
	UTMCampaign  string
	IPHash       string
	DwellMS      int
	Context      json.RawMessage
}

// InsertEvent grava um evento de telemetria singular (nome ja validado na allowlist
// pelo service). context jsonb opcional.
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

// InsertEventsBatch grava varias linhas de evento numa unica ida ao banco (pgx.Batch).
// Cada INSERT faz ON CONFLICT (restaurant_id, event_id) WHERE event_id <> ” DO NOTHING
// (dedupe: beacon/visibilitychange duplicado nao conta em dobro). Retorna quantas
// linhas foram efetivamente inseridas (accepted), descontando as deduplicadas.
// account_id e restaurant_id vem resolvidos pelo slug — defesa em profundidade.
func (s *Store) InsertEventsBatch(ctx context.Context, accountID, restaurantID string, rows []eventInsert) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	const q = `insert into cardapio.events
		(account_id, restaurant_id, name, session_id, device_id, event_id, occurred_at,
		 page_path, product_slug, device_type, browser, os, referrer_host,
		 utm_source, utm_medium, utm_campaign, ip_hash, dwell_ms, context)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		on conflict (restaurant_id, event_id) where event_id <> '' do nothing`

	batch := &pgx.Batch{}
	for i := range rows {
		r := rows[i]
		payload := r.Context
		if len(payload) == 0 {
			payload = json.RawMessage("{}")
		}
		batch.Queue(q, accountID, restaurantID, r.Name, r.SessionID, r.DeviceID, r.EventID,
			r.OccurredAt, r.PagePath, r.ProductSlug, r.DeviceType, r.Browser, r.OS, r.ReferrerHost,
			r.UTMSource, r.UTMMedium, r.UTMCampaign, r.IPHash, r.DwellMS, []byte(payload))
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	accepted := 0
	for range rows {
		tag, err := br.Exec()
		if err != nil {
			return accepted, err
		}
		accepted += int(tag.RowsAffected())
	}
	return accepted, nil
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
