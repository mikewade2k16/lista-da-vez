package omnichannel

import (
	"encoding/json"
	"time"
)

type InstagramAccountView struct {
	ID             string    `json:"id"`
	IGUserID       string    `json:"igUserId"`
	Username       *string   `json:"username"`
	DisplayName    *string   `json:"displayName"`
	PageID         *string   `json:"pageId"`
	IsActive       bool      `json:"isActive"`
	WebhookStatus  string    `json:"webhookStatus"`
	CredentialsSet bool      `json:"credentialsSet"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type InstagramAccountInput struct {
	IGUserID     string `json:"igUserId"`
	Username     string `json:"username"`
	DisplayName  string `json:"displayName"`
	PageID       string `json:"pageId"`
	GraphVersion string `json:"graphVersion"`
	AccessToken  string `json:"accessToken"`
	AppSecret    string `json:"appSecret"`
	VerifyToken  string `json:"verifyToken"`
}

type InstagramCommentView struct {
	ID             string          `json:"id"`
	AccountID      string          `json:"instagramAccountId"`
	ExternalID     string          `json:"externalCommentId"`
	MediaID        *string         `json:"externalMediaId"`
	ParentID       *string         `json:"parentCommentId"`
	ContactID      *string         `json:"contactId"`
	AuthorScopedID string          `json:"authorScopedId"`
	Username       *string         `json:"username"`
	Text           string          `json:"text"`
	EventKind      string          `json:"eventKind"`
	Status         string          `json:"status"`
	IsLive         bool            `json:"isLive"`
	OccurredAt     time.Time       `json:"occurredAt"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type InstagramCommentActionView struct {
	ID                    string     `json:"id"`
	CommentID             string     `json:"commentId"`
	ActionKind            string     `json:"actionKind"`
	Status                string     `json:"status"`
	ProposedText          *string    `json:"proposedText"`
	ApprovedText          *string    `json:"approvedText"`
	ApprovedByUserID      *string    `json:"approvedByUserId"`
	ExternalMessageID     *string    `json:"externalMessageId"`
	IdempotencyKey        string     `json:"idempotencyKey"`
	PrivateReplyExpiresAt *time.Time `json:"privateReplyExpiresAt"`
	LastError             string     `json:"lastError"`
	CreatedAt             time.Time  `json:"createdAt"`
	ExecutedAt            *time.Time `json:"executedAt"`
}

type InstagramActionDecisionInput struct {
	ApprovedText string `json:"approvedText"`
	ActionKind   string `json:"actionKind"`
}
