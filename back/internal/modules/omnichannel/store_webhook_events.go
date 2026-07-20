package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Persistencia do webhook inbound: a linha de dedupe (messaging.webhook_events) e a
// escrita de dominio (conversa + mensagem) na MESMA transacao (spec C4 — exactly-once sem
// lock distribuido). Mesma regra da casa: TODA query filtra por account_id.

// inboundWrite carrega tudo que a transacao precisa gravar de um evento canonico ja
// resolvido pelo service (instanceID resolvido, payload ja mascarado).
type inboundWrite struct {
	AccountID       string
	Provider        string
	ExternalEventID string
	EventKind       string
	InstanceName    string
	InstanceID      string // "" quando o evento nao amarra a uma instancia conhecida
	PayloadMasked   json.RawMessage
	// Message != nil => evento message_received: cria/atualiza conversa e grava a mensagem.
	Message *inboundMessageWrite
}

// inboundResult carrega o desfecho da persistencia. Duplicate=true quando o evento ja existia
// (nada de dominio escrito). ConversationID/MessageID sao os ids INTERNOS gerados (vazios em
// duplicate ou em eventos sem mensagem) — o service os usa para montar o `message.created` do
// realtime (F5) com o MESMO id que o GET de mensagens devolve (senao o front duplica no merge).
type inboundResult struct {
	Duplicate      bool
	ConversationID string
	MessageID      string
}

// inboundMessageWrite e a mensagem inbound a persistir (ja canonica).
type inboundMessageWrite struct {
	ExternalMessageID string
	Channel           string
	ContactExternalID string
	ContactPhone      string
	ContactName       string
	ContactAvatarURL  string
	MessageType       string
	Content           string
	MediaURL          string
	MediaMimeType     string
	MediaFileName     string
	MediaCaption      string
	OccurredAt        time.Time
	FromMe            bool // true => grava OUTBOUND (mensagem do aparelho), com dedup por external id
}

// PersistInbound grava o evento de forma idempotente. Retorna duplicate=true quando o
// (provider, external_event_id) ja existe — nesse caso NADA de dominio e escrito e o
// handler responde 202 {status:"duplicate"}. A escrita de dominio so acontece para o
// evento message_received; os demais kinds so registram a linha de dedupe (status/sessao
// viram dominio na F5/F6).
func (s *Store) PersistInbound(ctx context.Context, w inboundWrite) (inboundResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return inboundResult{}, err
	}
	// Rollback e no-op apos Commit; garante limpeza em qualquer caminho de erro.
	defer func() { _ = tx.Rollback(ctx) }()

	var eventID string
	err = tx.QueryRow(ctx, `insert into messaging.webhook_events
		(account_id, provider, external_event_id, event_kind, instance_name, payload_masked)
		values ($1::uuid, $2, $3, $4, $5, $6)
		on conflict (account_id, provider, external_event_id) do nothing
		returning id::text`,
		w.AccountID, w.Provider, w.ExternalEventID, w.EventKind, w.InstanceName, w.PayloadMasked,
	).Scan(&eventID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Conflito no indice unico: evento ja processado. Rollback (defer) e duplicate.
		return inboundResult{Duplicate: true}, nil
	case err != nil:
		return inboundResult{}, err
	}

	var result inboundResult
	if w.Message != nil {
		result.ConversationID, result.MessageID, err = s.writeInboundMessage(ctx, tx, w)
		if err != nil {
			return inboundResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return inboundResult{}, err
	}
	return result, nil
}

