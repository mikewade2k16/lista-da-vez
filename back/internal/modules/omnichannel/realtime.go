package omnichannel

import (
	"context"
	"strings"
	"time"
)

// Realtime (F5): montagem + sanitizacao dos eventos publicados pelo modulo. O CALL-SITE monta o
// subconjunto certo (spec F5 "shapes por call-site"); o transporte (realtime) so entrega. Aqui
// vive a sanitizacao de midia obrigatoria e o build do `message.created` do webhook inbound.

const realtimeMediaDataURLPrefix = "data:"

func newInvalidationSignal(accountID, reason string, occurredAt time.Time) RealtimeEvent {
	occurredAt = occurredAt.UTC()
	return RealtimeEvent{
		Type:      RealtimeEventInvalidate,
		AccountID: accountID,
		Payload: map[string]any{
			"reason":     reason,
			"occurredAt": occurredAt.Format(time.RFC3339Nano),
		},
	}
}

// sanitizeMediaURLForRealtime zera data URLs (base64): NUNCA trafegar midia embutida no WS
// (spec F5 §Sanitizacao). Retorna "" quando a URL e um data:, senao a URL como esta — o front
// busca a midia por GET .../messages/{mid}/media (F6). O publisher do realtime repete a checagem
// (cinto e suspensorio) para um call-site novo que esqueca nao vazar megabytes.
func sanitizeMediaURLForRealtime(mediaURL string) string {
	if strings.HasPrefix(strings.TrimSpace(mediaURL), realtimeMediaDataURLPrefix) {
		return ""
	}
	return mediaURL
}

// publishInboundMessage emite `message.created` para uma mensagem inbound recem-persistida
// (subconjunto do webhook, spec F5 / SPECS_PORT F4). Roda FORA da transacao (persiste -> commita
// -> publica): realtime e entrega, nao fila duravel. account_id vem do Principal do webhook (o
// slug -> conta ja resolvido). Sem ids persistidos => no-op (nada a referenciar).
func (s *InboundService) publishInboundMessage(ctx context.Context, accountID string, res inboundResult, m *inboundMessageWrite) {
	if m == nil || strings.TrimSpace(res.MessageID) == "" || strings.TrimSpace(res.ConversationID) == "" {
		return
	}

	// fromMe (mensagem do aparelho pareado) e OUTBOUND; o resto e INBOUND. Sem isto o painel
	// mostraria a mensagem enviada pelo celular no lado errado (esquerda).
	direction := "INBOUND"
	if m.FromMe {
		direction = "OUTBOUND"
	}
	status := "SENT"
	if strings.TrimSpace(res.ProviderStatus) != "" {
		status = res.ProviderStatus
	}
	payload := map[string]any{
		"id":             res.MessageID,
		"conversationId": res.ConversationID,
		"direction":      direction,
		"messageType":    m.MessageType,
		"content":        m.Content,
		"status":         status,
		"createdAt":      m.OccurredAt.UTC().Format(time.RFC3339),
	}
	if res.ProviderErrorCode != "" {
		payload["providerErrorCode"] = res.ProviderErrorCode
	}
	if mediaURL := sanitizeMediaURLForRealtime(m.MediaURL); mediaURL != "" {
		payload["mediaUrl"] = mediaURL
	}

	s.publisher.PublishOmnichannelEvent(ctx, RealtimeEvent{
		Type:       RealtimeEventMessageCreated,
		AccountID:  accountID,
		ResourceID: res.MessageID,
		Payload:    payload,
	})
}
