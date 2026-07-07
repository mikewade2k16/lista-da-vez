package calendar

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// planContext monta os insumos do payload C5 (contrato) para o disparo do plano:
// nome + perfil de cada cliente escolhido, feriados do mes (config da conta) e a
// nota do mes. Tudo no escopo da account (defesa em profundidade). Sem N+1: os
// nomes e perfis vem em UMA query cada (WHERE id/client_id = ANY($2)).
func (s *Store) planContext(ctx context.Context, accountID, month string, clientIDs []string) (planContext, error) {
	profiles, err := s.loadProfiles(ctx, accountID, clientIDs)
	if err != nil {
		return planContext{}, err
	}
	names, err := s.loadAccountNames(ctx, accountID, clientIDs)
	if err != nil {
		return planContext{}, err
	}
	clients := make([]planClient, 0, len(clientIDs))
	for _, id := range clientIDs {
		clients = append(clients, planClient{
			ID:      id,
			Name:    names[id],
			Profile: profiles[id],
		})
	}

	cfg, err := s.GetConfig(ctx, accountID)
	if err != nil {
		return planContext{}, err
	}
	from, to, err := monthBounds(month)
	if err != nil {
		return planContext{}, err
	}
	note, err := s.GetNotes(ctx, accountID, month)
	if err != nil {
		return planContext{}, err
	}
	return planContext{
		Clients:   clients,
		Holidays:  HolidaysInRange(from, to, cfg),
		MonthNote: note.Content,
	}, nil
}

// loadProfiles carrega os perfis dos clientes escolhidos em UMA query (sem N+1),
// no escopo da account. Cliente sem perfil => perfil zero (nao aparece no mapa).
func (s *Store) loadProfiles(ctx context.Context, accountID string, clientIDs []string) (map[string]planProfile, error) {
	out := map[string]planProfile{}
	if len(clientIDs) == 0 {
		return out, nil
	}
	const q = `select client_id::text, segment, positioning, description, history,
		site_url, instagram, address, objectives, brand_voice, coalesce(extra, '{}'::jsonb)
		from calendar.client_profiles
		where account_id = $1::uuid and client_id = any($2::uuid[])`
	rows, err := s.pool.Query(ctx, q, accountID, clientIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var p planProfile
		var extra json.RawMessage
		if err := rows.Scan(&id, &p.Segment, &p.Positioning, &p.Description, &p.History,
			&p.SiteURL, &p.Instagram, &p.Address, &p.Objectives, &p.BrandVoice, &extra); err != nil {
			return nil, err
		}
		p.Extra = decodeExtra(extra)
		out[id] = p
	}
	return out, rows.Err()
}

