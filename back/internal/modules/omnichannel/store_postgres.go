package omnichannel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store e a persistencia do modulo (schema messaging.*).
//
// REGRA DA CASA, sem excecao: TODA query filtra por account_id — inclusive as que
// recebem um id ja validado no service (defesa em profundidade, principio 2). IDs sao
// string + cast no SQL ($1::uuid); nao importamos pacote de uuid (padrao da casa).
type Store struct {
	pool         *pgxpool.Pool
	aiDispatchV2 atomic.Bool
}

// NewStore cria o Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type rowScanner interface {
	Scan(dest ...any) error
}

// ============================================================================
// Conversas
// ============================================================================

// conversationCols sai na ordem esperada por scanConversation. Os uuid saem como text
// (::text) para scanear em string/*string sem pacote de uuid. instance_name/display_name
// vem do LEFT JOIN na instancia (alias i) — exige o alias `c` na tabela de conversas.
const conversationCols = `c.id::text, c.instance_id::text, c.instance_scope_key,
	i.instance_name, i.display_name, c.channel, c.state, c.external_id, c.contact_id::text,
	c.contact_name, c.contact_avatar_url, c.contact_phone, c.assigned_to_id,
	c.created_at, c.updated_at, c.last_message_at`

// lastMessageCol resolve o preview da ultima mensagem como subquery ESCALAR de um jsonb
// (uma linha por conversa, sem multiplicar no JOIN), amarrada a MESMA account (defesa em
// profundidade: nunca mostra mensagem de outra conta). NULL = conversa sem mensagem.
// Usa o indice messaging_messages_conversation_created_idx — sem N+1.
const lastMessageCol = `,
	(select jsonb_build_object(
		'id', m.id::text, 'content', m.content, 'messageType', m.message_type,
		'mediaUrl', m.media_url, 'direction', m.direction, 'status', m.status,
		'createdAt', m.created_at)
	 from messaging.messages m
	 where m.conversation_id = c.id and m.account_id = c.account_id
	   and m.created_at > coalesce((select suppression.history_cleared_at
	       from messaging.contact_suppressions suppression
	       where suppression.account_id=c.account_id and suppression.contact_id=c.contact_id),
	       '-infinity'::timestamptz)
	 order by m.created_at desc, m.id desc
	 limit 1)`

const visibleConversationFilter = ` and not exists (
	select 1 from messaging.contact_suppressions suppression
	where suppression.account_id=c.account_id and suppression.contact_id=c.contact_id
	  and suppression.is_hidden=true)`

// conversationRow e a linha crua da conversa (colunas do banco, antes da projecao).
type conversationRow struct {
	ID                  string
	InstanceID          *string
	InstanceScopeKey    string
	InstanceName        *string
	InstanceDisplayName *string
	Channel             string
	State               string
	ExternalID          string
	ContactID           *string
	ContactName         *string
	ContactAvatarURL    *string
	ContactPhone        *string
	AssignedToID        *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LastMessageAt       time.Time
	// LastMessage vem do lastMessageCol como jsonb cru (padrao da casa: jsonb -> RawMessage,
	// nunca scan direto em struct). NULL/vazio = conversa sem mensagem. Quem desserializa
	// para LastMessageView e o service (view builder).
	LastMessage json.RawMessage
}

func scanConversation(row rowScanner) (conversationRow, error) {
	var c conversationRow
	err := row.Scan(&c.ID, &c.InstanceID, &c.InstanceScopeKey, &c.InstanceName,
		&c.InstanceDisplayName, &c.Channel, &c.State, &c.ExternalID, &c.ContactID,
		&c.ContactName, &c.ContactAvatarURL, &c.ContactPhone, &c.AssignedToID,
		&c.CreatedAt, &c.UpdatedAt, &c.LastMessageAt, &c.LastMessage)
	return c, err
}

// ConversationFilter filtra a listagem do inbox.
type ConversationFilter struct {
	ConversationPageFilter
	// ScopeKeys restringe as conversas as instancias que o usuario pode ver (A2:
	// filtro por responsible_user_id). nil = sem restricao (admin).
	ScopeKeys []string
}

// ListConversations devolve as conversas da account ordenadas por last_message_at DESC.
// SEM paginacao — o contrato do legado (e do front) e a lista inteira.
type conversationCursor struct {
	LastMessageAt time.Time
	ID            string
}

