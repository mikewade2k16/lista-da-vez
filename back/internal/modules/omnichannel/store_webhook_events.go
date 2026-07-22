package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

const MediaFetchJobKind = "omnichannel.media.fetch"

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
	// Status != nil => ACK do provider: atualiza a mensagem existente sem regressao.
	Status *inboundStatusWrite
}

// inboundResult carrega o desfecho da persistencia. Duplicate=true quando o evento ja existia
// (nada de dominio escrito). ConversationID/MessageID sao os ids INTERNOS gerados (vazios em
// duplicate ou em eventos sem mensagem) — o service os usa para montar o `message.created` do
// realtime (F5) com o MESMO id que o GET de mensagens devolve (senao o front duplica no merge).
type inboundResult struct {
	Duplicate         bool
	MessageCreated    bool
	ConversationID    string
	MessageID         string
	StatusChanged     bool
	ProviderStatus    string
	ProviderStatusAt  time.Time
	ProviderErrorCode string
}

// inboundMessageWrite e a mensagem inbound a persistir (ja canonica).
type inboundMessageWrite struct {
	ExternalMessageID    string
	Channel              string
	ContactExternalID    string
	ContactPhone         string
	ContactName          string
	ContactAvatarURL     string
	MessageType          string
	Content              string
	MediaURL             string
	MediaMimeType        string
	MediaFileName        string
	MediaCaption         string
	OccurredAt           time.Time
	FromMe               bool // true => grava OUTBOUND (mensagem do aparelho), com dedup por external id
	Reply                *channel.ReplyReference
	SocialEventKind      string
	SocialContentID      string
	SocialMediaID        string
	SocialParentID       string
	SocialIsLive         bool
	SocialReplyExpiresAt *time.Time
}

type inboundStatusWrite struct {
	ExternalMessageID string
	Status            string
	ErrorCode         string
	OccurredAt        time.Time
}

// PersistInbound grava o evento de forma idempotente. Retorna duplicate=true quando o
// (provider, external_event_id) ja existe — nesse caso NADA de dominio e escrito e o
// handler responde 202 {status:"duplicate"}. A escrita de dominio so acontece para o
// evento message_received; os demais kinds so registram a linha de dedupe (status/sessao
// viram dominio na F5/F6).
func (s *Store) PersistInbound(ctx context.Context, w inboundWrite) (inboundResult, error) {
	return s.persistInbound(ctx, w, nil)
}

// PersistInboundWithTransition lets the domain service supply the FSM decision
// that must commit with a newly-created provider-device message. It is used for
// fromMe takeover only; duplicate provider echoes never trigger a transition.
func (s *Store) PersistInboundWithTransition(ctx context.Context, w inboundWrite,
	decide func(convSnapshot) (stateUpdate, *decisionRecord, error)) (inboundResult, error) {
	return s.persistInbound(ctx, w, decide)
}

