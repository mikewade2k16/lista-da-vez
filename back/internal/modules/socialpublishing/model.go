package socialpublishing

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

const (
	ModuleID        = "social_publishing"
	GraphBaseEnv    = "SOCIAL_PUBLISHING_GRAPH_BASE"
	DefaultGraphURL = "https://graph.instagram.com/v24.0"
	DefaultTimezone = "America/Sao_Paulo"

	PermissionView      = "social_publishing.view"
	PermissionManage    = "social_publishing.manage"
	PermissionConnect   = "social_publishing.connect"
	PermissionAnalytics = "social_publishing.analytics"

	PublishJobKind   = "social.publish"
	AnalyticsJobKind = "social.analytics.refresh"
)

var (
	ErrNotFound             = errors.New("social publishing: recurso nao encontrado")
	ErrForbidden            = errors.New("social publishing: permissao negada")
	ErrConflict             = errors.New("social publishing: conflito de versao ou estado")
	ErrNotConnected         = errors.New("social publishing: instagram nao conectado")
	ErrSecretsUnavailable   = errors.New("social publishing: cofre de segredos indisponivel")
	ErrInvalidToken         = errors.New("social publishing: token ou conta profissional invalida")
	ErrInvalidInput         = errors.New("social publishing: entrada invalida")
	ErrInvalidMediaURL      = errors.New("social publishing: mediaUrl deve ser HTTPS")
	ErrInvalidTimezone      = errors.New("social publishing: timezone invalido")
	ErrScheduleInPast       = errors.New("social publishing: horario deve estar no futuro")
	ErrInvalidState         = errors.New("social publishing: estado invalido para a operacao")
	ErrConnectionTarget     = errors.New("social publishing: conexao ativa aponta para outro instagram")
	ErrProviderUnavailable  = errors.New("social publishing: instagram indisponivel")
	ErrRuntimeNotConfigured = errors.New("social publishing: runtime nao configurado")
)

type PostStatus string

const (
	PostStatusDraft      PostStatus = "draft"
	PostStatusScheduled  PostStatus = "scheduled"
	PostStatusPublishing PostStatus = "publishing"
	PostStatusPublished  PostStatus = "published"
	PostStatusFailed     PostStatus = "failed"
	PostStatusCancelled  PostStatus = "cancelled"
)

type PostListOrder string

const (
	PostListOrderCreated   PostListOrder = "created"
	PostListOrderScheduled PostListOrder = "scheduled"
)

type Connection struct {
	ID          string           `json:"id"`
	Provider    string           `json:"provider"`
	IGUserID    string           `json:"igUserId"`
	Username    string           `json:"username"`
	AccountType string           `json:"accountType"`
	MediaCount  int64            `json:"mediaCount"`
	Status      string           `json:"status"`
	Secret      secretbox.Status `json:"secret"`
	ConnectedAt time.Time        `json:"connectedAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	Version     int              `json:"version"`
}

// ConnectionRecord fica apenas no backend. Ciphertext nunca possui tag JSON.
type ConnectionRecord struct {
	Connection
	AccountID             string
	AccessTokenCiphertext string
}

type InstagramProfile struct {
	UserID      string
	Username    string
	AccountType string
	MediaCount  int64
}

type Post struct {
	ID                 string     `json:"id"`
	Caption            string     `json:"caption"`
	MediaURL           string     `json:"mediaUrl"`
	MediaType          string     `json:"mediaType"`
	AltText            string     `json:"altText,omitempty"`
	Status             PostStatus `json:"status"`
	ScheduledFor       *time.Time `json:"scheduledFor,omitempty"`
	Timezone           string     `json:"timezone,omitempty"`
	ScheduleRevision   int        `json:"scheduleRevision"`
	Version            int        `json:"version"`
	SourceType         string     `json:"sourceType,omitempty"`
	SourceRef          string     `json:"sourceRef,omitempty"`
	ExternalMediaID    string     `json:"externalMediaId,omitempty"`
	Permalink          string     `json:"permalink,omitempty"`
	LastErrorCode      string     `json:"lastErrorCode,omitempty"`
	LastErrorMessage   string     `json:"lastErrorMessage,omitempty"`
	PublishAttemptedAt *time.Time `json:"-"`
	PublishedAt        *time.Time `json:"publishedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	AccountID          string     `json:"-"`
	ConnectionID       string     `json:"-"`
	ExternalCreationID string     `json:"-"`
}

type CreatePostInput struct {
	Caption      string     `json:"caption"`
	MediaURL     string     `json:"mediaUrl"`
	MediaType    string     `json:"mediaType"`
	AltText      string     `json:"altText"`
	Status       PostStatus `json:"status"`
	ScheduledFor *time.Time `json:"scheduledFor"`
	Timezone     string     `json:"timezone"`
	SourceType   string     `json:"sourceType"`
	SourceRef    string     `json:"sourceRef"`
}

type PatchPostInput struct {
	Caption  *string `json:"caption"`
	MediaURL *string `json:"mediaUrl"`
	AltText  *string `json:"altText"`
	Timezone *string `json:"timezone"`
	Version  int     `json:"version"`
}

