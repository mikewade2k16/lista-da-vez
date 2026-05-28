package catalog

import "context"

type ProductSourceKey string

const (
	ProductSourceERPCurrent       ProductSourceKey = "erp_current"
	ProductSourceInternalProducts ProductSourceKey = "internal_products"
)

type StoreScope struct {
	TenantID  string
	StoreID   string
	StoreCode string
}

type AccessContext struct {
	UserID   string
	TenantID string
	Role     string
	StoreIDs []string
}

type AccessibleStore struct {
	ID       string
	TenantID string
	Code     string
	Name     string
	City     string
}

type SearchProductsInput struct {
	StoreID   string           `json:"storeId"`
	SourceKey ProductSourceKey `json:"sourceKey"`
	Term      string           `json:"term"`
	Limit     int              `json:"limit"`
}

type SearchProductsQuery struct {
	StoreScope
	SourceKey ProductSourceKey `json:"sourceKey"`
	Term      string           `json:"term"`
	Limit     int              `json:"limit"`
}

type ProductSearchItem struct {
	ID    string  `json:"id"`
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Price float64 `json:"price,omitempty"`
}

type SearchProductsResponse struct {
	SourceKey ProductSourceKey    `json:"sourceKey"`
	Term      string              `json:"term"`
	Limit     int                 `json:"limit"`
	Items     []ProductSearchItem `json:"items"`
}

type Repository interface {
	SearchProducts(ctx context.Context, query SearchProductsQuery) (SearchProductsResponse, error)
}

type StoreFinder interface {
	FindAccessible(ctx context.Context, access AccessContext, storeID string) (AccessibleStore, error)
}