// writeInboundMessage faz o upsert da conversa (chave natural do canonico) e grava a
// mensagem inbound com created_at = timestamp DO PROVIDER e status=SENT (spec armadilha 5).
// Devolve os ids INTERNOS (conversa + mensagem) para o realtime do webhook (F5).
func (s *Store) writeInboundMessage(ctx context.Context, tx pgx.Tx, w inboundWrite) (string, string, error) {
	m := w.Message
	contactID, knownContact, err := s.upsertInboundContact(ctx, tx, w)
	if err != nil {
		return "", "", err
	}
	contactStatus := "new_contact"
	if knownContact {
		contactStatus = "known_contact"
	}

	var conversationID string
	err = tx.QueryRow(ctx, `insert into messaging.conversations
		(account_id, instance_id, instance_scope_key, contact_id, channel, external_id,
		 contact_name, contact_phone, contact_avatar_url, state, extracted_fields, last_message_at)
		values ($1::uuid, nullif($2,'')::uuid, $3, nullif($4,'')::uuid, $5, $6, $7, $8, $9,
		        'new', jsonb_build_object('crm_contact_status', $10, 'source_channel', lower($5),
		        'source_provider', $11, 'source_kind', 'direct_message'), $12)
		on conflict (account_id, external_id, channel, instance_scope_key) do update
			set last_message_at = excluded.last_message_at,
				contact_id = coalesce(excluded.contact_id, conversations.contact_id),
				contact_name = coalesce(nullif(excluded.contact_name, ''), conversations.contact_name),
				contact_phone = coalesce(nullif(excluded.contact_phone, ''), conversations.contact_phone),
				contact_avatar_url = coalesce(nullif(excluded.contact_avatar_url, ''), conversations.contact_avatar_url),
				updated_at = now()
		returning id::text`,
		w.AccountID, w.InstanceID, w.InstanceName, contactID, m.Channel, m.ContactExternalID,
		m.ContactName, normalizePhoneDigits(m.ContactPhone), m.ContactAvatarURL, contactStatus,
		w.Provider, m.OccurredAt,
	).Scan(&conversationID)
	if err != nil {
		return "", "", err
	}

	// fromMe = enviada pelo aparelho pareado (ou eco do proprio envio da plataforma). Grava como
	// OUTBOUND e DEDUPA pelo external_message_id: se a mensagem ja existe (a plataforma enviou e o
	// Evolution ecoou, ou re-entrega), NAO regrava — devolve messageID vazio (o service trata como
	// duplicate, sem realtime). A conversa ja foi upsertada acima (bump de last_message_at, correto).
	direction := "INBOUND"
	if m.FromMe {
		direction = "OUTBOUND"
		if m.ExternalMessageID != "" {
			var existing string
			err = tx.QueryRow(ctx, `select id::text from messaging.messages
				where account_id = $1::uuid and external_message_id = $2 limit 1`,
				w.AccountID, m.ExternalMessageID).Scan(&existing)
			switch {
			case err == nil:
				return conversationID, "", nil
			case !errors.Is(err, pgx.ErrNoRows):
				return "", "", err
			}
		}
	}

	var messageID string
	err = tx.QueryRow(ctx, `insert into messaging.messages
		(account_id, conversation_id, instance_id, instance_scope_key, direction, message_type,
		 sender_name, content, media_url, media_mime_type, media_file_name, media_caption,
		 external_message_id, status, created_at)
		values ($1::uuid, $2::uuid, nullif($3,'')::uuid, $4, $5, $6,
		 nullif($7,''), $8, nullif($9,''), nullif($10,''), nullif($11,''), nullif($12,''),
		 nullif($13,''), 'SENT', $14)
		returning id::text`,
		w.AccountID, conversationID, w.InstanceID, w.InstanceName, direction, m.MessageType,
		m.ContactName, m.Content, m.MediaURL, m.MediaMimeType, m.MediaFileName, m.MediaCaption,
		m.ExternalMessageID, m.OccurredAt,
	).Scan(&messageID)
	if err != nil {
		return "", "", err
	}
	if contactID != "" && !m.FromMe {
		_, err = tx.Exec(ctx, `insert into messaging.contact_touchpoints
			(account_id, contact_id, conversation_id, message_id, channel, provider,
			 external_event_id, source_kind, occurred_at)
			values ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, nullif($7,''), 'direct_message', $8)
			on conflict (account_id, provider, external_event_id)
				where external_event_id is not null and external_event_id <> '' do nothing`,
			w.AccountID, contactID, conversationID, messageID, m.Channel, w.Provider,
			w.ExternalEventID, m.OccurredAt)
		if err != nil {
			return "", "", err
		}
	}
	return conversationID, messageID, nil
}

