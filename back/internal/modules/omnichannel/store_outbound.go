package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Persistencia do ENVIO (F6): criacao da mensagem OUTBOUND/PENDING + colunas de midia em
// disco, transicao de status pelo worker, descriptor da midia para o /media e a trilha de
// auditoria. Mesma regra da casa: TODA query filtra por account_id (defesa em profundidade).
//
// A serializacao de mediaUrl segue a spec C2: quando ha media_storage_key, media_url guarda a
// URL do ENDPOINT autenticado (nunca data URL, nunca o path de disco). media_storage_key e
// media_source_kind NAO entram no messageCols — nunca vao ao JSON.

// mediaEndpointPath monta a URL publica servida como mediaUrl (spec C2).
func mediaEndpointPath(conversationID, messageID string) string {
	return "/v1/omnichannel/conversations/" + conversationID + "/messages/" + messageID + "/media"
}

// outboundMessageInsert e o que a criacao grava. Media != nil => mensagem com anexo (as
// colunas de midia + media_source_kind='disk'); nil => TEXT.
type outboundMessageInsert struct {
	AccountID         string
	ConversationID    string
	InstanceID        *string
	InstanceScopeKey  string
	SenderUserID      string
	MessageType       string
	Content           string
	MetadataJSON      json.RawMessage
	Media             *StoredMedia
	MediaCaption      string
	MediaDurationSecs *int
}

// CreateOutboundMessage grava a mensagem PENDING/OUTBOUND, seta o mediaUrl do endpoint (2o
// passo — o id so existe apos o insert) e toca last_message_at da conversa, tudo na MESMA
// transacao. Devolve a MessageView completa (mesmo shape do GET), pronta para o realtime.
func (s *Store) CreateOutboundMessage(ctx context.Context, in outboundMessageInsert) (MessageView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return MessageView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	metadata := in.MetadataJSON
	if len(metadata) == 0 {
		metadata = nil
	}
	var (
		storageKey, mimeType, fileName string
		sizeBytes                      *int
		sourceKind                     *string
	)
	if in.Media != nil {
		storageKey = in.Media.StorageKey
		mimeType = in.Media.MimeType
		fileName = in.Media.FileName
		size := int(in.Media.SizeBytes)
		sizeBytes = &size
		disk := "disk"
		sourceKind = &disk
	}

	var messageID string
	err = tx.QueryRow(ctx, `insert into messaging.messages
		(account_id, conversation_id, instance_id, instance_scope_key, sender_user_id, direction,
		 message_type, content, media_mime_type, media_file_name, media_file_size_bytes, media_caption,
		 media_duration_seconds, media_storage_key, media_source_kind, metadata_json, status)
		values ($1::uuid, $2::uuid, nullif($3,'')::uuid, $4, nullif($5,'')::uuid, 'OUTBOUND',
		 $6, $7, nullif($8,''), nullif($9,''), $10, nullif($11,''),
		 $12, nullif($13,''), $14, $15, 'PENDING')
		returning id::text`,
		in.AccountID, in.ConversationID, deref(in.InstanceID), in.InstanceScopeKey, in.SenderUserID,
		in.MessageType, in.Content, mimeType, fileName, sizeBytes, in.MediaCaption,
		in.MediaDurationSecs, storageKey, sourceKind, metadata,
	).Scan(&messageID)
	if err != nil {
		return MessageView{}, err
	}

	if in.Media != nil {
		if _, err := tx.Exec(ctx, `update messaging.messages set media_url = $3, updated_at = now()
			where id = $1::uuid and account_id = $2::uuid`,
			messageID, in.AccountID, mediaEndpointPath(in.ConversationID, messageID)); err != nil {
			return MessageView{}, err
		}
	}

	if _, err := tx.Exec(ctx, `update messaging.conversations
		set last_message_at = now(), updated_at = now()
		where id = $1::uuid and account_id = $2::uuid`, in.ConversationID, in.AccountID); err != nil {
		return MessageView{}, err
	}

	view, err := scanMessage(tx.QueryRow(ctx, `select `+messageCols+` from messaging.messages m
		where m.id = $1::uuid and m.account_id = $2::uuid`, messageID, in.AccountID))
	if err != nil {
		return MessageView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MessageView{}, err
	}
	return view, nil
}

