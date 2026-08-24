package metaads

import "time"

const instagramIdentityMappingLimit = 1_000

// InstagramIdentityClientMapping e a atribuicao PostgreSQL de uma identidade
// real da Graph a um cliente. account_id e sempre a dona da conexao (agencia).
type InstagramIdentityClientMapping struct {
	ID              string
	AccountID       string
	ConnectionID    string
	ClientAccountID string
	IGUserID        string
	PageID          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// InstagramIdentityView combina a identidade viva da Graph com o vinculo
// autoritativo local. Nunca contem token ou payload Graph bruto.
type InstagramIdentityView struct {
	IGUserID        string  `json:"igUserId"`
	Username        string  `json:"username"`
	PageID          string  `json:"pageId"`
	PageName        string  `json:"pageName"`
	ClientAccountID *string `json:"clientAccountId"`
}
