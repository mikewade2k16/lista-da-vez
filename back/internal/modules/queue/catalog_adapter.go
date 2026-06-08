package queue

import (
	"context"
	"errors"
)

// ErrCRMNotEnabled e retornado pelo CatalogResolver quando o modulo CRM nao
// esta habilitado para a account — sinal para o caller cair no fallback local.
var ErrCRMNotEnabled = errors.New("crm: modulo nao habilitado para esta account")

// ProductRef representa um produto retornado pela busca no catalogo.
type ProductRef struct {
	ID    string
	Code  string
	Name  string
	Price float64
}

// CatalogResolver e a interface que o modulo queue usa para buscar produtos.
//
// Implementacao principal: crm/catalog.Service (usa ERP atual ou produtos
// internos da account). Quando o modulo CRM nao esta habilitado para a account,
// o resolver deve retornar ErrCRMNotEnabled para que o caller acesse o fallback
// local (tenant_catalog_products da propria fila).
type CatalogResolver interface {
	SearchProducts(ctx context.Context, accountID, storeID, term string, limit int) ([]ProductRef, error)
}

// CatalogAdapter envolve um CatalogResolver opcional e aplica fallback local
// quando o resolver retorna ErrCRMNotEnabled ou nao esta configurado.
//
// O fallback local nao esta implementado aqui — cada consumidor (ex: operations)
// passa sua propria funcao de fallback na construcao do adapter.
type CatalogAdapter struct {
	resolver CatalogResolver
	fallback func(ctx context.Context, storeID, term string, limit int) ([]ProductRef, error)
}

// NewCatalogAdapter cria um adapter com resolver CRM opcional e fallback local.
// Se resolver for nil, todas as buscas usam apenas o fallback.
func NewCatalogAdapter(resolver CatalogResolver, fallback func(ctx context.Context, storeID, term string, limit int) ([]ProductRef, error)) *CatalogAdapter {
	return &CatalogAdapter{resolver: resolver, fallback: fallback}
}

// Search busca produtos. Tenta o resolver CRM primeiro; se ErrCRMNotEnabled
// (ou resolver == nil), delega ao fallback local.
func (a *CatalogAdapter) Search(ctx context.Context, accountID, storeID, term string, limit int) ([]ProductRef, error) {
	if a.resolver != nil {
		items, err := a.resolver.SearchProducts(ctx, accountID, storeID, term, limit)
		if err == nil {
			return items, nil
		}
		if !errors.Is(err, ErrCRMNotEnabled) {
			return nil, err
		}
	}
	if a.fallback != nil {
		return a.fallback(ctx, storeID, term, limit)
	}
	return []ProductRef{}, nil
}
