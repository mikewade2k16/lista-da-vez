// Package channel e a CAMADA TRADUTORA do omnichannel (canonico §5.4). O dominio e o
// front SO enxergam o shape canonico deste pacote — nunca o payload cru de um provider.
// Trocar de provedor (Evolution -> WAHA -> Meta Cloud) = 1 adapter novo que implementa
// Provider; zero mudanca no dominio, zero no front.
//
// F4 nucleo: a interface Provider (5 metodos da spec), os eventos canonicos, a
// SessionManager (ciclo QR/conexao, fora do contrato de mensagem) e o Registry. O unico
// adapter registrado agora e o `mock` (sem rede). O `evolution` entra na fase seguinte
// pela MESMA interface — ponto de registro ja marcado no Registry.
package channel

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Provider e a interface do canonico §5.4: a traducao entre o mundo do provedor e o
// shape canonico. Os 5 metodos sao os da spec OMNI-F4 C1. NUNCA embutir o body cru em
// erro (canonico §10: payload nao vaza em log/erro/trace).
type Provider interface {
	// ID e a chave do provider (= whatsapp_instances.provider).
	ID() string

	// VerifyWebhook autentica a requisicao ANTES de parsear. Evolution/WAHA: token com
	// comparacao constant-time (hmac.Equal). Meta (F11): HMAC X-Hub-Signature-256. A
	// credencial vem decifrada por instancia (Credentials). Falha => erro SEM body.
	VerifyWebhook(hdr http.Header, body []byte, cred Credentials) error

	// ParseWebhook traduz o payload DINAMICO do provider em eventos canonicos. Parsear
	// defensivamente: nunca presumir que um campo existe. Erro NAO carrega o body.
	ParseWebhook(ctx context.Context, hdr http.Header, body []byte) ([]Event, error)

	// SendMessage envia uma mensagem de saida (consumido pelo outbox na F6).
	SendMessage(ctx context.Context, cred Credentials, out OutboundMessage) (SendResult, error)

	// DownloadMedia baixa a midia de uma mensagem recebida (F6).
	DownloadMedia(ctx context.Context, cred Credentials, ref MediaRef) (io.ReadCloser, MediaMeta, error)

	// SendReaction reage a UMA mensagem ja existente na conversa (F7 — acao SINCRONA ao
	// provider, NAO passa pelo outbox). Emoji vazio = REMOVER a reacao (contrato do legado).
	// Ofertada so quando Capabilities().SupportsReaction; o caller mapeia falha para 502. O
	// erro NUNCA carrega body/chave (canonico §10).
	SendReaction(ctx context.Context, cred Credentials, in ReactionInput) error

	// DeleteForAll apaga UMA mensagem para todos (F7 — acao SINCRONA, irreversivel, visivel ao
	// cliente). So mensagem OUTBOUND ja enviada e elegivel (o caller filtra). O erro NUNCA
	// carrega body/chave; o caller acumula a falha por-id em failedIds.
	DeleteForAll(ctx context.Context, cred Credentials, in DeleteInput) error

	// Capabilities sustenta o multi-provider na UI: a tela degrada POR NUMERO em vez de
	// mentir que todo numero faz tudo (canonico §12 risco 2).
	Capabilities() Capabilities
}

// SessionManager e o ciclo de sessao/QR — SEPARADO do Provider de proposito: gerir
// instancia (createInstance/connect/qrcode/logout/status no legado Evolution) nao e a
// traducao de mensagem, e o front nunca ve este shape. Um adapter implementa AS DUAS
// interfaces; o Registry resolve um Provider e o SessionService faz type-assert para
// SessionManager. Provider sem SessionManager => o modulo responde "sessao nao suportada".
type SessionManager interface {
	// Connect inicia/renova a sessao da instancia. Devolve o estado atual: QR para ler
	// (quando desconectado) ou Connected+PhoneNumber (quando ja pareado).
	Connect(ctx context.Context, cred Credentials, instanceName string) (SessionState, error)
	// Status consulta o estado da sessao sem forcar reconexao.
	Status(ctx context.Context, cred Credentials, instanceName string) (SessionState, error)
	// Logout desconecta a instancia no provider (deslogar o WhatsApp).
	Logout(ctx context.Context, cred Credentials, instanceName string) error
}

// ============================================================================
// Credenciais (resolvidas por instancia, decifradas por platform/secretbox)
// ============================================================================

// Credentials carrega o material de credencial de UMA instancia, ja decifrado. Nasce de
// whatsapp_instances.credentials_ciphertext (via secretbox) + provider_config (nao
// secreto). NUNCA volta ao front (so {set,last4}), nunca vai a log (canonico §10).
//
// Token e o segredo compartilhado (webhook token / api key). Config sao os parametros
// nao-secretos do provider (baseURL, etc). ambos opcionais: o mock nao usa nenhum.
type Credentials struct {
	Token  string
	Config map[string]string
}

// ============================================================================
// Eventos canonicos (o que o dominio ve; spec C1)
// ============================================================================

// EventKind e o tipo do evento canonico.
type EventKind string

