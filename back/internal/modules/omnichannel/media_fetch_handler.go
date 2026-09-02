package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type mediaFetchJobPayload struct {
	MessageID string `json:"messageId"`
}

type mediaFetchStore interface {
	GetMediaFetchData(ctx context.Context, accountID, messageID string) (mediaFetchData, error)
	IsMessageHistoryVisible(ctx context.Context, accountID, conversationID, messageID string) (bool, error)
	WithMessageExternalEffectLease(ctx context.Context, accountID, conversationID, messageID string, effect func() error) (bool, error)
	UpdateFetchedMedia(ctx context.Context, accountID, conversationID, messageID string, media StoredMedia) (MessageView, error)
	MarkMediaFetchFailed(ctx context.Context, accountID, conversationID, messageID, code string) (MessageView, error)
	InsertAudit(ctx context.Context, accountID, actorUserID, conversationID, messageID, eventType string, payload json.RawMessage) error
}

type mediaMessageAnalyzer interface {
	AnalyzeMessage(context.Context, string, string) error
}

// MediaFetchHandler baixa midia inbound fora do webhook. O job carrega somente messageId;
// instancia, credencial, limites e referencias sao sempre relidos do PostgreSQL.
type MediaFetchHandler struct {
	store     mediaFetchStore
	media     *DiskMediaStorage
	registry  *channel.Registry
	secretBox *secretbox.Box
	publisher Publisher
	logger    *slog.Logger
	analyzer  mediaMessageAnalyzer
}

