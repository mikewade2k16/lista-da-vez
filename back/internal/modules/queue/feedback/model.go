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
	ID               string
	TenantID         string
	StoreID          string
	UserID           string
	UserName         string
	Kind             string
	Status           string
	Subject          string
	Body             string
	AdminNote        string
	ImagePath        string
	ImageContentType string
	ImageSizeBytes   int
	ClosedAt         *time.Time
	UserLastReadAt   time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time

	// Agregados preenchidos so na listagem (List) — ver ToListView. Permitem o
	// sino e a lista contarem nao-lidos e exibirem o preview sem baixar as
	// mensagens de cada feedback.
	UnreadCount     int
	LastMessageBody string
	LastMessageAt   *time.Time
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

	// Preenchidos so na listagem (ToListView). Ponteiros com omitempty: nas
	// respostas de mutacao (criar/atualizar/marcar lido) saem ausentes, e o
	// front preserva o valor que veio do list em vez de zerar.
	UnreadCount     *int       `json:"unread_count,omitempty"`
	LastMessageBody *string    `json:"last_message_body,omitempty"`
	LastMessageAt   *time.Time `json:"last_message_at,omitempty"`
}

type FeedbackMessage struct {
	ID               string
	TenantID         string
	FeedbackID       string
	AuthorUserID     string
	AuthorName       string
	AuthorRole       string
	Body             string
	ImagePath        string
	ImageContentType string
	ImageSizeBytes   int
	ImageExpiresAt   *time.Time
	CreatedAt        time.Time
}

type FeedbackMessageView struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	FeedbackID       string     `json:"feedback_id"`
	AuthorUserID     string     `json:"author_user_id"`
	AuthorName       string     `json:"author_name"`
	AuthorRole       string     `json:"author_role"`
	Body             string     `json:"body"`
	ImageURL         string     `json:"image_url"`
	ImageContentType string     `json:"image_content_type"`
	ImageSizeBytes   int        `json:"image_size_bytes"`
	ImageExpiresAt   *time.Time `json:"image_expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
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

// ToListView e a projecao usada na listagem: inclui os agregados (unread_count
// sempre, mesmo zero; preview da ultima mensagem quando existe) para o front
// renderizar sino e badges sem buscar mensagens 1-a-1.
func (f *Feedback) ToListView() *FeedbackView {
	view := f.ToView()
	unread := f.UnreadCount
	view.UnreadCount = &unread
	if f.LastMessageAt != nil {
		body := f.LastMessageBody
		view.LastMessageBody = &body
		view.LastMessageAt = f.LastMessageAt
	}
	return view
}

func (m *FeedbackMessage) ToView() *FeedbackMessageView {
	return &FeedbackMessageView{
		ID:               m.ID,
		TenantID:         m.TenantID,
		FeedbackID:       m.FeedbackID,
		AuthorUserID:     m.AuthorUserID,
		AuthorName:       m.AuthorName,
		AuthorRole:       m.AuthorRole,
		Body:             m.Body,
		ImageURL:         m.ImagePath,
		ImageContentType: m.ImageContentType,
		ImageSizeBytes:   m.ImageSizeBytes,
		ImageExpiresAt:   m.ImageExpiresAt,
		CreatedAt:        m.CreatedAt,
	}
}

type ImageUpload struct {
	FileName    string
	ContentType string
	Content     []byte
}

type CreateInput struct {
	Kind    string       `json:"kind"`
	Subject string       `json:"subject"`
	Body    string       `json:"body"`
	Image   *ImageUpload `json:"-"`
}

type UpdateInput struct {
	Status    *string `json:"status"`
	AdminNote *string `json:"admin_note"`
}

type CreateMessageInput struct {
	Body  string       `json:"body"`
	Image *ImageUpload `json:"-"`
}

type ListInput struct {
	Kind         string
	Status       string
	Since        *time.Time
	UserID       string
	StoreIDs     []string
	ViewerUserID string
}

type ListMessagesInput struct {
	After *time.Time
}

type Repository interface {
	Create(feedback *Feedback) (*Feedback, error)
	// GetByID/MarkRead/Update/CreateMessage/ListMessages recebem o tenantID do
	// Principal para filtrar por tenant na query (defesa em profundidade). tenantID
	// vazio (platform_admin) ignora o filtro e enxerga todos os tenants.
	GetByID(tenantID string, id string) (*Feedback, error)
	List(tenantID string, input ListInput) ([]Feedback, error)
	MarkRead(tenantID string, feedbackID string, userID string, readAt time.Time) (*Feedback, error)
	Update(tenantID string, feedback *Feedback) error
	CreateMessage(tenantID string, message *FeedbackMessage) (*FeedbackMessage, error)
	ListMessages(tenantID string, feedbackID string, input ListMessagesInput) ([]FeedbackMessage, error)
	PurgeExpiredAttachments(cutoff time.Time, limit int) ([]string, error)
}
