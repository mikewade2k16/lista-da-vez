package site

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresTrackingRepository persiste eventos brutos de tracking do site.
type PostgresTrackingRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresTrackingRepository cria o repo de tracking.
func NewPostgresTrackingRepository(pool *pgxpool.Pool) *PostgresTrackingRepository {
	return &PostgresTrackingRepository{pool: pool}
}

// List devolve eventos brutos de tracking para a grade administrativa.
func (r *PostgresTrackingRepository) List(ctx context.Context, filter TrackingEventListFilter) ([]TrackingEventView, int, error) {
	args := []any{filter.AccountID}
	conds := []string{"te.account_id = $1::uuid"}
	n := 2

	if filter.Q != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(filter.Q)) + "%"
		conds = append(conds, fmt.Sprintf(
			`(lower(te.source_label) like $%d or lower(te.source) like $%d or lower(te.event_type) like $%d or lower(te.event_name) like $%d or lower(te.page_path) like $%d or lower(te.page_title) like $%d or lower(te.session_id) like $%d or lower(te.visitor_id) like $%d or lower(te.product_code) like $%d)`,
			n, n, n, n, n, n, n, n, n,
		))
		args = append(args, pattern)
		n++
	}
	if filter.Source != "" {
		value := strings.TrimSpace(filter.Source)
		conds = append(conds, fmt.Sprintf("(te.source = $%d or te.source_label = $%d)", n, n))
		args = append(args, value)
		n++
	}
	if filter.EventType != "" {
		conds = append(conds, fmt.Sprintf("te.event_type = $%d", n))
		args = append(args, strings.TrimSpace(filter.EventType))
		n++
	}
	if filter.PagePath != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(filter.PagePath)) + "%"
		conds = append(conds, fmt.Sprintf("lower(te.page_path) like $%d", n))
		args = append(args, pattern)
		n++
	}

	where := strings.Join(conds, " and ")

	var total int
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("select count(*) from site.tracking_events te where %s", where),
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}

	args = append(args, perPage, (page-1)*perPage)
	dataSQL := fmt.Sprintf(`
		select te.id, te.account_id, te.source_id, te.source_label,
		       te.source, te.batch_id, te.source_event_id, te.visitor_id, te.session_id,
		       te.event_type, te.event_name,
		       te.page_url, te.page_path, te.page_title, te.page_group, te.page_name, te.referrer,
		       te.element_tag, te.element_text, te.element_href, te.element_id, te.element_classes, te.element_role,
		       te.product_code,
		       te.active_seconds, te.scroll_depth, te.screen_width, te.screen_height, te.viewport_width, te.viewport_height,
		       te.device_type, te.browser_lang, te.timezone,
		       te.utm_source, te.utm_medium, te.utm_campaign, te.utm_term, te.utm_content,
		       te.event_data::text, te.raw_payload::text,
		       te.ip, te.user_agent, te.sent_at, te.received_at
		from site.tracking_events te
		where %s
		order by te.received_at desc
		limit $%d offset $%d
	`, where, n, n+1)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]TrackingEventView, 0)
	for rows.Next() {
		view, err := scanTrackingEvent(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, view)
	}

	return out, total, rows.Err()
}

// InsertBatch insere eventos validos; duplicados pelo source_event_id sao
// ignorados por indice unico parcial para manter retry idempotente.
func (r *PostgresTrackingRepository) InsertBatch(ctx context.Context, input TrackingBatchInput) (int, int, error) {
	inserted := 0
	skipped := 0

	for _, event := range input.Events {
		rawPayload, err := json.Marshal(event.RawPayload)
		if err != nil {
			return inserted, skipped, err
		}
		eventData, err := encodeJSONValue(event.EventData)
		if err != nil {
			return inserted, skipped, err
		}

		var id string
		err = r.pool.QueryRow(ctx, `
			insert into site.tracking_events (
				account_id, source_id, source_label, source, batch_id, source_event_id,
				visitor_id, session_id, event_type, event_name,
				page_url, page_path, page_title, page_group, page_name, referrer,
				element_tag, element_text, element_href, element_id, element_classes, element_role, product_code,
				active_seconds, scroll_depth, screen_width, screen_height, viewport_width, viewport_height,
				device_type, browser_lang, timezone,
				utm_source, utm_medium, utm_campaign, utm_term, utm_content,
				event_data, raw_payload, ip, user_agent, sent_at
			) values (
				$1::uuid, $2::uuid, $3, $4, $5, $6,
				$7, $8, $9, $10,
				$11, $12, $13, $14, $15, $16,
				$17, $18, $19, $20, $21, $22, $23,
				$24, $25, $26, $27, $28, $29,
				$30, $31, $32,
				$33, $34, $35, $36, $37,
				nullif($38, '')::jsonb, $39::jsonb, $40, $41, $42
			)
			on conflict do nothing
			returning id::text
		`,
			input.AccountID, input.SourceID, input.SourceLabel, input.Source, input.BatchID, event.SourceEventID,
			event.VisitorID, event.SessionID, event.EventType, event.EventName,
			event.PageURL, event.PagePath, event.PageTitle, event.PageGroup, event.PageName, event.Referrer,
			event.ElementTag, event.ElementText, event.ElementHref, event.ElementID, event.ElementClasses, event.ElementRole, event.ProductCode,
			event.ActiveSeconds, event.ScrollDepth, event.ScreenWidth, event.ScreenHeight, event.ViewportWidth, event.ViewportHeight,
			event.DeviceType, event.BrowserLang, event.Timezone,
			event.UTMSource, event.UTMMedium, event.UTMCampaign, event.UTMTerm, event.UTMContent,
			eventData, string(rawPayload), event.IP, event.UserAgent, input.SentAt,
		).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			skipped++
			continue
		}
		if err != nil {
			return inserted, skipped, err
		}
		inserted++
	}

	return inserted, skipped, nil
}

func encodeJSONValue(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	if text, ok := value.(string); ok {
		if text == "" {
			return "", nil
		}
		if json.Valid([]byte(text)) {
			return text, nil
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func scanTrackingEvent(row scannable) (TrackingEventView, error) {
	var view TrackingEventView
	var sourceID, eventData, rawPayload *string
	var sentAt *time.Time
	if err := row.Scan(
		&view.ID, &view.AccountID, &sourceID, &view.SourceLabel,
		&view.Source, &view.BatchID, &view.SourceEventID, &view.VisitorID, &view.SessionID,
		&view.EventType, &view.EventName,
		&view.PageURL, &view.PagePath, &view.PageTitle, &view.PageGroup, &view.PageName, &view.Referrer,
		&view.ElementTag, &view.ElementText, &view.ElementHref, &view.ElementID, &view.ElementClasses, &view.ElementRole,
		&view.ProductCode,
		&view.ActiveSeconds, &view.ScrollDepth, &view.ScreenWidth, &view.ScreenHeight, &view.ViewportWidth, &view.ViewportHeight,
		&view.DeviceType, &view.BrowserLang, &view.Timezone,
		&view.UTMSource, &view.UTMMedium, &view.UTMCampaign, &view.UTMTerm, &view.UTMContent,
		&eventData, &rawPayload,
		&view.IP, &view.UserAgent, &sentAt, &view.ReceivedAt,
	); err != nil {
		return TrackingEventView{}, err
	}
	if sourceID != nil {
		view.SourceID = *sourceID
	}
	if eventData != nil {
		view.EventData = *eventData
	}
	if rawPayload != nil {
		view.RawPayload = *rawPayload
	}
	view.SentAt = sentAt
	return view, nil
}
