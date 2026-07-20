package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// Handler do job de envio (spec F6 §2). Consumido pelo WORKER da F3 (platform/jobs): resolve
// a instancia/credencial, chama channel.Provider.SendMessage, transiciona SENT/FAILED e publica
// message.updated. NAO reimplementa fila — o claim/retry/dead-letter/FIFO sao do engine.
//
// Erro devolvido ao engine e CLASSIFICADO por jobs.Classify: transitorio/429/5xx reagenda com
// backoff (mensagem segue PENDING), 401/403/404/405/400/422 e esgotamento vao a dead-letter.
// Quando a falha e TERMINAL, o handler marca a mensagem FAILED + audita ANTES de devolver o erro
// (o engine so conhece o outbox, nao messaging.messages). Idempotente: um re-run sobre uma
// mensagem ja SENT nao reenvia.

// outboundStore e a fatia de persistencia que o handler consome (interface pequena — DI e
// testabilidade). *Store satisfaz.
type outboundStore interface {
	GetOutboundSendData(ctx context.Context, accountID, messageID string) (outboundSendData, error)
	FindProviderCredential(ctx context.Context, accountID, provider string) (string, map[string]string, bool, error)
	MarkMessageSent(ctx context.Context, accountID, messageID, externalID string) (time.Time, error)
	MarkMessageFailed(ctx context.Context, accountID, messageID string) (time.Time, error)
	InsertAudit(ctx context.Context, accountID, actorUserID, conversationID, messageID, eventType string, payload json.RawMessage) error
}

// OutboundHandler implementa jobs.Handler para o kind OutboundJobKind.
type OutboundHandler struct {
	store     outboundStore
	registry  *channel.Registry
	secretBox *secretbox.Box
	publisher Publisher
	logger    *slog.Logger
}