// loadAccountNames resolve o nome dos clientes (core.accounts) em UMA query, mas
// SO dos ids que a account dona do calendario ja referencia (em algum evento ou
// perfil dela). Sem essa amarra, uma conta poderia enviar UUIDs arbitrarios de
// contas de OUTROS donos e receber os nomes reais delas no payload/GET do plano —
// enumeracao de nomes cross-account. O escopo dos perfis/eventos e por account,
// entao o nome so vaza pelo lookup direto em core.accounts: aqui ele fica preso ao
// universo de clientes visiveis a esta account (mesma restricao do store.clients).
func (s *Store) loadAccountNames(ctx context.Context, accountID string, clientIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(clientIDs) == 0 {
		return out, nil
	}
	const q = `select a.id::text, coalesce(nullif(trim(a.name), ''), '')
		from core.accounts a
		where a.id = any($2::uuid[])
		  and (
			exists (select 1 from calendar.events e
				where e.account_id = $1::uuid and e.client_id = a.id)
			or exists (select 1 from calendar.client_profiles p
				where p.account_id = $1::uuid and p.client_id = a.id)
		  )`
	rows, err := s.pool.Query(ctx, q, accountID, clientIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// ListEventsLean devolve a projecao dos eventos da account na janela
// [from, to] (inclusive), opcionalmente filtrados por cliente, com teto de linhas
// (limit). Projecao date,type,title,status,client_id do contrato C9/C7: NAO carrega
// media/involved (evita over-fetch no agregado de contexto das IAs). Mesma ordem do
// ListEvents (event_date, event_time, created_at). Escopo por account_id (defesa em
// profundidade). limit <= 0 = sem teto.
func (s *Store) ListEventsLean(ctx context.Context, accountID, from, to, clientID string, limit int) ([]AIContextEvent, error) {
	q := `select id::text, event_date::text, event_time, type, title, status, priority,
		coalesce(client_id::text, ''), left(description, 500),
		coalesce(media, '[]'::jsonb), coalesce(linked_media, '[]'::jsonb)
		from calendar.events where account_id = $1::uuid`
	args := []any{accountID}
	if strings.TrimSpace(from) != "" {
		args = append(args, strings.TrimSpace(from))
		q += " and event_date >= $" + strconv.Itoa(len(args)) + "::date"
	}
	if strings.TrimSpace(to) != "" {
		args = append(args, strings.TrimSpace(to))
		q += " and event_date <= $" + strconv.Itoa(len(args)) + "::date"
	}
	if strings.TrimSpace(clientID) != "" {
		args = append(args, strings.TrimSpace(clientID))
		q += " and client_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	q += " order by event_date, event_time, created_at"
	if limit > 0 {
		args = append(args, limit)
		q += " limit $" + strconv.Itoa(len(args))
	}

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AIContextEvent, 0)
	for rows.Next() {
		e, err := scanAIContextEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListEventsLeanForClients projeta os eventos do mes SO dos clientes visiveis (contrato D4,
// modo 'all', WAVE 4): WHERE client_id = ANY($clientIDs) OR client_id IS NULL (eventos gerais
// da conta, sem cliente, entram). Fecha o vazamento de eventos de cliente fora do escopo do
// usuario (a agencia recebe todos os seus clientes; um usuario subset recebe so os que ve).
func (s *Store) ListEventsLeanForClients(ctx context.Context, accountID, from, to string, clientIDs []string, limit int) ([]AIContextEvent, error) {
	q := `select id::text, event_date::text, event_time, type, title, status, priority,
		coalesce(client_id::text, ''), left(description, 500),
		coalesce(media, '[]'::jsonb), coalesce(linked_media, '[]'::jsonb)
		from calendar.events where account_id = $1::uuid`
	args := []any{accountID}
	if strings.TrimSpace(from) != "" {
		args = append(args, strings.TrimSpace(from))
		q += " and event_date >= $" + strconv.Itoa(len(args)) + "::date"
	}
	if strings.TrimSpace(to) != "" {
		args = append(args, strings.TrimSpace(to))
		q += " and event_date <= $" + strconv.Itoa(len(args)) + "::date"
	}
	// Escopo de cliente: so os visiveis (+ eventos sem cliente = gerais da conta).
	args = append(args, clientIDs)
	q += " and (client_id = any($" + strconv.Itoa(len(args)) + "::uuid[]) or client_id is null)"
	q += " order by event_date, event_time, created_at"
	if limit > 0 {
		args = append(args, limit)
		q += " limit $" + strconv.Itoa(len(args))
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AIContextEvent, 0)
	for rows.Next() {
		e, err := scanAIContextEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanAIContextEvent(row rowScanner) (AIContextEvent, error) {
	var event AIContextEvent
	var ownMedia, linkedMedia json.RawMessage
	if err := row.Scan(&event.ID, &event.Date, &event.Time, &event.Type, &event.Title,
		&event.Status, &event.Priority, &event.ClientID, &event.Description,
		&ownMedia, &linkedMedia); err != nil {
		return AIContextEvent{}, err
	}
	event.Media = make([]MediaItem, 0)
	var items []MediaItem
	if err := json.Unmarshal(ownMedia, &items); err == nil {
		for _, item := range items {
			event.Media = appendContextMedia(event.Media, item)
		}
	}
	items = nil
	if err := json.Unmarshal(linkedMedia, &items); err == nil {
		for _, item := range items {
			event.Media = appendContextMedia(event.Media, item)
		}
	}
	return event, nil
}

// monthBounds devolve o primeiro e o ultimo dia do mes ('YYYY-MM') como
// 'YYYY-MM-DD'. Usado para a janela de feriados do payload.
func monthBounds(month string) (string, string, error) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return "", "", err
	}
	first := t
	last := first.AddDate(0, 1, -1)
	return first.Format("2006-01-02"), last.Format("2006-01-02"), nil
}
