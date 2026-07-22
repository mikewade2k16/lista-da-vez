package omnichannel

import (
	"encoding/json"
	"errors"
	"time"
)

// ============================================================================
// Erros do dominio (mapeados para HTTP em http.go)
// ============================================================================

var (
	// ErrNotFound cobre TAMBEM "existe, mas e de outra conta" — de proposito: fora de
	// escopo responde 404, nunca 403 (403 confirma que o recurso existe = enumeration).
	ErrNotFound      = errors.New("omnichannel: not found")
	ErrInvalidBody   = errors.New("omnichannel: invalid body")
	ErrInvalidPhone  = errors.New("omnichannel: invalid phone")
	ErrInvalidLimit  = errors.New("omnichannel: invalid limit")
	ErrPhoneConflict = errors.New("omnichannel: phone already registered")
	ErrForbidden     = errors.New("omnichannel: forbidden")
	// ErrUnauthorized: falha de autenticidade do webhook (token/assinatura invalido). 401.
	ErrUnauthorized = errors.New("omnichannel: unauthorized")
	// ErrNumberInUse: o numero ja esta em outra instancia da MESMA conta (C6). 409, com a
	// instancia que o usa nomeada no handler (feedback acionavel).
	ErrNumberInUse = errors.New("omnichannel: phone number already used by another instance")
	// ErrInstanceNameConflict: ja existe uma instancia com o mesmo instance_name na conta
	// (indice unico (account_id, instance_name) da 0200). 409 acionavel no handler.
	ErrInstanceNameConflict = errors.New("omnichannel: instance name already registered")
	// ErrChannelLimit: estouro do limite de canais/numeros da conta (C7 bootstrap). 409.
	ErrChannelLimit = errors.New("omnichannel: channel limit reached")
	// ErrProviderUnsupported: provider sem adapter registrado, ou sem ciclo de sessao.
	ErrProviderUnsupported = errors.New("omnichannel: provider not supported")
	// ErrSessionUnavailable: a instancia nao existe/nao e da conta no fluxo de sessao. 404.
	ErrSessionUnavailable = errors.New("omnichannel: session unavailable")
	// ErrInstanceHasConversations: DELETE de instancia com conversas atreladas. 409 — o front
	// usa "Desativar" (PATCH isActive:false) no caso comum; o delete duro so sem historico.
	ErrInstanceHasConversations = errors.New("omnichannel: instance has conversations")
)

// ============================================================================
// Estado da conversa (canonico §7.3)
// ============================================================================

// ConversationState e a VERDADE do ciclo de vida da conversa (coluna
// messaging.conversations.state). Os 7 valores nascem no CHECK da migration 0200
// (D-E, 2026-07-17) — a F8 nao faz ALTER para adicionar `pending`.
type ConversationState string

const (
	StateNew         ConversationState = "new"
	StateAIActive    ConversationState = "ai_active"
	StateRouting     ConversationState = "routing"
	StateQueued      ConversationState = "queued"
	StateHumanActive ConversationState = "human_active"
	// StatePending e ROTULO MANUAL do operador ("parei nesta, estou esperando algo"),
	// ortogonal ao roteamento. Quem o escreve e a F8 (evento human.pending); a F2 so
	// garante que a coluna o aceita e que a projecao o emite.
	StatePending ConversationState = "pending"
	StateClosed  ConversationState = "closed"
)

// ConversationStatus e a PROJECAO servida ao front verbatim — nao e coluna
// (canonico §7.3: coluna + projecao = duas verdades). Exatamente 3 valores, os do
// contrato do front (web-reference/app/types/index.ts:91).
type ConversationStatus string

const (
	StatusOpen    ConversationStatus = "OPEN"
	StatusPending ConversationStatus = "PENDING"
	StatusClosed  ConversationStatus = "CLOSED"
)

// projectStatus deriva o status (front) a partir do state (verdade). Mapa fechado
// do canonico §7.3: pending -> PENDING; closed -> CLOSED; todo o resto -> OPEN.
// O default OPEN e deliberado: state desconhecido nunca vira string fora do contrato
// do front (o pior caso e mostrar como aberta, nunca quebrar a tipagem).
func projectStatus(state string) ConversationStatus {
	switch ConversationState(state) {
	case StatePending:
		return StatusPending
	case StateClosed:
		return StatusClosed
	default:
		return StatusOpen
	}
}