// GetMessageByID devolve a mensagem por id, so escopada por conta (sem exclusao de hidden —
// e para o produtor/idempotencia, nao para a leitura do inbox). Outra conta => ErrNoRows.
func (s *Store) GetMessageByID(ctx context.Context, accountID, messageID string) (MessageView, error) {
	return scanMessage(s.pool.QueryRow(ctx, `select `+messageCols+` from messaging.messages m
		where m.id = $1::uuid and m.account_id = $2::uuid`, messageID, accountID))
}

// DeleteMessage remove uma mensagem PENDING da conta (usado so para limpar o orfao de uma
// corrida de idempotencia: dois POST concorrentes com a mesma chave criam 2 linhas, o outbox
// dedupa 1 e a perdedora e removida). So apaga se ainda PENDING — nunca uma ja enviada.
func (s *Store) DeleteMessage(ctx context.Context, accountID, messageID string) error {
	_, err := s.pool.Exec(ctx, `delete from messaging.messages
		where id = $1::uuid and account_id = $2::uuid and status = 'PENDING'`, messageID, accountID)
	return err
}

// FindOutboxMessageID le o messageId do payload de um job ja enfileirado com a
// (account_id, idempotency_key). found=false => nao ha job com essa chave nessa conta. Leitura
// (o engine da F3 e dono da escrita); filtra por conta.
func (s *Store) FindOutboxMessageID(ctx context.Context, accountID, idempotencyKey string) (string, bool, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `select payload from messaging.outbox
		where account_id = $1::uuid and idempotency_key = $2`, accountID, idempotencyKey).Scan(&payload)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	var p outboundJobPayload
	if json.Unmarshal(payload, &p) != nil || p.MessageID == "" {
		return "", false, nil
	}
	return p.MessageID, true, nil
}

// outboundSendData e tudo que o handler do job precisa para enviar (sem PII no payload do
// job — o handler recarrega daqui). Provider vazio => sem instancia resolvida.
type outboundSendData struct {
	MessageID        string
	ConversationID   string
	Status           string
	MessageType      string
	Content          string
	MediaCaption     *string
	MediaStorageKey  *string
	MediaURL         *string
	MediaMimeType    *string
	MediaFileName    *string
	ToPhone          *string
	ConversationExt  string
	InstanceScopeKey string
	Provider         string
}

// GetOutboundSendData carrega a mensagem + a conversa + a instancia para o envio. Escopado por
// conta; mensagem de outra conta => ErrNoRows.
func (s *Store) GetOutboundSendData(ctx context.Context, accountID, messageID string) (outboundSendData, error) {
	var d outboundSendData
	err := s.pool.QueryRow(ctx, `select m.id::text, m.conversation_id::text, m.status, m.message_type,
			m.content, m.media_caption, m.media_storage_key, m.media_url, m.media_mime_type, m.media_file_name,
			c.contact_phone, c.external_id, c.instance_scope_key, coalesce(i.provider, '')
		from messaging.messages m
		join messaging.conversations c on c.id = m.conversation_id and c.account_id = m.account_id
		left join messaging.whatsapp_instances i on i.id = m.instance_id and i.account_id = m.account_id
		where m.account_id = $1::uuid and m.id = $2::uuid`, accountID, messageID,
	).Scan(&d.MessageID, &d.ConversationID, &d.Status, &d.MessageType, &d.Content, &d.MediaCaption,
		&d.MediaStorageKey, &d.MediaURL, &d.MediaMimeType, &d.MediaFileName, &d.ToPhone, &d.ConversationExt,
		&d.InstanceScopeKey, &d.Provider)
	return d, err
}

// MarkMessageSent transiciona a mensagem para SENT com o external id do provider. Devolve o
// updated_at para o shape minimo do message.updated. Idempotente: um re-run do worker sobre
// uma ja SENT nao muda o external id (where status <> 'SENT' evita sobrescrever).
func (s *Store) MarkMessageSent(ctx context.Context, accountID, messageID, externalID string) (time.Time, error) {
	var updatedAt time.Time
	err := s.pool.QueryRow(ctx, `update messaging.messages
		set status = 'SENT', external_message_id = coalesce(nullif($3,''), external_message_id), updated_at = now()
		where id = $1::uuid and account_id = $2::uuid returning updated_at`,
		messageID, accountID, externalID).Scan(&updatedAt)
	return updatedAt, err
}