func (s *Store) persistInbound(ctx context.Context, w inboundWrite,
	decide func(convSnapshot) (stateUpdate, *decisionRecord, error)) (inboundResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return inboundResult{}, err
	}
	// Rollback e no-op apos Commit; garante limpeza em qualquer caminho de erro.
	defer func() { _ = tx.Rollback(ctx) }()

	var statusExternalID, providerStatus, providerErrorCode string
	var providerStatusAt *time.Time
	if w.Status != nil {
		statusExternalID = strings.TrimSpace(w.Status.ExternalMessageID)
		providerStatus = strings.TrimSpace(w.Status.Status)
		at := w.Status.OccurredAt
		providerStatusAt = &at
		if providerStatus == "FAILED" {
			providerErrorCode = strings.TrimSpace(w.Status.ErrorCode)
		}
	}

	var eventID string
	err = tx.QueryRow(ctx, `insert into messaging.webhook_events
		(account_id, provider, external_event_id, event_kind, instance_name, payload_masked,
		 external_message_id, provider_status, provider_status_at, provider_error_code)
		values ($1::uuid, $2, $3, $4, $5, $6, nullif($7,''), nullif($8,''), $9, $10)
		on conflict (account_id, provider, external_event_id) do nothing
		returning id::text`,
		w.AccountID, w.Provider, w.ExternalEventID, w.EventKind, w.InstanceName, w.PayloadMasked,
		statusExternalID, providerStatus, providerStatusAt, providerErrorCode,
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
		result.MessageCreated = result.MessageID != ""
		if result.MessageCreated && w.Message.FromMe && decide != nil {
			snap, lockErr := lockConversationSnapshotTx(ctx, tx, w.AccountID, result.ConversationID)
			if lockErr != nil {
				return inboundResult{}, lockErr
			}
			upd, decision, decisionErr := decide(snap)
			if decisionErr != nil {
				return inboundResult{}, decisionErr
			}
			if applyErr := applyStateUpdateTx(ctx, tx, w.AccountID, result.ConversationID, upd, s.AIDispatchV2Enabled()); applyErr != nil {
				return inboundResult{}, applyErr
			}
			if decisionErr := insertDecisionTx(ctx, tx, w.AccountID, result.ConversationID, decision); decisionErr != nil {
				return inboundResult{}, decisionErr
			}
		}
		if strings.TrimSpace(w.Message.ExternalMessageID) != "" {
			result, err = s.replayProviderStatuses(ctx, tx, w, result)
			if err != nil {
				return inboundResult{}, err
			}
		}
	}
	if w.Status != nil {
		result, err = s.writeProviderStatus(ctx, tx, w, result)
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
		        'new', jsonb_build_object('crm_contact_status', $10::text, 'source_channel', lower($5::text),
		        'source_provider', $11::text, 'source_kind', 'direct_message'), $12)
		on conflict (account_id, external_id, channel, instance_scope_key) do update
			set last_message_at = greatest(conversations.last_message_at, excluded.last_message_at),
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
	origin := "contact"
	if m.FromMe {
		direction = "OUTBOUND"
		origin = "provider_device"
	}

	var replyToMessageID string
	if m.Reply != nil && strings.TrimSpace(m.Reply.ExternalMessageID) != "" {
		err = tx.QueryRow(ctx, `select id::text from messaging.messages
			where account_id = $1::uuid and conversation_id = $2::uuid
			  and instance_scope_key = $3 and external_message_id = $4
			limit 1`, w.AccountID, conversationID, w.InstanceName,
			strings.TrimSpace(m.Reply.ExternalMessageID)).Scan(&replyToMessageID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", "", err
		}
	}

	metadata := map[string]any{}
	mediaSourceKind := ""
	if m.MessageType != "TEXT" {
		metadata["mediaState"] = "pending"
		metadata["requiresMediaDecrypt"] = true
		mediaSourceKind = "url_encrypted"
	}
	if m.Reply != nil {
		metadata["replySnapshot"] = map[string]any{
			"content":     strings.TrimSpace(m.Reply.Content),
			"messageType": strings.TrimSpace(m.Reply.MessageType),
			"participant": strings.TrimSpace(m.Reply.ParticipantID),
		}
	}
	if strings.TrimSpace(m.SocialEventKind) != "" {
		metadata["socialEventKind"] = strings.TrimSpace(m.SocialEventKind)
		metadata["socialContentId"] = strings.TrimSpace(m.SocialContentID)
		metadata["socialMediaId"] = strings.TrimSpace(m.SocialMediaID)
		metadata["socialParentId"] = strings.TrimSpace(m.SocialParentID)
		metadata["socialIsLive"] = m.SocialIsLive
		if m.SocialReplyExpiresAt != nil {
			metadata["socialReplyExpiresAt"] = m.SocialReplyExpiresAt.UTC().Format(time.RFC3339)
		}
	}
	var metadataJSON []byte
	if len(metadata) > 0 {
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return "", "", err
		}
	}
	replyExternalID := ""
	if m.Reply != nil {
		replyExternalID = strings.TrimSpace(m.Reply.ExternalMessageID)
	}

	var messageID string
	err = tx.QueryRow(ctx, `insert into messaging.messages
		(account_id, conversation_id, instance_id, instance_scope_key, direction, message_type,
		 sender_name, content, media_url, media_mime_type, media_file_name, media_caption,
		 external_message_id, status, created_at, origin, provider_status_at,
		 reply_to_message_id, reply_to_external_message_id, media_source_kind, metadata_json)
		values ($1::uuid, $2::uuid, nullif($3,'')::uuid, $4, $5, $6,
		 nullif($7,''), $8, nullif($9,''), nullif($10,''), nullif($11,''), nullif($12,''),
		 nullif($13,''), 'SENT', $14, $15, $14,
		 nullif($16,'')::uuid, nullif($17,''), nullif($18,''), $19)
		on conflict (account_id, instance_scope_key, external_message_id)
			where external_message_id is not null and btrim(external_message_id) <> ''
		do nothing
		returning id::text`,
		w.AccountID, conversationID, w.InstanceID, w.InstanceName, direction, m.MessageType,
		m.ContactName, m.Content, "", m.MediaMimeType, m.MediaFileName, m.MediaCaption,
		m.ExternalMessageID, m.OccurredAt, origin, replyToMessageID, replyExternalID,
		mediaSourceKind, metadataJSON,
	).Scan(&messageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return conversationID, "", nil
	}
	if err != nil {
		return "", "", err
	}
	if w.Provider == "meta_whatsapp_cloud" && !m.FromMe {
		// The 24-hour customer-service window is opened only by a real inbound
		// message and is monotonic under retries/out-of-order delivery.
		if _, err := tx.Exec(ctx, `insert into messaging.channel_windows
			(account_id, conversation_id, provider, window_kind, opened_at, expires_at, source_message_id)
			values ($1::uuid, $2::uuid, $3, 'customer_service', $4, $4 + interval '24 hours', $5::uuid)
			on conflict (account_id, conversation_id, provider, window_kind) do update
			set opened_at = greatest(channel_windows.opened_at, excluded.opened_at),
				expires_at = greatest(channel_windows.expires_at, excluded.expires_at),
				source_message_id = case when excluded.opened_at >= channel_windows.opened_at then excluded.source_message_id else channel_windows.source_message_id end,
				updated_at = now()`, w.AccountID, conversationID, w.Provider, m.OccurredAt, messageID); err != nil {
			return "", "", err
		}
	}
	if w.Provider == "instagram" && !m.FromMe && (m.SocialEventKind == "comment" || m.SocialEventKind == "mention") {
		// The comment is a first-class moderation record, while its text still
		// lives in the canonical messages table for the shared inbox/timeline.
		commentID := ""
		err = tx.QueryRow(ctx, `insert into messaging.instagram_comments
			(account_id,instagram_account_id,external_comment_id,external_media_id,parent_comment_id,contact_id,author_scoped_id,username,text,event_kind,status,is_live,occurred_at,metadata)
			select $1::uuid, ia.id, $2, nullif($3,''), nullif($4,''), nullif($5,'')::uuid, $6, nullif($7,''), $8, $9, 'pending_review', $10, $11, '{}'::jsonb
			from messaging.instagram_accounts ia where ia.account_id=$1::uuid and ia.ig_user_id=$12 and ia.is_active=true
			on conflict (account_id,instagram_account_id,external_comment_id) do update set
				text=excluded.text, status=case when instagram_comments.status='deleted' then 'deleted' else excluded.status end,
				updated_at=now()
			returning id::text`, w.AccountID, m.SocialContentID, m.SocialMediaID, m.SocialParentID, contactID,
			firstNonEmpty(m.ContactName, m.ContactExternalID), m.Content, m.SocialEventKind, m.SocialIsLive,
			m.OccurredAt, w.InstanceName).Scan(&commentID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", "", err
		}
		if commentID != "" {
			expires := m.SocialReplyExpiresAt
			_, err = tx.Exec(ctx, `insert into messaging.instagram_comment_actions
				(account_id,comment_id,action_kind,status,idempotency_key,private_reply_expires_at)
				values ($1::uuid,$2::uuid,'public_reply','pending_review',$3,$4)
				on conflict (account_id,idempotency_key) do nothing`, w.AccountID, commentID, "instagram-review:"+commentID, expires)
			if err != nil {
				return "", "", err
			}
		}
	}

	if m.ExternalMessageID != "" {
		if _, err := tx.Exec(ctx, `update messaging.messages
			set reply_to_message_id = $4::uuid, updated_at = now()
			where account_id = $1::uuid and conversation_id = $2::uuid
			  and instance_scope_key = $3 and reply_to_message_id is null
			  and reply_to_external_message_id = $5`,
			w.AccountID, conversationID, w.InstanceName, messageID, m.ExternalMessageID); err != nil {
			return "", "", err
		}
	}
	if m.MessageType != "TEXT" {
		if _, err := tx.Exec(ctx, `update messaging.messages
			set media_url = $3, updated_at = now()
			where account_id = $1::uuid and id = $2::uuid`,
			w.AccountID, messageID, mediaEndpointPath(conversationID, messageID)); err != nil {
			return "", "", err
		}
		if _, err := tx.Exec(ctx, `insert into messaging.outbox
			(account_id, ordering_key, idempotency_key, kind, payload, max_attempts)
			values ($1::uuid, $2::text, $3, $4,
				jsonb_build_object('messageId', $5::text, 'conversationId', $2::text), 5)
			on conflict (account_id, idempotency_key) do nothing`,
			w.AccountID, conversationID, "media-fetch:"+messageID, MediaFetchJobKind, messageID); err != nil {
			return "", "", err
		}
	}
	if contactID != "" && !m.FromMe {
		touchpointKind := "direct_message"
		if w.Provider == "instagram" && (m.SocialEventKind == "comment" || m.SocialEventKind == "mention") {
			touchpointKind = "instagram_" + m.SocialEventKind
		}
		_, err = tx.Exec(ctx, `insert into messaging.contact_touchpoints
			(account_id, contact_id, conversation_id, message_id, channel, provider,
			 external_event_id, source_kind, occurred_at)
			values ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, nullif($7,''), $8, $9)
			on conflict (account_id, provider, external_event_id)
				where external_event_id is not null and external_event_id <> '' do nothing`,
			w.AccountID, contactID, conversationID, messageID, m.Channel, w.Provider,
			w.ExternalEventID, touchpointKind, m.OccurredAt)
		if err != nil {
			return "", "", err
		}
	}
	return conversationID, messageID, nil
}

