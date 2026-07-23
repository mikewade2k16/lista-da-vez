package omnichannel

import "time"

const conversationPrivacyManagePermission = "omnichannel.conversations.privacy.manage"

type HideContactInput struct {
	ClearHistory bool `json:"clearHistory"`
}

type ContactAIRestrictionInput struct {
	Mode         string     `json:"mode"`
	BlockedUntil *time.Time `json:"blockedUntil,omitempty"`
}

type ContactAIRestrictionView struct {
	ContactID    string     `json:"contactId"`
	Blocked      bool       `json:"blocked"`
	Mode         string     `json:"mode"`
	BlockedUntil *time.Time `json:"blockedUntil"`
	UpdatedAt    *time.Time `json:"updatedAt"`
}

type HiddenContactView struct {
	ContactID            string     `json:"contactId"`
	ConversationID       string     `json:"conversationId"`
	ContactName          string     `json:"contactName"`
	ContactPhone         string     `json:"contactPhone"`
	HiddenAt             time.Time  `json:"hiddenAt"`
	HistoryClearedAt     *time.Time `json:"historyClearedAt"`
	HiddenByUserID       string     `json:"hiddenByUserId"`
	HistoryClearedByUser *string    `json:"historyClearedByUserId,omitempty"`
}

type contactPrivacyRow struct {
	HiddenContactView
}