// MarkMessageFailed transiciona a mensagem para FAILED (falha terminal do envio). Devolve o
// updated_at para o message.updated.
func (s *Store) MarkMessageFailed(ctx context.Context, accountID, messageID string) (time.Time, error) {
	var updatedAt time.Time
	err := s.pool.QueryRow(ctx, `update messaging.messages
		set status = 'FAILED', updated_at = now()
		where id = $1::uuid and account_id = $2::uuid returning updated_at`,
		messageID, accountID).Scan(&updatedAt)
	return updatedAt, err
}

// mediaDescriptor descreve a midia de uma mensagem para o /media. Exclui hidden do usuario
// (=> ErrNoRows => 404). Provider e o da instancia (para rehidratacao).
type mediaDescriptor struct {
	MessageID         string
	ConversationID    string
	InstanceScopeKey  string
	StorageKey        *string
	MimeType          *string
	FileName          *string
	MediaURL          *string
	SourceKind        *string
	ExternalMessageID *string
	Metadata          json.RawMessage
	Provider          *string
}

// GetMediaDescriptor resolve a midia da mensagem, escopada por conta+conversa e SEM as
// hidden_messages do usuario (apagar-para-mim => 404). Outra conta/conversa => ErrNoRows.
func (s *Store) GetMediaDescriptor(ctx context.Context, accountID, userID, conversationID, messageID string) (mediaDescriptor, error) {
	var d mediaDescriptor
	err := s.pool.QueryRow(ctx, `select m.id::text, m.conversation_id::text, m.instance_scope_key,
			m.media_storage_key, m.media_mime_type, m.media_file_name, m.media_url, m.media_source_kind,
			m.external_message_id, m.metadata_json, i.provider
		from messaging.messages m
		join messaging.conversations c on c.id = m.conversation_id and c.account_id = m.account_id
		left join messaging.whatsapp_instances i on i.id = m.instance_id and i.account_id = m.account_id
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid and m.id = $3::uuid
		  and not exists (select 1 from messaging.hidden_messages h
			where h.message_id = m.id and h.user_id = $4::uuid)`,
		accountID, conversationID, messageID, userID,
	).Scan(&d.MessageID, &d.ConversationID, &d.InstanceScopeKey, &d.StorageKey, &d.MimeType,
		&d.FileName, &d.MediaURL, &d.SourceKind, &d.ExternalMessageID, &d.Metadata, &d.Provider)
	return d, err
}

// UpdateRehydratedMedia persiste a midia baixada sob demanda (rehidratacao one-shot): grava as
// colunas de disco + o mediaUrl do endpoint e marca media_source_kind='disk'.
func (s *Store) UpdateRehydratedMedia(ctx context.Context, accountID, conversationID, messageID string, m StoredMedia) error {
	_, err := s.pool.Exec(ctx, `update messaging.messages
		set media_storage_key = $3, media_source_kind = 'disk', media_mime_type = $4,
			media_file_name = coalesce(nullif($5,''), media_file_name), media_file_size_bytes = $6,
			media_url = $7, updated_at = now()
		where id = $1::uuid and account_id = $2::uuid`,
		messageID, accountID, m.StorageKey, m.MimeType, m.FileName, int(m.SizeBytes),
		mediaEndpointPath(conversationID, messageID))
	return err
}

// InsertAudit grava um evento na trilha (messaging.audit_events). Escopado por conta;
// actorUserID/conversationID/messageID vazios viram NULL. payload_json sem PII crua.
func (s *Store) InsertAudit(ctx context.Context, accountID, actorUserID, conversationID, messageID, eventType string, payload json.RawMessage) error {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	_, err := s.pool.Exec(ctx, `insert into messaging.audit_events
		(account_id, actor_user_id, conversation_id, message_id, event_type, payload_json)
		values ($1::uuid, nullif($2,'')::uuid, nullif($3,'')::uuid, nullif($4,'')::uuid, $5, $6)`,
		accountID, actorUserID, conversationID, messageID, eventType, payload)
	return err
}

// GetMaxUploadBytes devolve o teto decodificado da conta (messaging.account_config.max_upload_mb,
// em bytes). Sem linha => default do schema (500 MB). E o teto que o Save aplica no 413.
func (s *Store) GetMaxUploadBytes(ctx context.Context, accountID string) (int64, error) {
	var mb int
	err := s.pool.QueryRow(ctx, `select coalesce(
		(select max_upload_mb from messaging.account_config where account_id = $1::uuid), 500)`,
		accountID).Scan(&mb)
	if err != nil {
		return 0, err
	}
	return int64(mb) << 20, nil
}
