package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// mediaFetchData contem somente o necessario para o job reler a mensagem e a instancia
// autoritativas. Credencial continua cifrada ate o handler.
type mediaFetchData struct {
	MessageID            string
	ConversationID       string
	InstanceScopeKey     string
	ExternalMessageID    string
	MessageType          string
	MediaURL             string
	MimeType             string
	FileName             string
	StorageKey           string
	SourceKind           string
	Provider             string
	CredentialCiphertext string
	ProviderConfig       map[string]string
	MaxBytes             int64
}

// GetMediaFetchData resolve mensagem, instancia exata e limite da conta. Todas as juncoes
// repetem account_id; id de outra conta resulta em pgx.ErrNoRows.
func (s *Store) GetMediaFetchData(ctx context.Context, accountID, messageID string) (mediaFetchData, error) {
	var d mediaFetchData
	var rawConfig []byte
	var maxUploadMB int
	err := s.pool.QueryRow(ctx, `select m.id::text, m.conversation_id::text,
			m.instance_scope_key, coalesce(m.external_message_id, ''), m.message_type,
			coalesce(m.media_url, ''), coalesce(m.media_mime_type, ''),
			coalesce(m.media_file_name, ''), coalesce(m.media_storage_key, ''),
			coalesce(m.media_source_kind, ''), coalesce(i.provider, ''),
			coalesce(i.credentials_ciphertext, ''), coalesce(i.provider_config, '{}'::jsonb),
			coalesce(ac.max_upload_mb, 500)
		from messaging.messages m
		join messaging.conversations c
		  on c.id = m.conversation_id and c.account_id = m.account_id
		left join messaging.whatsapp_instances i
		  on i.id = m.instance_id and i.account_id = m.account_id
		left join messaging.account_config ac on ac.account_id = m.account_id
		where m.account_id = $1::uuid and m.id = $2::uuid`+
		s.historyVisibleMessagePredicate("m", "c"), accountID, messageID).Scan(
		&d.MessageID, &d.ConversationID, &d.InstanceScopeKey, &d.ExternalMessageID,
		&d.MessageType, &d.MediaURL, &d.MimeType, &d.FileName, &d.StorageKey,
		&d.SourceKind, &d.Provider, &d.CredentialCiphertext, &rawConfig, &maxUploadMB)
	if err != nil {
		return mediaFetchData{}, err
	}
	d.ProviderConfig = decodeStringMap(rawConfig)
	d.MaxBytes = int64(maxUploadMB) << 20
	return d, nil
}

// UpdateFetchedMedia publica no banco somente depois do arquivo estar pronto. Remove flags
// transitórias e persiste apenas hash/estado seguros no metadata.
func (s *Store) UpdateFetchedMedia(ctx context.Context, accountID, conversationID, messageID string, media StoredMedia) (MessageView, error) {
	result, err := s.pool.Exec(ctx, `update messaging.messages
		set media_storage_key = $4, media_source_kind = 'disk', media_mime_type = $5,
			media_file_name = coalesce(nullif($6, ''), media_file_name),
			media_file_size_bytes = $7, media_url = $8,
			metadata_json = (coalesce(metadata_json, '{}'::jsonb)
			  - 'requiresMediaDecrypt' - 'mediaErrorCode') ||
			  jsonb_build_object('mediaState', 'ready', 'mediaSha256', $9::text),
			updated_at = now()
		where account_id = $1::uuid and conversation_id = $2::uuid and id = $3::uuid`,
		accountID, conversationID, messageID, media.StorageKey, media.MimeType,
		media.FileName, int(media.SizeBytes), mediaEndpointPath(conversationID, messageID), media.SHA256)
	if err != nil {
		return MessageView{}, err
	}
	if result.RowsAffected() != 1 {
		return MessageView{}, pgx.ErrNoRows
	}
	return s.GetMessageByID(ctx, accountID, messageID)
}