const (
	EventMessageReceived EventKind = "message_received"
	EventMessageStatus   EventKind = "message_status"
	EventSessionStatus   EventKind = "session_status"
	EventQRUpdated       EventKind = "qr_updated"
	// EventIgnored: o provider mandou algo que nao viramos dominio (presenca, typing,
	// etc). O webhook responde 202 {status:"ignored"} — nunca erro.
	EventIgnored EventKind = "ignored"
)

// Event e o evento canonico. ExternalEventID e a CHAVE DE DEDUPE (webhook_events UNIQUE
// (provider, external_event_id)) — para nao colidir entre contas, o adapter compoe
// {instanceName}:{providerMessageId} (armadilha 2 da spec). OccurredAt e o timestamp DO
// PROVIDER, nunca now().
type Event struct {
	Kind            EventKind
	ExternalEventID string
	InstanceName    string // = instance_scope_key (o NOME da instancia, nao o id)
	OccurredAt      time.Time
	Message         *InboundMessage
	Status          *StatusUpdate
	Session         *SessionState
}

// InboundMessage e a mensagem recebida, ja canonica. Os campos espelham messaging.messages
// (direction=INBOUND). Channel default WHATSAPP.
type InboundMessage struct {
	ExternalMessageID string
	Channel           string // WHATSAPP | INSTAGRAM
	ContactExternalID string // o id de conversa do provider (ex.: jid do WhatsApp)
	ContactPhone      string
	ContactName       string
	ContactAvatarURL  string
	// FromMe = a mensagem foi enviada PELO aparelho pareado (ou e o eco do proprio envio da
	// plataforma). O ingest grava como OUTBOUND (nao INBOUND) e dedupa pelo external id.
	FromMe        bool
	MessageType   string // TEXT | IMAGE | AUDIO | VIDEO | DOCUMENT
	Content       string
	MediaURL      string
	MediaMimeType string
	MediaFileName string
	MediaCaption  string
}

// StatusUpdate e a mudanca de status de uma mensagem JA enviada (ACK). F6/F5 consomem.
type StatusUpdate struct {
	ExternalMessageID string
	Status            string // SENT | FAILED (o legado nao tem DELIVERED/READ)
}

// SessionState e o estado da sessao (QR/conexao). QRCode e data URL normalizada (vazio
// quando conectado ou n/a). PhoneNumber resolve so apos parear.
type SessionState struct {
	Connected   bool
	PhoneNumber string
	QRCode      string
}

// ============================================================================
// Envio e midia (F6 — a interface fecha aqui; o consumo e la)
// ============================================================================

// OutboundMessage e a mensagem a enviar.
type OutboundMessage struct {
	InstanceName      string
	ToPhone           string
	MessageType       string
	Content           string
	MediaURL          string
	MediaMimeType     string
	MediaFileName     string
	MediaCaption      string
	IdempotencyKey    string
	ConversationExtID string
}

// SendResult e o retorno do envio: o id que o provider atribuiu.
type SendResult struct {
	ExternalMessageID string
	Status            string // SENT | FAILED
}

// MediaRef aponta a midia a baixar (a partir de uma InboundMessage).
type MediaRef struct {
	InstanceName      string
	ExternalMessageID string
	MediaURL          string
}

// ReactionInput descreve uma reacao a UMA mensagem ja existente na conversa (F7). RemoteJID e
// o id de conversa do provider (ex.: o `remoteJid` do WhatsApp = conversations.external_id).
// ExternalMessageID e o id da mensagem alvo NO provedor. FromMe = a mensagem alvo saiu deste
// numero (OUTBOUND). Emoji vazio = REMOVER a reacao.
type ReactionInput struct {
	InstanceName      string
	RemoteJID         string
	ExternalMessageID string
	FromMe            bool
	Emoji             string
}

// DeleteInput descreve o apagar-para-todos de UMA mensagem OUTBOUND ja enviada (F7,
// irreversivel). RemoteJID = conversations.external_id; FromMe e sempre true (so OUTBOUND e
// elegivel). ParticipantJID so e usado em grupo (remoteJid terminando em @g.us); vazio senao.
type DeleteInput struct {
	InstanceName      string
	RemoteJID         string
	ExternalMessageID string
	FromMe            bool
	ParticipantJID    string
}

// MediaMeta descreve a midia baixada.
type MediaMeta struct {
	MimeType  string
	FileName  string
	SizeBytes int64
}

// ============================================================================
// Capabilities
// ============================================================================

// Capabilities descreve o que o provider suporta (a UI degrada por numero — canonico
// §12 risco 2).
type Capabilities struct {
	SupportsTemplates bool  `json:"supportsTemplates"` // Meta: true. Evolution/WAHA/mock: false
	Requires24hWindow bool  `json:"requires24hWindow"` // Meta: true — fora da janela a UI EXIGE template
	SupportsReaction  bool  `json:"supportsReaction"`
	SupportsSticker   bool  `json:"supportsSticker"`
	SupportsGroups    bool  `json:"supportsGroups"`
	MaxMediaBytes     int64 `json:"maxMediaBytes"`
}