func encodeConversationCursor(lastMessageAt time.Time, id string) string {
	raw := lastMessageAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeConversationCursor(raw string) (conversationCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return conversationCursor{}, ErrInvalidBody
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || !omnichannelUUIDPattern.MatchString(parts[1]) {
		return conversationCursor{}, ErrInvalidBody
	}
	lastMessageAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return conversationCursor{}, ErrInvalidBody
	}
	return conversationCursor{LastMessageAt: lastMessageAt, ID: parts[1]}, nil
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func (s *Store) ListConversations(ctx context.Context, accountID string, f ConversationFilter) ([]conversationRow, error) {
	query := `select ` + conversationCols + lastMessageCol + `
		from messaging.conversations c
		left join messaging.whatsapp_instances i
			on i.id = c.instance_id and i.account_id = c.account_id
		where c.account_id = $1::uuid` + visibleConversationFilter
	args := []any{accountID}

	if strings.TrimSpace(f.InstanceID) != "" {
		args = append(args, strings.TrimSpace(f.InstanceID))
		query += " and c.instance_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	if f.ScopeKeys != nil {
		args = append(args, f.ScopeKeys)
		query += " and c.instance_scope_key = any($" + strconv.Itoa(len(args)) + "::text[])"
	}
	if f.Channel != "" {
		args = append(args, f.Channel)
		query += " and c.channel = $" + strconv.Itoa(len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		query += ` and case
			when c.state = 'closed' then 'CLOSED'
			when c.state = 'pending' then 'PENDING'
			else 'OPEN' end = $` + strconv.Itoa(len(args))
	}
	if f.QueueID != "" {
		args = append(args, f.QueueID)
		query += " and c.queue_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	if f.ResponsibleID != "" {
		args = append(args, f.ResponsibleID)
		query += " and coalesce(c.assigned_user_id::text, c.assigned_to_id) = $" + strconv.Itoa(len(args))
	}
	if f.Search != "" {
		args = append(args, "%"+escapeLike(strings.ToLower(f.Search))+"%")
		position := strconv.Itoa(len(args))
		query += ` and (lower(coalesce(c.contact_name, '')) like $` + position + ` escape '\'
			or lower(coalesce(c.contact_phone, '')) like $` + position + ` escape '\'
			or lower(c.external_id) like $` + position + ` escape '\'
			or exists (select 1 from messaging.messages sm
				where sm.account_id = c.account_id and sm.conversation_id = c.id
				  and sm.created_at > coalesce((select suppression.history_cleared_at
				      from messaging.contact_suppressions suppression
				      where suppression.account_id=c.account_id and suppression.contact_id=c.contact_id),
				      '-infinity'::timestamptz)
				  and lower(sm.content) like $` + position + ` escape '\'))`
	}
	if f.BeforeCursor != "" {
		cursor, err := decodeConversationCursor(f.BeforeCursor)
		if err != nil {
			return nil, err
		}
		args = append(args, cursor.LastMessageAt, cursor.ID)
		query += " and (c.last_message_at, c.id) < ($" + strconv.Itoa(len(args)-1) + ", $" + strconv.Itoa(len(args)) + "::uuid)"
	}
	args = append(args, f.Limit)
	query += " order by c.last_message_at desc, c.id desc limit $" + strconv.Itoa(len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]conversationRow, 0)
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) UpdateConversationGroupName(ctx context.Context, accountID, instanceName, externalID, name string) error {
	_, err := s.pool.Exec(ctx, `update messaging.conversations
		set contact_name=$4, updated_at=now()
		where account_id=$1::uuid and instance_scope_key=$2 and external_id=$3
		  and channel='WHATSAPP' and lower(trim(external_id)) like '%@g.us'`,
		accountID, strings.TrimSpace(instanceName), strings.TrimSpace(externalID), strings.TrimSpace(name))
	return err
}

// GetConversation devolve uma conversa da account. Conversa de OUTRA account cai no
// filtro e volta pgx.ErrNoRows -> o service traduz para ErrNotFound (404, nunca 403).
func (s *Store) GetConversation(ctx context.Context, accountID, id string) (conversationRow, error) {
	query := `select ` + conversationCols + lastMessageCol + `
		from messaging.conversations c
		left join messaging.whatsapp_instances i
			on i.id = c.instance_id and i.account_id = c.account_id
		where c.account_id = $1::uuid and c.id = $2::uuid`
	return scanConversation(s.pool.QueryRow(ctx, query, accountID, id))
}

// ============================================================================
// Mensagens
// ============================================================================

const messageCols = `m.id::text, m.account_id::text, m.conversation_id::text,
	m.sender_user_id::text, m.direction, m.message_type, m.sender_name, m.sender_avatar_url,
	m.content, m.media_url, m.media_mime_type, m.media_file_name, m.media_file_size_bytes,
	m.media_caption, m.media_duration_seconds, m.metadata_json, m.status,
	m.origin, m.reply_to_message_id::text, m.reply_to_external_message_id,
	(select r.sender_name from messaging.messages r where r.id = m.reply_to_message_id
	  and r.account_id = m.account_id and r.conversation_id = m.conversation_id),
	(select r.content from messaging.messages r where r.id = m.reply_to_message_id
	  and r.account_id = m.account_id and r.conversation_id = m.conversation_id),
	(select r.message_type from messaging.messages r where r.id = m.reply_to_message_id
	  and r.account_id = m.account_id and r.conversation_id = m.conversation_id),
	m.provider_status_at, m.provider_error_code,
	case when m.message_type = 'TEXT' then ''
	  when m.media_storage_key is not null and m.media_source_kind = 'disk' then 'ready'
	  else coalesce(m.metadata_json->>'mediaState', 'pending') end,
	m.external_message_id, m.created_at, m.updated_at`

func scanMessage(row rowScanner) (MessageView, error) {
	var m MessageView
	var replyMessageID, replyExternalID, replySender, replyContent, replyType *string
	err := row.Scan(&m.ID, &m.TenantID, &m.ConversationID, &m.SenderUserID, &m.Direction,
		&m.MessageType, &m.SenderName, &m.SenderAvatarURL, &m.Content, &m.MediaURL,
		&m.MediaMimeType, &m.MediaFileName, &m.MediaFileSizeBytes, &m.MediaCaption,
		&m.MediaDurationSeconds, &m.MetadataJSON, &m.Status, &m.Origin, &replyMessageID,
		&replyExternalID, &replySender, &replyContent, &replyType,
		&m.ProviderStatusAt, &m.ProviderErrorCode, &m.MediaState, &m.ExternalMessageID,
		&m.CreatedAt, &m.UpdatedAt)
	if err == nil && (replyMessageID != nil || replyExternalID != nil) {
		m.ReplyTo = &ReplyToView{
			MessageID:         replyMessageID,
			ExternalMessageID: deref(replyExternalID),
			SenderName:        deref(replySender),
			Content:           deref(replyContent),
			MessageType:       deref(replyType),
		}
		fillReplySnapshot(&m)
	}
	m.CanRetryMedia = m.MediaState == "failed"
	return m, err
}

func fillReplySnapshot(m *MessageView) {
	if m == nil || m.ReplyTo == nil || len(m.MetadataJSON) == 0 {
		return
	}
	var metadata struct {
		ReplySnapshot struct {
			Content     string `json:"content"`
			MessageType string `json:"messageType"`
			Participant string `json:"participant"`
		} `json:"replySnapshot"`
	}
	if json.Unmarshal(m.MetadataJSON, &metadata) != nil {
		return
	}
	if m.ReplyTo.Content == "" {
		m.ReplyTo.Content = metadata.ReplySnapshot.Content
	}
	if m.ReplyTo.MessageType == "" {
		m.ReplyTo.MessageType = metadata.ReplySnapshot.MessageType
	}
	if m.ReplyTo.SenderName == "" {
		m.ReplyTo.SenderName = metadata.ReplySnapshot.Participant
	}
}

// hiddenFilter exclui as mensagens que o usuario apagou "para mim"
// (messaging.hidden_messages). O legado aplica isso na listagem do historico.
const hiddenFilter = ` and not exists (
	select 1 from messaging.hidden_messages h
	where h.message_id = m.id and h.user_id = $3::uuid)`

const clearedHistoryFilter = ` and m.created_at > coalesce((
	select suppression.history_cleared_at
	from messaging.conversations privacy_conversation
	join messaging.contact_suppressions suppression
	  on suppression.account_id=privacy_conversation.account_id
	 and suppression.contact_id=privacy_conversation.contact_id
	where privacy_conversation.account_id=m.account_id
	  and privacy_conversation.id=m.conversation_id
),'-infinity'::timestamptz)`

type messageCursor struct {
	CreatedAt time.Time
	ID        string
}

func encodeMessageCursor(createdAt time.Time, id string) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeMessageCursor(raw string) (messageCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return messageCursor{}, ErrInvalidBody
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return messageCursor{}, ErrInvalidBody
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return messageCursor{}, ErrInvalidBody
	}
	return messageCursor{CreatedAt: createdAt, ID: parts[1]}, nil
}