// MarkMediaFetchFailed mantem a mensagem visivel e grava somente um codigo enumerado seguro.
func (s *Store) MarkMediaFetchFailed(ctx context.Context, accountID, conversationID, messageID, code string) (MessageView, error) {
	result, err := s.pool.Exec(ctx, `update messaging.messages
		set metadata_json = (coalesce(metadata_json, '{}'::jsonb) - 'requiresMediaDecrypt') ||
			jsonb_build_object('mediaState', 'failed', 'mediaErrorCode', $4::text),
			updated_at = now()
		where account_id = $1::uuid and conversation_id = $2::uuid and id = $3::uuid`,
		accountID, conversationID, messageID, safeMediaErrorCode(code))
	if err != nil {
		return MessageView{}, err
	}
	if result.RowsAffected() != 1 {
		return MessageView{}, pgx.ErrNoRows
	}
	return s.GetMessageByID(ctx, accountID, messageID)
}

// RetryMediaFetch rearma apenas o job desta mensagem e desta conta. Jobs de outros módulos
// ou outros kinds nunca entram no predicado.
func (s *Store) RetryMediaFetch(ctx context.Context, accountID, conversationID, messageID string) (MessageView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return MessageView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var messageType, storageKey, sourceKind string
	err = tx.QueryRow(ctx, `select m.message_type, coalesce(m.media_storage_key, ''),
			coalesce(m.media_source_kind, '')
		from messaging.messages m
		join messaging.conversations c on c.account_id=m.account_id and c.id=m.conversation_id
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid and m.id = $3::uuid`+
		s.historyVisibleMessagePredicate("m", "c")+`
		for update of m`, accountID, conversationID, messageID).Scan(&messageType, &storageKey, &sourceKind)
	if err != nil {
		return MessageView{}, err
	}
	if strings.EqualFold(messageType, "TEXT") {
		return MessageView{}, ErrMediaInvalid
	}
	if storageKey != "" && sourceKind == "disk" {
		if err := tx.Commit(ctx); err != nil {
			return MessageView{}, err
		}
		return s.GetMessageByID(ctx, accountID, messageID)
	}

	_, err = tx.Exec(ctx, `update messaging.messages
		set metadata_json = (coalesce(metadata_json, '{}'::jsonb) - 'mediaErrorCode') ||
			jsonb_build_object('mediaState', 'pending'), updated_at = now()
		where account_id = $1::uuid and conversation_id = $2::uuid and id = $3::uuid`,
		accountID, conversationID, messageID)
	if err != nil {
		return MessageView{}, err
	}

	reset, err := tx.Exec(ctx, `update messaging.outbox
		set status = 'pending', attempts = 0, max_attempts = 5, run_after = now(),
			locked_at = null, locked_by = '', last_error = '', updated_at = now()
		where account_id = $1::uuid and idempotency_key = $2 and kind = $3
		  and status in ('done', 'failed', 'dead')`, accountID, "media-fetch:"+messageID, MediaFetchJobKind)
	if err != nil {
		return MessageView{}, err
	}
	if reset.RowsAffected() == 0 {
		payload, _ := json.Marshal(mediaFetchJobPayload{MessageID: messageID})
		_, err = tx.Exec(ctx, `insert into messaging.outbox
			(account_id, ordering_key, idempotency_key, kind, payload, max_attempts)
			values ($1::uuid, $2, $3, $4, $5, 5)
			on conflict (account_id, idempotency_key) do nothing`, accountID, conversationID,
			"media-fetch:"+messageID, MediaFetchJobKind, payload)
		if err != nil {
			return MessageView{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return MessageView{}, err
	}
	return s.GetMessageByID(ctx, accountID, messageID)
}

func safeMediaErrorCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	for _, allowed := range []string{
		"unauthorized", "forbidden", "provider_not_ready", "rate_limited",
		"provider_unavailable", "invalid_media", "unsupported_media", "media_too_large",
		"storage_error", "download_failed", "configuration_error",
	} {
		if code == allowed {
			return code
		}
	}
	return "download_failed"
}

func isMissingRow(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