// projectAIStatus e uma projecao somente informativa para o inbox. O estado persistido
// continua sendo a autoridade; este campo jamais participa de transicao, filtro ou roteamento.
func projectAIStatus(state string) string {
	switch ConversationState(state) {
	case StateAIActive:
		return "analyzing"
	case StateRouting, StateQueued:
		return "transferring"
	case StatePending:
		return "awaiting_client"
	case StateHumanActive:
		return "human"
	case StateClosed:
		return "closed"
	default:
		return "idle"
	}
}

// ============================================================================
// Views servidas ao front (JSON camelCase — divergir um campo quebra o front)
// ============================================================================

// MessageView espelha `Message` (web-reference/app/types/index.ts:93).
// TenantID e OBRIGATORIO no contrato do front (`tenantId: string`, :95) — como o Omni
// mapeia tenantId -> account_id, aqui o account_id volta serializado COMO tenantId.
type MessageView struct {
	ID                   string          `json:"id"`
	TenantID             string          `json:"tenantId"`
	ConversationID       string          `json:"conversationId"`
	SenderUserID         *string         `json:"senderUserId"`
	Direction            string          `json:"direction"`
	MessageType          string          `json:"messageType"`
	SenderName           *string         `json:"senderName"`
	SenderAvatarURL      *string         `json:"senderAvatarUrl"`
	Content              string          `json:"content"`
	MediaURL             *string         `json:"mediaUrl"`
	MediaMimeType        *string         `json:"mediaMimeType"`
	MediaFileName        *string         `json:"mediaFileName"`
	MediaFileSizeBytes   *int            `json:"mediaFileSizeBytes"`
	MediaCaption         *string         `json:"mediaCaption"`
	MediaDurationSeconds *int            `json:"mediaDurationSeconds"`
	MetadataJSON         json.RawMessage `json:"metadataJson"`
	Status               string          `json:"status"`
	Origin               string          `json:"origin"`
	ReplyTo              *ReplyToView    `json:"replyTo"`
	ProviderStatusAt     *time.Time      `json:"providerStatusAt"`
	ProviderErrorCode    string          `json:"providerErrorCode"`
	MediaState           string          `json:"mediaState"`
	CanRetryMedia        bool            `json:"canRetryMedia"`
	ExternalMessageID    *string         `json:"externalMessageId"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
}

type ReplyToView struct {
	MessageID         *string `json:"messageId"`
	ExternalMessageID string  `json:"externalMessageId"`
	SenderName        string  `json:"senderName"`
	Content           string  `json:"content"`
	MessageType       string  `json:"messageType"`
}

// LastMessageView e o preview aninhado em Conversation.lastMessage (types/index.ts:157).
// E um SUBCONJUNTO do Message — nao reusar MessageView aqui: o front tipa so estes campos.
type LastMessageView struct {
	ID          string    `json:"id"`
	Content     string    `json:"content"`
	MessageType string    `json:"messageType"`
	MediaURL    *string   `json:"mediaUrl"`
	Direction   string    `json:"direction"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ConversationView espelha `Conversation` (types/index.ts:149). NAO tem tenantId no
// contrato do front (ao contrario de Message e Contact) — nao inventar o campo.
// `status` e projecao de `state` (projectStatus), nao coluna.
type ConversationView struct {
	ID                  string             `json:"id"`
	InstanceID          *string            `json:"instanceId"`
	InstanceScopeKey    *string            `json:"instanceScopeKey"`
	InstanceName        *string            `json:"instanceName"`
	InstanceDisplayName *string            `json:"instanceDisplayName"`
	Channel             string             `json:"channel"`
	Status              ConversationStatus `json:"status"`
	AIStatus            string             `json:"aiStatus"`
	ExternalID          string             `json:"externalId"`
	ContactID           *string            `json:"contactId"`
	ContactName         *string            `json:"contactName"`
	ContactAvatarURL    *string            `json:"contactAvatarUrl"`
	ContactPhone        *string            `json:"contactPhone"`
	AssignedToID        *string            `json:"assignedToId"`
	CreatedAt           time.Time          `json:"createdAt"`
	UpdatedAt           time.Time          `json:"updatedAt"`
	LastMessageAt       time.Time          `json:"lastMessageAt"`
	LastMessage         *LastMessageView   `json:"lastMessage"`
}

type ConversationPageView struct {
	Conversations []ConversationView `json:"conversations"`
	HasMore       bool               `json:"hasMore"`
	NextCursor    string             `json:"nextCursor,omitempty"`
}

// ContactView espelha `Contact` (types/index.ts:134). TenantID e OBRIGATORIO (:136) —
// mesmo caso do MessageView: account_id serializado de volta como tenantId.
type ContactView struct {
	ID                      string     `json:"id"`
	TenantID                string     `json:"tenantId"`
	Name                    string     `json:"name"`
	Phone                   string     `json:"phone"`
	AvatarURL               *string    `json:"avatarUrl"`
	Source                  string     `json:"source"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
	LastConversationID      *string    `json:"lastConversationId"`
	LastConversationAt      *time.Time `json:"lastConversationAt"`
	LastConversationChannel *string    `json:"lastConversationChannel"`
	LastConversationStatus  *string    `json:"lastConversationStatus"`
}

// InstanceView espelha `WhatsAppInstanceRecord` (types/index.ts:39).
//
// UserScopePolicy e AssignedUserIDs NAO tem fonte: no legado sao CONSTANTES hardcoded
// ("MULTI_INSTANCE" e []) — nao ha tabela por tras (routes-whatsapp-instances.ts:41,48).
// Emitidos com os mesmos valores fixos porque o front os tipa; registrados em
// docs/LEGADO.md como vestigio. Nao inventar tabela para eles nesta fase (armadilha A3).
type InstanceView struct {
	ID                  string    `json:"id"`
	TenantID            string    `json:"tenantId"`
	InstanceName        string    `json:"instanceName"`
	Provider            string    `json:"provider"`
	DisplayName         *string   `json:"displayName"`
	PhoneNumber         *string   `json:"phoneNumber"`
	QueueLabel          *string   `json:"queueLabel"`
	UserScopePolicy     string    `json:"userScopePolicy"`
	ResponsibleUserID   *string   `json:"responsibleUserId"`
	ResponsibleUserName *string   `json:"responsibleUserName"`
	ResponsibleUserMail *string   `json:"responsibleUserEmail"`
	IsDefault           bool      `json:"isDefault"`
	IsActive            bool      `json:"isActive"`
	HasEvolutionAPIKey  bool      `json:"hasEvolutionApiKey"`
	AssignedUserIDs     []string  `json:"assignedUserIds"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// AssignableUserView espelha `WhatsAppInstanceAssignableUser` (types/index.ts:57).
type AssignableUserView struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	Role              string `json:"role"`
	AtendimentoAccess bool   `json:"atendimentoAccess"`
}

// TenantUserView espelha `TenantUser` (types/index.ts:81) — o picker de atribuicao do inbox
// (GET /users). Mesma fonte que AssignableUserView (membros ativos da conta).
type TenantUserView struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenantId"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// InstanceManagementView espelha `WhatsAppInstanceManagementResponse` (types/index.ts:65).
type InstanceManagementView struct {
	MaxChannels     int                  `json:"maxChannels"`
	CurrentChannels int                  `json:"currentChannels"`
	Instances       []InstanceView       `json:"instances"`
	Users           []AssignableUserView `json:"users"`
}

// ChannelLimitInput/ChannelLimitView controlam o teto contratado de numeros da
// conta ativa. A escrita e exclusiva do platform_admin; numeros inativos nao contam.
type ChannelLimitInput struct {
	MaxChannels int `json:"maxChannels"`
}

type ChannelLimitView struct {
	MaxChannels     int `json:"maxChannels"`
	CurrentChannels int `json:"currentChannels"`
}

// InstanceAccessView espelha `WhatsAppInstanceAccessResponse` (types/index.ts:71).
type InstanceAccessView struct {
	HasMultipleActiveInstances bool           `json:"hasMultipleActiveInstances"`
	Instances                  []InstanceView `json:"instances"`
}

// AccountSettingsView espelha `TenantSettings` (types/index.ts:14) — o shape do
// mapTenantResponse do legado. Tres campos mudam de FONTE aqui (spec C4):
//   - id/slug/name: core.accounts (nunca duplicados em messaging.*)
//   - maxChannels/maxUsers: core.account_modules.config jsonb (canonico §5.3)
//   - hasEvolutionApiKey: credentials_ciphertext is not null (nao env global — env
//     global nao sobrevive a D-A, multi-provider por conta/numero)
//
// evolutionApiKey responde SEMPRE null (o legado ja faz isso — manter).
type AccountSettingsView struct {
	ID                         string         `json:"id"`
	Slug                       string         `json:"slug"`
	Name                       string         `json:"name"`
	WhatsAppInstance           *string        `json:"whatsappInstance"`
	WhatsAppInstances          []InstanceView `json:"whatsappInstances"`
	MaxChannels                int            `json:"maxChannels"`
	MaxUsers                   int            `json:"maxUsers"`
	RetentionDays              int            `json:"retentionDays"`
	MaxUploadMb                int            `json:"maxUploadMb"`
	CanManageAtendimentoLimits bool           `json:"canManageAtendimentoLimits"`
	CurrentChannels            int            `json:"currentChannels"`
	CurrentUsers               int            `json:"currentUsers"`
	HasEvolutionAPIKey         bool           `json:"hasEvolutionApiKey"`
	WebhookURL                 string         `json:"webhookUrl"`
	CreatedAt                  time.Time      `json:"createdAt"`
	UpdatedAt                  time.Time      `json:"updatedAt"`
	CanViewSensitive           bool           `json:"canViewSensitive"`
	EvolutionAPIKey            *string        `json:"evolutionApiKey"`
}

// SaveContactView e a resposta de POST /contacts. NAO e o contato cru: o front le
// `response.contact` E `response.conversation`
// (useOmnichannelInboxContactActions.ts:41-56) — devolver o contato pelado quebraria a
// tela. `conversation` so vem quando o body trouxe conversationId; null caso contrario
// (o front recarrega a lista quando vem null).
type SaveContactView struct {
	Contact      ContactView       `json:"contact"`
	Conversation *ConversationView `json:"conversation"`
}

// MessagePageView e a resposta de GET /conversations/{id}/messages.
// Contrato: { conversationId, messages[], hasMore } (SPECS_PORT F2.3).
type MessagePageView struct {
	ConversationID string        `json:"conversationId"`
	Messages       []MessageView `json:"messages"`
	HasMore        bool          `json:"hasMore"`
	NextCursor     string        `json:"nextCursor,omitempty"`
}

// ============================================================================
// Inputs
// ============================================================================

// MessagePageFilter e a paginacao do historico. NAO e cursor: e `limit` + `beforeId`
// (o legado resolve beforeId -> created_at e filtra por data). Replicar exato — divergir
// quebra o scroll infinito do front.
type MessagePageFilter struct {
	Limit        int
	BeforeID     string
	BeforeCursor string
}

type ConversationPageFilter struct {
	Limit         int
	BeforeCursor  string
	Search        string
	Channel       string
	Status        string
	InstanceID    string
	QueueID       string
	ResponsibleID string
}

const (
	defaultMessageLimit      = 100
	maxMessageLimit          = 100
	defaultConversationLimit = 50
	maxConversationLimit     = 100
)

// ContactInput e o body de POST /contacts (createContactSchema do legado).
// NAO tem accountId/tenantId: o escopo vem SEMPRE do Principal (principio 2).
type ContactInput struct {
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	AvatarURL      string `json:"avatarUrl"`
	Source         string `json:"source"`
	ConversationID string `json:"conversationId"`
}

// ContactPatch e o body de PATCH /contacts/{id} (updateContactSchema do legado).
// Ponteiros distinguem "campo ausente" (nao mexe) de "campo null" (limpa) — o legado
// trata avatarUrl: null como limpeza e avatarUrl ausente como no-op.
type ContactPatch struct {
	Name      *string `json:"name"`
	Phone     *string `json:"phone"`
	AvatarURL *string `json:"avatarUrl"`
}

// AccountSettingsPatch e o body de PATCH /account. Na F2 so os limites de retencao/upload
// sao gravaveis (messaging.account_config); os demais campos do TenantSettings tem outra
// fonte (core.accounts, account_modules) e nao se editam por aqui.
type AccountSettingsPatch struct {
	RetentionDays *int `json:"retentionDays"`
	MaxUploadMb   *int `json:"maxUploadMb"`
}