// resolveBeforeMessage traduz o beforeId legado para o cursor estavel (created_at,id).
// O id permanece preso a account+conversa, logo um id de outro tenant nunca e cursor valido.
func (s *Store) resolveBeforeMessage(ctx context.Context, accountID, conversationID, beforeID string) (*messageCursor, error) {
	var cursor messageCursor
	err := s.pool.QueryRow(ctx, `select m.created_at, m.id::text from messaging.messages m
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid and m.id = $3::uuid`,
		accountID, conversationID, beforeID).Scan(&cursor.CreatedAt, &cursor.ID)
	switch {
	// beforeId inexistente (ou de outra conta/conversa) => sem filtro de data: a pagina
	// volta como se nao houvesse cursor. E o comportamento do legado (beforeMessage null).
	case errors.Is(err, pgx.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return &cursor, nil
	}
}

// ListMessages devolve a pagina do historico em ordem ASC (a mais antiga primeiro).
//
// O algoritmo e o do legado, replicado EXATO (divergir quebra o scroll infinito):
// resolve beforeId -> created_at, filtra created_at <, ordena DESC, take limit e
// INVERTE o array. Ver routes-message-read-list.ts:97-170.
func (s *Store) ListMessages(ctx context.Context, accountID, userID, conversationID string, f MessagePageFilter) ([]MessageView, error) {
	query := `select ` + messageCols + ` from messaging.messages m
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid` + hiddenFilter + clearedHistoryFilter
	args := []any{accountID, conversationID, userID}

	var before *messageCursor
	if strings.TrimSpace(f.BeforeCursor) != "" {
		decoded, err := decodeMessageCursor(f.BeforeCursor)
		if err != nil {
			return nil, err
		}
		before = &decoded
	} else if strings.TrimSpace(f.BeforeID) != "" {
		resolved, err := s.resolveBeforeMessage(ctx, accountID, conversationID, f.BeforeID)
		if err != nil {
			return nil, err
		}
		before = resolved
	}
	if before != nil {
		args = append(args, before.CreatedAt, before.ID)
		query += " and (m.created_at, m.id) < ($" + strconv.Itoa(len(args)-1) + ", $" + strconv.Itoa(len(args)) + "::uuid)"
	}
	args = append(args, f.Limit)
	query += " order by m.created_at desc, m.id desc limit $" + strconv.Itoa(len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	desc := make([]MessageView, 0, f.Limit)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		desc = append(desc, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Inverte para ASC: o front espera a mais antiga primeiro.
	out := make([]MessageView, 0, len(desc))
	for i := len(desc) - 1; i >= 0; i-- {
		out = append(out, desc[i])
	}
	return out, nil
}

// HasOlderMessage responde o `hasMore`: existe mensagem MAIS ANTIGA que a primeira da
// pagina? Nao conta as escondidas do usuario — senao o front pediria uma pagina que
// voltaria vazia e o scroll infinito travaria.
func (s *Store) HasOlderMessage(ctx context.Context, accountID, userID, conversationID string, oldest time.Time, oldestID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `select exists (
		select 1 from messaging.messages m
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid
		  and (m.created_at, m.id) < ($4, $5::uuid)
		  and not exists (select 1 from messaging.hidden_messages h
			where h.message_id = m.id and h.user_id = $3::uuid)`+clearedHistoryFilter+`)`,
		accountID, conversationID, userID, oldest, oldestID).Scan(&exists)
	return exists, err
}

// GetMessage devolve uma mensagem da conversa. Filtra por account E conversa: id de
// outra conta (ou de outra conversa) volta pgx.ErrNoRows -> 404.
func (s *Store) GetMessage(ctx context.Context, accountID, userID, conversationID, messageID string) (MessageView, error) {
	query := `select ` + messageCols + ` from messaging.messages m
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid` + hiddenFilter + clearedHistoryFilter +
		` and m.id = $4::uuid`
	return scanMessage(s.pool.QueryRow(ctx, query, accountID, conversationID, userID, messageID))
}