// upsertInboundContact salva automaticamente o contato e sua identidade externa na MESMA
// transacao do webhook. Telefone e chave natural do WhatsApp; external_id permite que o
// mesmo CRM receba identidades do Instagram sem exigir telefone.
func (s *Store) upsertInboundContact(ctx context.Context, tx pgx.Tx, w inboundWrite) (string, bool, error) {
	m := w.Message
	if m == nil || m.FromMe {
		return "", false, nil
	}
	externalID := strings.TrimSpace(m.ContactExternalID)
	phone := normalizePhoneDigits(m.ContactPhone)
	if externalID == "" && phone == "" {
		return "", false, nil
	}

	var existingID string
	var lookupErr error
	if phone != "" {
		lookupErr = tx.QueryRow(ctx, `select id::text from messaging.contacts
			where account_id = $1::uuid and phone = $2 limit 1`, w.AccountID, phone).Scan(&existingID)
	} else {
		lookupErr = tx.QueryRow(ctx, `select contact_id::text from messaging.contact_identities
			where account_id = $1::uuid and channel = $2 and provider = $3
			  and instance_scope_key = $4 and external_id = $5 limit 1`,
			w.AccountID, m.Channel, w.Provider, w.InstanceName, externalID).Scan(&existingID)
	}
	if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		return "", false, lookupErr
	}

	name := normalizeContactName(m.ContactName, firstNonEmpty(phone, externalID))
	source := m.Channel + "_INBOUND"
	var contactID string
	if existingID != "" {
		contactID = existingID
		_, err := tx.Exec(ctx, `update messaging.contacts
			set name = case when name = '' or name = coalesce(phone, '') then $3 else name end,
			    avatar_url = coalesce(nullif($4, ''), avatar_url), last_seen_at = $5,
			    last_channel = $6, updated_at = now()
			where account_id = $1::uuid and id = $2::uuid`,
			w.AccountID, contactID, name, m.ContactAvatarURL, m.OccurredAt, m.Channel)
		if err != nil {
			return "", false, err
		}
	} else {
		err := tx.QueryRow(ctx, `insert into messaging.contacts
			(account_id, name, phone, avatar_url, source, first_seen_at, last_seen_at,
			 first_channel, last_channel, relationship_status)
			values ($1::uuid, $2, nullif($3,''), nullif($4,''), $5, $6, $6, $7, $7, 'lead')
			on conflict (account_id, phone) where phone is not null and phone <> '' do update
				set last_seen_at = excluded.last_seen_at, last_channel = excluded.last_channel,
				    avatar_url = coalesce(excluded.avatar_url, contacts.avatar_url), updated_at = now()
			returning id::text`,
			w.AccountID, name, phone, m.ContactAvatarURL, source, m.OccurredAt, m.Channel).Scan(&contactID)
		if err != nil {
			return "", false, err
		}
	}

	_, err := tx.Exec(ctx, `insert into messaging.contact_identities
		(account_id, contact_id, channel, provider, instance_scope_key, external_id,
		 display_name, avatar_url, first_seen_at, last_seen_at)
		values ($1::uuid, $2::uuid, $3, $4, $5, $6, nullif($7,''), nullif($8,''), $9, $9)
		on conflict (account_id, channel, provider, instance_scope_key, external_id) do update
			set display_name = coalesce(excluded.display_name, contact_identities.display_name),
			    avatar_url = coalesce(excluded.avatar_url, contact_identities.avatar_url),
			    last_seen_at = excluded.last_seen_at, updated_at = now()`,
		w.AccountID, contactID, m.Channel, w.Provider, w.InstanceName,
		firstNonEmpty(externalID, phone), m.ContactName, m.ContactAvatarURL, m.OccurredAt)
	return contactID, existingID != "", err
}

// ResolveWebhookAccount resolve o accountID a partir do slug PUBLICO do webhook. Devolve
// pgx.ErrNoRows (o service traduz para ErrNotFound -> 404) quando o slug nao existe, a
// conta esta inativa OU o modulo omnichannel nao esta habilitado para ela (spec C2: os
// tres casos respondem 404, nunca 403 — nao revelar existencia). account_id NUNCA vem do
// body do provider; resolve sempre do slug do path, no server.
func (s *Store) ResolveWebhookAccount(ctx context.Context, slug string) (string, error) {
	var accountID string
	err := s.pool.QueryRow(ctx, `select a.id::text
		from core.accounts a
		join core.account_modules am
			on am.account_id = a.id and am.module_id = 'omnichannel' and am.enabled = true
		where lower(a.slug) = lower($1) and a.is_active = true`, slug).Scan(&accountID)
	return accountID, err
}

// FindInstanceIDByName resolve o id da instancia pelo nome (instance_scope_key) dentro da
// conta. found=false quando nao existe: o webhook responde 202 {status:"ignored"} e NAO
// auto-cria a instancia (armadilha 1 — input nao-confiavel nao cadastra numero).
func (s *Store) FindInstanceIDByName(ctx context.Context, accountID, instanceName string) (string, bool, error) {
	var id string
	err := s.pool.QueryRow(ctx, `select id::text from messaging.whatsapp_instances
		where account_id = $1::uuid and instance_name = $2`, accountID, instanceName).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, err
	default:
		return id, true, nil
	}
}
