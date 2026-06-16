package automation

import "time"

// Message e uma mensagem trocada com o contato, persistida pelo runtime (n8n).
type Message struct {
	ID           string
	AutomationID string
	AccountID    string
	ContactID    string
	Direction    string // in | out
	Type         string // text | audio | image
	Content      string
	MediaURL     string
	Segment      string
	CreatedAt    time.Time
}

// MessageView e a projecao da mensagem para o runtime.
type MessageView struct {
	ID        string `json:"id"`
	ContactID string `json:"contactId"`
	Direction string `json:"direction"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	MediaURL  string `json:"mediaUrl,omitempty"`
	Segment   string `json:"segment,omitempty"`
	CreatedAt string `json:"createdAt"`
}

func toMessageView(m Message) MessageView {
	return MessageView{
		ID:        m.ID,
		ContactID: m.ContactID,
		Direction: m.Direction,
		Type:      m.Type,
		Content:   m.Content,
		MediaURL:  m.MediaURL,
		Segment:   m.Segment,
		CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// LeadState e o estado do lead por contato (funil + follow-up).
type LeadState struct {
	ContactID       string
	AutomationID    string
	AccountID       string
	Status          string
	LastInteraction time.Time
	FollowUpCount   int
}

// LeadStateView e o contrato de leitura/escrita do estado do lead (runtime).
type LeadStateView struct {
	ContactID       string `json:"contactId"`
	Status          string `json:"status"`
	LastInteraction string `json:"lastInteraction"`
	FollowUpCount   int    `json:"followUpCount"`
}

func toLeadStateView(l LeadState) LeadStateView {
	return LeadStateView{
		ContactID:       l.ContactID,
		Status:          l.Status,
		LastInteraction: l.LastInteraction.UTC().Format(time.RFC3339),
		FollowUpCount:   l.FollowUpCount,
	}
}

const (
	directionIn   = "in"
	directionOut  = "out"
	msgTypeText   = "text"
	defaultStatus = "new"
)
