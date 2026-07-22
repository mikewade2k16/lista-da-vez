package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	Origin            string
	Reply             *replyReferenceRow
}

type replyReferenceRow struct {
	MessageID         string
	ExternalMessageID string
	Content           string
	MessageType       string
	SenderName        string
}

// createOutboundMessageTx grava a mensagem PENDING/OUTBOUND, monta a URL autenticada de midia e
// toca last_message_at usando a transacao composta do caller.
func createOutboundMessageTx(ctx context.Context, tx pgx.Tx, in outboundMessageInsert) (MessageView, error) {
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
	replyMessageID, replyExternalID := "", ""
	if in.Reply != nil {
		replyMessageID = in.Reply.MessageID
		replyExternalID = in.Reply.ExternalMessageID
	}
	err := tx.QueryRow(ctx, `insert into messaging.messages
		(account_id, conversation_id, instance_id, instance_scope_key, sender_user_id, direction,
		 message_type, content, media_mime_type, media_file_name, media_file_size_bytes, media_caption,
		 media_duration_seconds, media_storage_key, media_source_kind, metadata_json, status,
		 origin, reply_to_message_id, reply_to_external_message_id)
		values ($1::uuid, $2::uuid, nullif($3,'')::uuid, $4, nullif($5,'')::uuid, 'OUTBOUND',
		 $6, $7, nullif($8,''), nullif($9,''), $10, nullif($11,''),
		 $12, nullif($13,''), $14, $15, 'PENDING', $16,
		 nullif($17,'')::uuid, nullif($18,''))
		returning id::text`,
		in.AccountID, in.ConversationID, deref(in.InstanceID), in.InstanceScopeKey, in.SenderUserID,
		in.MessageType, in.Content, mimeType, fileName, sizeBytes, in.MediaCaption,
		in.MediaDurationSecs, storageKey, sourceKind, metadata,
		firstNonEmpty(in.Origin, "human"), replyMessageID, replyExternalID,
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
	return view, nil
}

// outboundMediaPreparer so e chamado depois que a chave idempotente foi reservada pelo lock
// transacional e confirmada como nova. cleanup remove o arquivo se qualquer escrita seguinte
// falhar/rollbackar; em sucesso ele nao e executado.
type outboundMediaPreparer func() (media *StoredMedia, cleanup func(), err error)

// CreateHumanOutboundMessage persiste o efeito inteiro do POST humano em uma unica transacao:
// idempotencia, msg.outbound.human, invalidacao da IA, mensagem e outbox. Assim, falha ao criar a
// outbox nao deixa takeover/mensagem parcialmente commitados e uma repeticao nunca executa o
// efeito novamente. O advisory lock e transacional, tenant-scoped e funciona entre processos.
func (s *Store) CreateHumanOutboundMessage(ctx context.Context, in outboundMessageInsert,
	idempotencyKey string, decide func(convSnapshot) (stateUpdate, *decisionRecord, error),
	prepareMedia outboundMediaPreparer) (MessageView, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return MessageView{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	lockKey := "omnichannel:human-outbound:" + in.AccountID + ":" + idempotencyKey
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return MessageView{}, false, err
	}

	var existingMessageID, existingConversationID string
	err = tx.QueryRow(ctx, `select coalesce(payload->>'messageId', ''),
		coalesce(payload->>'conversationId', '') from messaging.outbox
		where account_id = $1::uuid and idempotency_key = $2`,
		in.AccountID, idempotencyKey).Scan(&existingMessageID, &existingConversationID)
	if err == nil {
		if existingMessageID == "" || existingConversationID == "" {
			return MessageView{}, false, errors.New("omnichannel: outbox idempotente sem messageId")
		}
		if existingConversationID != in.ConversationID {
			return MessageView{}, false, ErrInvalidBody
		}
		view, scanErr := scanMessage(tx.QueryRow(ctx, `select `+messageCols+` from messaging.messages m
			where m.account_id = $1::uuid and m.id = $2::uuid`, in.AccountID, existingMessageID))
		if scanErr != nil {
			return MessageView{}, false, scanErr
		}
		if err := tx.Commit(ctx); err != nil {
			return MessageView{}, false, err
		}
		return view, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return MessageView{}, false, err
	}

	snap, err := lockConversationSnapshotTx(ctx, tx, in.AccountID, in.ConversationID)
	if err != nil {
		return MessageView{}, false, err
	}
	upd, dec, err := decide(snap)
	if err != nil {
		return MessageView{}, false, err
	}
	if err := applyStateUpdateTx(ctx, tx, in.AccountID, in.ConversationID, upd, s.AIDispatchV2Enabled()); err != nil {
		return MessageView{}, false, err
	}
	if err := insertDecisionTx(ctx, tx, in.AccountID, in.ConversationID, dec); err != nil {
		return MessageView{}, false, err
	}

	cleanup := func() {}
	keepMedia := false
	if prepareMedia != nil {
		media, preparedCleanup, prepareErr := prepareMedia()
		if prepareErr != nil {
			return MessageView{}, false, prepareErr
		}
		in.Media = media
		if preparedCleanup != nil {
			cleanup = preparedCleanup
		}
	}
	defer func() {
		if !keepMedia {
			cleanup()
		}
	}()

	view, err := createOutboundMessageTx(ctx, tx, in)
	if err != nil {
		return MessageView{}, false, err
	}
	payload, _ := json.Marshal(outboundJobPayload{MessageID: view.ID, ConversationID: in.ConversationID})
	if _, err := tx.Exec(ctx, `insert into messaging.outbox
		(account_id, ordering_key, idempotency_key, kind, payload, max_attempts)
		values ($1::uuid, $2, $3, $4, $5::jsonb, 5)`,
		in.AccountID, in.ConversationID, idempotencyKey, OutboundJobKind, payload); err != nil {
		return MessageView{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		// Commit pode ter resultado incerto se a conexao cair depois de o servidor confirmar.
		// Confere a fonte duravel antes de decidir se o arquivo deve ser removido. Se a
		// confirmação também falhar, preserva o arquivo: um registro commitado apontando
		// para mídia removida é irrecuperável; o scanner de órfãos trata o caso inconclusivo
		// depois da janela de retenção.
		checkCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, found, checkErr := s.FindOutboxMessageID(checkCtx, in.AccountID, idempotencyKey)
		cancel()
		keepMedia = found || checkErr != nil
		return MessageView{}, false, err
	}
	keepMedia = true
	return view, true, nil
}

// ResolveReplyReference valida que a mensagem citada pertence a mesma conta E conversa.
// O external id e obrigatorio para o provider; a API decide se permite degradacao explicita.
func (s *Store) ResolveReplyReference(ctx context.Context, accountID, conversationID, messageID string) (replyReferenceRow, error) {
	var r replyReferenceRow
	err := s.pool.QueryRow(ctx, `select m.id::text, coalesce(m.external_message_id, ''),
		m.content, m.message_type, coalesce(m.sender_name, '')
		from messaging.messages m
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid and m.id = $3::uuid`,
		accountID, conversationID, messageID,
	).Scan(&r.MessageID, &r.ExternalMessageID, &r.Content, &r.MessageType, &r.SenderName)
	return r, err
}

// GetMessageByID devolve a mensagem por id, so escopada por conta (sem exclusao de hidden —
// e para o produtor/idempotencia, nao para a leitura do inbox). Outra conta => ErrNoRows.
func (s *Store) GetMessageByID(ctx context.Context, accountID, messageID string) (MessageView, error) {
	return scanMessage(s.pool.QueryRow(ctx, `select `+messageCols+` from messaging.messages m
		where m.id = $1::uuid and m.account_id = $2::uuid`, messageID, accountID))
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
	MessageID                string
	ConversationID           string
	Status                   string
	Origin                   string
	ProviderErrorCode        string
	ProviderStatusAt         *time.Time
	MessageType              string
	Content                  string
	MediaCaption             *string
	MediaStorageKey          *string
	MediaURL                 *string
	MediaMimeType            *string
	MediaFileName            *string
	ToPhone                  *string
	ToExternalID             string
	ConversationExt          string
	InstanceScopeKey         string
	Provider                 string
	ReplyExternalID          *string
	ReplyContent             *string
	ReplyMessageType         *string
	ConversationState        State
	ConversationAIGeneration int64
	MessageAIGeneration      *int64
	CredentialCiphertext     string
	CredentialConfig         map[string]string
	TemplateName             string
	TemplateLanguage         string
	TemplateParameters       []string
}

// outboundDispatchResult e o estado realmente persistido depois do envio. O handler usa
// este resultado no realtime; nunca presume SENT porque um eco pode ja estar READ.
type outboundDispatchResult struct {
	MessageID         string
	ConversationID    string
	Status            string
	ExternalMessageID string
	UpdatedAt         time.Time
	Dispatched        bool
}

type outboundAIMetadata struct {
	AIGeneration     *int64 `json:"aiGeneration"`
	WhatsAppTemplate *struct {
		Name       string   `json:"name"`
		Language   string   `json:"language"`
		Parameters []string `json:"parameters"`
	} `json:"whatsappTemplate"`
}

// GetOutboundSendData carrega a mensagem + a conversa + a instancia para o envio. Escopado por
// conta; mensagem de outra conta => ErrNoRows.
func (s *Store) GetOutboundSendData(ctx context.Context, accountID, messageID string) (outboundSendData, error) {
	return scanOutboundSendData(s.pool.QueryRow(ctx, outboundSendDataSQL(""), accountID, messageID))
}

func outboundSendDataSQL(lockClause string) string {
	return `select m.id::text, m.conversation_id::text, m.status, m.origin,
			m.provider_error_code, m.provider_status_at, m.message_type,
			m.content, m.media_caption, m.media_storage_key, m.media_url, m.media_mime_type, m.media_file_name,
			c.contact_phone, c.external_id, c.external_id, c.instance_scope_key,
			coalesce(i.provider, case when c.channel='INSTAGRAM' then 'instagram' else '' end),
			m.reply_to_external_message_id,
			(select r.content from messaging.messages r where r.id = m.reply_to_message_id
			  and r.account_id = m.account_id and r.conversation_id = m.conversation_id),
			(select r.message_type from messaging.messages r where r.id = m.reply_to_message_id
			  and r.account_id = m.account_id and r.conversation_id = m.conversation_id),
			c.state, c.ai_generation, m.metadata_json,
			coalesce(i.credentials_ciphertext, ia.credentials_ciphertext, ''), coalesce(i.provider_config, ia.provider_config, '{}'::jsonb)
		from messaging.messages m
		join messaging.conversations c on c.id = m.conversation_id and c.account_id = m.account_id
		left join messaging.whatsapp_instances i on i.id = m.instance_id and i.account_id = m.account_id
		left join messaging.instagram_accounts ia on c.channel='INSTAGRAM' and ia.account_id=c.account_id and ia.ig_user_id=c.instance_scope_key
		where m.account_id = $1::uuid and m.id = $2::uuid ` + lockClause
}

func scanOutboundSendData(row rowScanner) (outboundSendData, error) {
	var d outboundSendData
	var metadata, credentialConfig []byte
	err := row.Scan(&d.MessageID, &d.ConversationID, &d.Status, &d.Origin,
		&d.ProviderErrorCode, &d.ProviderStatusAt, &d.MessageType, &d.Content, &d.MediaCaption,
		&d.MediaStorageKey, &d.MediaURL, &d.MediaMimeType, &d.MediaFileName, &d.ToPhone, &d.ToExternalID, &d.ConversationExt,
		&d.InstanceScopeKey, &d.Provider, &d.ReplyExternalID, &d.ReplyContent, &d.ReplyMessageType,
		&d.ConversationState, &d.ConversationAIGeneration, &metadata,
		&d.CredentialCiphertext, &credentialConfig)
	if err != nil {
		return outboundSendData{}, err
	}
	var aiMeta outboundAIMetadata
	if len(metadata) > 0 && json.Unmarshal(metadata, &aiMeta) == nil {
		d.MessageAIGeneration = aiMeta.AIGeneration
		if aiMeta.WhatsAppTemplate != nil {
			d.TemplateName = strings.TrimSpace(aiMeta.WhatsAppTemplate.Name)
			d.TemplateLanguage = strings.TrimSpace(aiMeta.WhatsAppTemplate.Language)
			d.TemplateParameters = append([]string(nil), aiMeta.WhatsAppTemplate.Parameters...)
		}
	}
	d.CredentialConfig = decodeStringMap(credentialConfig)
	return d, err
}

// DispatchOutbound serializa a revalidacao do lease IA, a chamada ao provider e a
// persistencia do ACK local pelo mesmo lock da conversa usado em ApplyTransition. O callback
// deve respeitar o timeout do contexto; em erro a transacao faz rollback e libera o lock.
func (s *Store) DispatchOutbound(ctx context.Context, accountID, messageID string,
	send func(outboundSendData) (string, error)) (outboundDispatchResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return outboundDispatchResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// ApplyTransition sempre bloqueia a conversa antes de tocar mensagens/outbox. Repetir
	// explicitamente essa ordem evita o ciclo conversa->mensagem versus mensagem->conversa
	// quando dispatch e takeover começam ao mesmo tempo.
	var conversationID string
	err = tx.QueryRow(ctx, `select conversation_id::text from messaging.messages
		where account_id = $1::uuid and id = $2::uuid`, accountID, messageID).Scan(&conversationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return outboundDispatchResult{}, nil
	}
	if err != nil {
		return outboundDispatchResult{}, err
	}
	var lockedConversationID string
	err = tx.QueryRow(ctx, `select id::text from messaging.conversations
		where account_id = $1::uuid and id = $2::uuid for update`,
		accountID, conversationID).Scan(&lockedConversationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return outboundDispatchResult{}, nil
	}
	if err != nil {
		return outboundDispatchResult{}, err
	}

	data, err := scanOutboundSendData(tx.QueryRow(ctx,
		outboundSendDataSQL("for update of m"), accountID, messageID))
	if errors.Is(err, pgx.ErrNoRows) {
		return outboundDispatchResult{}, nil
	}
	if err != nil {
		return outboundDispatchResult{}, err
	}
	result := outboundDispatchResult{
		MessageID: data.MessageID, ConversationID: data.ConversationID,
		Status: data.Status, ExternalMessageID: "",
	}
	if data.Origin == "ai" && (data.Status == "FAILED" && data.ProviderErrorCode == "ai_handoff_canceled" ||
		data.ConversationState != StateAIActive || data.MessageAIGeneration == nil ||
		*data.MessageAIGeneration != data.ConversationAIGeneration) {
		return outboundDispatchResult{}, ErrAILeaseInvalid
	}
	if data.Status == "SENT" || data.Status == "DELIVERED" || data.Status == "READ" ||
		data.Status == "FAILED" || data.Status == "DELETED" {
		return result, nil
	}
	if err := s.enforceWhatsAppCloudPolicy(ctx, tx, accountID, data); err != nil {
		return result, err
	}

	externalID, err := send(data)
	if err != nil {
		return result, err
	}
	delivery, err := markMessageSentTx(ctx, tx, accountID, messageID, externalID)
	if err != nil {
		return result, err
	}
	if err := tx.Commit(ctx); err != nil {
		return outboundDispatchResult{}, err
	}
	delivery.Dispatched = true
	return delivery, nil
}

// MarkMessageSent transiciona a mensagem para SENT com o external id do provider. Devolve o
// updated_at para o shape minimo do message.updated. Idempotente: um re-run do worker sobre
// uma ja SENT nao muda o external id (where status <> 'SENT' evita sobrescrever).
func (s *Store) MarkMessageSent(ctx context.Context, accountID, messageID, externalID string) (time.Time, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	delivery, err := markMessageSentTx(ctx, tx, accountID, messageID, externalID)
	if err != nil {
		return time.Time{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}
	return delivery.UpdatedAt, nil
}

type deliverySnapshot struct {
	Status    string
	StatusAt  *time.Time
	ErrorCode string
}

func markMessageSentTx(ctx context.Context, tx pgx.Tx, accountID, messageID, externalID string) (outboundDispatchResult, error) {
	var conversationID, instanceScopeKey, currentExternalID string
	var current deliverySnapshot
	preserveEchoFailure := false
	err := tx.QueryRow(ctx, `select conversation_id::text, instance_scope_key,
		coalesce(external_message_id, ''), status, provider_status_at, provider_error_code
		from messaging.messages
		where account_id = $1::uuid and id = $2::uuid
		for update`, accountID, messageID,
	).Scan(&conversationID, &instanceScopeKey, &currentExternalID,
		&current.Status, &current.StatusAt, &current.ErrorCode)
	if err != nil {
		return outboundDispatchResult{}, err
	}

	externalID = strings.TrimSpace(externalID)
	if externalID != "" && currentExternalID == "" {
		var echoID, echoConversationID, echoOrigin string
		var echo deliverySnapshot
		err = tx.QueryRow(ctx, `select id::text, conversation_id::text, origin,
			status, provider_status_at, provider_error_code
			from messaging.messages
			where account_id = $1::uuid and instance_scope_key = $2
			  and external_message_id = $3 and id <> $4::uuid
			for update`, accountID, instanceScopeKey, externalID, messageID,
		).Scan(&echoID, &echoConversationID, &echoOrigin,
			&echo.Status, &echo.StatusAt, &echo.ErrorCode)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// O eco ainda nao chegou. O update abaixo reserva o external id na unique.
		case err != nil:
			return outboundDispatchResult{}, err
		case echoConversationID != conversationID || echoOrigin != "provider_device":
			return outboundDispatchResult{}, errors.New("omnichannel: external message id already belongs to another message")
		default:
			// O webhook fromMe venceu a corrida. A mensagem local/optimista permanece canonica:
			// referencias tardias apontam para ela e a linha espelho e removida antes de reservar
			// o external id. O front reconcilia pelo externalMessageId do message.updated.
			if _, err = tx.Exec(ctx, `update messaging.messages
				set reply_to_message_id = $3::uuid, updated_at = now()
				where account_id = $1::uuid and reply_to_message_id = $2::uuid`,
				accountID, echoID, messageID); err != nil {
				return outboundDispatchResult{}, err
			}
			if _, err = tx.Exec(ctx, `delete from messaging.outbox
				where account_id = $1::uuid and idempotency_key = $2
				  and kind = $3 and status in ('pending','processing')`,
				accountID, "media-fetch:"+echoID, MediaFetchJobKind); err != nil {
				return outboundDispatchResult{}, err
			}
			if _, err = tx.Exec(ctx, `delete from messaging.messages
				where account_id = $1::uuid and id = $2::uuid and origin = 'provider_device'`,
				accountID, echoID); err != nil {
				return outboundDispatchResult{}, err
			}
			current = mergeEchoDelivery(current, echo)
			preserveEchoFailure = current.Status == "FAILED" && echo.Status == "FAILED"
		}
	}

	if currentExternalID == "" {
		currentExternalID = externalID
	}
	if (current.Status == "FAILED" && !preserveEchoFailure) ||
		(current.Status != "FAILED" && deliveryRank(current.Status) < deliveryRank("SENT")) {
		now := time.Now().UTC()
		current = deliverySnapshot{Status: "SENT", StatusAt: &now}
	}
	if current.StatusAt == nil {
		now := time.Now().UTC()
		current.StatusAt = &now
	}

	var result outboundDispatchResult
	err = tx.QueryRow(ctx, `update messaging.messages
		set status = $4,
			external_message_id = coalesce(nullif($3,''), external_message_id),
			provider_status_at = $5,
			provider_error_code = $6,
			updated_at = now()
		where id = $1::uuid and account_id = $2::uuid
		returning id::text, conversation_id::text, status,
			coalesce(external_message_id, ''), updated_at`,
		messageID, accountID, currentExternalID, current.Status, current.StatusAt,
		current.ErrorCode).Scan(&result.MessageID, &result.ConversationID, &result.Status,
		&result.ExternalMessageID, &result.UpdatedAt)
	if err != nil {
		return outboundDispatchResult{}, err
	}
	return result, nil
}

func mergeEchoDelivery(current, echo deliverySnapshot) deliverySnapshot {
	if current.Status == "DELETED" {
		return current
	}
	if echo.Status == "DELETED" {
		return echo
	}
	currentRank, echoRank := deliveryRank(current.Status), deliveryRank(echo.Status)
	if echoRank > currentRank || (current.Status == "FAILED" && echoRank >= 1) {
		return echo
	}
	if echo.Status == "FAILED" && currentRank < deliveryRank("DELIVERED") &&
		isDeliveryTimeAfter(echo.StatusAt, current.StatusAt) {
		return echo
	}
	if echo.Status == current.Status && isDeliveryTimeAfter(echo.StatusAt, current.StatusAt) {
		return echo
	}
	return current
}

func deliveryRank(status string) int {
	switch status {
	case "PENDING":
		return 0
	case "SENT":
		return 1
	case "DELIVERED":
		return 2
	case "READ":
		return 3
	case "DELETED":
		return 4
	default:
		return -1
	}
}

func isDeliveryTimeAfter(candidate, current *time.Time) bool {
	if candidate == nil {
		return false
	}
	return current == nil || candidate.After(*current)
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
