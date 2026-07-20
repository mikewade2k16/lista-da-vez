package omnichannel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// Envio via outbox (spec F6 §1/§2). Fluxo do POST: valida escopo -> valida permissao (reply)
// -> valida/grava a midia em disco -> cria a mensagem PENDING/OUTBOUND -> toca last_message_at
// -> ENFILEIRA no platform/jobs -> publica message.created -> audita -> 200. Falha ao
// enfileirar -> FAILED + publica + 202. account_id vem SEMPRE do Principal.
//
// A fila em si e da F3 (platform/jobs): aqui so PRODUZIMOS o job. ordering_key = conversation_id
// garante o FIFO por conversa (nao burlar). idempotency_key vai CRU — o unique
// (account_id, idempotency_key) do outbox e o mecanismo (nao prefixar com account_id).

// OutboundJobKind despacha o handler de envio no worker (outbound_handler.go).
const OutboundJobKind = "omnichannel.outbound_message"

// maxContentRunes e o teto do corpo textual (contrato do legado).
const maxContentRunes = 4000

// outboundJobPayload e o payload MINIMO do job — sem PII crua (spec §Seguranca). O handler
// recarrega telefone/conteudo/midia do banco a partir destes ids.
type outboundJobPayload struct {
	MessageID      string `json:"messageId"`
	ConversationID string `json:"conversationId"`
}

// Enqueuer e a fatia do outbox que o produtor usa (interface pequena — DI). O
// jobs.PostgresStore satisfaz. created=false => a (account_id, idempotency_key) ja existia.
type Enqueuer interface {
	Enqueue(ctx context.Context, job jobs.NewJob) (id string, created bool, err error)
}

// sendOutcome distingue 200 (enfileirado) de 202 (falhou ao enfileirar, mensagem FAILED).
type sendOutcome int

const (
	outcomeQueued sendOutcome = iota
	outcomeFailedToQueue
)

// SendMessageInput e o body do POST /conversations/{id}/messages (contrato verbatim do legado
// + idempotencyKey acrescentado pela F6). account_id/tenant NAO vem daqui.
type SendMessageInput struct {
	Type                 string          `json:"type"`
	Content              string          `json:"content"`
	MediaURL             string          `json:"mediaUrl"`
	MediaMimeType        string          `json:"mediaMimeType"`
	MediaFileName        string          `json:"mediaFileName"`
	MediaFileSizeBytes   *int            `json:"mediaFileSizeBytes"`
	MediaCaption         string          `json:"mediaCaption"`
	MediaDurationSeconds *int            `json:"mediaDurationSeconds"`
	MetadataJSON         json.RawMessage `json:"metadataJson"`
	IdempotencyKey       string          `json:"idempotencyKey"`
}

// SendService orquestra o envio. scope reusa a validacao de escopo do Service de leitura (A2).
type SendService struct {
	store     *Store
	scope     *Service
	enq       Enqueuer
	media     *DiskMediaStorage
	publisher Publisher
	logger    *slog.Logger
}

