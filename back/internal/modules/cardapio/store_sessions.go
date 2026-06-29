package cardapio

import (
	"context"
	"time"
)

// sessionUpsert e a agregacao de UMA ingestao (um lote) para a sessao. Acumula no
// upsert: pageviews/events somam; last_seen avanca; duracao reflete o maior valor
// observado. Os campos de origem (utm/referrer/device_type/landing) so preenchem se a
// sessao ainda estiver vazia (primeira escrita vence). had_order NAO e setado aqui —
// fica false; outra frente liga ao pedido. account_id/restaurant_id resolvidos pelo
// slug (defesa em profundidade).
type sessionUpsert struct {
	AccountID    string
	RestaurantID string
	SessionID    string
	DeviceID     string
	LastSeenAt   time.Time
	DurationMS   int64
	Pageviews    int
	Events       int
	UTMSource    string
	UTMMedium    string
	UTMCampaign  string
	ReferrerHost string
	DeviceType   string
	LandingPath  string
}

// UpsertSession cria ou atualiza a sessao agregada. ON CONFLICT (restaurant_id,
// session_id): last_seen_at = greatest(atual, novo); duration_ms = greatest;
// pageviews/events somam; device_id e os campos de origem (utm/referrer/device_type/
// landing) so sao preenchidos quando ainda vazios (NULLIF(atual,”) vazio => usa o
// novo, via COALESCE). updated_at avanca.
func (s *Store) UpsertSession(ctx context.Context, in sessionUpsert) error {
	if in.SessionID == "" {
		return nil
	}
	const q = `insert into cardapio.sessions
		(account_id, restaurant_id, session_id, device_id, first_seen_at, last_seen_at,
		 duration_ms, pageviews, events, utm_source, utm_medium, utm_campaign,
		 referrer_host, device_type, landing_path)
		values ($1,$2,$3,$4,$5,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		on conflict (restaurant_id, session_id) do update set
			last_seen_at  = greatest(cardapio.sessions.last_seen_at, excluded.last_seen_at),
			duration_ms   = greatest(cardapio.sessions.duration_ms, excluded.duration_ms),
			pageviews     = cardapio.sessions.pageviews + excluded.pageviews,
			events        = cardapio.sessions.events + excluded.events,
			device_id     = coalesce(nullif(cardapio.sessions.device_id, ''), excluded.device_id),
			utm_source    = coalesce(nullif(cardapio.sessions.utm_source, ''), excluded.utm_source),
			utm_medium    = coalesce(nullif(cardapio.sessions.utm_medium, ''), excluded.utm_medium),
			utm_campaign  = coalesce(nullif(cardapio.sessions.utm_campaign, ''), excluded.utm_campaign),
			referrer_host = coalesce(nullif(cardapio.sessions.referrer_host, ''), excluded.referrer_host),
			device_type   = coalesce(nullif(cardapio.sessions.device_type, ''), excluded.device_type),
			landing_path  = coalesce(nullif(cardapio.sessions.landing_path, ''), excluded.landing_path),
			updated_at    = now()`
	_, err := s.pool.Exec(ctx, q, in.AccountID, in.RestaurantID, in.SessionID, in.DeviceID,
		in.LastSeenAt, in.DurationMS, in.Pageviews, in.Events, in.UTMSource, in.UTMMedium,
		in.UTMCampaign, in.ReferrerHost, in.DeviceType, in.LandingPath)
	return err
}

// listProductSlugs retorna o conjunto de slugs de produto do restaurante, para o
// service validar product_slug do context antes de promover a coluna (slug que nao
// pertence ao restaurante => grava ”). Escopado por account_id + restaurant_id.
func (s *Store) listProductSlugs(ctx context.Context, accountID, restaurantID string) (map[string]struct{}, error) {
	const q = `select slug from cardapio.products
		where restaurant_id = $1 and account_id = $2`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		out[slug] = struct{}{}
	}
	return out, rows.Err()
}

// PruneTelemetry remove eventos e sessoes mais antigos que retentionDays (retencao
// LGPD). DELETE parametrizado via make_interval; events por created_at (relogio do
// servidor) e sessions por last_seen_at. Retorna quantas linhas foram apagadas em cada.
func (s *Store) PruneTelemetry(ctx context.Context, retentionDays int) (events, sessions int64, err error) {
	const delEvents = `delete from cardapio.events where created_at < now() - make_interval(days => $1)`
	tagE, err := s.pool.Exec(ctx, delEvents, retentionDays)
	if err != nil {
		return 0, 0, err
	}
	const delSessions = `delete from cardapio.sessions where last_seen_at < now() - make_interval(days => $1)`
	tagS, err := s.pool.Exec(ctx, delSessions, retentionDays)
	if err != nil {
		return tagE.RowsAffected(), 0, err
	}
	return tagE.RowsAffected(), tagS.RowsAffected(), nil
}