// NewOutboundHandler monta o handler. publisher nil => no-op; secretBox nil => sem credencial
// decifrada (o mock nao precisa). Registrar no worker: w.Register(OutboundJobKind, handler).
func NewOutboundHandler(store outboundStore, registry *channel.Registry, box *secretbox.Box, publisher Publisher, logger *slog.Logger) *OutboundHandler {
	if publisher == nil {
		publisher = noopPublisher{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OutboundHandler{store: store, registry: registry, secretBox: box, publisher: publisher, logger: logger}
}

// Handle processa um job de envio. Devolve nil quando nao ha o que fazer (mensagem sumiu ou ja
// SENT) ou quando o envio deu certo; devolve erro para o engine reagendar/dead-letter.
func (h *OutboundHandler) Handle(ctx context.Context, job jobs.Job) error {
	var p outboundJobPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil || strings.TrimSpace(p.MessageID) == "" {
		// Payload malformado: reprocessar nao resolve. Dead-letter direto.
		return &jobs.StatusError{Unrecoverable: true, Err: errors.New("outbound: payload invalido")}
	}

	data, err := h.store.GetOutboundSendData(ctx, job.AccountID, p.MessageID)
	if err != nil {
		// Mensagem inexistente (apagada): nada a enviar. Concluido.
		return nil
	}
	if data.Status == "SENT" || data.Status == "FAILED" {
		return nil // ja resolvida em run anterior (idempotencia)
	}

	sendErr := h.dispatch(ctx, job.AccountID, data)
	if sendErr == nil {
		return nil
	}
	if isTerminalJobError(sendErr, job) {
		h.settleFailed(ctx, job.AccountID, data)
	}
	return sendErr
}

// dispatch resolve provider + credencial e chama SendMessage; no sucesso marca SENT, publica
// message.updated e audita. Erros de envio/config sobem para a classificacao do engine.
func (h *OutboundHandler) dispatch(ctx context.Context, accountID string, data outboundSendData) error {
	if data.Provider == "" || strings.TrimSpace(deref(data.ToPhone)) == "" {
		// Sem instancia resolvida ou sem numero de destino: correcao humana, nao tempo.
		return &jobs.StatusError{Unrecoverable: true, Err: errors.New("outbound: instancia ou destino ausente")}
	}
	provider, err := h.registry.Get(data.Provider)
	if err != nil {
		return &jobs.StatusError{Unrecoverable: true, Err: errors.New("outbound: provider sem adapter")}
	}
	cred, err := h.resolveCredentials(ctx, accountID, data.Provider)
	if err != nil {
		return err
	}

	result, err := provider.SendMessage(ctx, cred, channel.OutboundMessage{
		InstanceName:      data.InstanceScopeKey,
		ToPhone:           deref(data.ToPhone),
		MessageType:       data.MessageType,
		Content:           data.Content,
		MediaURL:          deref(data.MediaURL),
		MediaMimeType:     deref(data.MediaMimeType),
		MediaFileName:     deref(data.MediaFileName),
		MediaCaption:      deref(data.MediaCaption),
		IdempotencyKey:    data.MessageID, // estavel por mensagem => dedup no provider em re-run
		ConversationExtID: data.ConversationExt,
	})
	if err != nil {
		return err
	}

	if _, mErr := h.store.MarkMessageSent(ctx, accountID, data.MessageID, result.ExternalMessageID); mErr != nil {
		return mErr
	}
	h.publishUpdated(ctx, accountID, data.MessageID, "SENT", result.ExternalMessageID)
	if aErr := h.store.InsertAudit(ctx, accountID, "", data.ConversationID, data.MessageID, "MESSAGE_OUTBOUND_SENT", nil); aErr != nil {
		h.logger.Error("omnichannel_outbound_audit", "account_id", accountID, "event", "MESSAGE_OUTBOUND_SENT")
	}
	return nil
}

// settleFailed marca a mensagem FAILED, publica o minimo e audita — chamado so quando a falha
// e terminal (o engine vai mandar o job para a dead-letter em seguida).
func (h *OutboundHandler) settleFailed(ctx context.Context, accountID string, data outboundSendData) {
	if _, err := h.store.MarkMessageFailed(ctx, accountID, data.MessageID); err != nil {
		h.logger.Error("omnichannel_outbound_mark_failed", "account_id", accountID, "message_id", data.MessageID)
		return
	}
	h.publishUpdated(ctx, accountID, data.MessageID, "FAILED", "")
	if err := h.store.InsertAudit(ctx, accountID, "", data.ConversationID, data.MessageID, "MESSAGE_OUTBOUND_FAILED", nil); err != nil {
		h.logger.Error("omnichannel_outbound_audit", "account_id", accountID, "event", "MESSAGE_OUTBOUND_FAILED")
	}
}

// resolveCredentials monta as Credentials de (conta, provider): ciphertext decifrado pelo
// secretbox + provider_config nao-secreto. Sem ciphertext (mock) => Credentials vazio. A chave
// crua NUNCA vai a log (canonico §10).
func (h *OutboundHandler) resolveCredentials(ctx context.Context, accountID, provider string) (channel.Credentials, error) {
	cipher, config, found, err := h.store.FindProviderCredential(ctx, accountID, provider)
	if err != nil {
		return channel.Credentials{}, err
	}
	cred := channel.Credentials{Config: config}
	if !found || strings.TrimSpace(cipher) == "" || h.secretBox == nil {
		return cred, nil
	}
	token, err := h.secretBox.Decrypt(cipher)
	if err != nil {
		h.logger.Error("omnichannel_credential_decrypt_failed", "account_id", accountID, "provider", provider)
		return channel.Credentials{}, err
	}
	cred.Token = token
	return cred, nil
}

// publishUpdated emite o message.updated MINIMO (spec F4 §2, call-site do worker):
// {id, status, externalMessageId, updatedAt, correlationId}. correlationId = id da mensagem.
func (h *OutboundHandler) publishUpdated(ctx context.Context, accountID, messageID, status, externalID string) {
	payload := minimalUpdatePayload(messageID, status, externalID, messageID)
	payload["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	h.publisher.PublishOmnichannelEvent(ctx, RealtimeEvent{
		Type:       RealtimeEventMessageUpdated,
		AccountID:  accountID,
		ResourceID: messageID,
		Payload:    payload,
	})
}

// isTerminalJobError espelha jobs.Worker.settleFailure: classe unrecoverable OU tentativas
// esgotadas => o engine vai para a dead-letter, entao a mensagem tem de virar FAILED agora.
// job.Attempts ja vem incrementado pelo claim (1 = primeira tentativa).
func isTerminalJobError(err error, job jobs.Job) bool {
	class := jobs.Classify(err)
	limit := class.MaxAttempts
	if job.MaxAttempts > 0 && job.MaxAttempts < limit {
		limit = job.MaxAttempts
	}
	return class.Unrecoverable || job.Attempts >= limit
}
