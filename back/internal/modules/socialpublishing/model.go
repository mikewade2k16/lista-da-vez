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
	DefaultGraphURL = "https://graph.instagram.com/v23.0"

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
	Status PostStatus
	Limit  int
	Offset int
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

type Overview struct {
	Connection *Connection      `json:"connection,omitempty"`
	Counts     map[string]int64 `json:"counts"`
	Analytics  Analytics        `json:"analytics"`
	Upcoming   []Post           `json:"upcoming"`
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

func marshalJobPayload(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	return json.RawMessage(raw), err
}