// writeProviderStatus aplica ACKs de forma atomica e monotona. O timestamp do provider
// impede regressao por reentrega fora de ordem; FAILED nunca substitui DELIVERED/READ e
// DELETED e terminal.
func (s *Store) writeProviderStatus(ctx context.Context, tx pgx.Tx, w inboundWrite, result inboundResult) (inboundResult, error) {
	st := w.Status
	if st == nil || strings.TrimSpace(st.ExternalMessageID) == "" {
		return result, nil
	}
	return s.applyProviderStatus(ctx, tx, w.AccountID, w.InstanceName, *st, result)
}

// replayProviderStatuses reprocessa ACKs que chegaram antes da mensagem. A fonte e a propria
// tabela duravel de dedupe, escopada por conta+provider+instancia+external id. Todos passam
// por applyProviderStatus, a mesma regra monotona do ACK ao vivo.
func (s *Store) replayProviderStatuses(ctx context.Context, tx pgx.Tx, w inboundWrite, result inboundResult) (inboundResult, error) {
	rows, err := tx.Query(ctx, `select provider_status, provider_status_at, provider_error_code
		from messaging.webhook_events
		where account_id = $1::uuid and provider = $2 and instance_name = $3
		  and external_message_id = $4 and provider_status is not null
		order by provider_status_at, id`, w.AccountID, w.Provider, w.InstanceName,
		strings.TrimSpace(w.Message.ExternalMessageID))
	if err != nil {
		return inboundResult{}, err
	}
	statuses := make([]inboundStatusWrite, 0)
	for rows.Next() {
		var status inboundStatusWrite
		status.ExternalMessageID = strings.TrimSpace(w.Message.ExternalMessageID)
		if err := rows.Scan(&status.Status, &status.OccurredAt, &status.ErrorCode); err != nil {
			rows.Close()
			return inboundResult{}, err
		}
		statuses = append(statuses, status)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return inboundResult{}, err
	}
	rows.Close()
	for _, status := range statuses {
		result, err = s.applyProviderStatus(ctx, tx, w.AccountID, w.InstanceName, status, result)
		if err != nil {
			return inboundResult{}, err
		}
	}
	return result, nil
}

