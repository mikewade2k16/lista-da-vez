package omnichannel

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// Servico do GET /media (spec F6 §3 / C4). Valida escopo, exclui as hidden_messages do usuario
// (=> 404), rehidrata a midia inbound sob demanda (one-shot, via provider.DownloadMedia) e abre
// o arquivo do disco para o handler fazer stream com Range. A midia vive em disco (D2) — jamais
// base64 no Postgres.

// openedMedia e o arquivo pronto para http.ServeContent (Range/206 de graca). O handler fecha o File.
type openedMedia struct {
	File     *os.File
	MimeType string
	FileName string
	ModTime  time.Time
	Size     int64
}

// MediaService orquestra a leitura da midia.
type MediaService struct {
	store     *Store
	scope     *Service
	media     *DiskMediaStorage
	registry  *channel.Registry
	secretBox *secretbox.Box
	publisher Publisher
	logger    *slog.Logger
}

// NewMediaService monta o service. publisher nil => no-op.
func NewMediaService(store *Store, media *DiskMediaStorage, registry *channel.Registry, box *secretbox.Box, publisher Publisher, logger *slog.Logger) *MediaService {
	if publisher == nil {
		publisher = noopPublisher{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &MediaService{
		store:     store,
		scope:     NewService(store),
		media:     media,
		registry:  registry,
		secretBox: box,
		publisher: publisher,
		logger:    logger,
	}
}

// OpenMedia valida escopo/permissao de leitura, resolve o descriptor (sem as hidden do usuario)
// e devolve o arquivo em disco. Rehidrata uma unica vez quando a midia ainda nao esta no disco
// (inbound com requiresMediaDecrypt / url_encrypted). Falha de rehidratacao => 404.
func (s *MediaService) OpenMedia(ctx context.Context, accountID string, caller Caller, conversationID, messageID string) (openedMedia, error) {
	if err := s.scope.assertConversationScope(ctx, accountID, caller, conversationID); err != nil {
		return openedMedia{}, err
	}
	d, err := s.store.GetMediaDescriptor(ctx, accountID, caller.UserID, conversationID, messageID)
	if err != nil {
		return openedMedia{}, translate(err) // hidden ou fora de escopo => 404
	}

	if mediaNeedsRehydration(d) {
		return openedMedia{}, ErrNotFound
	}
	if strings.TrimSpace(deref(d.StorageKey)) == "" {
		return openedMedia{}, ErrNotFound
	}

	file, info, err := s.media.Open(deref(d.StorageKey))
	if err != nil {
		return openedMedia{}, ErrNotFound
	}
	mime := deref(d.MimeType)
	if mime == "" {
		mime = "application/octet-stream"
	}
	return openedMedia{File: file, MimeType: mime, FileName: deref(d.FileName), ModTime: info.ModTime(), Size: info.Size()}, nil
}

// RetryMedia rearma exclusivamente o job de midia desta mensagem. Escopo e permissao sao
// checados antes; o Store repete account_id+conversation_id no lock e no update.
func (s *MediaService) RetryMedia(ctx context.Context, accountID string, principal auth.Principal, conversationID, messageID string) (MessageView, error) {
	caller := Caller{UserID: principal.UserID, IsAdmin: isAdminPrincipal(principal)}
	if err := s.scope.assertConversationScope(ctx, accountID, caller, conversationID); err != nil {
		return MessageView{}, err
	}
	if err := s.scope.requirePermission(ctx, accountID, principal, "omnichannel.conversations.reply"); err != nil {
		return MessageView{}, err
	}
	view, err := s.store.RetryMediaFetch(ctx, accountID, conversationID, messageID)
	if err != nil {
		return MessageView{}, translate(err)
	}
	s.publisher.PublishOmnichannelEvent(ctx, RealtimeEvent{
		Type:       RealtimeEventMessageUpdated,
		AccountID:  accountID,
		ResourceID: messageID,
		Payload:    messageViewPayload(view),
	})
	if err := s.store.InsertAudit(ctx, accountID, principal.UserID, conversationID, messageID, "MESSAGE_MEDIA_RETRY", nil); err != nil {
		s.logger.Error("omnichannel_media_audit", "account_id", accountID, "event", "MESSAGE_MEDIA_RETRY")
	}
	return view, nil
}

// ListAnalyses returns only derived metadata for a message in the caller's
// conversation scope. It never returns the signed stream token or storage path.
func (s *MediaService) ListAnalyses(ctx context.Context, accountID string, principal auth.Principal, conversationID, messageID string) ([]MediaAnalysisView, error) {
	caller := Caller{UserID: principal.UserID, IsAdmin: isAdminPrincipal(principal)}
	if err := s.scope.assertConversationScope(ctx, accountID, caller, conversationID); err != nil {
		return nil, err
	}
	if err := s.scope.requirePermission(ctx, accountID, principal, "omnichannel.conversations.view"); err != nil {
		return nil, err
	}
	if _, err := s.store.GetMediaDescriptor(ctx, accountID, principal.UserID, conversationID, messageID); err != nil {
		return nil, translate(err)
	}
	rows, err := s.store.ListMediaAnalyses(ctx, accountID, messageID)
	if err != nil {
		return nil, err
	}
	out := make([]MediaAnalysisView, 0, len(rows))
	for _, row := range rows {
		out = append(out, mediaAnalysisView(row))
	}
	return out, nil
}

// rehydrate baixa a midia pelo provider, grava em disco, persiste as colunas e emite
// message.updated COMPLETO (spec F4 §2: rehidratacao -> Message completo, SEM correlationId).
func (s *MediaService) rehydrate(ctx context.Context, accountID, conversationID, messageID string, d mediaDescriptor) error {
	provider, err := s.registry.Get(deref(d.Provider))
	if err != nil {
		return err
	}
	cred, err := s.resolveCredentials(ctx, accountID, deref(d.Provider))
	if err != nil {
		return err
	}
	rc, meta, err := provider.DownloadMedia(ctx, cred, channel.MediaRef{
		InstanceName:      d.InstanceScopeKey,
		ExternalMessageID: deref(d.ExternalMessageID),
		MediaURL:          deref(d.MediaURL),
	})
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	mime := meta.MimeType
	if mime == "" {
		mime = deref(d.MimeType)
	}
	stored, err := s.media.SaveReader(accountID, conversationID, mime, firstNonEmpty(meta.FileName, deref(d.FileName)), rc, defaultMaxMediaBytes)
	if err != nil {
		return err
	}
	if err := s.store.UpdateRehydratedMedia(ctx, accountID, conversationID, messageID, stored); err != nil {
		return err
	}
	if view, gErr := s.store.GetMessageByID(ctx, accountID, messageID); gErr == nil {
		s.publisher.PublishOmnichannelEvent(ctx, RealtimeEvent{
			Type:       RealtimeEventMessageUpdated,
			AccountID:  accountID,
			ResourceID: messageID,
			Payload:    messageViewPayload(view), // Message completo, sem correlationId
		})
	}
	return nil
}

// resolveCredentials decifra a credencial da instancia (mesma mecanica do inbound/outbound):
// sem ciphertext (mock) => Credentials vazio. Chave crua NUNCA em log.
func (s *MediaService) resolveCredentials(ctx context.Context, accountID, provider string) (channel.Credentials, error) {
	cipher, config, found, err := s.store.FindProviderCredential(ctx, accountID, provider)
	if err != nil {
		return channel.Credentials{}, err
	}
	cred := channel.Credentials{Config: config}
	if !found || strings.TrimSpace(cipher) == "" || s.secretBox == nil {
		return cred, nil
	}
	token, err := s.secretBox.Decrypt(cipher)
	if err != nil {
		s.logger.Error("omnichannel_credential_decrypt_failed", "account_id", accountID, "provider", provider)
		return channel.Credentials{}, err
	}
	cred.Token = token
	return cred, nil
}

// mediaNeedsRehydration decide se a midia ainda nao esta pronta no disco: sem storage_key, OU
// media_source_kind = url_encrypted, OU metadata.requiresMediaDecrypt = true (spec F5 §3).
func mediaNeedsRehydration(d mediaDescriptor) bool {
	if strings.TrimSpace(deref(d.StorageKey)) == "" {
		return true
	}
	if strings.EqualFold(deref(d.SourceKind), "url_encrypted") {
		return true
	}
	return metadataRequiresDecrypt(d.Metadata)
}

// metadataRequiresDecrypt le o flag requiresMediaDecrypt do metadata_json (defensivo).
func metadataRequiresDecrypt(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	v, ok := m["requiresMediaDecrypt"].(bool)
	return ok && v
}
