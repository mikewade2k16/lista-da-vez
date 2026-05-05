package feedback

import (
	"time"
)

const (
	KindSuggestion = "suggestion"
	KindQuestion   = "question"
	KindProblem    = "problem"

	StatusOpen     = "open"
	StatusProgress = "in_progress"
	StatusResolved = "resolved"
	StatusClosed   = "closed"
)

type Feedback struct {
	ID             string
	TenantID       string
	StoreID        string
	UserID         string
	UserName       string
	Kind           string
	Status         string
	Subject        string
	Body           string
	AdminNote      string
	ImagePath      string
	ImageContentType string
	ImageSizeBytes int
	ClosedAt       *time.Time
	UserLastReadAt time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type FeedbackView struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	StoreID        string    `json:"store_id"`
	UserID         string    `json:"user_id"`
	UserName       string    `json:"user_name"`
	Kind           string    `json:"kind"`
	Status         string    `json:"status"`
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	AdminNote      string    `json:"admin_note"`
	UserLastReadAt time.Time `json:"user_last_read_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type FeedbackMessage struct {
	ID           string
	TenantID     string
	FeedbackID   string
	AuthorUserID string
	AuthorName   string
	AuthorRole   string
	Body         string
	ImagePath    string
	ImageContentType string
	ImageSizeBytes int
	ImageExpiresAt *time.Time
	CreatedAt    time.Time
}

type FeedbackMessageView struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	FeedbackID   string    `json:"feedback_id"`
	AuthorUserID string    `json:"author_user_id"`
	AuthorName   string    `json:"author_name"`
	AuthorRole   string    `json:"author_role"`
	Body         string    `json:"body"`
	ImageURL     string    `json:"image_url"`
	ImageContentType string `json:"image_content_type"`
	ImageSizeBytes int     `json:"image_size_bytes"`
	ImageExpiresAt *time.Time `json:"image_expires_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func (f *Feedback) ToView() *FeedbackView {
	return &FeedbackView{
		ID:             f.ID,
		TenantID:       f.TenantID,
		StoreID:        f.StoreID,
		UserID:         f.UserID,
		UserName:       f.UserName,
		Kind:           f.Kind,
		Status:         f.Status,
		Subject:        f.Subject,
		Body:           f.Body,
		AdminNote:      f.AdminNote,
		UserLastReadAt: f.UserLastReadAt,
		CreatedAt:      f.CreatedAt,
		UpdatedAt:      f.UpdatedAt,
	}
}

func (m *FeedbackMessage) ToView() *FeedbackMessageView {
	return &FeedbackMessageView{
		ID:           m.ID,
		TenantID:     m.TenantID,
		FeedbackID:   m.FeedbackID,
		AuthorUserID: m.AuthorUserID,
		AuthorName:   m.AuthorName,
		AuthorRole:   m.AuthorRole,
		Body:         m.Body,
		ImageURL:     m.ImagePath,
		ImageContentType: m.ImageContentType,
		ImageSizeBytes: m.ImageSizeBytes,
		ImageExpiresAt: m.ImageExpiresAt,
		CreatedAt:    m.CreatedAt,
	}
}

type ImageUpload struct {
	FileName    string
	ContentType string
	Content     []byte
}

type CreateInput struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Image   *ImageUpload `json:"-"`
}

type UpdateInput struct {
	Status    *string `json:"status"`
	AdminNote *string `json:"admin_note"`
}

type CreateMessageInput struct {
	Body string `json:"body"`
	Image *ImageUpload `json:"-"`
}

type ListInput struct {
	Kind         string
	Status       string
	Since        *time.Time
	UserID       string
	ViewerUserID string
}

type ListMessagesInput struct {
	After *time.Time
}

type Repository interface {
	Create(feedback *Feedback) (*Feedback, error)
	GetByID(id string) (*Feedback, error)
	List(tenantID string, input ListInput) ([]Feedback, error)
	MarkRead(feedbackID string, userID string, readAt time.Time) (*Feedback, error)
	Update(feedback *Feedback) error
	CreateMessage(message *FeedbackMessage) (*FeedbackMessage, error)
	ListMessages(feedbackID string, input ListMessagesInput) ([]FeedbackMessage, error)
	PurgeExpiredAttachments(cutoff time.Time, limit int) ([]string, error)
}
