package site

import "errors"

var (
	// ErrLeadNotFound — site.leads sem o id (ou nao pertencente a account).
	ErrLeadNotFound = errors.New("site: lead not found")

	// ErrProductNotFound — site.products sem o id (ou nao da account).
	ErrProductNotFound = errors.New("site: product not found")

	// ErrSourceNotFound — site.webhook_sources sem o id/slug.
	ErrSourceNotFound = errors.New("site: webhook source not found")

	// ErrSourceSlugConflict — slug duplicado dentro da mesma account.
	ErrSourceSlugConflict = errors.New("site: webhook source slug already exists")

	// ErrInvalidSignature — header X-Signature ausente, vazio ou nao bate
	// com HMAC do body usando o secret da source.
	ErrInvalidSignature = errors.New("site: invalid webhook signature")

	// ErrInvalidEntityType — entityType fora de 'leads'|'products'.
	ErrInvalidEntityType = errors.New("site: invalid entity type")
)
