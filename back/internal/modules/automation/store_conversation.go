package automation

import "context"

// InsertMessage grava uma mensagem (in/out) de um contato. account_id e resolvido
// a partir da automacao (nunca vem do body do runtime).
func (s *Store) InsertMessage(ctx context.Context, automationID, contactID, direction, msgType, content, mediaURL, segment string) (Message, error) {
	const q = `insert into automation.messages
			(automation_id, account_id, contact_id, direction, type, content, media_url, segment)
		values ($1, (select account_id from automation.automations where id = $1), $2, $3, $4, $5, $6, $7)
		returning id, automation_id, account_id, contact_id, direction, type, content, media_url, segment, created_at`
	var m Message
	err := s.pool.QueryRow(ctx, q, automationID, contactID, direction, msgType, content, mediaURL, segment).Scan(
		&m.ID, &m.AutomationID, &m.AccountID, &m.ContactID, &m.Direction, &m.Type,
		&m.Content, &m.MediaURL, &m.Segment, &m.CreatedAt)
	return m, err
}

// GetLeadState retorna o estado do lead de um contato (ou pgx.ErrNoRows).
func (s *Store) GetLeadState(ctx context.Context, automationID, contactID string) (LeadState, error) {
	const q = `select contact_id, automation_id, account_id, status, last_interaction, follow_up_count
		from automation.lead_state
		where automation_id = $1 and contact_id = $2`
	var l LeadState
	err := s.pool.QueryRow(ctx, q, automationID, contactID).Scan(
		&l.ContactID, &l.AutomationID, &l.AccountID, &l.Status, &l.LastInteraction, &l.FollowUpCount)
	return l, err
}

// UpsertLeadState grava o estado do lead. account_id resolvido a partir da automacao.
func (s *Store) UpsertLeadState(ctx context.Context, automationID, contactID, status string, followUpCount int) (LeadState, error) {
	const q = `insert into automation.lead_state
			(contact_id, automation_id, account_id, status, last_interaction, follow_up_count, updated_at)
		values ($2, $1, (select account_id from automation.automations where id = $1), $3, now(), $4, now())
		on conflict (automation_id, contact_id) do update
		set status = excluded.status,
		    last_interaction = now(),
		    follow_up_count = excluded.follow_up_count,
		    updated_at = now()
		returning contact_id, automation_id, account_id, status, last_interaction, follow_up_count`
	var l LeadState
	err := s.pool.QueryRow(ctx, q, automationID, contactID, status, followUpCount).Scan(
		&l.ContactID, &l.AutomationID, &l.AccountID, &l.Status, &l.LastInteraction, &l.FollowUpCount)
	return l, err
}