func (s *Store) applyProviderStatus(ctx context.Context, tx pgx.Tx, accountID, instanceName string,
	st inboundStatusWrite, result inboundResult) (inboundResult, error) {
	err := tx.QueryRow(ctx, `update messaging.messages m
		set status = $4,
			provider_status_at = $5,
			provider_error_code = case when $4 = 'FAILED' then $6 else '' end,
			updated_at = now()
		where m.account_id = $1::uuid
		  and m.instance_scope_key = $2
		  and m.external_message_id = $3
		  and m.status <> 'DELETED'
		  and (m.provider_status_at is null or $5 >= m.provider_status_at)
		  and (
			$4 = 'DELETED'
			or ($4 = 'FAILED' and m.status not in ('DELIVERED','READ'))
			or ($4 in ('SENT','DELIVERED','READ') and
				case $4 when 'SENT' then 1 when 'DELIVERED' then 2 when 'READ' then 3 end >
				case m.status when 'PENDING' then 0 when 'SENT' then 1 when 'DELIVERED' then 2 when 'READ' then 3 else -1 end)
		  )
		returning m.id::text, m.conversation_id::text, m.status, m.provider_status_at,
			m.provider_error_code`,
		accountID, instanceName, st.ExternalMessageID, st.Status, st.OccurredAt,
		strings.TrimSpace(st.ErrorCode),
	).Scan(&result.MessageID, &result.ConversationID, &result.ProviderStatus,
		&result.ProviderStatusAt, &result.ProviderErrorCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return inboundResult{}, err
	}
	result.StatusChanged = true
	return result, nil
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
	// A identidade exata do provider/canal tem precedência sobre telefone. Isso evita
	// colar uma identidade Instagram/WhatsApp em outro contato apenas porque o provider
	// informou um telefone reutilizado; só depois do match exato usamos o telefone como
	// regra de convergência confiável.
	if externalID != "" {
		lookupErr = tx.QueryRow(ctx, `select contact_id::text from messaging.contact_identities
			where account_id = $1::uuid and channel = $2 and provider = $3
			  and instance_scope_key = $4 and external_id = $5 limit 1`,
			w.AccountID, m.Channel, w.Provider, w.InstanceName, externalID).Scan(&existingID)
	}
	if (externalID == "" || errors.Is(lookupErr, pgx.ErrNoRows)) && phone != "" {
		lookupErr = tx.QueryRow(ctx, `select id::text from messaging.contacts
			where account_id = $1::uuid and phone = $2 and archived_at is null limit 1`,
			w.AccountID, phone).Scan(&existingID)
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
			    avatar_url = coalesce(nullif($4, ''), avatar_url),
			    last_seen_at = greatest(coalesce(last_seen_at, $5), $5),
			    last_channel = case when last_seen_at is null or $5 >= last_seen_at then $6 else last_channel end,
			    updated_at = now()
			where account_id = $1::uuid and id = $2::uuid`,
			w.AccountID, contactID, name, m.ContactAvatarURL, m.OccurredAt, m.Channel)
		if err != nil {
			return "", false, err
		}
	} else {
		err := tx.QueryRow(ctx, `insert into messaging.contacts
			(account_id, name, phone, avatar_url, source, first_seen_at, last_seen_at,
			 first_channel, last_channel, relationship_status, classification_source, classification_confidence)
			values ($1::uuid, $2, nullif($3,''), nullif($4,''), $5, $6, $6, $7, $7, 'new_lead', 'rule', 1)
			on conflict (account_id, phone) where phone is not null and phone <> '' do update
				set last_seen_at = greatest(coalesce(contacts.last_seen_at, excluded.last_seen_at), excluded.last_seen_at),
				    last_channel = case when contacts.last_seen_at is null or excluded.last_seen_at >= contacts.last_seen_at then excluded.last_channel else contacts.last_channel end,
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
			last_seen_at = greatest(contact_identities.last_seen_at, excluded.last_seen_at), updated_at = now()`,
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
func (s *Store) FindInstanceIDByName(ctx context.Context, accountID, provider, instanceName string) (string, bool, error) {
	var id string
	if provider == "instagram" {
		err := s.pool.QueryRow(ctx, `select 1 from messaging.instagram_accounts
			where account_id=$1::uuid and ig_user_id=$2 and is_active=true`, accountID, instanceName).Scan(new(int))
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		// instagram_accounts is intentionally not an instance_id FK for conversations;
		// the stable ig_user_id remains the instance_scope_key.
		return "", true, nil
	}
	err := s.pool.QueryRow(ctx, `select id::text from messaging.whatsapp_instances
		where account_id = $1::uuid and provider = $2
		  and (instance_name = $3 or (provider = 'meta_whatsapp_cloud' and provider_config->>'phoneNumberId' = $3))`,
		accountID, provider, instanceName).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, err
	default:
		return id, true, nil
	}
}
