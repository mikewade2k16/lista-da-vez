package omnichannel

import "time"

type AICredentialView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Last4     string    `json:"last4"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type aiCredentialRow struct {
	AICredentialView
	SecretCiphertext string
}

type AICredentialInput struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
}

type AICredentialPatch struct {
	Name   *string `json:"name"`
	APIKey *string `json:"apiKey"`
}

type AICredentialImportView struct {
	Imported int `json:"imported"`
	Existing int `json:"existing"`
}
