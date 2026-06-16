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

	// ErrProductSyncUnavailable — service sem repo/cliente de fonte externa
	// configurado (WithProductSync nao foi chamado).
	ErrProductSyncUnavailable = errors.New("site: product sync not configured")

	// ErrNoProductSource — account nao tem nenhuma fonte externa habilitada.
	ErrNoProductSource = errors.New("site: no enabled product source for account")

	// ErrErpItemNotFound — sku informado nao existe no ERP (erp_item_current)
	// dentro do tenant da account.
	ErrErpItemNotFound = errors.New("site: erp item not found")

	// ErrInvalidProductSourceMode — modo de fonte fora de 'local'|'online'.
	ErrInvalidProductSourceMode = errors.New("site: invalid product source mode")
)