func NewMediaFetchHandler(store mediaFetchStore, media *DiskMediaStorage, registry *channel.Registry, box *secretbox.Box, publisher Publisher, logger *slog.Logger, analyzers ...mediaMessageAnalyzer) *MediaFetchHandler {
	if publisher == nil {
		publisher = noopPublisher{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	var analyzer mediaMessageAnalyzer
	if len(analyzers) > 0 {
		analyzer = analyzers[0]
	}
	return &MediaFetchHandler{store: store, media: media, registry: registry, secretBox: box, publisher: publisher, logger: logger, analyzer: analyzer}
}

func (h *MediaFetchHandler) Handle(ctx context.Context, job jobs.Job) error {
	var payload mediaFetchJobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil || strings.TrimSpace(payload.MessageID) == "" {
		return &jobs.StatusError{Unrecoverable: true, Err: errors.New("media: payload invalido")}
	}

	data, err := h.store.GetMediaFetchData(ctx, job.AccountID, payload.MessageID)
	if isMissingRow(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if data.StorageKey != "" && data.SourceKind == "disk" {
		if h.analyzer != nil {
			if err := h.ensureHistoryVisible(ctx, job.AccountID, data); err != nil {
				return err
			}
			return h.analyzer.AnalyzeMessage(ctx, job.AccountID, data.MessageID)
		}
		return nil
	}
	if strings.EqualFold(data.MessageType, "TEXT") {
		return h.fail(ctx, job, data, &jobs.StatusError{Unrecoverable: true, Err: errors.New("media: mensagem sem midia")}, "invalid_media")
	}
	if data.Provider == "" || data.InstanceScopeKey == "" || data.ExternalMessageID == "" {
		return h.fail(ctx, job, data, &jobs.StatusError{Unrecoverable: true, Err: errors.New("media: configuracao ausente")}, "configuration_error")
	}

	provider, err := h.registry.Get(data.Provider)
	if err != nil {
		return h.fail(ctx, job, data, &jobs.StatusError{Unrecoverable: true, Err: errors.New("media: provider sem adapter")}, "configuration_error")
	}
	credentials, err := h.credentials(data)
	if err != nil {
		return h.fail(ctx, job, data, err, "configuration_error")
	}
	var providerErr, storageErr, persistErr error
	allowed, leaseErr := h.store.WithMessageExternalEffectLease(ctx, job.AccountID, data.ConversationID, data.MessageID, func() error {
		reader, meta, downloadErr := provider.DownloadMedia(ctx, credentials, channel.MediaRef{
			InstanceName:      data.InstanceScopeKey,
			ExternalMessageID: data.ExternalMessageID,
			MediaURL:          data.MediaURL,
		})
		if downloadErr != nil {
			providerErr = downloadErr
			return nil
		}
		defer func() { _ = reader.Close() }()
		mimeType := firstNonEmpty(meta.MimeType, data.MimeType)
		fileName := firstNonEmpty(meta.FileName, data.FileName)
		maxBytes := mediaFetchLimit(data.MaxBytes, provider.Capabilities().MaxMediaBytes)
		stored, saveErr := h.media.SaveInboundReader(job.AccountID, data.ConversationID, data.MessageID,
			mimeType, fileName, reader, maxBytes)
		if saveErr != nil {
			storageErr = saveErr
			return nil
		}
		view, updateErr := h.store.UpdateFetchedMedia(ctx, job.AccountID, data.ConversationID, data.MessageID, stored)
		if updateErr != nil {
			persistErr = updateErr
			return nil
		}
		h.publish(ctx, job.AccountID, view)
		if auditErr := h.store.InsertAudit(ctx, job.AccountID, "", data.ConversationID, data.MessageID, "MESSAGE_MEDIA_READY", nil); auditErr != nil {
			h.logger.Error("omnichannel_media_audit", "account_id", job.AccountID, "event", "MESSAGE_MEDIA_READY")
		}
		return nil
	})
	if leaseErr != nil {
		return leaseErr
	}
	if !allowed {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrHistoryResetInvalidated}
	}
	if providerErr != nil {
		code, classified := classifyMediaProviderError(providerErr)
		return h.fail(ctx, job, data, classified, code)
	}
	if storageErr != nil {
		code, classified := classifyMediaStorageError(storageErr)
		return h.fail(ctx, job, data, classified, code)
	}
	if persistErr != nil {
		return persistErr
	}
	if h.analyzer != nil {
		return h.analyzer.AnalyzeMessage(ctx, job.AccountID, data.MessageID)
	}
	return nil
}

func (h *MediaFetchHandler) ensureHistoryVisible(ctx context.Context, accountID string, data mediaFetchData) error {
	visible, err := h.store.IsMessageHistoryVisible(ctx, accountID, data.ConversationID, data.MessageID)
	if err != nil {
		return err
	}
	if !visible {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrHistoryResetInvalidated}
	}
	return nil
}

func (h *MediaFetchHandler) credentials(data mediaFetchData) (channel.Credentials, error) {
	credentials := channel.Credentials{Config: data.ProviderConfig}
	if data.CredentialCiphertext == "" || h.secretBox == nil {
		return credentials, nil
	}
	token, err := h.secretBox.Decrypt(data.CredentialCiphertext)
	if err != nil {
		h.logger.Error("omnichannel_credential_decrypt_failed", "provider", data.Provider)
		return channel.Credentials{}, &jobs.StatusError{Unrecoverable: true, Err: errors.New("media: credencial invalida")}
	}
	credentials.Token = token
	return credentials, nil
}

func (h *MediaFetchHandler) fail(ctx context.Context, job jobs.Job, data mediaFetchData, jobErr error, code string) error {
	if !isTerminalJobError(jobErr, job) {
		return jobErr
	}
	if visibilityErr := h.ensureHistoryVisible(ctx, job.AccountID, data); visibilityErr != nil {
		return visibilityErr
	}
	view, err := h.store.MarkMediaFetchFailed(ctx, job.AccountID, data.ConversationID, data.MessageID, code)
	if err != nil {
		h.logger.Error("omnichannel_media_mark_failed", "account_id", job.AccountID, "message_id", data.MessageID)
		return jobErr
	}
	h.publish(ctx, job.AccountID, view)
	if err := h.store.InsertAudit(ctx, job.AccountID, "", data.ConversationID, data.MessageID, "MESSAGE_MEDIA_FAILED", nil); err != nil {
		h.logger.Error("omnichannel_media_audit", "account_id", job.AccountID, "event", "MESSAGE_MEDIA_FAILED")
	}
	return jobErr
}

func (h *MediaFetchHandler) publish(ctx context.Context, accountID string, view MessageView) {
	h.publisher.PublishOmnichannelEvent(ctx, RealtimeEvent{
		Type:       RealtimeEventMessageUpdated,
		AccountID:  accountID,
		ResourceID: view.ID,
		Payload:    messageViewPayload(view),
	})
}

func mediaFetchLimit(accountLimit, providerLimit int64) int64 {
	if accountLimit <= 0 {
		accountLimit = defaultMaxMediaBytes
	}
	if providerLimit > 0 && providerLimit < accountLimit {
		return providerLimit
	}
	return accountLimit
}

func classifyMediaProviderError(err error) (string, error) {
	status, ok := channel.ErrorHTTPStatus(err)
	if !ok {
		return "provider_unavailable", &jobs.StatusError{Err: errors.New("media: provider indisponivel")}
	}
	switch {
	case status == 401:
		return "unauthorized", &jobs.StatusError{StatusCode: status, Err: errors.New("media: provider recusou credencial")}
	case status == 403:
		return "forbidden", &jobs.StatusError{StatusCode: status, Err: errors.New("media: provider recusou acesso")}
	case status == 404:
		// A Evolution pode ainda nao ter materializado a midia quando o webhook chega.
		return "provider_not_ready", &jobs.StatusError{Err: errors.New("media: provider ainda nao disponibilizou arquivo")}
	case status == 429:
		return "rate_limited", &jobs.StatusError{StatusCode: status, Err: errors.New("media: provider limitou requisicao")}
	case status >= 500:
		return "provider_unavailable", &jobs.StatusError{StatusCode: status, Err: errors.New("media: provider indisponivel")}
	case status == 400 || status == 422:
		return "invalid_media", &jobs.StatusError{StatusCode: status, Unrecoverable: true, Err: errors.New("media: referencia rejeitada")}
	default:
		return "download_failed", &jobs.StatusError{StatusCode: status, Err: errors.New("media: download rejeitado")}
	}
}

func classifyMediaStorageError(err error) (string, error) {
	switch {
	case errors.Is(err, ErrMediaUnsupported):
		return "unsupported_media", &jobs.StatusError{Unrecoverable: true, Err: errors.New("media: mime nao suportado")}
	case errors.Is(err, ErrMediaTooLarge):
		return "media_too_large", &jobs.StatusError{Unrecoverable: true, Err: errors.New("media: limite excedido")}
	case errors.Is(err, ErrMediaInvalid):
		return "invalid_media", &jobs.StatusError{Unrecoverable: true, Err: errors.New("media: arquivo invalido")}
	default:
		return "storage_error", &jobs.StatusError{Err: errors.New("media: falha no storage")}
	}
}
