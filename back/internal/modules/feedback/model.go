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
	ID        string
	TenantID  string
	StoreID   string
	UserID    string
	UserName  string
	Kind      string
	Status    string
	Subject   string
	Body      string
	AdminNote string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FeedbackView struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	StoreID   string    `json:"store_id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	AdminNote string    `json:"admin_note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (f *Feedback) ToView() *FeedbackView {
	return &FeedbackView{
		ID:        f.ID,
		TenantID:  f.TenantID,
		StoreID:   f.StoreID,
		UserID:    f.UserID,
		UserName:  f.UserName,
		Kind:      f.Kind,
		Status:    f.Status,
		Subject:   f.Subject,
		Body:      f.Body,
		AdminNote: f.AdminNote,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
	}
}

type CreateInput struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type UpdateInput struct {
	Status    *string `json:"status"`
	AdminNote *string `json:"admin_note"`
}

type ListInput struct {
	Kind   string
	Status string
}

type Repository interface {
	Create(feedback *Feedback) (*Feedback, error)
	GetByID(id string) (*Feedback, error)
	List(tenantID string, input ListInput) ([]Feedback, error)
	Update(feedback *Feedback) error
}