// NewSendService monta o service de envio. publisher nil => no-op; enq obrigatorio (sem ele
// nao ha como enfileirar). O scope de leitura nasce do mesmo store (reuso de assertConversationScope).
func NewSendService(store *Store, enq Enqueuer, media *DiskMediaStorage, publisher Publisher, logger *slog.Logger) *SendService {
	if publisher == nil {
		publisher = noopPublisher{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SendService{
		store:     store,
		scope:     NewService(store),
		enq:       enq,
		media:     media,
		publisher: publisher,
		logger:    logger,
	}
}

// SendMessage executa o fluxo completo. canReply=false (VIEWER) => ErrForbidden (403 — e
// permissao, nao escopo). Devolve a MessageView criada, o desfecho (200/202) e o erro.
func (s *SendService) SendMessage(ctx context.Context, accountID string, caller Caller, canReply bool, conversationID string, in SendMessageInput) (MessageView, sendOutcome, error) {
	row, err := s.resolveConversationForSend(ctx, accountID, caller, conversationID)
	if err != nil {
		return MessageView{}, outcomeQueued, err
	}
	if !canReply {
		return MessageView{}, outcomeQueued, ErrForbidden
	}

	msgType := normalizeMessageType(in.Type)
	if err := validateSendBody(msgType, in); err != nil {
		return MessageView{}, outcomeQueued, err
	}

	// Idempotencia (chave do cliente): pre-checagem ANTES de gravar midia/mensagem, para nao
	// criar linha nem arquivo orfao no reenvio. A 2a chamada devolve a MESMA mensagem (200).
	idempotencyKey := strings.TrimSpace(in.IdempotencyKey)
	if idempotencyKey != "" {
		if existingID, found, ferr := s.store.FindOutboxMessageID(ctx, accountID, idempotencyKey); ferr != nil {
			return MessageView{}, outcomeQueued, ferr
		} else if found {
			view, gerr := s.store.GetMessageByID(ctx, accountID, existingID)
			return view, outcomeQueued, translate(gerr)
		}
	} else {
		idempotencyKey = deriveIdempotencyKey()
	}

	var media *StoredMedia
	if categoryForType(msgType) != "" {
		maxBytes, merr := s.store.GetMaxUploadBytes(ctx, accountID)
		if merr != nil {
			return MessageView{}, outcomeQueued, merr
		}
		stored, serr := s.media.Save(ctx, accountID, conversationID, msgType, in.MediaMimeType, in.MediaFileName, in.MediaURL, maxBytes)
		if serr != nil {
			return MessageView{}, outcomeQueued, serr
		}
		media = &stored
	}

	view, err := s.store.CreateOutboundMessage(ctx, outboundMessageInsert{
		AccountID:         accountID,
		ConversationID:    conversationID,
		InstanceID:        row.InstanceID,
		InstanceScopeKey:  row.InstanceScopeKey,
		SenderUserID:      caller.UserID,
		MessageType:       msgType,
		Content:           in.Content,
		MetadataJSON:      in.MetadataJSON,
		Media:             media,
		MediaCaption:      in.MediaCaption,
		MediaDurationSecs: in.MediaDurationSeconds,
	})
	if err != nil {
		return MessageView{}, outcomeQueued, err
	}

	return s.enqueueAndPublish(ctx, accountID, caller.UserID, conversationID, idempotencyKey, view)
}

// SendAIMessage e o unico caminho de saida da IA. Mesmo quando o cerebro roda no n8n,
// a resposta volta ao Go e passa pela mensagem PENDING + outbox + adapter do canal. Nao
// existe envio direto do workflow para Evolution/Meta.
func (s *SendService) SendAIMessage(ctx context.Context, accountID, conversationID, content, runID, inboundMessageID string) error {
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > maxContentRunes {
		return ErrInvalidBody
	}
	idempotencyKey := "ai-reply:" + firstNonEmpty(runID, inboundMessageID)
	if existingID, found, err := s.store.FindOutboxMessageID(ctx, accountID, idempotencyKey); err != nil {
		return err
	} else if found && existingID != "" {
		return nil
	}

	row, err := s.store.GetConversation(ctx, accountID, conversationID)
	if err != nil {
		return translate(err)
	}
	metadata, _ := json.Marshal(map[string]any{"source": "ai", "aiRunId": runID})
	view, err := s.store.CreateOutboundMessage(ctx, outboundMessageInsert{
		AccountID: accountID, ConversationID: conversationID, InstanceID: row.InstanceID,
		InstanceScopeKey: row.InstanceScopeKey, MessageType: "TEXT", Content: content,
		MetadataJSON: metadata,
	})
	if err != nil {
		return err
	}
	_, outcome, err := s.enqueueAndPublish(ctx, accountID, "", conversationID, idempotencyKey, view)
	if err != nil {
		return err
	}
	if outcome == outcomeFailedToQueue {
		return errors.New("omnichannel: ai reply failed to enqueue")
	}
	return nil
}

// enqueueAndPublish enfileira o job e resolve o desfecho: sucesso => message.created + audit +
// 200; conflito de idempotencia (corrida) => remove o orfao e devolve a mensagem vencedora;
// falha real ao enfileirar => FAILED + message.updated + 202.
func (s *SendService) enqueueAndPublish(ctx context.Context, accountID, userID, conversationID, idempotencyKey string, view MessageView) (MessageView, sendOutcome, error) {
	payload, _ := json.Marshal(outboundJobPayload{MessageID: view.ID, ConversationID: conversationID})
	_, created, err := s.enq.Enqueue(ctx, jobs.NewJob{
		AccountID:      accountID,
		OrderingKey:    conversationID,
		IdempotencyKey: idempotencyKey,
		Kind:           OutboundJobKind,
		Payload:        payload,
		MaxAttempts:    5,
	})
	switch {
	case err != nil:
		// Nao enfileirou: a mensagem nao vai sair. Marca FAILED, publica o minimo e devolve 202.
		if _, ferr := s.store.MarkMessageFailed(ctx, accountID, view.ID); ferr != nil {
			s.logger.Error("omnichannel_outbound_mark_failed", "account_id", accountID, "message_id", view.ID)
		}
		view.Status = "FAILED"
		s.publishMessageUpdated(ctx, accountID, view.ID, "FAILED", "")
		s.audit(ctx, accountID, userID, conversationID, view.ID, "MESSAGE_OUTBOUND_FAILED")
		return view, outcomeFailedToQueue, nil
	case !created:
		// Corrida: outra requisicao com a mesma chave venceu. Remove o orfao PENDING e devolve
		// a mensagem ja enfileirada (mesmo id de sempre — zero linha nova visivel).
		_ = s.store.DeleteMessage(ctx, accountID, view.ID)
		if existingID, found, _ := s.store.FindOutboxMessageID(ctx, accountID, idempotencyKey); found {
			winner, gerr := s.store.GetMessageByID(ctx, accountID, existingID)
			return winner, outcomeQueued, translate(gerr)
		}
		return view, outcomeQueued, nil
	default:
		s.publishMessageCreated(ctx, accountID, view)
		s.audit(ctx, accountID, userID, conversationID, view.ID, "MESSAGE_OUTBOUND_QUEUED")
		return view, outcomeQueued, nil
	}
}

// resolveConversationForSend valida escopo (A2) e devolve a linha da conversa (instancia +
// scope key para gravar a mensagem). Fora de escopo => ErrNotFound (404, nunca 403).
func (s *SendService) resolveConversationForSend(ctx context.Context, accountID string, caller Caller, conversationID string) (conversationRow, error) {
	row, err := s.store.GetConversation(ctx, accountID, conversationID)
	if err != nil {
		return conversationRow{}, translate(err)
	}
	if caller.IsAdmin {
		return row, nil
	}
	keys, err := s.scope.accessibleScopeKeys(ctx, accountID, caller)
	if err != nil {
		return conversationRow{}, err
	}
	for _, k := range keys {
		if k == row.InstanceScopeKey {
			return row, nil
		}
	}
	return conversationRow{}, ErrNotFound
}

// validateSendBody aplica o contrato: TEXT exige content; midia exige mediaUrl; content <= 4000.
func validateSendBody(msgType string, in SendMessageInput) error {
	if len([]rune(in.Content)) > maxContentRunes {
		return ErrInvalidBody
	}
	if categoryForType(msgType) == "" {
		if strings.TrimSpace(in.Content) == "" {
			return ErrInvalidBody
		}
		return nil
	}
	if strings.TrimSpace(in.MediaURL) == "" {
		return ErrInvalidBody
	}
	return nil
}

// publishMessageCreated emite message.created com o Message COMPLETO + correlationId (spec F4
// §2, call-site do envio HTTP). mediaUrl e a URL do endpoint (nunca data URL) — sanitizada
// mesmo assim (cinto e suspensorio). correlationId = id da mensagem (estavel, nao "sync-history:").
func (s *SendService) publishMessageCreated(ctx context.Context, accountID string, view MessageView) {
	payload := messageViewPayload(view)
	payload["correlationId"] = view.ID
	s.publisher.PublishOmnichannelEvent(ctx, RealtimeEvent{
		Type:       RealtimeEventMessageCreated,
		AccountID:  accountID,
		ResourceID: view.ID,
		Payload:    payload,
	})
}

// publishMessageUpdated emite o shape MINIMO de message.updated (spec F4 §2, call-site do
// worker): {id, status, externalMessageId, updatedAt, correlationId}. Usado no caminho de
// falha ao enfileirar; o worker usa o mesmo helper.
func (s *SendService) publishMessageUpdated(ctx context.Context, accountID, messageID, status, externalID string) {
	s.publisher.PublishOmnichannelEvent(ctx, RealtimeEvent{
		Type:       RealtimeEventMessageUpdated,
		AccountID:  accountID,
		ResourceID: messageID,
		Payload:    minimalUpdatePayload(messageID, status, externalID, ""),
	})
}

// audit grava um evento na trilha, sem interromper o fluxo em caso de falha (best-effort).
func (s *SendService) audit(ctx context.Context, accountID, userID, conversationID, messageID, eventType string) {
	if err := s.store.InsertAudit(ctx, accountID, userID, conversationID, messageID, eventType, nil); err != nil {
		s.logger.Error("omnichannel_outbound_audit", "account_id", accountID, "event", eventType, "error", err.Error())
	}
}

// messageViewPayload serializa a MessageView para o map camelCase do realtime, sanitizando
// mediaUrl com data: (nunca base64 no WS).
func messageViewPayload(view MessageView) map[string]any {
	raw, _ := json.Marshal(view)
	payload := map[string]any{}
	_ = json.Unmarshal(raw, &payload)
	if mu, ok := payload["mediaUrl"].(string); ok {
		if clean := sanitizeMediaURLForRealtime(mu); clean == "" {
			payload["mediaUrl"] = nil
		} else {
			payload["mediaUrl"] = clean
		}
	}
	return payload
}

// minimalUpdatePayload monta o subconjunto minimo do message.updated. correlationID vazio =>
// omitido; senao usado (o worker passa o id da mensagem; backfill usaria "sync-history:").
func minimalUpdatePayload(messageID, status, externalID, correlationID string) map[string]any {
	payload := map[string]any{"id": messageID, "status": status}
	if externalID != "" {
		payload["externalMessageId"] = externalID
	}
	if correlationID != "" {
		payload["correlationId"] = correlationID
	}
	return payload
}

// deriveIdempotencyKey gera uma chave estavel-por-requisicao quando o cliente nao manda uma.
// Cada POST sem chave e distinto (o front nao reenvia com a mesma) — 16 bytes aleatorios.
func deriveIdempotencyKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "auto"
	}
	return "auto:" + hex.EncodeToString(b)
}