type SchedulePostInput struct {
	ScheduledFor time.Time `json:"scheduledFor"`
	Timezone     string    `json:"timezone"`
	Version      int       `json:"version"`
}

type VersionInput struct {
	Version int `json:"version"`
}

type ListPostsFilter struct {
	Status   PostStatus
	Statuses []PostStatus
	Order    PostListOrder
	Limit    int
	Offset   int
}

type ListAnalyticsFilter struct {
	PostIDs []string
	Limit   int
}

type CreatePostResult struct {
	Post    Post
	Created bool
}

type Analytics struct {
	PostID            string    `json:"postId"`
	Views             int64     `json:"views"`
	Reach             int64     `json:"reach"`
	Likes             int64     `json:"likes"`
	Comments          int64     `json:"comments"`
	Saved             int64     `json:"saved"`
	Shares            int64     `json:"shares"`
	TotalInteractions int64     `json:"totalInteractions"`
	CapturedAt        time.Time `json:"capturedAt"`
}

type Summary struct {
	Counts   map[string]int64 `json:"counts"`
	Upcoming []Post           `json:"upcoming"`
}

type Overview struct {
	Analytics Analytics `json:"analytics"`
}

// PublishingClient identifica uma conta-cliente da plataforma que possui o
// modulo social_publishing habilitado. Nao representa contato/lead do CRM.
type PublishingClient struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PublishingScope e o escopo de clientes visivel a partir da account ativa.
// Contas-cliente ficam travadas nelas mesmas; contas-agencia e platform_admin
// podem selecionar um cliente ou consultar o portfolio consolidado.
type PublishingScope struct {
	CanSelect      bool               `json:"canSelect"`
	LockedClientID string             `json:"lockedClientId"`
	Clients        []PublishingClient `json:"clients"`
}

// PortfolioClient e uma projecao administrativa enxuta. Segredo, connection ID,
// token e ciphertext nao fazem parte deste contrato.
type PortfolioClient struct {
	AccountID         string     `json:"accountId"`
	AccountName       string     `json:"accountName"`
	Connected         bool       `json:"connected"`
	Username          string     `json:"username"`
	Draft             int64      `json:"draft"`
	Scheduled         int64      `json:"scheduled"`
	Publishing        int64      `json:"publishing"`
	Published         int64      `json:"published"`
	Failed            int64      `json:"failed"`
	Reach             int64      `json:"reach"`
	TotalInteractions int64      `json:"totalInteractions"`
	NextScheduledFor  *time.Time `json:"nextScheduledFor"`
}

// Portfolio consolida o estado de todas as contas do PublishingScope. Os
// totais ficam no root para consumo direto pelos cards da workspace.
type Portfolio struct {
	ClientCount       int               `json:"clientCount"`
	ConnectedClients  int               `json:"connectedClients"`
	Draft             int64             `json:"draft"`
	Scheduled         int64             `json:"scheduled"`
	Publishing        int64             `json:"publishing"`
	Published         int64             `json:"published"`
	Failed            int64             `json:"failed"`
	Views             int64             `json:"views"`
	Reach             int64             `json:"reach"`
	TotalInteractions int64             `json:"totalInteractions"`
	Likes             int64             `json:"likes"`
	Comments          int64             `json:"comments"`
	Saved             int64             `json:"saved"`
	Shares            int64             `json:"shares"`
	CapturedAt        *time.Time        `json:"capturedAt"`
	Clients           []PortfolioClient `json:"clients"`
}

// portfolioClientRecord carrega as metricas necessarias para somar o root sem
// ampliar o JSON de cada cliente.
type portfolioClientRecord struct {
	Client     PortfolioClient
	Views      int64
	Likes      int64
	Comments   int64
	Saved      int64
	Shares     int64
	CapturedAt *time.Time
}

type RuntimeContext struct {
	AccountID   string           `json:"accountId"`
	GeneratedAt time.Time        `json:"generatedAt"`
	Counts      map[string]int64 `json:"counts"`
	Upcoming    []Post           `json:"upcoming"`
	Analytics   Analytics        `json:"analytics"`
}

type publishJobPayload struct {
	PostID   string `json:"postId"`
	Revision int    `json:"revision"`
}

type analyticsJobPayload struct {
	PostID string `json:"postId"`
}

type createPostCommand struct {
	AccountID    string
	UserID       string
	ConnectionID string
	Input        CreatePostInput
}

type updatePostCommand struct {
	AccountID string
	UserID    string
	Post      Post
	Version   int
}

type schedulePostCommand struct {
	AccountID       string
	UserID          string
	PostID          string
	ConnectionID    string
	ScheduledFor    time.Time
	Timezone        string
	ExpectedVersion int
}

type publishTarget struct {
	PostID             string
	AccountID          string
	Revision           int
	IGUserID           string
	TokenCiphertext    string
	Caption            string
	MediaURL           string
	AltText            string
	ExternalCreationID string
	ExternalMediaID    string
	PublishAttemptedAt *time.Time
}

type analyticsTarget struct {
	PostID          string
	AccountID       string
	ExternalMediaID string
	TokenCiphertext string
}

type connectionTarget struct {
	ID       string
	IGUserID string
}

func marshalJobPayload(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	return json.RawMessage(raw), err
}
