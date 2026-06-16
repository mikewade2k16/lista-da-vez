package automation

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// SaveMessage persiste uma mensagem (in/out) de um contato (chamado pelo n8n).
// Resolve a sessao -> automacao; account_id e resolvido no store a partir dela.
func (s *Service) SaveMessage(ctx context.Context, session, contactID, direction, msgType, content, mediaURL, segment string) (MessageView, error) {
	ch, err := s.store.GetChannelBySession(ctx, session)
	if err != nil {
		return MessageView{}, err
	}
	if direction != directionIn && direction != directionOut {
		direction = directionIn
	}
	if msgType == "" {
		msgType = msgTypeText
	}
	m, err := s.store.InsertMessage(ctx, ch.AutomationID, contactID, direction, msgType, content, mediaURL, segment)
	if err != nil {
		return MessageView{}, err
	}
	return toMessageView(m), nil
}

// LeadState retorna o estado do lead de um contato (consumido pelo n8n).
// Defaults zerados (status "new", follow_up 0) se ainda nao existe.
func (s *Service) LeadState(ctx context.Context, session, contactID string) (LeadStateView, error) {
	ch, err := s.store.GetChannelBySession(ctx, session)
	if err != nil {
		return LeadStateView{}, err
	}
	l, err := s.store.GetLeadState(ctx, ch.AutomationID, contactID)
	if errors.Is(err, pgx.ErrNoRows) {
		return LeadStateView{ContactID: contactID, Status: defaultStatus}, nil
	}
	if err != nil {
		return LeadStateView{}, err
	}
	return toLeadStateView(l), nil
}

// SetLeadState grava o estado do lead (chamado pelo n8n). status vazio = "new".
func (s *Service) SetLeadState(ctx context.Context, session, contactID, status string, followUpCount int) (LeadStateView, error) {
	ch, err := s.store.GetChannelBySession(ctx, session)
	if err != nil {
		return LeadStateView{}, err
	}
	if status == "" {
		status = defaultStatus
	}
	if followUpCount < 0 {
		followUpCount = 0
	}
	l, err := s.store.UpsertLeadState(ctx, ch.AutomationID, contactID, status, followUpCount)
	if err != nil {
		return LeadStateView{}, err
	}
	return toLeadStateView(l), nil
}

// SetHandover (M4) pausa ou retoma o bot para um contato. pausedMinutes > 0 e resume
// false => paused_until = now() + N min (atendente humano assumiu). resume true ou
// pausedMinutes <= 0 => limpa a pausa (paused_until = NULL). Resolve a sessao ->
// automacao; account_id e do banco, nunca do body. Retorna a memoria atualizada
// (com paused/pausedUntil) para o n8n confirmar o estado.
func (s *Service) SetHandover(ctx context.Context, session, contactID string, pausedMinutes int, resume bool) (ContactMemoryView, error) {
	ch, err := s.store.GetChannelBySession(ctx, session)
	if err != nil {
		return ContactMemoryView{}, err
	}
	var pausedUntil *time.Time
	if !resume && pausedMinutes > 0 {
		until := time.Now().Add(time.Duration(pausedMinutes) * time.Minute)
		pausedUntil = &until
	}
	if err := s.store.SetContactPause(ctx, ch.AutomationID, contactID, pausedUntil); err != nil {
		return ContactMemoryView{}, err
	}
	c, err := s.store.GetContact(ctx, ch.AutomationID, contactID)
	if err != nil {
		return ContactMemoryView{}, err
	}
	return toContactMemoryView(c), nil
}
