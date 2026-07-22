package omnichannel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// Envio via outbox (spec F6 §1/§2). Fluxo do POST: valida escopo/permissao/body, resolve quote e
// executa uma transacao unica com idempotencia + takeover + mensagem PENDING + outbox. Midia e
// preparada somente para a requisicao vencedora e removida se a transacao rollbackar. Depois do
// commit publica message.created, audita e devolve 200. account_id vem SEMPRE do Principal.
//
// A fila em si e da F3 (platform/jobs): aqui so PRODUZIMOS a linha do job. ordering_key = conversation_id
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

// sendOutcome preserva o contrato interno dos callers; a transacao atomica so devolve sucesso
// depois que a outbox existe. Falha de persistencia agora e erro, nunca 202 parcialmente commitado.
type sendOutcome int

const outcomeQueued sendOutcome = 0

// SendMessageInput e o body do POST /conversations/{id}/messages (contrato verbatim do legado
// + idempotencyKey acrescentado pela F6). account_id/tenant NAO vem daqui.
type SendMessageInput struct {
	Type                  string          `json:"type"`
	Content               string          `json:"content"`
	MediaURL              string          `json:"mediaUrl"`
	MediaMimeType         string          `json:"mediaMimeType"`
	MediaFileName         string          `json:"mediaFileName"`
	MediaFileSizeBytes    *int            `json:"mediaFileSizeBytes"`
	MediaCaption          string          `json:"mediaCaption"`
	MediaDurationSeconds  *int            `json:"mediaDurationSeconds"`
	MetadataJSON          json.RawMessage `json:"metadataJson"`
	IdempotencyKey        string          `json:"idempotencyKey"`
	ReplyToMessageID      string          `json:"replyToMessageId"`
	QuotedMessageID       string          `json:"quotedMessageId"`
	QuotedMessageIDSnake  string          `json:"quoted_message_id"`
	AllowUnquotedFallback bool            `json:"allowUnquotedFallback"`
	TemplateName          string          `json:"templateName"`
	TemplateLanguage      string          `json:"templateLanguage"`
	TemplateParameters    []string        `json:"templateParameters"`
}

// SendService orquestra o envio. scope reusa a validacao de escopo do Service de leitura (A2).
type SendService struct {
	store     *Store
	scope     *Service
	media     *DiskMediaStorage
	publisher Publisher
	logger    *slog.Logger
}

