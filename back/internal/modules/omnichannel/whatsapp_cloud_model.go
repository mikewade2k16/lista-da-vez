package omnichannel

import (
	"encoding/json"
	"time"
)

type WhatsAppTemplateView struct {
	ID            string          `json:"id"`
	InstanceID    string          `json:"instanceId"`
	ExternalID    string          `json:"metaTemplateId"`
	Name          string          `json:"name"`
	Language      string          `json:"language"`
	Category      string          `json:"category"`
	Status        string          `json:"status"`
	Components    json.RawMessage `json:"components"`
	QualityRating *string         `json:"qualityRating"`
	LastSyncedAt  *time.Time      `json:"lastSyncedAt"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

type MetaCloudConfigView struct {
	InstanceID          string    `json:"instanceId"`
	Provider            string    `json:"provider"`
	WABAID              string    `json:"wabaId"`
	PhoneNumberID       string    `json:"phoneNumberId"`
	BusinessPortfolioID string    `json:"businessPortfolioId"`
	AppID               string    `json:"appId"`
	GraphVersion        string    `json:"graphVersion"`
	WebhookMode         string    `json:"webhookMode"`
	CredentialsSet      bool      `json:"credentialsSet"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type MetaCloudConfigInput struct {
	WABAID              string `json:"wabaId"`
	PhoneNumberID       string `json:"phoneNumberId"`
	BusinessPortfolioID string `json:"businessPortfolioId"`
	AppID               string `json:"appId"`
	GraphVersion        string `json:"graphVersion"`
	WebhookMode         string `json:"webhookMode"`
	AccessToken         string `json:"accessToken"`
	AppSecret           string `json:"appSecret"`
	VerifyToken         string `json:"verifyToken"`
}