// NewSendService monta o service de envio. O scope de leitura nasce do mesmo store (reuso de
// assertConversationScope); mensagem + outbox sao persistidas atomicamente pelo Store.
func NewSendService(store *Store, media *DiskMediaStorage, publisher Publisher, logger *slog.Logger) *SendService {
	if publisher == nil {
		publisher = noopPublisher{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SendService{
		store:     store,
		scope:     NewService(store),
		media:     media,
		publisher: publisher,
		logger:    logger,
	}
}

// SendMessage executa o fluxo completo. A permissao efetiva vem do RBAC canonico da conta;
// papel legado nunca concede reply. Devolve a MessageView criada, o desfecho e o erro.
func (s *SendService) SendMessage(ctx context.Context, accountID string, principal auth.Principal, conversationID string, in SendMessageInput) (MessageView, sendOutcome, error) {
	caller := Caller{UserID: principal.UserID, IsAdmin: isAdminPrincipal(principal)}
	row, err := s.resolveConversationForSend(ctx, accountID, caller, conversationID)
	if err != nil {
		return MessageView{}, outcomeQueued, err
	}
	if err := s.scope.requirePermission(ctx, accountID, principal, "omnichannel.conversations.reply"); err != nil {
		return MessageView{}, outcomeQueued, err
	}

	msgType := normalizeMessageType(in.Type)
	if strings.EqualFold(strings.TrimSpace(in.Type), "TEMPLATE") {
		msgType = "TEMPLATE"
	}
	if err := validateSendBody(msgType, in); err != nil {
		return MessageView{}, outcomeQueued, err
	}

	replyMessageID := firstNonEmpty(in.ReplyToMessageID, in.QuotedMessageID, in.QuotedMessageIDSnake)
	var reply *replyReferenceRow
	if strings.TrimSpace(replyMessageID) != "" {
		resolved, rerr := s.store.ResolveReplyReference(ctx, accountID, conversationID, replyMessageID)
		if rerr != nil {
			return MessageView{}, outcomeQueued, translate(rerr)
		}
		if strings.TrimSpace(resolved.ExternalMessageID) == "" {
			if !in.AllowUnquotedFallback {
				return MessageView{}, outcomeQueued, ErrInvalidBody
			}
		} else {
			reply = &resolved
		}
	}

	// A chave e resolvida antes da transacao composta. Mesmo a chave automatica e unica por POST;
	// quando o cliente repete uma chave, o Store devolve a mensagem vencedora sem preparar midia.
	idempotencyKey := strings.TrimSpace(in.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = deriveIdempotencyKey()
	}

	var prepareMedia outboundMediaPreparer
	if categoryForType(msgType) != "" {
		if s.media == nil {
			return MessageView{}, outcomeQueued, ErrMediaInvalid
		}
		maxBytes, merr := s.store.GetMaxUploadBytes(ctx, accountID)
		if merr != nil {
			return MessageView{}, outcomeQueued, merr
		}
		prepareMedia = func() (*StoredMedia, func(), error) {
			stored, serr := s.media.Save(ctx, accountID, conversationID, msgType,
				in.MediaMimeType, in.MediaFileName, in.MediaURL, maxBytes)
			if serr != nil {
				return nil, nil, serr
			}
			cleanup := func() {
				if removeErr := s.media.Remove(stored.StorageKey); removeErr != nil {
					s.logger.Error("omnichannel_outbound_media_rollback_cleanup",
						"account_id", accountID, "conversation_id", conversationID)
				}
			}
			return &stored, cleanup, nil
		}
	}

	// Takeover, invalidacao da IA, mensagem e outbox commitam juntos. Publicacao e auditoria so
	// acontecem para a requisicao que criou o registro; replay idempotente apenas devolve a view.
	metadataJSON := in.MetadataJSON
	if msgType == "TEMPLATE" {
		metadataJSON, err = withWhatsAppTemplateMetadata(metadataJSON, in)
		if err != nil {
			return MessageView{}, outcomeQueued, err
		}
	}
	view, created, err := s.store.CreateHumanOutboundMessage(ctx, outboundMessageInsert{
		AccountID:         accountID,
		ConversationID:    conversationID,
		InstanceID:        row.InstanceID,
		InstanceScopeKey:  row.InstanceScopeKey,
		SenderUserID:      principal.UserID,
		MessageType:       msgType,
		Content:           in.Content,
		MetadataJSON:      metadataJSON,
		MediaCaption:      in.MediaCaption,
		MediaDurationSecs: in.MediaDurationSeconds,
		Origin:            "human",
		Reply:             reply,
	}, idempotencyKey, func(snap convSnapshot) (stateUpdate, *decisionRecord, error) {
		return s.scope.decideTransition(ctx, accountID, EventMsgOutboundHuman,
			TransitionPayload{ActorUserID: principal.UserID}, snap)
	}, prepareMedia)
	if err != nil {
		return MessageView{}, outcomeQueued, err
	}
	if created {
		s.publishMessageCreated(ctx, accountID, view)
		s.audit(ctx, accountID, principal.UserID, conversationID, view.ID, "MESSAGE_OUTBOUND_QUEUED")
	}
	return view, outcomeQueued, nil
}

// SendAIMessage e o unico caminho de saida da IA. Mesmo quando o cerebro roda no n8n,
// a resposta volta ao Go e passa pela mensagem PENDING + outbox + adapter do canal. Nao
// existe envio direto do workflow para Evolution/Meta.
func (s *SendService) SendAIMessage(ctx context.Context, accountID, conversationID, content, runID, inboundMessageID string, generation int64) error {
	_, err := s.SendAIMessageWithResult(ctx, accountID, conversationID, content, runID, inboundMessageID, generation)
	return err
}

// SendAIMessageWithResult is the close-aware variant. The returned message ID
// can be preserved while the close transition invalidates every other AI item.
func (s *SendService) SendAIMessageWithResult(ctx context.Context, accountID, conversationID, content, runID, inboundMessageID string, generation int64) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > maxContentRunes {
		return "", ErrInvalidBody
	}
	idempotencyKey := "ai-reply:" + firstNonEmpty(runID, inboundMessageID)
	view, created, err := s.store.CreateAIOutboundMessage(ctx, accountID, conversationID,
		content, runID, idempotencyKey, generation)
	if err != nil {
		return "", err
	}
	if !created {
		return view.ID, nil
	}
	s.publishMessageCreated(ctx, accountID, view)
	s.audit(ctx, accountID, "", conversationID, view.ID, "MESSAGE_OUTBOUND_QUEUED")
	return view.ID, nil
}

// PublishAIAutoCloseResult emits the final reply created atomically with an
// accepted close. Rejected or idempotently replayed evaluations emit nothing.
func (s *SendService) PublishAIAutoCloseResult(ctx context.Context, accountID, conversationID string, decision AutoCloseDecisionView) {
	if decision.finalMessage == nil || !decision.finalMessageCreated {
		return
	}
	s.publishMessageCreated(ctx, accountID, *decision.finalMessage)
	s.audit(ctx, accountID, "", conversationID, decision.finalMessage.ID, "MESSAGE_OUTBOUND_QUEUED")
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
	if msgType == "TEMPLATE" {
		if strings.TrimSpace(in.TemplateName) == "" || len([]rune(in.TemplateName)) > 512 || strings.TrimSpace(in.TemplateLanguage) == "" || len([]rune(in.TemplateLanguage)) > 32 || len(in.TemplateParameters) > 100 {
			return ErrInvalidBody
		}
		for _, value := range in.TemplateParameters {
			if len([]rune(value)) > 1024 {
				return ErrInvalidBody
			}
		}
		return nil
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

func withWhatsAppTemplateMetadata(raw json.RawMessage, in SendMessageInput) (json.RawMessage, error) {
	metadata := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return nil, ErrInvalidBody
		}
	}
	metadata["whatsappTemplate"] = map[string]any{
		"name": in.TemplateName, "language": in.TemplateLanguage, "parameters": in.TemplateParameters,
	}
	return json.Marshal(metadata)
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
